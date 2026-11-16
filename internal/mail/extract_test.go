package mail

import "testing"

func TestTemplateExtractorFullMatch(t *testing.T) {
	e := TemplateExtractor{}.Extract(
		"Transaction alert",
		"Your A/c XX1234 has been debited with INR 1,249.50 at SWIGGY on 12/09/2026.",
	)
	if e == nil {
		t.Fatal("should extract")
	}
	if e.Direction != DirectionDebit {
		t.Errorf("direction = %q", e.Direction)
	}
	if e.Amount != 1249.50 {
		t.Errorf("amount = %v", e.Amount)
	}
	if e.AccountHint == nil || *e.AccountHint != "1234" {
		t.Errorf("accountHint = %v", e.AccountHint)
	}
	if e.OccurredAt == nil || *e.OccurredAt != "2026-09-12" {
		t.Errorf("occurredAt = %v", e.OccurredAt)
	}
	if e.Confidence != 1.0 {
		t.Errorf("confidence = %v", e.Confidence)
	}
	if e.Merchant != "SWIGGY" {
		t.Errorf("merchant = %q", e.Merchant)
	}
}

func TestTemplateExtractorCredit(t *testing.T) {
	e := TemplateExtractor{}.Extract(
		"Credit alert",
		"INR 50,000.00 credited to your account 9876 from ACME PAYROLL on 01-10-2026",
	)
	if e == nil {
		t.Fatal("should extract")
	}
	if e.Direction != DirectionCredit || e.Amount != 50000 {
		t.Fatalf("extraction = %+v", e)
	}
	if e.AccountHint == nil || *e.AccountHint != "9876" {
		t.Errorf("accountHint = %v", e.AccountHint)
	}
	if e.OccurredAt == nil || *e.OccurredAt != "2026-10-01" {
		t.Errorf("occurredAt = %v", e.OccurredAt)
	}
}

func TestTemplateExtractorPartialConfidence(t *testing.T) {
	e := TemplateExtractor{}.Extract("Debited", "Rs. 500 debited at a shop")
	if e == nil {
		t.Fatal("still a transaction")
	}
	if e.Amount != 500 {
		t.Errorf("amount = %v", e.Amount)
	}
	if e.Confidence != 0.3 {
		t.Errorf("confidence = %v, want 0.3 (no account and no date)", e.Confidence)
	}
	if e.AccountHint != nil {
		t.Errorf("accountHint = %v", e.AccountHint)
	}
}

func TestTemplateExtractorOneFieldConfidence(t *testing.T) {
	e := TemplateExtractor{}.Extract("Debited", "Rs. 250 debited at CAFE on 12/09/2026")
	if e == nil {
		t.Fatal("should extract")
	}
	if e.Confidence != 0.6 {
		t.Errorf("confidence = %v, want 0.6 (date only)", e.Confidence)
	}
}

func TestTemplateExtractorNonTransactions(t *testing.T) {
	cases := [][2]string{
		{"Your OTP", "Use 123456 to log in"},
		{"Statement ready", "Your statement for August is attached"},
		{"Payment", "INR 100 debited and INR 50 credited"}, // ambiguous direction
		{"Debited", "No amount here"},
	}
	for _, c := range cases {
		if e := (TemplateExtractor{}).Extract(c[0], c[1]); e != nil {
			t.Errorf("extract(%q, %q) = %+v, want nil", c[0], c[1], e)
		}
	}
}

func TestResolveAccountUnique(t *testing.T) {
	ptr := func(s string) *string { return &s }
	accounts := [][2]*string{
		{ptr("acc-1"), ptr("1234")},
		{ptr("acc-2"), ptr("9876")},
	}
	got := ResolveAccount(ptr("1234"), accounts)
	if got == nil || *got != "acc-1" {
		t.Fatalf("resolve = %v", got)
	}
	if got := ResolveAccount(ptr("0000"), accounts); got != nil {
		t.Fatalf("unknown hint resolved to %v", *got)
	}
	if got := ResolveAccount(nil, accounts); got != nil {
		t.Fatal("nil hint must not resolve")
	}
}

func TestResolveAccountAmbiguous(t *testing.T) {
	ptr := func(s string) *string { return &s }
	accounts := [][2]*string{
		{ptr("acc-1"), ptr("1234")},
		{ptr("acc-2"), ptr("1234")},
	}
	if got := ResolveAccount(ptr("1234"), accounts); got != nil {
		t.Fatalf("two accounts ending 1234 must not resolve, got %v", *got)
	}
}
