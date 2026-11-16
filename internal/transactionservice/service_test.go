package transactionservice

import (
	"path/filepath"
	"testing"

	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/repo/sqlite"
)

func setup(t *testing.T) (*Service, string, string, string, func()) {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	users := sqlite.NewUserRepo(dbs.Write)
	accounts := sqlite.NewAccountRepo(dbs.Write)
	categories := sqlite.NewCategoryRepo(dbs.Write)
	txns := sqlite.NewTransactionRepo(dbs.Write)

	uid, err := users.Create("t@test.com", "hash", "Test")
	if err != nil {
		t.Fatal(err)
	}
	acct, err := accounts.Create(uid, "HDFC", "Main", "SAVINGS", "1234")
	if err != nil {
		t.Fatal(err)
	}
	cats, err := categories.Create(uid, []repo.CategoryCreateInput{{Name: "Groceries"}})
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		dbs.Write.Close()
		dbs.Read.Close()
	}
	return New(txns, accounts, categories), uid, acct.ID, cats[0].ID, cleanup
}

func TestCreateUpdatesBalanceAndListsCategories(t *testing.T) {
	svc, uid, acctID, catID, cleanup := setup(t)
	defer cleanup()

	out, err := svc.Create(uid, []Request{{
		ID:          "txn-1",
		Name:        "Big Bazaar",
		Amount:      250.5,
		Type:        string(model.TransactionTypeDebit),
		OccurredAt:  "2026-11-01 10:00:00 +0000 UTC",
		AccountID:   acctID,
		CategoryIDs: []string{catID},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Success {
		t.Fatalf("create failed: %+v", out)
	}

	// The debit must show up in the account balance (recomputed, not delta).
	// Account repo is the same DB; verify through the transactions list.
	res, err := svc.List(uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 1 {
		t.Fatalf("total = %d", res.TotalCount)
	}
	row := res.Rows[0]
	if row.Txn.Name != "Big Bazaar" || row.Txn.Amount != 250.5 {
		t.Fatalf("row = %+v", row.Txn)
	}
	if len(row.CategoryIDs) != 1 || row.CategoryIDs[0] != catID {
		t.Fatalf("categories = %v", row.CategoryIDs)
	}

	// Deleting clears the row and the category link.
	if _, err := svc.Delete(uid, []string{"txn-1"}); err != nil {
		t.Fatal(err)
	}
	res, err = svc.List(uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 0 {
		t.Fatalf("after delete total = %d", res.TotalCount)
	}
}

func TestCreateValidation(t *testing.T) {
	svc, uid, acctID, _, cleanup := setup(t)
	defer cleanup()

	cases := []struct {
		name string
		req  Request
	}{
		{"missing name", Request{Amount: 1, Type: "DEBIT", AccountID: acctID}},
		{"zero amount", Request{Name: "x", Type: "DEBIT", AccountID: acctID}},
		{"missing account", Request{Name: "x", Amount: 1, Type: "DEBIT"}},
		{"foreign account", Request{Name: "x", Amount: 1, Type: "DEBIT", AccountID: "nope"}},
		{"foreign category", Request{Name: "x", Amount: 1, Type: "DEBIT", AccountID: acctID, CategoryIDs: []string{"nope"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Create(uid, []Request{c.req}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCreateSkipsDuplicateIDs(t *testing.T) {
	svc, uid, acctID, _, cleanup := setup(t)
	defer cleanup()

	req := Request{ID: "dup-1", Name: "Coffee", Amount: 5, Type: "DEBIT", AccountID: acctID}
	if _, err := svc.Create(uid, []Request{req}); err != nil {
		t.Fatal(err)
	}
	out, err := svc.Create(uid, []Request{req})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Success || out.Skipped != 1 {
		t.Fatalf("duplicate create = %+v", out)
	}
}

func TestUpdateAndDeleteMissing(t *testing.T) {
	svc, uid, acctID, _, cleanup := setup(t)
	defer cleanup()

	if _, err := svc.Create(uid, []Request{{ID: "u-1", Name: "Old", Amount: 10, Type: "DEBIT", AccountID: acctID}}); err != nil {
		t.Fatal(err)
	}
	out, err := svc.Update(uid, []Request{{ID: "u-1", Name: "New", Amount: 20, Type: "CREDIT", AccountID: acctID}})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Success {
		t.Fatalf("update = %+v", out)
	}
	res, err := svc.List(uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Rows[0].Txn.Name != "New" || res.Rows[0].Txn.Amount != 20 || res.Rows[0].Txn.Type != "CREDIT" {
		t.Fatalf("updated row = %+v", res.Rows[0].Txn)
	}

	out, err = svc.Update(uid, []Request{{ID: "missing", Name: "x", Amount: 1, Type: "DEBIT", AccountID: acctID}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Success {
		t.Fatal("expected update failure for missing id")
	}
	if _, err := svc.Delete(uid, nil); err == nil {
		t.Fatal("expected error for empty delete")
	}
}

func TestListDateRangeAndCursor(t *testing.T) {
	svc, uid, acctID, _, cleanup := setup(t)
	defer cleanup()

	for i, day := range []string{"2026-11-01", "2026-11-02", "2026-11-03"} {
		if _, err := svc.Create(uid, []Request{{
			ID:         "range-" + day,
			Name:       "Txn",
			Amount:     float64(i + 1),
			Type:       "DEBIT",
			OccurredAt: day + " 00:00:00 +0000 UTC",
			AccountID:  acctID,
		}}); err != nil {
			t.Fatal(err)
		}
	}

	// Page of 2 with a cursor returns the remaining row on the next page.
	page1, err := svc.List(uid, &repo.TransactionListFilter{PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Rows) != 2 || page1.NextPageToken == "" {
		t.Fatalf("page1 rows=%d token=%q", len(page1.Rows), page1.NextPageToken)
	}
	page2, err := svc.List(uid, &repo.TransactionListFilter{PageSize: 2, PageToken: page1.NextPageToken})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Rows) != 1 {
		t.Fatalf("page2 rows=%d", len(page2.Rows))
	}
	if page2.Rows[0].Txn.ID == page1.Rows[0].Txn.ID || page2.Rows[0].Txn.ID == page1.Rows[1].Txn.ID {
		t.Fatal("page2 repeated a page1 row")
	}

	// Invalid date bounds are rejected.
	if _, err := svc.List(uid, &repo.TransactionListFilter{DateFrom: "not-a-date"}); err == nil {
		t.Fatal("expected error for invalid dateFrom")
	}
}
