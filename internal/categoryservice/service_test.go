package categoryservice

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/repo/sqlite"
)

func setup(t *testing.T) (*Service, string, string, func()) {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	users := sqlite.NewUserRepo(dbs.Write)
	uid, err := users.Create("budget@test.com", "hash", "Budget")
	if err != nil {
		t.Fatal(err)
	}
	svc := New(sqlite.NewCategoryRepo(dbs.Write))
	if res := svc.Create(uid, []string{"Food"}); !res.Success {
		t.Fatalf("create category: %+v", res)
	}
	list, _, err := svc.List(uid, 10, "")
	if err != nil || len(list) != 1 {
		t.Fatalf("categories = %+v err=%v", list, err)
	}
	return svc, uid, list[0].ID, func() {
		dbs.Write.Close()
		dbs.Read.Close()
	}
}

func TestSetBudgetLifecycle(t *testing.T) {
	svc, uid, id, cleanup := setup(t)
	defer cleanup()

	got, err := svc.SetBudget(uid, id, 8000.129)
	if err != nil {
		t.Fatal(err)
	}
	if got.MonthlyBudget != 8000.13 {
		t.Fatalf("budget = %v, want 8000.13", got.MonthlyBudget)
	}

	list, _, err := svc.List(uid, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if list[0].MonthlyBudget != 8000.13 {
		t.Fatalf("listed budget = %v", list[0].MonthlyBudget)
	}

	cleared, err := svc.SetBudget(uid, id, 0)
	if err != nil || cleared.MonthlyBudget != 0 {
		t.Fatalf("clear budget = %+v err=%v", cleared, err)
	}
}

func TestSetBudgetValidation(t *testing.T) {
	svc, uid, id, cleanup := setup(t)
	defer cleanup()

	for _, bad := range []float64{-1, math.Inf(1), math.NaN(), maxMonthlyBudget + 1} {
		if _, err := svc.SetBudget(uid, id, bad); err == nil {
			t.Errorf("budget %v should be rejected", bad)
		}
	}
	if _, err := svc.SetBudget(uid, "missing", 100); err == nil {
		t.Fatal("unknown category must return an error")
	}
}
