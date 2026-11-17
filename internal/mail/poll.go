package mail

import (
	"context"
	"encoding/json"
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
func PollAccount(
	ctx context.Context,
	source MailSource,
	store repo.MailRepo,
	account *repo.MailAccountRow,
	generate TemplateGenerator,
	minSamples int64,
) (*PollSummary, error) {
	headers, err := source.Headers(WindowDays)
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
	rules, err := store.ListRules(account.UserID)
	if err != nil {
		return nil, err
	}

	summary := &PollSummary{Matched: len(headers)}
	for i := range headers {
		header := &headers[i]
		if known[header.MessageID] {
			summary.Skipped++
			continue
		}
		rawText, err := source.Body(header.UID)
		if err != nil {
			return nil, err
		}

		// Ignore rules run first: never fetch an LLM or parse junk mail.
		if rule := firstRule(rules, repo.MailRuleActionIgnore, header, rawText); rule != nil {
			if err := insertMessage(store, account, header, rawText, repo.MailStatusIgnored, nil, 0, "rule", nil); err != nil {
				return nil, err
			}
			summary.Stored++
			continue
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
		if err := insertMessage(store, account, header, rawText, status, parsed, confidence, sourceName, templateID); err != nil {
			return nil, err
		}
		summary.Stored++
	}
	return summary, nil
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

func insertMessage(store repo.MailRepo, account *repo.MailAccountRow, header *MailHeader, raw string,
	status repo.MailStatus, parsed *string, confidence float64, source string, templateID *string) error {
	_, err := store.InsertMessage(account.UserID, &repo.NewMailMessage{
		MailAccountID: account.ID, MessageID: header.MessageID, FromAddr: header.FromAddr,
		ReceivedAt: header.ReceivedAt, Subject: header.Subject, RawText: raw,
		Status: status, ParsedJSON: parsed, Confidence: confidence,
		ExtractorSource: source, TemplateID: templateID,
	})
	return err
}
