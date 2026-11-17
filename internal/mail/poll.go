package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mohan9182/financer/internal/repo"
)

// WindowDays is how far back each poll looks. The poll is stateless: there is
// no UID cursor, and `mail_messages.message_id` provides idempotency.
const WindowDays = 2

// MailHeader is the envelope the poll needs.
type MailHeader struct {
	UID        uint32
	MessageID  string
	FromAddr   string
	Subject    string
	ReceivedAt *string
}

// MailSource is an IMAP-like mailbox the poll can read from.
type MailSource interface {
	Headers(sinceDays int) ([]MailHeader, error)
	Body(uid uint32) (string, error)
}

// TemplateGenerator asks the LLM for templates from sample emails. Implemented
// by the mail service (per-user LiteLLM settings + cooldown).
type TemplateGenerator func(ctx context.Context, userID, sender string, samples []LLMSample) ([]TemplateSpec, error)

// PollSummary reports one mailbox poll.
type PollSummary struct {
	Matched int
	Stored  int
	Skipped int
	MaxUID  uint32
}

// fromPatterns returns the sender patterns to pre-filter with. ok is false when
// at least one enabled rule has no from condition, because IMAP could then hide
// mail that a subject/body rule would match.
func fromPatterns(rules []*repo.MailRuleRow) ([]string, bool) {
	seen := map[string]bool{}
	var patterns []string
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		pattern := strings.TrimSpace(rule.FromPattern)
		if pattern == "" {
			return nil, false
		}
		key := strings.ToLower(pattern)
		if !seen[key] {
			seen[key] = true
			patterns = append(patterns, pattern)
		}
	}
	return patterns, true
}

// RuleMatches: every non-empty condition must hold; empty means "don't care".
func RuleMatches(rule *repo.MailRuleRow, header *MailHeader, body string) bool {
	hits := func(haystack, needle string) bool {
		return needle == "" || strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
	}
	return hits(header.FromAddr, rule.FromPattern) &&
		hits(header.Subject, rule.SubjectContains) &&
		hits(body, rule.BodyContains)
}

// PollAccount fetches the recent window, decides each message's disposition and
// stores it. Templates parse first; the generator runs only on failure.
// PollAccount scans the recent window and stores only rule-matched mail.
func PollAccount(
	ctx context.Context,
	source MailSource,
	store repo.MailRepo,
	account *repo.MailAccountRow,
	generate TemplateGenerator,
	minSamples int64,
) (*PollSummary, error) {
	return pollAccount(ctx, source, store, account, generate, minSamples, false)
}

// PollAccountFull scans the whole INBOX (no date window) and batches inserts.
func PollAccountFull(
	ctx context.Context,
	source MailSource,
	store repo.MailRepo,
	account *repo.MailAccountRow,
	generate TemplateGenerator,
	minSamples int64,
) (*PollSummary, error) {
	return pollAccount(ctx, source, store, account, generate, minSamples, true)
}

