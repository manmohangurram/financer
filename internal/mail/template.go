package mail

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TemplateGuards narrow which messages a template applies to.
type TemplateGuards struct {
	SubjectContains string   `json:"subject_contains"`
	BodyContains    string   `json:"body_contains"`
	BodyNotContains []string `json:"body_not_contains,omitempty"`
}

// TemplatePatterns are RE2 patterns for the fields we extract. A pattern may
// expose a named capture (amount/account/merchant/date); otherwise the first
// capture group is used.
type TemplatePatterns struct {
	Amount   string `json:"amount"`
	Debit    string `json:"debit,omitempty"`
	Credit   string `json:"credit,omitempty"`
	Account  string `json:"account,omitempty"`
	Merchant string `json:"merchant,omitempty"`
	Date     string `json:"date"`
}

// TemplateSpec is the JSON shape stored in mail_templates and produced by the
// LLM.
type TemplateSpec struct {
	Name     string           `json:"name"`
	Guards   TemplateGuards   `json:"guards"`
	Patterns TemplatePatterns `json:"patterns"`
}

// Template is a compiled, executable template.
type Template struct {
	Spec       TemplateSpec
	guards     TemplateGuards
	amount     *regexp.Regexp
	debit      *regexp.Regexp
	credit     *regexp.Regexp
	account    *regexp.Regexp
	merchant   *regexp.Regexp
	date       *regexp.Regexp
	compiledAt time.Time
}

// CompileTemplate validates and compiles a spec. Amount and date are required;
// at least one direction pattern is required. Account and merchant are optional.
func CompileTemplate(spec TemplateSpec) (*Template, error) {
	if strings.TrimSpace(spec.Patterns.Amount) == "" {
		return nil, fmt.Errorf("template %q has no amount pattern", spec.Name)
	}
	if strings.TrimSpace(spec.Patterns.Date) == "" {
		return nil, fmt.Errorf("template %q has no date pattern", spec.Name)
	}
	if strings.TrimSpace(spec.Patterns.Debit) == "" && strings.TrimSpace(spec.Patterns.Credit) == "" {
		return nil, fmt.Errorf("template %q has no debit/credit pattern", spec.Name)
	}
	t := &Template{Spec: spec, guards: spec.Guards, compiledAt: time.Now()}
	var err error
	if t.amount, err = compileField("amount", spec.Patterns.Amount); err != nil {
		return nil, err
	}
	if t.date, err = compileField("date", spec.Patterns.Date); err != nil {
		return nil, err
	}
	if t.debit, err = compileOptional("debit", spec.Patterns.Debit); err != nil {
		return nil, err
	}
	if t.credit, err = compileOptional("credit", spec.Patterns.Credit); err != nil {
		return nil, err
	}
	if t.account, err = compileOptional("account", spec.Patterns.Account); err != nil {
		return nil, err
	}
	if t.merchant, err = compileOptional("merchant", spec.Patterns.Merchant); err != nil {
		return nil, err
	}
	return t, nil
}

func compileField(name, pattern string) (*regexp.Regexp, error) {
	if len(pattern) > 500 {
		return nil, fmt.Errorf("%s pattern exceeds 500 characters", name)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%s pattern: %w", name, err)
	}
	return re, nil
}

func compileOptional(name, pattern string) (*regexp.Regexp, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, nil
	}
	return compileField(name, pattern)
}

// Matches reports whether the guards admit this message.
func (t *Template) Matches(subject, body string) bool {
	if t.guards.SubjectContains != "" &&
		!strings.Contains(strings.ToLower(subject), strings.ToLower(t.guards.SubjectContains)) {
		return false
	}
	if t.guards.BodyContains != "" &&
		!strings.Contains(strings.ToLower(body), strings.ToLower(t.guards.BodyContains)) {
		return false
	}
	for _, banned := range t.guards.BodyNotContains {
		if banned == "" {
			continue
		}
		if strings.Contains(strings.ToLower(body), strings.ToLower(banned)) {
			return false
		}
	}
	return true
}

// TemplateMatch is the result of applying a template to one message.
type TemplateMatch struct {
	Extraction  *Extraction
	Complete    bool
	NeedsReview bool
	Reason      string
}

