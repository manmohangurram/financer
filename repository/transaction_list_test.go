package repository

import (
	"context"
	"testing"
	"time"

	"github.com/mohan9182/financer/api"
)

func seedListTxns(t *testing.T) (*testDB, string) {
	t.Helper()
	tdb := newTestDB(t)
	tdb.insertUser(t, "u1", "u1@example.com")
	tdb.insertAccount(t, "u1", "a1", api.AccountType_ACCOUNT_TYPE_CHECKING, 0)
	d1 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	tdb.insertTxn(t, "u1", &api.TransactionResponse{Name: "Coffee", Amount: 5.5, Type: api.TransactionType_DEBIT, AccountId: "a1", OccurredAt: d1}, nil)
	tdb.insertTxn(t, "u1", &api.TransactionResponse{Name: "Rent", Amount: 1500, Type: api.TransactionType_DEBIT, AccountId: "a1", OccurredAt: d2}, nil)
	tdb.insertTxn(t, "u1", &api.TransactionResponse{Name: "Salary", Amount: 5000, Type: api.TransactionType_CREDIT, AccountId: "a1", OccurredAt: d3}, nil)
	return tdb, "u1"
}

func TestListFilters(t *testing.T) {
	tdb, userID := seedListTxns(t)
	ctx := context.Background()

	res, err := tdb.txnRepo.List(ctx, userID, TxnListFilter{MinAmount: 100, Name: "Rent", NameMatch: "contains"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Transactions) != 1 || res.Transactions[0].Name != "Rent" {
		t.Fatalf("expected exactly Rent, got %d rows", len(res.Transactions))
	}
}

func TestCountMatchesList(t *testing.T) {
	tdb, userID := seedListTxns(t)
	ctx := context.Background()
	f := TxnListFilter{Name: "ent", NameMatch: "contains"}
	res, err := tdb.txnRepo.List(ctx, userID, f)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if int(res.TotalCount) != len(res.Transactions) {
		t.Fatalf("count=%d list=%d", res.TotalCount, len(res.Transactions))
	}
}

func TestListDateToIsInclusive(t *testing.T) {
	tdb, userID := seedListTxns(t)
	ctx := context.Background()
	// dateTo inclusive of Aug 5: Coffee (Aug 1) + Rent (Aug 5), not Salary (Aug 10)
	res, err := tdb.txnRepo.List(ctx, userID, TxnListFilter{DateTo: time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Transactions) != 2 {
		t.Fatalf("expected 2 txns through Aug 5, got %d", len(res.Transactions))
	}
}

func TestListSortByAmount(t *testing.T) {
	tdb, userID := seedListTxns(t)
	ctx := context.Background()
	// seed: Coffee(5.5 debit), Rent(1500 debit), Salary(5000 credit)
	res, err := tdb.txnRepo.List(ctx, userID, TxnListFilter{SortBy: "debit", SortDir: "desc", PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Transactions) != 3 {
		t.Fatalf("expected 3 txns, got %d", len(res.Transactions))
	}
	if res.Transactions[0].Name != "Rent" || res.Transactions[1].Name != "Coffee" || res.Transactions[2].Name != "Salary" {
		t.Fatalf("expected Rent,Coffee,Salary (debits first desc, credit last), got %s,%s,%s",
			res.Transactions[0].Name, res.Transactions[1].Name, res.Transactions[2].Name)
	}
}

func TestListSortSecondaryReversed(t *testing.T) {
	tdb, userID := seedListTxns(t)
	ctx := context.Background()
	// add a second credit (9000) so the credit group has two rows
	tdb.insertTxn(t, "u1", &api.TransactionResponse{Name: "Bonus", Amount: 9000, Type: api.TransactionType_CREDIT, AccountId: "a1", OccurredAt: time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)}, nil)
	res, err := tdb.txnRepo.List(ctx, userID, TxnListFilter{SortBy: "debit", SortDir: "desc", PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// debits high->low: Rent(1500), Coffee(5.5); credits LOW->HIGH: Salary(5000), Bonus(9000)
	want := []string{"Rent", "Coffee", "Salary", "Bonus"}
	for i, name := range want {
		if res.Transactions[i].Name != name {
			t.Fatalf("position %d: expected %s, got %s (all: %v)", i, name, res.Transactions[i].Name, namesOf(res.Transactions))
		}
	}
}

func namesOf(txns []*api.TransactionResponse) []string {
	names := make([]string, len(txns))
	for i, t := range txns {
		names[i] = t.Name
	}
	return names
}

func TestDashboardSums(t *testing.T) {
	tdb, userID := seedListTxns(t)
	ctx := context.Background()
	tdb.insertAccount(t, "u1", "a2", api.AccountType_ACCOUNT_TYPE_SAVINGS, 0)
	for _, s := range [][2]string{{"a1", "100"}, {"a2", "50"}} {
		if _, err := tdb.accRepo.writeDB.Exec("UPDATE accounts SET balance = ? WHERE id = ?", s[1], s[0]); err != nil {
			t.Fatalf("set balance %s: %v", s[0], err)
		}
	}

	balance, err := tdb.accRepo.SumBalance(ctx, userID)
	if err != nil {
		t.Fatalf("SumBalance: %v", err)
	}
	if balance != 150 {
		t.Fatalf("expected balance 150, got %v", balance)
	}

	// the cached totals are maintained by the service, not the repo — apply
	// them here to test the SUM aggregation
	if err := tdb.accRepo.ApplyTotals(ctx, "a1", 5000, 1505.5); err != nil {
		t.Fatalf("ApplyTotals a1: %v", err)
	}
	if err := tdb.accRepo.ApplyTotals(ctx, "a2", 0, 0); err != nil {
		t.Fatalf("ApplyTotals a2: %v", err)
	}
	income, expenses, err := tdb.accRepo.SumTotals(ctx, userID)
	if err != nil {
		t.Fatalf("SumTotals: %v", err)
	}
	if income != 5000 || expenses != 1505.5 {
		t.Fatalf("expected income=5000 expenses=1505.5, got income=%v expenses=%v", income, expenses)
	}
}

func seedSpending(t *testing.T) (*testDB, string) {
	t.Helper()
	tdb := newTestDB(t)
	tdb.insertUser(t, "u1", "u1@example.com")
	tdb.insertAccount(t, "u1", "a1", api.AccountType_ACCOUNT_TYPE_CHECKING, 0)    // non-debt
	tdb.insertAccount(t, "u1", "a2", api.AccountType_ACCOUNT_TYPE_SAVINGS, 0)     // non-debt
	tdb.insertAccount(t, "u1", "a3", api.AccountType_ACCOUNT_TYPE_CREDIT_CARD, 0) // debt
	d := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	// $50 debit in checking linked to $50 credit in savings: both non-debt -> excluded
	tdb.insertTxn(t, "u1", &api.TransactionResponse{Id: "t1", Name: "Transfer out", Amount: 50, Type: api.TransactionType_DEBIT, AccountId: "a1", OccurredAt: d}, nil)
	tdb.insertTxn(t, "u1", &api.TransactionResponse{Id: "t2", Name: "Transfer in", Amount: 50, Type: api.TransactionType_CREDIT, AccountId: "a2", OccurredAt: d}, nil)
	// $10 debit in credit card: debt account -> included even if transferred
	tdb.insertTxn(t, "u1", &api.TransactionResponse{Id: "t3", Name: "Card spend", Amount: 10, Type: api.TransactionType_DEBIT, AccountId: "a3", OccurredAt: d}, nil)
	// $30 plain debit in checking: not transferred -> included
	tdb.insertTxn(t, "u1", &api.TransactionResponse{Id: "t4", Name: "Groceries", Amount: 30, Type: api.TransactionType_DEBIT, AccountId: "a1", OccurredAt: d}, nil)
	// link t1 <-> t2
	if errs := tdb.linkRepo.Create(context.Background(), "u1", []LinkTransferInput{{DebitTransactionID: "t1", CreditTransactionID: "t2"}}); len(errs) > 0 {
		t.Fatalf("link: %v", errs[0])
	}
	return tdb, "u1"
}

func TestSpendingBuckets(t *testing.T) {
	tdb, userID := seedSpending(t)
	f := SpendingFilter{Granularity: "day", From: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
	rows, err := tdb.txnRepo.SpendingBuckets(context.Background(), userID, f)
	if err != nil {
		t.Fatalf("SpendingBuckets: %v", err)
	}
	if len(rows) != 1 || rows[0].Amount != 40 {
		t.Fatalf("expected one day-bucket of 40 (10+30, transfer 50 excluded), got %+v", rows)
	}
}

func TestSpendingCategoriesUncategorized(t *testing.T) {
	tdb, userID := seedSpending(t)
	f := SpendingFilter{Granularity: "day", From: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
	cats, err := tdb.txnRepo.SpendingCategories(context.Background(), userID, f)
	if err != nil {
		t.Fatalf("SpendingCategories: %v", err)
	}
	if len(cats) != 1 || cats[0].Id != "__uncategorized__" || cats[0].Debit != 40 {
		t.Fatalf("expected one Uncategorized row with debit 40, got %+v", cats)
	}
}
