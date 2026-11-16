package mail

import (
	"regexp"
	"strconv"
	"strings"
)

// Extraction is a transaction draft read out of a bank alert.
type Extraction struct {
	// Amount as written; the currency is the account's.
	Amount float64 `json:"amount"`
	// Direction is DEBIT or CREDIT.
	Direction string `json:"direction"`
	// OccurredAt is YYYY-MM-DD, or nil when the alert names no date.
	OccurredAt *string `json:"occurredAt"`
	// Merchant is best-effort; "Unknown" when no phrase was found.
	Merchant string `json:"merchant"`
	// AccountHint is the trailing digits of the account or card named.
	AccountHint *string `json:"accountHint"`
	// Confidence is 1.0 only for a full match; lower values must not auto-create.
	Confidence float64 `json:"confidence"`
}

const (
	DirectionDebit  = "DEBIT"
	DirectionCredit = "CREDIT"
)

// Extractor turns a fetched alert into a draft. Nil means the mail is not a
// transaction (a statement, promo, OTP) — never a guess.
type Extractor interface {
	Extract(subject, body string) *Extraction
	Source() string
}

var (
	debitVerb    = regexp.MustCompile(`(?i)\b(?:debited|withdrawn|withdrawal|spent|paid|purchase[d]?)\b`)
	creditVerb   = regexp.MustCompile(`(?i)\b(?:credited|deposited|received|refund(?:ed)?)\b`)
	amountRe     = regexp.MustCompile(`(?i)(?:INR|Rs\.?|₹)\s*([\d,]+(?:\.\d{1,2})?)`)
	accountRe    = regexp.MustCompile(`(?i)(?:\ba/c|\baccount|\bcard)\b[^.\n]{0,24}?([*xX]{0,4})(\d{4})\b`)
	merchantRe   = regexp.MustCompile(`(?i)\b(?:at|to|towards|from)\s+([A-Za-z0-9][A-Za-z0-9 .&_'-]{1,40})`)
	dateDMY      = regexp.MustCompile(`\b(\d{1,2})[-/](\d{1,2})[-/](\d{2,4})\b`)
	merchantTail = regexp.MustCompile(`(?i)\s+(?:on|dated|dt|at|for)\s+.*$`)
)

// TemplateExtractor is the deterministic, regex-based extractor.
type TemplateExtractor struct{}

func (TemplateExtractor) Source() string { return "template" }

func (TemplateExtractor) Extract(subject, body string) *Extraction {
	text := subject + "\n" + body
	debit := debitVerb.MatchString(text)
	credit := creditVerb.MatchString(text)
	if debit == credit {
		// Both or neither: the direction is ambiguous, so do not guess.
		return nil
	}
	direction := DirectionCredit
	if debit {
		direction = DirectionDebit
	}
	m := amountRe.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	amount, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	if err != nil || amount <= 0 {
		return nil
	}

	var accountHint *string
	if g := accountRe.FindStringSubmatch(text); g != nil {
		hint := g[2]
		accountHint = &hint
	}
	merchant := "Unknown"
	if g := merchantRe.FindStringSubmatch(text); g != nil {
		if trimmed := trimMerchant(g[1]); len(trimmed) >= 2 {
			merchant = trimmed
		}
	}
	var occurredAt *string
	if g := dateDMY.FindStringSubmatch(text); g != nil {
		if d, ok := buildDate(g[1], g[2], g[3]); ok {
			occurredAt = &d
		}
	}

	confidence := 0.3
	switch {
	case accountHint != nil && occurredAt != nil:
		confidence = 1.0
	case accountHint != nil || occurredAt != nil:
		confidence = 0.6
	}
	return &Extraction{
		Amount:      amount,
		Direction:   direction,
		OccurredAt:  occurredAt,
		Merchant:    merchant,
		AccountHint: accountHint,
		Confidence:  confidence,
	}
}

// trimMerchant keeps only the merchant from "SWIGGY on 12/09/2026".
func trimMerchant(raw string) string {
	return strings.TrimRight(merchantTail.ReplaceAllString(raw, ""), "., ")
}

// ResolveAccount returns the single account whose stored trailing digits match
// the hint, or nil when none or more than one does — two accounts ending 1234
// is exactly where guessing would post to the wrong one.
func ResolveAccount(hint *string, accounts [][2]*string) *string {
	if hint == nil {
		return nil
	}
	var match *string
	for _, acct := range accounts {
		if acct[1] != nil && *acct[1] == *hint {
			if match != nil {
				return nil
			}
			id := acct[0]
			match = id
		}
	}
	return match
}

func buildDate(d, m, y string) (string, bool) {
	day, err1 := strconv.Atoi(d)
	month, err2 := strconv.Atoi(m)
	year, err3 := strconv.Atoi(y)
	if err1 != nil || err2 != nil || err3 != nil {
		return "", false
	}
	if year < 100 {
		year += 2000
	}
	if day < 1 || day > 31 || month < 1 || month > 12 || year < 2000 || year > 2100 {
		return "", false
	}
	return pad4(year) + "-" + pad2(month) + "-" + pad2(day), true
}

func pad2(v int) string {
	if v < 10 {
		return "0" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}

func pad4(v int) string {
	s := strconv.Itoa(v)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}
