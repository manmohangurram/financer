package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mohan9182/financer/db"
	"github.com/mohan9182/financer/api"
)

type testDB struct {
	txnRepo  *TransactionRepository
	accRepo  *AccountRepository
	catRepo  *CategoryRepository
	linkRepo *TransferRepository
}

func newTestDB(t *testing.T) *testDB {
	t.Helper()
	writeDB, readDB, err := db.OpenDBs(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenDBs: %v", err)
	}
	if err := db.RunMigrations(writeDB); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	t.Cleanup(func() {
		writeDB.Close()
		readDB.Close()
	})
	base := NewBaseRepository(writeDB, readDB)
	return &testDB{
		txnRepo:  NewTransactionRepository(base),
		accRepo:  NewAccountRepository(base),
		catRepo:  NewCategoryRepository(base),
		linkRepo: NewTransferRepository(base),
	}
}

func (tdb *testDB) insertUser(t *testing.T, id, email string) {
	t.Helper()
	if _, err := tdb.accRepo.writeDB.Exec(
		"INSERT INTO users (id, email, password_hash, name, created_at) VALUES (?, ?, ?, ?, ?)",
		id, email, "x", "Test", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func (tdb *testDB) insertAccount(t *testing.T, userID, id string, accountType api.AccountType, balance float32) {
	t.Helper()
	if _, err := tdb.accRepo.writeDB.Exec(
		"INSERT INTO accounts (id, user_id, bank_name, account_nickname, balance, type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, userID, "Bank", id, balance, int(accountType), time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert account: %v", err)
	}
}

func (tdb *testDB) insertTxn(t *testing.T, userID string, txn *api.TransactionResponse, categoryIDs []string) {
	t.Helper()
	if txn.Id == "" {
		txn.Id = uuid.New().String()
	}
	if txn.CreatedAt.IsZero() {
		txn.CreatedAt = time.Now().UTC()
	}
	_, errs := tdb.txnRepo.Create(context.Background(), userID, []CreateTransactionInput{{Txn: txn, CategoryIDs: categoryIDs}})
	if len(errs) > 0 {
		t.Fatalf("create txn: %v", errs[0])
	}
}
