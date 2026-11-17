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
