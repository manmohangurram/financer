package mail

import (
	"bytes"
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

// PatternList is one or more RE2 patterns for a field. The first pattern that
// yields a value wins, so a manual template can list one regex per mail shape.
// It unmarshals from either a JSON string (learned/AI templates) or an array.
type PatternList []string

func (p *PatternList) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*p = nil
		return nil
	}
	if data[0] == '[' {
		var list []string
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		*p = list
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	if strings.TrimSpace(single) == "" {
		*p = nil
		return nil
	}
	*p = PatternList{single}
	return nil
}

func (p PatternList) MarshalJSON() ([]byte, error) {
	if p == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(p))
}

// Declared reports whether the field has at least one usable pattern.
func (p PatternList) Declared() bool {
	for _, pattern := range p {
		if strings.TrimSpace(pattern) != "" {
			return true
		}
	}
	return false
}

// Key is a stable identity for a pattern list, used to de-duplicate variants.
func (p PatternList) Key() string { return strings.Join(p, "\x00") }

// TemplatePatterns are RE2 patterns for the fields we extract. A pattern may
// expose a named capture (amount/account/merchant/date); otherwise the first
// capture group is used. Each field accepts several patterns tried in order.
type TemplatePatterns struct {
	Amount   PatternList `json:"amount"`
	Debit    PatternList `json:"debit,omitempty"`
	Credit   PatternList `json:"credit,omitempty"`
	Account  PatternList `json:"account,omitempty"`
	Merchant PatternList `json:"merchant,omitempty"`
	Date     PatternList `json:"date"`
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
	amount     []*regexp.Regexp
	debit      []*regexp.Regexp
	credit     []*regexp.Regexp
	account    []*regexp.Regexp
	merchant   []*regexp.Regexp
	date       []*regexp.Regexp
	compiledAt time.Time
}

