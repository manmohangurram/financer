package repository

import (
	"context"
	"testing"
	"time"

	"github.com/mohan9182/financer/api"
)

func insertTestInvestment(t *testing.T, tdb *testDB, userID, symbol, name string) string {
	t.Helper()
	id := "inv-" + symbol
	_, err := tdb.txnRepo.BaseRepository.writeDB.ExecContext(context.Background(),
		`INSERT INTO investments (id, user_id, symbol, name, investment_type) VALUES (?, ?, ?, ?, ?)`,
		id, userID, symbol, name, int(api.InvestmentType_INVESTMENT_TYPE_STOCK))
	if err != nil {
		t.Fatalf("insert investment: %v", err)
	}
	return id
}

func TestInvestmentGetBySymbol(t *testing.T) {
	tdb := newTestDB(t)
	ctx := context.Background()
	tdb.insertUser(t, "u1", "u1@test")
	insertTestInvestment(t, tdb, "u1", "NTPC.NS", "NTPC Ltd")

	repo := NewInvestmentRepository(tdb.txnRepo.BaseRepository)
	inst, err := repo.GetBySymbol(ctx, "u1", "NTPC.NS")
	if err != nil {
		t.Fatalf("GetBySymbol: %v", err)
	}
	if inst == nil || inst.Name != "NTPC Ltd" {
		t.Fatalf("expected NTPC Ltd, got %+v", inst)
	}
	other, err := repo.GetBySymbol(ctx, "u2", "NTPC.NS")
	if err != nil {
		t.Fatalf("GetBySymbol other: %v", err)
	}
	if other != nil {
		t.Fatalf("expected nil for other user, got %+v", other)
	}
}

func TestInvestmentInsertLotIdempotent(t *testing.T) {
	tdb := newTestDB(t)
	ctx := context.Background()
	tdb.insertUser(t, "u1", "u1@test")
	invID := insertTestInvestment(t, tdb, "u1", "RELIANCE.NS", "Reliance")

	repo := NewInvestmentRepository(tdb.txnRepo.BaseRepository)
	now := time.Now().UTC()
	first, err := repo.InsertLot(ctx, "u1", invID, 1, 10, 2500, now, "file:0")
	if err != nil {
		t.Fatalf("InsertLot: %v", err)
	}
	if !first {
		t.Fatal("expected first insert to be inserted")
	}
	second, err := repo.InsertLot(ctx, "u1", invID, 1, 10, 2500, now, "file:0")
	if err != nil {
		t.Fatalf("InsertLot second: %v", err)
	}
	if second {
		t.Fatal("expected duplicate insert to be skipped")
	}
	lots, err := repo.ListLots(ctx, invID)
	if err != nil {
		t.Fatalf("ListLots: %v", err)
	}
	if len(lots) != 1 {
		t.Fatalf("expected 1 lot, got %d", len(lots))
	}
}
