package services

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/db"
	"github.com/mohan9182/financer/repository"
)

func newInvestmentImportHarness(t *testing.T) (*InvestmentService, context.Context) {
	t.Helper()
	writeDB, readDB, err := db.OpenDBs(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenDBs: %v", err)
	}
	t.Cleanup(func() { writeDB.Close(); readDB.Close() })
	if err := db.RunMigrations(writeDB); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	if _, err := writeDB.Exec(
		`INSERT INTO users (id, email, password_hash, name, created_at) VALUES (?, ?, ?, ?, ?)`,
		"u1", "u1@test", "x", "U1", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	base := repository.NewBaseRepository(writeDB, readDB)
	svc := NewInvestmentService(repository.NewInvestmentRepository(base), nil)
	ctx := context.WithValue(context.Background(), auth.UserIDKey, "u1")
	return svc, ctx
}

func TestImportInvestments(t *testing.T) {
	svc, ctx := newInvestmentImportHarness(t)
	now := time.Now().UTC()

	resp, err := svc.ImportInvestments(ctx, &api.ImportInvestmentsRequest{
		Rows: []*api.ImportInvestmentRow{
			{Symbol: "NTPC.NS", Name: "NTPC Ltd", InvestmentType: api.InvestmentType_INVESTMENT_TYPE_STOCK, Side: 1, Quantity: 10, Price: 342.5, OccurredAt: now, ExternalId: "file:0"},
			{Symbol: "RELIANCE.NS", Name: "Reliance", InvestmentType: api.InvestmentType_INVESTMENT_TYPE_STOCK, Side: 1, Quantity: 5, Price: 2500, OccurredAt: now, ExternalId: "file:1"},
		},
	})
	if err != nil {
		t.Fatalf("ImportInvestments: %v", err)
	}
	if resp.Created != 2 || resp.Skipped != 0 {
		t.Fatalf("expected created=2 skipped=0, got created=%d skipped=%d", resp.Created, resp.Skipped)
	}

	// Re-import: investments merge by symbol, lots skip via external_id.
	resp2, err := svc.ImportInvestments(ctx, &api.ImportInvestmentsRequest{
		Rows: []*api.ImportInvestmentRow{
			{Symbol: "NTPC.NS", Name: "NTPC Ltd", InvestmentType: api.InvestmentType_INVESTMENT_TYPE_STOCK, Side: 1, Quantity: 10, Price: 342.5, OccurredAt: now, ExternalId: "file:0"},
			{Symbol: "NTPC.NS", Name: "NTPC Ltd", InvestmentType: api.InvestmentType_INVESTMENT_TYPE_STOCK, Side: 1, Quantity: 3, Price: 350, OccurredAt: now, ExternalId: "file:2"},
		},
	})
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if resp2.Created != 1 || resp2.Skipped != 1 {
		t.Fatalf("re-import: expected created=1 skipped=1, got created=%d skipped=%d", resp2.Created, resp2.Skipped)
	}

	// Sell beyond holdings is skipped.
	resp3, err := svc.ImportInvestments(ctx, &api.ImportInvestmentsRequest{
		Rows: []*api.ImportInvestmentRow{
			{Symbol: "NTPC.NS", Name: "NTPC Ltd", InvestmentType: api.InvestmentType_INVESTMENT_TYPE_STOCK, Side: -1, Quantity: 999, Price: 400, OccurredAt: now, ExternalId: "file:3"},
		},
	})
	if err != nil {
		t.Fatalf("oversell: %v", err)
	}
	if resp3.Created != 0 || resp3.Skipped != 1 {
		t.Fatalf("oversell: expected created=0 skipped=1, got created=%d skipped=%d", resp3.Created, resp3.Skipped)
	}
}