func pollAccount(
	ctx context.Context,
	source MailSource,
	store repo.MailRepo,
	account *repo.MailAccountRow,
	generate TemplateGenerator,
	minSamples int64,
	full bool,
) (*PollSummary, error) {
	var headers []MailHeader
	var err error
	// Rules are needed before the search: with a from pattern on every enabled
	// rule we can ask IMAP to return only those senders.
	rules, err := store.ListRules(account.UserID)
	if err != nil {
		return nil, err
	}
	enabledRules := 0
	for _, r := range rules {
		if r.Enabled {
			enabledRules++
		}
	}
	if enabledRules == 0 {
		return &PollSummary{}, nil
	}
	patterns, canFilter := fromPatterns(rules)
	filtered := false
	if canFilter && len(patterns) > 0 {
		if fs, ok := source.(interface {
			HeadersFiltered(uint32, []string) ([]MailHeader, error)
		}); ok {
			headers, err = fs.HeadersFiltered(account.LastUID, patterns)
			filtered = true
		}
	}

	if !filtered {
		if full {
			fullSource, ok := source.(interface{ HeadersAll() ([]MailHeader, error) })
			if !ok {
				return nil, fmt.Errorf("mail source does not support a full scan")
			}
			headers, err = fullSource.HeadersAll()
		} else if uidSource, ok := source.(interface {
			HeadersAfterUID(uint32) ([]MailHeader, error)
		}); ok {
			// Incremental: everything above the last seen UID. A fresh mailbox
			// (cursor 0) therefore scans the whole INBOX once.
			headers, err = uidSource.HeadersAfterUID(account.LastUID)
		} else {
			headers, err = source.Headers(WindowDays)
		}
	}
	if filtered {
		slog.Debug("imap from filter", "account", account.ID, "patterns", patterns,
			"last_uid", account.LastUID)
	}
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(headers))
	for _, h := range headers {
		ids = append(ids, h.MessageID)
	}
	knownList, err := store.KnownMessageIDs(account.UserID, ids)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(knownList))
	for _, id := range knownList {
		known[id] = true
	}

	summary := &PollSummary{Matched: len(headers)}
	for i := range headers {
		if headers[i].UID > summary.MaxUID {
			summary.MaxUID = headers[i].UID
		}
	}
	// Small write batches keep a long poll from holding one large transaction
	// while the rest of the app keeps reading and writing.
	const batchSize = 5
	var pending []*repo.NewMailMessage
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		if _, err := store.InsertMessagesBatch(account.UserID, pending); err != nil {
			return err
		}
		pending = pending[:0]
		return nil
	}
	for i := range headers {
		header := &headers[i]
		if known[header.MessageID] {
			slog.Debug("mail skipped", "message_id", header.MessageID, "reason", "already stored")
			summary.Skipped++
			continue
		}
		rawText, err := source.Body(header.UID)
		if err != nil {
			return nil, err
		}
		// Debug: which rules were compared and what each field saw.
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			slog.Debug("rule evaluate",
				"message_id", header.MessageID, "from", header.FromAddr,
				"rule", rule.ID, "action", string(rule.Action),
				"from_pattern", rule.FromPattern,
				"subject", header.Subject, "subject_contains", rule.SubjectContains,
				"body_contains", rule.BodyContains,
				"matched", RuleMatches(rule, header, rawText))
		}

		// Ignore rules run first: never fetch an LLM or parse junk mail.
		if rule := firstRule(rules, repo.MailRuleActionIgnore, header, rawText); rule != nil {
			pending = append(pending, buildMessage(account, header, rawText, repo.MailStatusIgnored, nil, 0, "rule", nil))
			slog.Info("mail matched",
				"user", account.UserID, "account", account.ID, "rule", rule.ID, "action", string(rule.Action),
				"message_id", header.MessageID, "from", header.FromAddr, "subject", header.Subject,
				"status", string(repo.MailStatusIgnored))
			summary.Stored++
			if len(pending) >= batchSize {
				if err := flush(); err != nil {
					return nil, err
				}
			}
			continue
		}

		// Rule gating: unmatched mail is noise and is never stored.
		reviewRule := firstRule(rules, repo.MailRuleActionReview, header, rawText)
		autoRule := firstRule(rules, repo.MailRuleActionAuto, header, rawText)
		if reviewRule == nil && autoRule == nil {
			slog.Debug("mail skipped", "message_id", header.MessageID, "reason", "no rule matched", "from", header.FromAddr)
			summary.Skipped++
			continue
		}
		matchedRule := reviewRule
		if matchedRule == nil {
			matchedRule = autoRule
		}

		extraction, status, templateID, templateIsActive, sourceName := parseWithTemplates(store, account.UserID, header, rawText, minSamples)
		needsReview := status == repo.MailStatusUnparsed && extraction != nil

		if extraction == nil || needsReview {
			// No complete template: learn a variant from recent samples.
			samples := gatherSamples(store, account.UserID, header, rawText, minSamples)
			if generate != nil && len(samples) > 0 {
				specs, genErr := generate(ctx, account.UserID, header.FromAddr, samples)
				if genErr == nil {
					for _, spec := range specs {
						tpl, compileErr := CompileTemplate(spec)
						if compileErr != nil {
							continue
						}
						stored, createErr := storeTemplate(store, account.UserID, header.FromAddr, spec, int64(len(samples)), minSamples)
						if createErr != nil {
							continue
						}
						if match, matched := tpl.Extract(header.Subject, rawText); matched && match.Complete && match.Extraction != nil {
							extraction = match.Extraction
							needsReview = match.NeedsReview
							templateID = &stored.ID
							templateIsActive = stored.Status == "active"
							sourceName = "template"
							_ = store.RecordTemplateResult(stored.ID, true)
							// The new template applies to mail already waiting in the
							// queue, not only to mail from now on.
							applyTemplateToExisting(store, account.UserID, header.FromAddr, tpl, stored, rules, header.MessageID)
							break
						}
					}
				}
			}
		}

		if extraction == nil {
			// Fall back to deterministic hints so the UI can pre-fill fields.
			extraction = (TemplateExtractor{}).Extract(header.Subject, rawText)
		}

		status = repo.MailStatusUnparsed
		if extraction != nil {
			status = repo.MailStatusPending
		}
		if extraction != nil && !needsReview {
			if rule := firstRule(rules, repo.MailRuleActionAuto, header, rawText); rule != nil &&
				templateIsActive && extraction.AccountHint != nil {
				status = repo.MailStatusAuto
			}
		}

		var parsed *string
		if extraction != nil {
			if b, err := json.Marshal(extraction); err == nil {
				s := string(b)
				parsed = &s
			}
		}
		confidence := 0.0
		if extraction != nil {
			confidence = extraction.Confidence
		}
		if extraction != nil {
			date := ""
			if extraction.OccurredAt != nil {
				date = *extraction.OccurredAt
			}
			account := ""
			if extraction.AccountHint != nil {
				account = *extraction.AccountHint
			}
			slog.Debug("mail extraction",
				"message_id", header.MessageID, "amount", extraction.Amount,
				"direction", extraction.Direction, "date", date, "account", account,
				"merchant", extraction.Merchant, "status", string(status),
				"confidence", confidence, "extractor", sourceName)
		} else {
			slog.Debug("mail extraction", "message_id", header.MessageID,
				"status", string(status), "extractor", sourceName, "empty", true)
		}
		ruleID := ""
		ruleAction := ""
		if matchedRule != nil {
			ruleID = matchedRule.ID
			ruleAction = string(matchedRule.Action)
		}
		slog.Info("mail matched",
			"user", account.UserID, "account", account.ID, "rule", ruleID, "action", ruleAction,
			"message_id", header.MessageID, "from", header.FromAddr, "subject", header.Subject,
			"status", string(status), "extractor", sourceName, "confidence", confidence)
		pending = append(pending, buildMessage(account, header, rawText, status, parsed, confidence, sourceName, templateID))
		summary.Stored++
		if len(pending) >= batchSize {
			if err := flush(); err != nil {
				return nil, err
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return summary, nil
}

// applyTemplateToExisting re-parses the sender's pending/unparsed rows with a
// template that was just learned, so the queue clears without another poll.
func applyTemplateToExisting(store repo.MailRepo, userID, sender string, tpl *Template,
	stored *repo.MailTemplateRow, rules []*repo.MailRuleRow, currentMessageID string) {
	rows, err := store.RecentMessagesFromSender(userID, sender, 200)
	if err != nil {
		return
	}
	for _, row := range rows {
		if row.MessageID == currentMessageID || row.RawText == nil {
			continue
		}
		if row.Status != repo.MailStatusPending && row.Status != repo.MailStatusUnparsed {
			continue
		}
		match, matched := tpl.Extract(row.Subject, *row.RawText)
		if !matched || !match.Complete || match.Extraction == nil || match.NeedsReview {
			continue
		}
		b, err := json.Marshal(match.Extraction)
		if err != nil {
			continue
		}
		parsed := string(b)
		status := repo.MailStatusPending
		if stored.Status == "active" && match.Extraction.AccountHint != nil {
			header := &MailHeader{FromAddr: row.FromAddr, Subject: row.Subject}
			if firstRule(rules, repo.MailRuleActionAuto, header, *row.RawText) != nil {
				status = repo.MailStatusAuto
			}
		}
		_ = store.UpdateMessageExtraction(userID, row.ID, &parsed, status, match.Extraction.Confidence, &stored.ID)
	}
}

func firstRule(rules []*repo.MailRuleRow, action repo.MailRuleAction, header *MailHeader, body string) *repo.MailRuleRow {
	for _, rule := range rules {
		if rule.Enabled && rule.Action == action && RuleMatches(rule, header, body) {
			return rule
		}
	}
	return nil
}

func parseWithTemplates(store repo.MailRepo, userID string, header *MailHeader, body string, minSamples int64) (*Extraction, repo.MailStatus, *string, bool, string) {
	templates, err := store.ListTemplates(userID, header.FromAddr)
	if err != nil {
		return nil, repo.MailStatusUnparsed, nil, false, "template"
	}
	if minSamples < 1 {
		minSamples = 1
	}
	for _, row := range templates {
		tpl, err := compileRow(row)
		if err != nil {
			continue
		}
		match, matched := tpl.Extract(header.Subject, body)
		if !matched {
			continue
		}
		active := row.Status == "active"
		if match.Complete && match.Extraction != nil && !match.NeedsReview {
			_ = store.RecordTemplateResult(row.ID, true)
			if !active {
				if count, err := store.AddTemplateSample(row.ID); err == nil && count >= minSamples {
					if err := store.PromoteTemplate(row.ID, count); err == nil {
						active = true
					}
				}
			}
			id := row.ID
			return match.Extraction, repo.MailStatusPending, &id, active, "template"
		}
		_ = store.RecordTemplateResult(row.ID, false)
		if match.Extraction != nil {
			id := row.ID
			return match.Extraction, repo.MailStatusUnparsed, &id, active, "template"
		}
	}
	return nil, repo.MailStatusUnparsed, nil, false, "template"
}

func gatherSamples(store repo.MailRepo, userID string, header *MailHeader, body string, minSamples int64) []LLMSample {
	limit := minSamples
	if limit < 1 {
		limit = 1
	}
	samples := []LLMSample{{From: header.FromAddr, Subject: header.Subject, Body: body}}
	recent, err := store.RecentMessagesFromSender(userID, header.FromAddr, limit)
	if err == nil {
		for _, m := range recent {
			if m.MessageID == header.MessageID || m.RawText == nil {
				continue
			}
			samples = append(samples, LLMSample{From: m.FromAddr, Subject: m.Subject, Body: *m.RawText})
			if int64(len(samples)) >= limit {
				break
			}
		}
	}
	return samples
}

func storeTemplate(store repo.MailRepo, userID, sender string, spec TemplateSpec, samples, minSamples int64) (*repo.MailTemplateRow, error) {
	guards, err := json.Marshal(spec.Guards)
	if err != nil {
		return nil, err
	}
	patterns, err := json.Marshal(spec.Patterns)
	if err != nil {
		return nil, err
	}
	if minSamples < 1 {
		minSamples = 1
	}
	status := "provisional"
	if samples >= minSamples {
		status = "active"
	}
	return store.CreateTemplate(userID, &repo.MailTemplateRow{
		Sender: sender, Name: spec.Name, GuardsJSON: string(guards),
		PatternsJSON: string(patterns), Status: status, SampleCount: samples,
	})
}

func compileRow(row *repo.MailTemplateRow) (*Template, error) {
	var spec TemplateSpec
	spec.Name = row.Name
	if err := json.Unmarshal([]byte(row.GuardsJSON), &spec.Guards); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(row.PatternsJSON), &spec.Patterns); err != nil {
		return nil, err
	}
	return CompileTemplate(spec)
}

// buildMessage parses the display form once and returns the row to insert.
func buildMessage(account *repo.MailAccountRow, header *MailHeader, raw string,
	status repo.MailStatus, parsed *string, confidence float64, source string, templateID *string) *repo.NewMailMessage {
	display := ParseEmail(raw)
	return &repo.NewMailMessage{
		MailAccountID: account.ID, MessageID: header.MessageID, FromAddr: header.FromAddr,
		FromName: display.FromName, ReceivedAt: header.ReceivedAt, Subject: header.Subject,
		RawText: raw, BodyText: display.Body,
		Status: status, ParsedJSON: parsed, Confidence: confidence,
		ExtractorSource: source, TemplateID: templateID,
	}
}
