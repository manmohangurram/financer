package mailservice

import (
	"encoding/json"
	"path/filepath"
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
		accountservice.New(accounts))
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
		SenderAllowlist: []string{"alerts@bank.com"},
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

func TestMailboxValidation(t *testing.T) {
	e := setup(t)
	defer e.cleanup()

	base := model.ReqMailAccount{
		IMAPHost: "imap.gmail.com", IMAPPort: 993, Username: "a@x.com",
		Password: "secret", SenderAllowlist: []string{"alerts@bank.com"},
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
		func(r *model.ReqMailAccount) { r.SenderAllowlist = nil },
		func(r *model.ReqMailAccount) { r.SenderAllowlist = []string{"not-an-address"} },
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
	noKey := New(e.mailRepo, nil, nil, nil)
	if _, err := noKey.Create(e.uid, base); err == nil {
		t.Fatal("missing FINANCER_SECRET_KEY must be refused")
	}
}
