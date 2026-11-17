package mail

import (
	"strings"
	"testing"
)

func sampleTemplate() TemplateSpec {
	return TemplateSpec{
		Name: "HDFC debit",
		Guards: TemplateGuards{
			SubjectContains: "debited",
			BodyNotContains: []string{"OTP"},
		},
		Patterns: TemplatePatterns{
			Amount:   `INR\s*(?P<amount>[\d,]+(?:\.\d{1,2})?)`,
			Debit:    `(?i)\bdebited\b`,
			Credit:   `(?i)\bcredited\b`,
			Account:  `(?i)(?:a/c|account|card)[^.\n]{0,24}?(?:[Xx*]{2,})?(?P<account>\d{4})\b`,
			Merchant: `(?i)\bat\s+(?P<merchant>[A-Za-z0-9][A-Za-z0-9 .&_'-]{1,40})`,
			Date:     `(?P<date>\d{1,2}\s+[A-Z]{3}\s+\d{4}|\d{1,2}[-/]\d{1,2}[-/]\d{2,4})`,
		},
	}
}

func TestCompileTemplateValidation(t *testing.T) {
	if _, err := CompileTemplate(TemplateSpec{}); err == nil {
		t.Fatal("empty spec must fail")
	}
	spec := sampleTemplate()
	spec.Patterns.Amount = "("
	if _, err := CompileTemplate(spec); err == nil {
		t.Fatal("invalid regex must fail")
	}
	spec = sampleTemplate()
	spec.Patterns.Debit = ""
	spec.Patterns.Credit = ""
	if _, err := CompileTemplate(spec); err == nil {
		t.Fatal("missing direction must fail")
	}
	spec = sampleTemplate()
	spec.Patterns.Amount = strings.Repeat("a", 501)
	if _, err := CompileTemplate(spec); err == nil {
		t.Fatal("oversized pattern must fail")
	}
}

func TestTemplateGuards(t *testing.T) {
	tpl, err := CompileTemplate(sampleTemplate())
	if err != nil {
		t.Fatal(err)
	}
	if !tpl.Matches("Debited", "body") {
		t.Fatal("subject guard should match case-insensitively")
	}
	if tpl.Matches("Credited", "body") {
		t.Fatal("wrong subject must not match")
	}
	if tpl.Matches("Debited", "your OTP is 1234") {
		t.Fatal("negative body guard must reject OTP mail")
	}
}

func TestTemplateExtract(t *testing.T) {
	tpl, err := CompileTemplate(sampleTemplate())
	if err != nil {
		t.Fatal(err)
	}
	body := "Your A/c XX2467 has been debited with INR 378.64 at SWIGGY on 17 NOV 2026."
	match, matched := tpl.Extract("Debited", body)
	if !matched || match == nil || !match.Complete || match.Extraction == nil {
		t.Fatalf("match = %+v matched=%v", match, matched)
	}
	e := match.Extraction
	if e.Amount != 378.64 || e.Direction != DirectionDebit {
		t.Fatalf("extraction = %+v", e)
	}
	if e.AccountHint == nil || *e.AccountHint != "2467" {
		t.Fatalf("account = %v", e.AccountHint)
	}
	if e.OccurredAt == nil || *e.OccurredAt != "2026-11-17" {
		t.Fatalf("date = %v", e.OccurredAt)
	}
	if match.NeedsReview {
		t.Fatalf("unexpected review flag: %s", match.Reason)
	}
}

func TestTemplateExtractCreditAndDDMMYYYY(t *testing.T) {
	spec := sampleTemplate()
	spec.Guards = TemplateGuards{}
	tpl, err := CompileTemplate(spec)
	if err != nil {
		t.Fatal(err)
	}
	body := "INR 50,000.00 credited to your account 9876 from ACME PAYROLL on 01/10/2026"
	match, matched := tpl.Extract("Credit alert", body)
	if !matched || match == nil || !match.Complete {
		t.Fatalf("match = %+v", match)
	}
	if match.Extraction.Direction != DirectionCredit || match.Extraction.Amount != 50000 {
		t.Fatalf("extraction = %+v", match.Extraction)
	}
	if match.Extraction.OccurredAt == nil || *match.Extraction.OccurredAt != "2026-10-01" {
		t.Fatalf("date = %v", match.Extraction.OccurredAt)
	}
}

func TestTemplateIncompleteWhenDateMissing(t *testing.T) {
	tpl, err := CompileTemplate(sampleTemplate())
	if err != nil {
		t.Fatal(err)
	}
	match, matched := tpl.Extract("Debited", "INR 500 debited at SHOP from a/c XX1234")
	if !matched || match == nil {
		t.Fatal("guards should match")
	}
	if match.Complete {
		t.Fatalf("missing date must be incomplete: %+v", match)
	}
}

func TestAmountAndAccountParsing(t *testing.T) {
	if v, err := parseAmountValue("1,249.50"); err != nil || v != 1249.5 {
		t.Fatalf("amount = %v %v", v, err)
	}
	if got := lastFour("XX2467"); got != "2467" {
		t.Fatalf("lastFour = %q", got)
	}
	if got := lastFour("12"); got != "" {
		t.Fatalf("short digits = %q", got)
	}
}

func TestTemplateExtractHDFCSampleTwoDigitYear(t *testing.T) {
	spec := TemplateSpec{
		Name: "HDFC UPI debit",
		Guards: TemplateGuards{
			BodyContains: "debited",
		},
		Patterns: TemplatePatterns{
			Amount:   `Rs\.?\s*(?P<amount>[\d,]+(?:\.\d{1,2})?)`,
			Debit:    `(?i)\bdebited\b`,
			Credit:   `(?i)\bcredited\b`,
			Account:  `(?i)account ending\s*(?P<account>\d{4})`,
			Merchant: `(?i)towards VPA\s+\S+\s*\((?P<merchant>[^)]+)\)`,
			Date:     `(?i)\bon\s+(?P<date>\d{2}-\d{2}-\d{2})`,
		},
	}
	tpl, err := CompileTemplate(spec)
	if err != nil {
		t.Fatal(err)
	}
	body := "Rs.2050.00 is debited from your account ending 7713 towards VPA ******8172@ybl (BALAJI WINES WALK  IN) on 12-11-26."
	match, matched := tpl.Extract("You have done a UPI txn. Check details!", body)
	if !matched || match == nil || !match.Complete || match.Extraction == nil {
		t.Fatalf("match = %+v matched=%v", match, matched)
	}
	e := match.Extraction
	if e.Amount != 2050 || e.Direction != DirectionDebit {
		t.Fatalf("extraction = %+v", e)
	}
	if e.AccountHint == nil || *e.AccountHint != "7713" {
		t.Fatalf("account = %v", e.AccountHint)
	}
	if e.Merchant != "BALAJI WINES WALK IN" {
		t.Fatalf("merchant = %q", e.Merchant)
	}
	if e.OccurredAt == nil || *e.OccurredAt != "2026-11-12" {
		t.Fatalf("date = %v", e.OccurredAt)
	}
}

func TestParseAmountValueFormats(t *testing.T) {
	cases := map[string]float64{
		"2050.00":    2050,
		"Rs.2050.00": 2050,
		"1,234.56":   1234.56,
		"1.234,56":   1234.56,
		"1,234":      1234,
		"12,50":      12.5,
		"1,234,567":  1234567,
		"50.00":      50,
	}
	for raw, want := range cases {
		got, err := parseAmountValue(raw)
		if err != nil || got != want {
			t.Fatalf("parseAmountValue(%q) = %v, %v; want %v", raw, got, err, want)
		}
	}
}
