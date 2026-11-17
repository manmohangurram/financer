package mail

import (
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
	Headers(senders []string, sinceDays int) ([]MailHeader, error)
	Body(uid uint32) (string, error)
}

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

// PollAccount fetches the allowlisted window of mail, decides each message's
// disposition, and stores it. Known Message-IDs are skipped.
func PollAccount(source MailSource, store repo.MailRepo, account *repo.MailAccountRow, extractor Extractor) (*PollSummary, error) {
	headers, err := source.Headers(account.SenderAllowlist, WindowDays)
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
		extraction := extractor.Extract(header.Subject, rawText)
		status := repo.MailStatusUnparsed
		if extraction != nil {
			status = repo.MailStatusPending
		}
		// The first enabled rule whose conditions all hold decides the disposition.
		for _, rule := range rules {
			if !rule.Enabled || !RuleMatches(rule, header, rawText) {
				continue
			}
			switch rule.Action {
			case repo.MailRuleActionIgnore:
				status = repo.MailStatusIgnored
			case repo.MailRuleActionAuto:
				// Eligible for unattended creation only on a full parse;
				// anything less goes to review.
				if extraction != nil && extraction.Confidence >= 1.0 {
					status = repo.MailStatusAuto
				}
			}
			break
		}
		var parsedJSON *string
		if extraction != nil {
			if b, err := marshalExtraction(extraction); err == nil {
				parsedJSON = &b
			}
		}
		confidence := 0.0
		if extraction != nil {
			confidence = extraction.Confidence
		}
		stored, err := store.InsertMessage(account.UserID, &repo.NewMailMessage{
			MailAccountID:   account.ID,
			MessageID:       header.MessageID,
			FromAddr:        header.FromAddr,
			ReceivedAt:      header.ReceivedAt,
			Subject:         header.Subject,
			RawText:         rawText,
			Status:          status,
			ParsedJSON:      parsedJSON,
			Confidence:      confidence,
			ExtractorSource: extractor.Source(),
		})
		if err != nil {
			return nil, err
		}
		if stored {
			summary.Stored++
		} else {
			summary.Skipped++
		}
	}
	return summary, nil
}

func marshalExtraction(e *Extraction) (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
