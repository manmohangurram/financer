package transactionservice

import (
	"testing"

	"github.com/mohan9182/financer/internal/repo"
)

func TestRefNoCreateUpdateAndKeep(t *testing.T) {
	svc, uid, acct, _, cleanup := setup(t)
	defer cleanup()

	if _, err := svc.Create(uid, []Request{{
		Name: "Coffee", Amount: 5, Type: "DEBIT", AccountID: acct, RefNo: "REF-1",
	}}); err != nil {
		t.Fatal(err)
	}
	list := func() *repo.TransactionRow {
		t.Helper()
		res, err := svc.List(uid, &repo.TransactionListFilter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Rows) != 1 {
			t.Fatalf("rows = %d", len(res.Rows))
		}
		return res.Rows[0].Txn
	}

	row := list()
	if row.RefNo != "REF-1" {
		t.Fatalf("ref = %q after create", row.RefNo)
	}

	if _, err := svc.Update(uid, []Request{{
		ID: row.ID, Name: "Coffee", Amount: 5, Type: "DEBIT", AccountID: acct, RefNo: "REF-2",
	}}); err != nil {
		t.Fatal(err)
	}
	if got := list().RefNo; got != "REF-2" {
		t.Fatalf("ref = %q after update", got)
	}

	// An empty ref on update keeps the stored one.
	if _, err := svc.Update(uid, []Request{{
		ID: row.ID, Name: "Coffee", Amount: 5, Type: "DEBIT", AccountID: acct,
	}}); err != nil {
		t.Fatal(err)
	}
	if got := list().RefNo; got != "REF-2" {
		t.Fatalf("ref = %q, want it kept", got)
	}
}
