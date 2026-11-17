package transferservice

import (
	"path/filepath"
	"testing"

	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/repo/sqlite"
)

func setup(t *testing.T) (*Service, repo.TransactionRepo, string, string, string, func()) {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	users := sqlite.NewUserRepo(dbs.Write)
	accounts := sqlite.NewAccountRepo(dbs.Write)
	txns := sqlite.NewTransactionRepo(dbs.Write)

	uid, err := users.Create("t@test.com", "hash", "Test")
	if err != nil {
		t.Fatal(err)
	}
	a, err := accounts.Create(uid, "HDFC", "Main", "SAVINGS", "1111")
	if err != nil {
		t.Fatal(err)
	}
	b, err := accounts.Create(uid, "ICICI", "Savings", "SAVINGS", "2222")
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		dbs.Write.Close()
		dbs.Read.Close()
	}
	return New(txns, accounts), txns, uid, a.ID, b.ID, cleanup
}

func seedDebit(t *testing.T, txns repo.TransactionRepo, uid, acctID, id string, amount float64) {
	t.Helper()
	_, err := txns.Create(uid, []repo.CreateTransactionInput{{Txn: &repo.TransactionRow{
		ID: id, Name: "Moved to savings", Amount: amount, Type: "DEBIT",
		OccurredAt: "2026-11-02 09:00:00 +0000 UTC", AccountID: acctID,
		CreatedAt: "2026-11-02 09:00:00 +0000 UTC",
	}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateCounterpartLinksAndBalances(t *testing.T) {
	svc, txns, uid, a, b, cleanup := setup(t)
	defer cleanup()
	seedDebit(t, txns, uid, a, "debit-1", 500)

	resp, err := svc.CreateCounterpart(uid, "debit-1", b)
	if err != nil {
		t.Fatal(err)
	}
	if resp.DebitTransactionID != "debit-1" || resp.CreditTransactionID == "" {
		t.Fatalf("resp = %+v", resp)
	}

	rows, err := txns.List(uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if rows.TotalCount != 2 {
		t.Fatalf("rows = %d", rows.TotalCount)
	}
	for _, row := range rows.Rows {
		if row.LinkID == "" {
			t.Fatalf("row %s not linked", row.Txn.ID)
		}
	}

	// Re-running against an already-linked transaction is rejected.
	if _, err := svc.CreateCounterpart(uid, "debit-1", b); err == nil {
		t.Fatal("expected error for already-linked transaction")
	}
	// Same-account transfer is rejected.
	seedDebit(t, txns, uid, a, "debit-2", 5)
	if _, err := svc.CreateCounterpart(uid, "debit-2", a); err == nil {
		t.Fatal("expected error for same-account transfer")
	}
	// Unknown transaction is a 404.
	if _, err := svc.CreateCounterpart(uid, "missing", b); err == nil {
		t.Fatal("expected not-found")
	}
}

func TestLinkTransfersValidation(t *testing.T) {
	svc, txns, uid, a, b, cleanup := setup(t)
	defer cleanup()

	seedDebit(t, txns, uid, a, "debit-1", 100)
	if _, err := txns.Create(uid, []repo.CreateTransactionInput{{Txn: &repo.TransactionRow{
		ID: "credit-1", Name: "Received", Amount: 100, Type: "CREDIT",
		OccurredAt: "2026-11-02 09:00:00 +0000 UTC", AccountID: b,
		CreatedAt: "2026-11-02 09:00:00 +0000 UTC",
	}}}); err != nil {
		t.Fatal(err)
	}

	out, err := svc.LinkTransfers(uid, [][2]string{{"debit-1", "credit-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Success {
		t.Fatalf("link = %+v", out)
	}

	// Validation failures are reported per link, not as hard errors.
	out, err = svc.LinkTransfers(uid, [][2]string{{"debit-1", "credit-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Success || len(out.FailedIDs) == 0 {
		t.Fatalf("expected already-linked failure, got %+v", out)
	}
	out, err = svc.LinkTransfers(uid, [][2]string{{"credit-1", "debit-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Success || len(out.FailedIDs) == 0 {
		t.Fatalf("expected wrong-direction failure, got %+v", out)
	}
	if _, err := svc.LinkTransfers(uid, nil); err == nil {
		t.Fatal("expected bad request for empty links")
	}
}

func TestUnlinkTransfers(t *testing.T) {
	svc, txns, uid, a, b, cleanup := setup(t)
	defer cleanup()
	seedDebit(t, txns, uid, a, "debit-1", 10)
	resp, err := svc.CreateCounterpart(uid, "debit-1", b)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp

	rows, err := txns.List(uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	linkID := rows.Rows[0].LinkID
	if linkID == "" {
		t.Fatal("expected link")
	}
	out, err := svc.UnlinkTransfers(uid, []string{linkID})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Success {
		t.Fatalf("unlink = %+v", out)
	}
	rows, err = txns.List(uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows.Rows {
		if row.LinkID != "" {
			t.Fatal("link not removed")
		}
	}
	// Removing again reports the missing link.
	out, err = svc.UnlinkTransfers(uid, []string{linkID})
	if err != nil {
		t.Fatal(err)
	}
	if out.Success {
		t.Fatal("expected unlink failure for missing link")
	}
}

var _ = model.TransactionTypeDebit