// Extract applies the template. Complete requires amount, direction and date;
// account is optional but required for unattended AUTO. Regex hints from the
// self-contained extractor are compared against the template values.
func (t *Template) Extract(subject, body string) (*TemplateMatch, bool) {
	if !t.Matches(subject, body) {
		return nil, false
	}
	text := subject + "\n" + body
	match := &TemplateMatch{}

	rawAmount := capture(t.amount, text, "amount")
	amount, err := parseAmountValue(rawAmount)
	if err != nil {
		match.Reason = "amount not found"
		return match, true
	}

	debit := t.debit != nil && t.debit.MatchString(text)
	credit := t.credit != nil && t.credit.MatchString(text)
	if debit == credit {
		match.Reason = "direction ambiguous"
		return match, true
	}
	direction := DirectionCredit
	if debit {
		direction = DirectionDebit
	}

	dateRaw := capture(t.date, text, "date")
	occurredAt, ok := parseFlexibleDate(dateRaw)
	if !ok {
		match.Reason = "date not parsed"
		return match, true
	}

	extraction := &Extraction{
		Amount:     amount,
		Direction:  direction,
		OccurredAt: &occurredAt,
		Merchant:   "Unknown",
		Confidence: 0.7,
	}

	if t.account != nil {
		if raw := capture(t.account, text, "account"); raw != "" {
			hint := lastFour(raw)
			if hint != "" {
				extraction.AccountHint = &hint
			}
		}
	}
	if t.merchant != nil {
		if raw := capture(t.merchant, text, "merchant"); raw != "" {
			if m := trimMerchant(raw); len(m) >= 2 {
				extraction.Merchant = m
			}
		}
	}

	extraction.Confidence = 1.0

	// Cross-check with the deterministic hints; disagreements need a human.
	if hint := (TemplateExtractor{}).Extract(subject, body); hint != nil {
		if abs(hint.Amount-amount) > 0.009 {
			match.NeedsReview = true
			match.Reason = "amount differs from regex hint"
		}
		if hint.OccurredAt != nil && *hint.OccurredAt != occurredAt {
			match.NeedsReview = true
			match.Reason = "date differs from regex hint"
		}
		if hint.AccountHint != nil {
			if extraction.AccountHint == nil || *extraction.AccountHint != *hint.AccountHint {
				match.NeedsReview = true
				match.Reason = "account differs from regex hint"
			}
		}
	}

	match.Extraction = extraction
	match.Complete = extraction.AccountHint != nil || t.account == nil
	return match, true
}

func capture(re *regexp.Regexp, text, name string) string {
	if re == nil {
		return ""
	}
	m := re.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	for i, group := range re.SubexpNames() {
		if group == name && i < len(m) {
			return strings.TrimSpace(m[i])
		}
	}
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(m[0])
}

func parseAmountValue(raw string) (float64, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.TrimPrefix(clean, "₹")
	clean = strings.TrimPrefix(clean, "INR")
	clean = strings.TrimSpace(clean)
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid amount %q", raw)
	}
	return v, nil
}

func lastFour(raw string) string {
	digits := make([]byte, 0, 4)
	for _, b := range []byte(raw) {
		if b >= '0' && b <= '9' {
			digits = append(digits, b)
		}
	}
	if len(digits) < 4 {
		return ""
	}
	return string(digits[len(digits)-4:])
}

var dateLayouts = []string{
	"02 Jan 2006", "2 Jan 2006", "02 January 2006", "2 January 2006",
	"02/01/2006", "2/1/2006", "02-01-2006", "2-1-2006",
	"2006-01-02",
}

// parseFlexibleDate normalises common bank-alert dates to YYYY-MM-DD.
func parseFlexibleDate(raw string) (string, bool) {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimSuffix(clean, ".")
	if clean == "" {
		return "", false
	}
	for _, layout := range dateLayouts {
		if d, err := time.Parse(layout, clean); err == nil {
			return d.Format("2006-01-02"), true
		}
	}
	// Some templates capture a longer phrase; look for a date substring.
	fields := strings.Fields(clean)
	for i := 0; i < len(fields); i++ {
		for j := i + 1; j <= len(fields) && j-i <= 3; j++ {
			candidate := strings.Join(fields[i:j], " ")
			for _, layout := range dateLayouts {
				if d, err := time.Parse(layout, candidate); err == nil {
					return d.Format("2006-01-02"), true
				}
			}
		}
	}
	return "", false
}

// SpecJSON serialises the stored form.
func (t *Template) SpecJSON() (string, error) {
	b, err := json.Marshal(t.Spec)
	return string(b), err
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
