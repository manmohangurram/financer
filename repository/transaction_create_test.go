package repository

import (
	"context"
	"testing"
	"time"

	"github.com/mohan9182/financer/api"
)

func TestCreateIdempotentExternalId(t *testing.T) {
	tdb := newTestDB(t)
	tdb.insertUser(t, "u1", "u1@example.com")
	tdb.insertAccount(t, "u1", "a1", api.AccountType_ACCOUNT_TYPE_CHECKING, 0)
	ctx := context.Background()

	mk := func() CreateTransactionInput {
		return CreateTransactionInput{Txn: &api.TransactionResponse{
			Name: "Import", Amount: 10, Type: api.TransactionType_DEBIT,
			AccountId: "a1", OccurredAt: time.Now().UTC(), CreatedAt: time.Now().UTC(),
			ExternalId: "file1:0",
		}}
	}

	// first import inserts
	inserted, errs := tdb.txnRepo.Create(ctx, "u1", []CreateTransactionInput{mk()})
	if len(errs) > 0 {
		t.Fatalf("first create: %v", errs[0])
	}
	if len(inserted) != 1 {
		t.Fatalf("expected 1 inserted on first import, got %d", len(inserted))
	}

	// re-import of the same file skips the duplicate
	inserted, errs = tdb.txnRepo.Create(ctx, "u1", []CreateTransactionInput{mk()})
	if len(errs) > 0 {
		t.Fatalf("second create: %v", errs[0])
	}
	if len(inserted) != 0 {
		t.Fatalf("expected 0 inserted on re-import, got %d", len(inserted))
	}

	res, err := tdb.txnRepo.List(ctx, "u1", TxnListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Transactions) != 1 {
		t.Fatalf("expected exactly 1 transaction after re-import, got %d", len(res.Transactions))
	}
}

func TestCreateDistinctExternalIdsBothInsert(t *testing.T) {
	tdb := newTestDB(t)
	tdb.insertUser(t, "u1", "u1@example.com")
	tdb.insertAccount(t, "u1", "a1", api.AccountType_ACCOUNT_TYPE_CHECKING, 0)
	ctx := context.Background()

	// two identical rows in the same file have distinct indices -> both insert
	inputs := []CreateTransactionInput{
		{Txn: &api.TransactionResponse{Name: "Coffee", Amount: 99, Type: api.TransactionType_DEBIT, AccountId: "a1", OccurredAt: time.Now().UTC(), CreatedAt: time.Now().UTC(), ExternalId: "f:0"}},
		{Txn: &api.TransactionResponse{Name: "Coffee", Amount: 99, Type: api.TransactionType_DEBIT, AccountId: "a1", OccurredAt: time.Now().UTC(), CreatedAt: time.Now().UTC(), ExternalId: "f:1"}},
	}
	inserted, errs := tdb.txnRepo.Create(ctx, "u1", inputs)
	if len(errs) > 0 {
		t.Fatalf("create: %v", errs[0])
	}
	if len(inserted) != 2 {
		t.Fatalf("expected both identical rows inserted, got %d", len(inserted))
	}
}