// CompileTemplate validates and compiles a spec. Amount and date are required;
// at least one direction pattern is required. Account and merchant are optional.
func CompileTemplate(spec TemplateSpec) (*Template, error) {
	if !spec.Patterns.Amount.Declared() {
		return nil, fmt.Errorf("template %q has no amount pattern", spec.Name)
	}
	if !spec.Patterns.Date.Declared() {
		return nil, fmt.Errorf("template %q has no date pattern", spec.Name)
	}
	if !spec.Patterns.Debit.Declared() && !spec.Patterns.Credit.Declared() {
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
	if t.debit, err = compileField("debit", spec.Patterns.Debit); err != nil {
		return nil, err
	}
	if t.credit, err = compileField("credit", spec.Patterns.Credit); err != nil {
		return nil, err
	}
	if t.account, err = compileField("account", spec.Patterns.Account); err != nil {
		return nil, err
	}
	if t.merchant, err = compileField("merchant", spec.Patterns.Merchant); err != nil {
		return nil, err
	}
	return t, nil
}

// compileField compiles every pattern in a field, skipping blanks.
func compileField(name string, patterns PatternList) ([]*regexp.Regexp, error) {
	var out []*regexp.Regexp
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		if len(pattern) > 500 {
			return nil, fmt.Errorf("%s pattern exceeds 500 characters", name)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("%s pattern: %w", name, err)
		}
		out = append(out, re)
	}
	return out, nil
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

	rawAmount := captureAny(t.amount, text, "amount")
	amount, err := parseAmountValue(rawAmount)
	if err != nil {
		match.Reason = "amount not found"
		return match, true
	}

	debit := anyMatch(t.debit, text)
	credit := anyMatch(t.credit, text)
	if debit == credit {
		match.Reason = "direction ambiguous"
		return match, true
	}
	direction := DirectionCredit
	if debit {
		direction = DirectionDebit
	}

	dateRaw := captureAny(t.date, text, "date")
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

	if len(t.account) > 0 {
		if raw := captureAny(t.account, text, "account"); raw != "" {
			hint := lastFour(raw)
			if hint != "" {
				extraction.AccountHint = &hint
			}
		}
	}
	if len(t.merchant) > 0 {
		if raw := captureAny(t.merchant, text, "merchant"); raw != "" {
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
		if hint.AccountHint != nil && len(t.account) > 0 {
			if extraction.AccountHint == nil || *extraction.AccountHint != *hint.AccountHint {
				match.NeedsReview = true
				match.Reason = "account differs from regex hint"
			}
		}
	}

	match.Extraction = extraction
	match.Complete = extraction.AccountHint != nil || len(t.account) == 0
	return match, true
}

// PatternCheck is one pattern's result against a sample mail.
type PatternCheck struct {
	Pattern string
	Matched bool
	Value   string
	Error   string
}

// FieldCheck groups pattern results for one field.
type FieldCheck struct {
	Field    string
	Patterns []PatternCheck
}

// SpecValidation is the editor's preview: what each pattern did and what the
// template as a whole extracted.
type SpecValidation struct {
	Fields     []FieldCheck
	Extraction *Extraction
	Complete   bool
	Reason     string
}

// ValidateSpec tests a template against one sample mail without storing it.
func ValidateSpec(spec TemplateSpec, subject, body string) *SpecValidation {
	text := subject + "\n" + body
	fields := []struct {
		name  string
		list  PatternList
		group string
	}{
		{"amount", spec.Patterns.Amount, "amount"},
		{"debit", spec.Patterns.Debit, "debit"},
		{"credit", spec.Patterns.Credit, "credit"},
		{"account", spec.Patterns.Account, "account"},
		{"merchant", spec.Patterns.Merchant, "merchant"},
		{"date", spec.Patterns.Date, "date"},
	}
	out := &SpecValidation{}
	for _, field := range fields {
		check := FieldCheck{Field: field.name}
		for _, pattern := range field.list {
			if strings.TrimSpace(pattern) == "" {
				continue
			}
			item := PatternCheck{Pattern: pattern}
			re, err := regexp.Compile(pattern)
			if err != nil {
				item.Error = err.Error()
				check.Patterns = append(check.Patterns, item)
				continue
			}
			if field.name == "debit" || field.name == "credit" {
				if raw := re.FindString(text); raw != "" {
					item.Matched = true
					item.Value = raw
				}
			} else {
				item.Value = capture(re, text, field.group)
				item.Matched = item.Value != ""
			}
			check.Patterns = append(check.Patterns, item)
		}
		out.Fields = append(out.Fields, check)
	}
	tpl, err := CompileTemplate(spec)
	if err != nil {
		out.Reason = err.Error()
		return out
	}
	match, matched := tpl.Extract(subject, body)
	if !matched || match == nil {
		out.Reason = "the patterns did not match this mail"
		return out
	}
	out.Extraction = match.Extraction
	out.Complete = match.Complete && !match.NeedsReview
	out.Reason = match.Reason
	return out
}

// captureAny returns the first non-empty value from a field's patterns, so a
// field can list one regex per mail shape.
func captureAny(res []*regexp.Regexp, text, name string) string {
	for _, re := range res {
		if value := capture(re, text, name); value != "" {
			return value
		}
	}
	return ""
}

// anyMatch reports whether any pattern in the field matches.
func anyMatch(res []*regexp.Regexp, text string) bool {
	for _, re := range res {
		if re.MatchString(text) {
			return true
		}
	}
	return false
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
	clean := keepAmountDigits(raw)
	clean = normalizeAmountSeparators(clean)
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid amount %q", raw)
	}
	return v, nil
}

// keepAmountDigits drops currency symbols, ISO codes and spaces, so
// "Rs.2050.00", "₹ 1,234.50", "USD 12.50" and "12.50 EUR" all become numbers.
func keepAmountDigits(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' {
			b.WriteRune(r)
		}
	}
	return strings.TrimLeft(b.String(), ".,")
}

// normalizeAmountSeparators drops thousands grouping and keeps the decimal
// mark, so 1,234.56 and 1.234,56 both become 1234.56.
func normalizeAmountSeparators(s string) string {
	lastComma := strings.LastIndex(s, ",")
	lastDot := strings.LastIndex(s, ".")
	switch {
	case lastComma >= 0 && lastDot >= 0:
		if lastComma > lastDot {
			// 1.234,56 — the comma is the decimal mark.
			s = strings.ReplaceAll(s, ".", "")
			return strings.ReplaceAll(s, ",", ".")
		}
		return strings.ReplaceAll(s, ",", "")
	case lastComma >= 0:
		// One comma with two trailing digits is a decimal mark (12,50);
		// anything else groups thousands (1,234 or 1,234,567).
		if strings.Count(s, ",") == 1 && len(s)-lastComma-1 == 2 {
			return strings.ReplaceAll(s, ",", ".")
		}
		return strings.ReplaceAll(s, ",", "")
	default:
		return s
	}
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
	"02/01/06", "2/1/06", "02-01-06", "2-1-6",
	"02 Jan 06", "2 Jan 06",
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
