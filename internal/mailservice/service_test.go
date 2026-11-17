package mailservice

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mohan9182/financer/internal/accountservice"
	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/mail"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/repo/sqlite"
	"github.com/mohan9182/financer/internal/transactionservice"
)

const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

type env struct {
	svc      *Service
	mailRepo repo.MailRepo
	txnRepo  repo.TransactionRepo
	acctRepo repo.AccountRepo
	uid      string
	account  string
	acct2    string
	cleanup  func()
}

func setup(t *testing.T) *env {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	users := sqlite.NewUserRepo(dbs.Write)
	accounts := sqlite.NewAccountRepo(dbs.Write)
	categories := sqlite.NewCategoryRepo(dbs.Write)
	txns := sqlite.NewTransactionRepo(dbs.Write)
	mails := sqlite.NewMailRepo(dbs.Write)

	uid, err := users.Create("m@test.com", "hash", "Mail")
	if err != nil {
		t.Fatal(err)
	}
	a, err := accounts.Create(uid, "HDFC", "Main", "SAVINGS", "1234")
	if err != nil {
		t.Fatal(err)
	}
	key := testKey
	svc := New(mails, &key,
		transactionservice.New(txns, accounts, categories),
		accountservice.New(accounts), nil)
	return &env{
		svc: svc, mailRepo: mails, txnRepo: txns, acctRepo: accounts,
		uid: uid, account: a.ID,
		cleanup: func() {
			dbs.Write.Close()
			dbs.Read.Close()
		},
	}
}

