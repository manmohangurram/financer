package mail

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/repo/sqlite"
)

// fakeSource serves a fixed set of envelopes and bodies.
type fakeSource struct {
	headers []MailHeader
	bodies  map[uint32]string
	fetched []uint32
}

func (f *fakeSource) Headers(sinceDays int) ([]MailHeader, error) {
	return f.headers, nil
}

func (f *fakeSource) Body(uid uint32) (string, error) {
	f.fetched = append(f.fetched, uid)
	return f.bodies[uid], nil
}

func newMailEnv(t *testing.T) (repo.MailRepo, *repo.MailAccountRow, string, func()) {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	users := sqlite.NewUserRepo(dbs.Write)
	mailRepo := sqlite.NewMailRepo(dbs.Write)

	uid, err := users.Create("m@test.com", "hash", "Mail")
	if err != nil {
		t.Fatal(err)
	}
	account, err := mailRepo.CreateAccount(uid, &repo.NewMailAccount{
		Label: "Personal", IMAPHost: "imap.x.com", IMAPPort: 993,
		Username: "a@x.com", SecretEnc: "enc",
	})
	if err != nil {
		t.Fatal(err)
	}
	return mailRepo, account, uid, func() {
		dbs.Write.Close()
		dbs.Read.Close()
	}
}

func seedReviewRule(t *testing.T, store repo.MailRepo, uid, sender string) {
	t.Helper()
	if _, err := store.CreateRule(uid, &repo.MailRuleInput{
		Position: 1, Enabled: true, FromPattern: sender, Action: repo.MailRuleActionReview,
	}); err != nil {
		t.Fatal(err)
	}
}

func header(uid uint32, messageID, from string) MailHeader {
	return MailHeader{UID: uid, MessageID: messageID, FromAddr: from, Subject: "Debited"}
}

func debitTemplate(sender string) TemplateSpec {
	return TemplateSpec{
		Name: "Debit alert",
		Guards: TemplateGuards{
			SubjectContains: "debited",
		},
		Patterns: TemplatePatterns{
			Amount:  `INR\s*(?P<amount>[\d,]+(?:\.\d{1,2})?)`,
			Debit:   `(?i)\bdebited\b`,
			Account: `(?i)(?:a/c|account|card)[^.\n]{0,24}?(?:[Xx*]{2,})?(?P<account>\d{4})\b`,
			Date:    `(?P<date>\d{1,2}\s+[A-Z]{3}\s+\d{4})`,
		},
	}
}

