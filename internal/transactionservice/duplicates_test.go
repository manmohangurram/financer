package transactionservice

import (
	"testing"
)

func TestFindDuplicates(t *testing.T) {
	svc, uid, acctID, _, cleanup := setup(t)
	defer cleanup()

	if _, err := svc.Create(uid, []Request{{
		ID: "seed-1", Name: "Coffee", Amount: 250, Type: "DEBIT",
		OccurredAt: "2026-11-12", AccountID: acctID,
	}}); err != nil {
		t.Fatal(err)
	}

	candidates := []Request{
		{Name: "Same", Amount: 250, Type: "DEBIT", OccurredAt: "2026-11-12", AccountID: acctID},
		{Name: "Different amount", Amount: 251, Type: "DEBIT", OccurredAt: "2026-11-12", AccountID: acctID},
		{Name: "Different type", Amount: 250, Type: "CREDIT", OccurredAt: "2026-11-12", AccountID: acctID},
		{Name: "Different day", Amount: 250, Type: "DEBIT", OccurredAt: "2026-11-13", AccountID: acctID},
	}
	got, err := svc.FindDuplicates(uid, candidates)
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false, false, false}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d = %v, want %v (all: %v)", i, got[i], want[i], got)
		}
	}
}

func TestFindDuplicatesMatchesEachStoredRowOnce(t *testing.T) {
	svc, uid, acctID, _, cleanup := setup(t)
	defer cleanup()

	if _, err := svc.Create(uid, []Request{{
		ID: "seed-2", Name: "Tea", Amount: 20, Type: "DEBIT",
		OccurredAt: "2026-11-12", AccountID: acctID,
	}}); err != nil {
		t.Fatal(err)
	}

	// Two identical candidates, one stored row: only the first is a duplicate.
	got, err := svc.FindDuplicates(uid, []Request{
		{Name: "Tea", Amount: 20, Type: "DEBIT", OccurredAt: "2026-11-12", AccountID: acctID},
		{Name: "Tea", Amount: 20, Type: "DEBIT", OccurredAt: "2026-11-12", AccountID: acctID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got[0] || got[1] {
		t.Fatalf("duplicates = %v, want [true false]", got)
	}
}

func TestFindDuplicatesPrefersRefNo(t *testing.T) {
	svc, uid, acctID, _, cleanup := setup(t)
	defer cleanup()

	if _, err := svc.Create(uid, []Request{{
		ID: "ref-seed", Name: "Coffee", Amount: 250, Type: "DEBIT",
		OccurredAt: "2026-11-12", AccountID: acctID, RefNo: "R1",
	}}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.FindDuplicates(uid, []Request{
		// Same ref, edited date and amount: still the same transaction.
		{Name: "Coffee", Amount: 260, Type: "DEBIT", OccurredAt: "2026-11-13", AccountID: acctID, RefNo: "R1"},
		// New ref, same date and amount: a different transaction.
		{Name: "Tea", Amount: 250, Type: "DEBIT", OccurredAt: "2026-11-12", AccountID: acctID, RefNo: "R2"},
		// No ref: the date/amount heuristic applies.
		{Name: "Tea", Amount: 250, Type: "DEBIT", OccurredAt: "2026-11-12", AccountID: acctID},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false, true}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d = %v, want %v (all: %v)", i, got[i], want[i], got)
		}
	}
}