func seedMessage(t *testing.T, e *env, messageID, hint string) string {
	t.Helper()
	acct, err := e.mailRepo.CreateAccount(e.uid, &repo.NewMailAccount{
		Label: "P", IMAPHost: "imap.x.com", IMAPPort: 993,
		Username: "a@x.com", SecretEnc: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	occurred := "2026-09-12"
	extraction := &mail.Extraction{
		Amount: 500, Direction: mail.DirectionDebit, OccurredAt: &occurred,
		Merchant: "SHOP", AccountHint: &hint, Confidence: 1.0,
	}
	parsed, _ := json.Marshal(extraction)
	raw := "body"
	parsedStr := string(parsed)
	if _, err := e.mailRepo.InsertMessage(e.uid, &repo.NewMailMessage{
		MailAccountID: acct.ID, MessageID: messageID, FromAddr: "alerts@bank.com",
		Subject: "Debited", RawText: raw, Status: repo.MailStatusPending,
		ParsedJSON: &parsedStr, Confidence: 1.0, ExtractorSource: "template",
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := e.mailRepo.ListMessages(e.uid, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.MessageID == messageID {
			return row.ID
		}
	}
	t.Fatalf("seeded message %s not found", messageID)
	return ""
}

func TestRuleValidationAndOrdering(t *testing.T) {
	e := setup(t)
	defer e.cleanup()

	if _, err := e.svc.CreateRule(e.uid, model.ReqMailRule{Action: "REVIEW"}); err == nil {
		t.Fatal("a rule with no conditions must be rejected")
	}
	if _, err := e.svc.CreateRule(e.uid, model.ReqMailRule{
		FromPattern: "  ", SubjectContains: " ", Action: "REVIEW",
	}); err == nil {
		t.Fatal("whitespace is not a condition")
	}
	if _, err := e.svc.CreateRule(e.uid, model.ReqMailRule{
		FromPattern: "x@y", Action: "BOGUS",
	}); err == nil {
		t.Fatal("unknown action must be rejected")
	}

	if _, err := e.svc.CreateRule(e.uid, model.ReqMailRule{
		Position: 20, FromPattern: "later@bank.com", Action: "REVIEW",
	}); err != nil {
		t.Fatal(err)
	}
	first, err := e.svc.CreateRule(e.uid, model.ReqMailRule{
		Position: 10, FromPattern: "otp@bank.com", Action: "IGNORE",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Enabled {
		t.Error("enabled defaults to true")
	}

	rules, err := e.svc.Rules(e.uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 || rules[0].FromPattern != "otp@bank.com" || rules[0].Action != "IGNORE" {
		t.Fatalf("rules = %+v", rules)
	}

	if err := e.svc.UpdateRule(e.uid, first.ID, model.ReqMailRule{
		Position: 5, FromPattern: "otp2@bank.com", Action: "AUTO",
	}); err != nil {
		t.Fatal(err)
	}
	rules, _ = e.svc.Rules(e.uid)
	if rules[0].FromPattern != "otp2@bank.com" || rules[0].Action != "AUTO" {
		t.Fatalf("after update = %+v", rules[0])
	}
	if err := e.svc.DeleteRule(e.uid, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.DeleteRule(e.uid, first.ID); err == nil {
		t.Fatal("second delete must be not-found")
	}
}

func TestApproveCreatesTransactionAndMarksRow(t *testing.T) {
	e := setup(t)
	defer e.cleanup()

	msgID := seedMessage(t, e, "<m1>", "1234")
	outcome, err := e.svc.Act(e.uid, []model.MailActionItem{{ID: msgID, Action: "APPROVE"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Succeeded) != 1 || len(outcome.Failed) != 0 {
		t.Fatalf("outcome = %+v", outcome)
	}

	msg, err := e.mailRepo.GetMessage(e.uid, msgID)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Status != repo.MailStatusAuto || msg.TransactionID == nil {
		t.Fatalf("message = %+v", msg)
	}

	rows, err := e.txnRepo.List(e.uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if rows.TotalCount != 1 {
		t.Fatalf("transactions = %d", rows.TotalCount)
	}
	txn := rows.Rows[0].Txn
	if txn.Name != "SHOP" || txn.Amount != 500 || txn.Type != "DEBIT" {
		t.Fatalf("txn = %+v", txn)
	}
	if txn.ExternalID == nil || *txn.ExternalID != "mail:<m1>" {
		t.Fatalf("external id = %v", txn.ExternalID)
	}
	if txn.OccurredAt != "2026-09-12T00:00:00Z" && txn.OccurredAt != "2026-09-12 00:00:00 +0000 UTC" {
		t.Fatalf("occurredAt = %q", txn.OccurredAt)
	}

	// A decided row is refused.
	again, err := e.svc.Act(e.uid, []model.MailActionItem{{ID: msgID, Action: "APPROVE"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Failed) != 1 {
		t.Fatalf("expected already-decided failure, got %+v", again)
	}
}

func TestRejectCreatesNothing(t *testing.T) {
	e := setup(t)
	defer e.cleanup()
	msgID := seedMessage(t, e, "<m2>", "1234")

	if _, err := e.svc.Act(e.uid, []model.MailActionItem{{ID: msgID, Action: "REJECT"}}); err != nil {
		t.Fatal(err)
	}
	msg, err := e.mailRepo.GetMessage(e.uid, msgID)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Status != repo.MailStatusRejected {
		t.Fatalf("status = %s", msg.Status)
	}
	rows, err := e.txnRepo.List(e.uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if rows.TotalCount != 0 {
		t.Fatalf("transactions = %d", rows.TotalCount)
	}
}

func TestAmbiguousAccountFailsInsteadOfGuessing(t *testing.T) {
	e := setup(t)
	defer e.cleanup()

	// A second bank account ending in the same digits, so the hint is ambiguous.
	if _, err := e.acctRepo.Create(e.uid, "ICICI", "Second", "SAVINGS", "1234"); err != nil {
		t.Fatal(err)
	}
	msg1 := seedMessage(t, e, "<m3>", "1234")
	msg2 := seedMessage(t, e, "<m4>", "1234")

	outcome, err := e.svc.Act(e.uid, []model.MailActionItem{
		{ID: msg1, Action: "APPROVE"},
		{ID: msg2, Action: "APPROVE"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Failed) != 2 {
		t.Fatalf("expected both rows to fail, got %+v", outcome)
	}
}

func TestExplicitAccountBypassesHint(t *testing.T) {
	e := setup(t)
	defer e.cleanup()
	msgID := seedMessage(t, e, "<m5>", "0000") // no match

	outcome, err := e.svc.Act(e.uid, []model.MailActionItem{{
		ID: msgID, Action: "APPROVE", AccountID: &e.account,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Succeeded) != 1 {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestUpdateMailboxPreservesPassword(t *testing.T) {
	e := setup(t)
	defer e.cleanup()
	enabled := true
	created, err := e.svc.Create(e.uid, model.ReqMailAccount{
		Label: "Old", IMAPHost: "imap.x.com", IMAPPort: 993,
		Username: "a@x.com", Password: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := e.mailRepo.ListAccounts(e.uid)
	if err != nil {
		t.Fatal(err)
	}
	encBefore := before[0].SecretEnc

	updated, err := e.svc.Update(e.uid, created.ID, model.ReqUpdateMailAccount{
		Label: "New", IMAPHost: "imap.y.com", IMAPPort: 993,
		Username: "b@y.com", Password: "", Enabled: &enabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Label != "New" || updated.IMAPHost != "imap.y.com" || updated.Username != "b@y.com" {
		t.Fatalf("updated = %+v", updated)
	}
	after, _ := e.mailRepo.ListAccounts(e.uid)
	if after[0].SecretEnc != encBefore {
		t.Fatal("empty password must keep the stored secret")
	}

	if _, err := e.svc.Update(e.uid, created.ID, model.ReqUpdateMailAccount{
		Label: "New", IMAPHost: "imap.y.com", IMAPPort: 993,
		Username: "b@y.com", Password: "new-secret", Enabled: &enabled,
	}); err != nil {
		t.Fatal(err)
	}
	changed, _ := e.mailRepo.ListAccounts(e.uid)
	if changed[0].SecretEnc == encBefore {
		t.Fatal("a supplied password must replace the stored secret")
	}
	if _, err := e.svc.Update(e.uid, "missing", model.ReqUpdateMailAccount{
		IMAPHost: "imap.y.com", IMAPPort: 993, Username: "b@y.com",
	}); err == nil {
		t.Fatal("updating a missing mailbox must fail")
	}
}

func TestApproveWithFieldOverrides(t *testing.T) {
	e := setup(t)
	defer e.cleanup()
	msgID := seedMessage(t, e, "<override>", "1234")

	name := "CORRECTED SHOP"
	amount := 123.45
	typ := "CREDIT"
	date := "2026-11-17"
	outcome, err := e.svc.Act(e.uid, []model.MailActionItem{{
		ID: msgID, Action: "APPROVE", AccountID: &e.account,
		Name: &name, Amount: &amount, Type: &typ, OccurredAt: &date,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Succeeded) != 1 {
		t.Fatalf("outcome = %+v", outcome)
	}
	rows, err := e.txnRepo.List(e.uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if rows.TotalCount != 1 {
		t.Fatalf("transactions = %d", rows.TotalCount)
	}
	txn := rows.Rows[0].Txn
	if txn.Name != name || txn.Amount != amount || txn.Type != typ {
		t.Fatalf("txn = %+v", txn)
	}
	if txn.OccurredAt != "2026-11-17T00:00:00Z" && txn.OccurredAt != "2026-11-17 00:00:00 +0000 UTC" {
		t.Fatalf("occurredAt = %q", txn.OccurredAt)
	}

	msg, err := e.mailRepo.GetMessage(e.uid, msgID)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ApprovedJSON == nil || !strings.Contains(*msg.ApprovedJSON, "CORRECTED SHOP") {
		t.Fatalf("approved_json = %v", msg.ApprovedJSON)
	}
}

func TestPollUserAsyncThrottles(t *testing.T) {
	e := setup(t)
	defer e.cleanup()
	disabled := false
	created, err := e.svc.Create(e.uid, model.ReqMailAccount{
		Label: "P", IMAPHost: "imap.x.com", IMAPPort: 993,
		Username: "a@x.com", Password: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Update(e.uid, created.ID, model.ReqUpdateMailAccount{
		Label: "P", IMAPHost: "imap.x.com", IMAPPort: 993,
		Username: "a@x.com", Enabled: &disabled,
	}); err != nil {
		t.Fatal(err)
	}
	if got := e.svc.PollUserAsync(e.uid, false); got != "started" {
		t.Fatalf("first poll = %q", got)
	}
	if got := e.svc.PollUserAsync(e.uid, false); got != "throttled" {
		t.Fatalf("second poll = %q", got)
	}
	if got := e.svc.PollUserAsync(e.uid, true); got != "started" {
		t.Fatalf("forced poll = %q", got)
	}
}

func TestMailboxValidation(t *testing.T) {
	e := setup(t)
	defer e.cleanup()

	base := model.ReqMailAccount{
		IMAPHost: "imap.gmail.com", IMAPPort: 993, Username: "a@x.com",
		Password: "secret",
	}
	if _, err := e.svc.Create(e.uid, base); err != nil {
		t.Fatal(err)
	}

	bad := []func(r *model.ReqMailAccount){
		func(r *model.ReqMailAccount) { r.IMAPHost = "169.254.169.254" },
		func(r *model.ReqMailAccount) { r.IMAPHost = "user@host" },
		func(r *model.ReqMailAccount) { r.IMAPPort = 143 },
		func(r *model.ReqMailAccount) { r.IMAPPort = 8080 },
		func(r *model.ReqMailAccount) { r.Username = " " },
		func(r *model.ReqMailAccount) { r.Password = "" },
	}
	for i, mutate := range bad {
		req := base
		mutate(&req)
		if _, err := e.svc.Create(e.uid, req); err == nil {
			t.Errorf("case %d should be rejected", i)
		}
	}

	// The stored secret is encrypted and decryptable.
	accounts, err := e.svc.List(e.uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].IMAPHost != "imap.gmail.com" {
		t.Fatalf("accounts = %+v", accounts)
	}

	// Missing secret key is a server error, not a 400.
	noKey := New(e.mailRepo, nil, nil, nil, nil)
	if _, err := noKey.Create(e.uid, base); err == nil {
		t.Fatal("missing FINANCER_SECRET_KEY must be refused")
	}
}

func TestMessagesListOmitsBodyAndGetReturnsIt(t *testing.T) {
	e := setup(t)
	defer e.cleanup()
	msgID := seedMessage(t, e, "<detail>", "1234")

	list, _, _, err := e.svc.Messages(e.uid, nil, "", "", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("messages = %d", len(list))
	}
	if list[0].RawText != nil {
		t.Fatal("list rows must not carry the raw message")
	}
	if list[0].BodyText != nil {
		t.Fatal("list rows must not carry the full body")
	}

	full, err := e.svc.Get(e.uid, msgID)
	if err != nil {
		t.Fatal(err)
	}
	if full.BodyText == nil || *full.BodyText != "body" {
		t.Fatalf("bodyText = %v", full.BodyText)
	}
	if full.RawText == nil || *full.RawText != "body" {
		t.Fatalf("rawText = %v", full.RawText)
	}

	if _, err := e.svc.Get(e.uid, "missing"); err == nil {
		t.Fatal("unknown message must return an error")
	}
}

func TestMessagesCursorPagination(t *testing.T) {
	e := setup(t)
	defer e.cleanup()
	ids := map[string]bool{}
	for _, id := range []string{"<p1>", "<p2>", "<p3>"} {
		ids[seedMessage(t, e, id, "1234")] = true
	}

	page1, total, next, err := e.svc.Messages(e.uid, nil, "", "", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(page1) != 2 || next == "" {
		t.Fatalf("page1: total=%d rows=%d next=%q", total, len(page1), next)
	}
	seen := map[string]bool{}
	for _, m := range page1 {
		if seen[m.ID] {
			t.Fatalf("duplicate row %s on page1", m.ID)
		}
		if !ids[m.ID] {
			t.Fatalf("unexpected row %s", m.ID)
		}
		seen[m.ID] = true
	}

	page2, total2, next2, err := e.svc.Messages(e.uid, nil, "", "", next, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total2 != 3 || len(page2) != 1 || next2 != "" {
		t.Fatalf("page2: total=%d rows=%d next=%q", total2, len(page2), next2)
	}
	if seen[page2[0].ID] {
		t.Fatalf("page2 repeated %s", page2[0].ID)
	}

	if _, _, _, err := e.svc.Messages(e.uid, nil, "", "", "not-a-token", 2); err == nil {
		t.Fatal("invalid pageToken must be rejected")
	}
}

func TestMessagesFilters(t *testing.T) {
	e := setup(t)
	defer e.cleanup()
	seedMessage(t, e, "<f1>", "1234")

	// "review" is the union of pending and unparsed.
	rows, _, _, err := e.svc.Messages(e.uid, ptr("review"), "", "swiggy", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("search should not match: %+v", rows)
	}
	rows, _, _, err = e.svc.Messages(e.uid, ptr("review"), "", "debited", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("subject search = %d rows", len(rows))
	}
	if _, _, _, err := e.svc.Messages(e.uid, ptr("nonsense"), "", "", "", 20); err == nil {
		t.Fatal("unknown status must be rejected")
	}
}

func ptr(s string) *string { return &s }

func TestActBlockedWhileAIParseRuns(t *testing.T) {
	e := setup(t)
	defer e.cleanup()
	msgID := seedMessage(t, e, "<locked>", "1234")
	e.svc.aiStates[e.uid] = &model.AIParseStatus{Running: true}
	if _, err := e.svc.Act(e.uid, []model.MailActionItem{{ID: msgID, Action: "REJECT"}}); err == nil {
		t.Fatal("decisions must be refused while AI parse runs")
	}
	e.svc.aiStates[e.uid] = &model.AIParseStatus{Running: false}
	if _, err := e.svc.Act(e.uid, []model.MailActionItem{{ID: msgID, Action: "REJECT"}}); err != nil {
		t.Fatalf("decision after the job failed: %v", err)
	}
}