func seedTemplate(t *testing.T, store repo.MailRepo, uid, sender string, spec TemplateSpec, status string, samples int64) *repo.MailTemplateRow {
	t.Helper()
	guards, _ := json.Marshal(spec.Guards)
	patterns, _ := json.Marshal(spec.Patterns)
	row, err := store.CreateTemplate(uid, &repo.MailTemplateRow{
		Sender: sender, Name: spec.Name, GuardsJSON: string(guards),
		PatternsJSON: string(patterns), Status: status, SampleCount: samples,
	})
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestPollAccountStoresAndSkips(t *testing.T) {
	mailRepo, account, uid, cleanup := newMailEnv(t)
	defer cleanup()
	seedReviewRule(t, mailRepo, uid, "alerts@bank.com")

	src := &fakeSource{
		headers: []MailHeader{
			header(1, "<m1>", "alerts@bank.com"),
			header(2, "<m2>", "alerts@bank.com"),
		},
		bodies: map[uint32]string{
			1: "INR 1,000 debited at SHOP on 12/09/2026 from a/c XX1234",
			2: "Your OTP is 123456",
		},
	}
	summary, err := PollAccount(context.Background(), src, mailRepo, account, nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Matched != 2 || summary.Stored != 2 || summary.Skipped != 0 {
		t.Fatalf("summary = %+v", summary)
	}

	rows, err := mailRepo.ListMessages(uid, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("stored = %d", len(rows))
	}
	byID := map[string]*repo.MailMessageRow{}
	for _, r := range rows {
		byID[r.MessageID] = r
	}
	if byID["<m1>"].Status != repo.MailStatusPending {
		t.Errorf("<m1> status = %s", byID["<m1>"].Status)
	}
	if byID["<m2>"].Status != repo.MailStatusUnparsed {
		t.Errorf("<m2> status = %s", byID["<m2>"].Status)
	}

	// A second poll skips known Message-IDs without refetching bodies.
	src.fetched = nil
	again, err := PollAccount(context.Background(), src, mailRepo, account, nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if again.Stored != 0 || again.Skipped != 2 {
		t.Fatalf("second poll = %+v", again)
	}
	if len(src.fetched) != 0 {
		t.Fatalf("known messages must not be refetched, fetched %v", src.fetched)
	}
}

func TestPollUsesStoredTemplate(t *testing.T) {
	mailRepo, account, uid, cleanup := newMailEnv(t)
	defer cleanup()
	seedReviewRule(t, mailRepo, uid, "alerts@bank.com")
	seedTemplate(t, mailRepo, uid, "alerts@bank.com", debitTemplate("alerts@bank.com"), "active", 3)

	src := &fakeSource{
		headers: []MailHeader{header(1, "<m1>", "alerts@bank.com")},
		bodies: map[uint32]string{
			1: "Your A/c XX2467 has been debited with INR 378.64 on 17 NOV 2026.",
		},
	}
	// A generator that would fail the test if called.
	called := false
	gen := func(ctx context.Context, userID, sender string, samples []LLMSample) ([]TemplateSpec, error) {
		called = true
		return nil, nil
	}
	summary, err := PollAccount(context.Background(), src, mailRepo, account, gen, 3)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Stored != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if called {
		t.Fatal("a stored template must not trigger the LLM")
	}
	rows, _ := mailRepo.ListMessages(uid, nil, 5)
	if len(rows) != 1 || rows[0].Status != repo.MailStatusPending {
		t.Fatalf("message = %+v", rows)
	}
	var extraction Extraction
	if err := json.Unmarshal([]byte(*rows[0].ParsedJSON), &extraction); err != nil {
		t.Fatal(err)
	}
	if extraction.Amount != 378.64 || extraction.AccountHint == nil || *extraction.AccountHint != "2467" {
		t.Fatalf("extraction = %+v", extraction)
	}
	if rows[0].TemplateID == nil {
		t.Fatal("message did not record its template")
	}
}

func TestPollGeneratesTemplateOnFailure(t *testing.T) {
	mailRepo, account, uid, cleanup := newMailEnv(t)
	defer cleanup()
	seedReviewRule(t, mailRepo, uid, "newbank@example.com")

	src := &fakeSource{
		headers: []MailHeader{header(1, "<m1>", "newbank@example.com")},
		bodies: map[uint32]string{
			1: "Your card XX2467 was debited INR 42.50 on 17 NOV 2026.",
		},
	}
	gen := func(ctx context.Context, userID, sender string, samples []LLMSample) ([]TemplateSpec, error) {
		if sender != "newbank@example.com" || len(samples) == 0 {
			t.Fatalf("generator args = %q %d", sender, len(samples))
		}
		return []TemplateSpec{debitTemplate(sender)}, nil
	}
	if _, err := PollAccount(context.Background(), src, mailRepo, account, gen, 3); err != nil {
		t.Fatal(err)
	}
	templates, err := mailRepo.ListTemplates(uid, "newbank@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 1 || templates[0].Status != "provisional" {
		t.Fatalf("templates = %+v", templates)
	}
	rows, _ := mailRepo.ListMessages(uid, nil, 5)
	if len(rows) != 1 || rows[0].Status != repo.MailStatusPending || rows[0].TemplateID == nil {
		t.Fatalf("message = %+v", rows)
	}
}

func TestPollIgnoreRuleSkipsGeneration(t *testing.T) {
	mailRepo, account, uid, cleanup := newMailEnv(t)
	defer cleanup()
	if _, err := mailRepo.CreateRule(uid, &repo.MailRuleInput{
		Position: 1, Enabled: true, FromPattern: "otp@bank.com",
		Action: repo.MailRuleActionIgnore,
	}); err != nil {
		t.Fatal(err)
	}
	src := &fakeSource{
		headers: []MailHeader{header(1, "<otp>", "otp@bank.com")},
		bodies:  map[uint32]string{1: "INR 10 debited at X on 12/09/2026"},
	}
	called := false
	gen := func(ctx context.Context, userID, sender string, samples []LLMSample) ([]TemplateSpec, error) {
		called = true
		return nil, nil
	}
	if _, err := PollAccount(context.Background(), src, mailRepo, account, gen, 3); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("IGNORE rules must not trigger the LLM")
	}
	rows, _ := mailRepo.ListMessages(uid, nil, 5)
	if len(rows) != 1 || rows[0].Status != repo.MailStatusIgnored {
		t.Fatalf("message = %+v", rows)
	}
}

func TestParseFlexibleDate(t *testing.T) {
	cases := map[string]string{
		"17 NOV 2026": "2026-11-17",
		"17 Nov 2026": "2026-11-17",
		"17/11/2026":  "2026-11-17",
		"17-11-2026":  "2026-11-17",
		"2026-11-17":  "2026-11-17",
	}
	for raw, want := range cases {
		got, ok := parseFlexibleDate(raw)
		if !ok || got != want {
			t.Errorf("parseFlexibleDate(%q) = %q,%v want %q", raw, got, ok, want)
		}
	}
}

func TestGeneratedTemplateAppliesToExistingQueue(t *testing.T) {
	mailRepo, account, uid, cleanup := newMailEnv(t)
	defer cleanup()
	seedReviewRule(t, mailRepo, uid, "newbank@example.com")

	oldRaw := "Your card XX2467 was debited INR 42.50 on 17 NOV 2026."
	recv := "2026-11-17T10:00:00Z"
	if _, err := mailRepo.InsertMessage(uid, &repo.NewMailMessage{
		MailAccountID: account.ID, MessageID: "<old>", FromAddr: "newbank@example.com",
		ReceivedAt: &recv, Subject: "Debited", RawText: oldRaw, BodyText: oldRaw,
		Status: repo.MailStatusUnparsed,
	}); err != nil {
		t.Fatal(err)
	}

	src := &fakeSource{
		headers: []MailHeader{header(1, "<new>", "newbank@example.com")},
		bodies:  map[uint32]string{1: "Your card XX2467 was debited INR 99.00 on 18 NOV 2026."},
	}
	gen := func(ctx context.Context, userID, sender string, samples []LLMSample) ([]TemplateSpec, error) {
		return []TemplateSpec{debitTemplate(sender)}, nil
	}
	if _, err := PollAccount(context.Background(), src, mailRepo, account, gen, 3); err != nil {
		t.Fatal(err)
	}

	rows, err := mailRepo.ListMessages(uid, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	var old *repo.MailMessageRow
	for _, row := range rows {
		if row.MessageID == "<old>" {
			old = row
		}
	}
	if old == nil {
		t.Fatal("old message missing")
	}
	if old.Status != repo.MailStatusPending || old.TemplateID == nil {
		t.Fatalf("existing row was not reparsed: status=%s template=%v", old.Status, old.TemplateID)
	}
}

func TestFromPatterns(t *testing.T) {
	rules := []*repo.MailRuleRow{
		{Enabled: true, FromPattern: "hdfcbank", Action: repo.MailRuleActionReview},
		{Enabled: true, FromPattern: "HDFCBANK", Action: repo.MailRuleActionAuto},
		{Enabled: false, FromPattern: "", Action: repo.MailRuleActionReview},
	}
	patterns, ok := fromPatterns(rules)
	if !ok || len(patterns) != 1 || patterns[0] != "hdfcbank" {
		t.Fatalf("patterns=%v ok=%v", patterns, ok)
	}
	rules = append(rules, &repo.MailRuleRow{Enabled: true, SubjectContains: "alert", Action: repo.MailRuleActionReview})
	if _, ok := fromPatterns(rules); ok {
		t.Fatal("a rule without a from pattern must disable the IMAP pre-filter")
	}
}
