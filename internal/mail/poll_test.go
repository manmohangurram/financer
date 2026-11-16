package mail

import (
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

func (f *fakeSource) Headers(senders []string, sinceDays int) ([]MailHeader, error) {
	var out []MailHeader
	for _, h := range f.headers {
		for _, s := range senders {
			if h.FromAddr == s {
				out = append(out, h)
				break
			}
		}
	}
	return out, nil
}

func (f *fakeSource) Body(uid uint32) (string, error) {
	f.fetched = append(f.fetched, uid)
	return f.bodies[uid], nil
}

func newMailEnv(t *testing.T) (repo.MailRepo, *repo.MailAccountRow, string, func()) {
	return newMailEnvWith(t, "alerts@bank.com")
}

func newMailEnvWith(t *testing.T, senders ...string) (repo.MailRepo, *repo.MailAccountRow, string, func()) {
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
		SenderAllowlist: senders,
	})
	if err != nil {
		t.Fatal(err)
	}
	return mailRepo, account, uid, func() {
		dbs.Write.Close()
		dbs.Read.Close()
	}
}

func header(uid uint32, messageID, from string) MailHeader {
	return MailHeader{UID: uid, MessageID: messageID, FromAddr: from, Subject: "Debited"}
}

func TestPollAccountStoresAndSkips(t *testing.T) {
	mailRepo, account, uid, cleanup := newMailEnv(t)
	defer cleanup()

	src := &fakeSource{
		headers: []MailHeader{
			header(1, "<m1>", "alerts@bank.com"),
			header(2, "<m2>", "alerts@bank.com"),
			header(3, "<m3>", "someone@else.com"), // not allowlisted
		},
		bodies: map[uint32]string{
			1: "INR 1,000 debited at SHOP on 12/09/2026 from a/c XX1234",
			2: "Your OTP is 123456",
		},
	}
	summary, err := PollAccount(src, mailRepo, account, TemplateExtractor{})
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
	again, err := PollAccount(src, mailRepo, account, TemplateExtractor{})
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

func TestPollAccountRules(t *testing.T) {
	mailRepo, account, uid, cleanup := newMailEnvWith(t, "otp@bank.com", "auto@bank.com")
	defer cleanup()

	if _, err := mailRepo.CreateRule(uid, &repo.MailRuleInput{
		Position: 1, Enabled: true, FromPattern: "otp@bank.com",
		SubjectContains: "", BodyContains: "", Action: repo.MailRuleActionIgnore,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := mailRepo.CreateRule(uid, &repo.MailRuleInput{
		Position: 2, Enabled: true, FromPattern: "auto@bank.com",
		Action: repo.MailRuleActionAuto,
	}); err != nil {
		t.Fatal(err)
	}

	src := &fakeSource{
		headers: []MailHeader{
			header(1, "<otp>", "otp@bank.com"),
			header(2, "<auto>", "auto@bank.com"),
			header(3, "<weak>", "auto@bank.com"),
		},
		bodies: map[uint32]string{
			1: "INR 10 debited at X on 12/09/2026 from a/c 1234",
			2: "INR 500 debited at SHOP on 12/09/2026 from a/c XX1234",
			3: "Rs. 500 debited at a shop", // partial extraction -> review, not AUTO
		},
	}
	if _, err := PollAccount(src, mailRepo, account, TemplateExtractor{}); err != nil {
		t.Fatal(err)
	}
	rows, err := mailRepo.ListMessages(uid, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]repo.MailStatus{}
	for _, r := range rows {
		byID[r.MessageID] = r.Status
	}
	if byID["<otp>"] != repo.MailStatusIgnored {
		t.Errorf("<otp> = %s, want IGNORED", byID["<otp>"])
	}
	if byID["<auto>"] != repo.MailStatusAuto {
		t.Errorf("<auto> = %s, want AUTO", byID["<auto>"])
	}
	if byID["<weak>"] != repo.MailStatusPending {
		t.Errorf("<weak> = %s, want PENDING (low confidence)", byID["<weak>"])
	}
}
