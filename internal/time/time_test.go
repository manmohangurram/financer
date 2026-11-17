package time

import "testing"

func TestSetLocEmbedsTZData(t *testing.T) {
	old := Loc
	defer func() { Loc = old }()

	SetLoc("Asia/Kolkata")
	if Loc.String() != "Asia/Kolkata" {
		t.Fatalf("Loc = %q, want Asia/Kolkata (tzdata not embedded?)", Loc.String())
	}
	got := TSRFC3339("2026-09-10 20:30:00 +0000 UTC")
	if got != "2026-09-11T02:00:00+05:30" {
		t.Fatalf("TSRFC3339 = %q", got)
	}
}

func TestLocalDayBounds(t *testing.T) {
	old := Loc
	defer func() { Loc = old }()
	SetLoc("Asia/Kolkata")

	start, ok := LocalDayStart("2026-09-11")
	if !ok {
		t.Fatal("LocalDayStart failed")
	}
	// 00:00 IST == 18:30 UTC on the previous day.
	if start != "2026-09-10 18:30:00 +0000 UTC" {
		t.Errorf("LocalDayStart = %q", start)
	}
	end, ok := LocalDayEnd("2026-09-11")
	if !ok {
		t.Fatal("LocalDayEnd failed")
	}
	if end != "2026-09-11 18:30:00 +0000 UTC" {
		t.Errorf("LocalDayEnd = %q", end)
	}
}

func TestLocalKey(t *testing.T) {
	old := Loc
	defer func() { Loc = old }()
	SetLoc("Asia/Kolkata")
	if got := LocalKey("2026-09-10 20:30:00 +0000 UTC", "day"); got != "2026-09-11" {
		t.Errorf("day key = %q", got)
	}
	if got := LocalKey("2026-09-10 20:30:00 +0000 UTC", "month"); got != "2026-09" {
		t.Errorf("month key = %q", got)
	}
}

func TestParseUTCAcceptsDriverForms(t *testing.T) {
	old := Loc
	defer func() { Loc = old }()
	SetLoc("Asia/Kolkata")

	inputs := []string{
		"2026-09-10 20:30:00 +0000 UTC",
		"2026-09-10 20:30:00",
		"2026-09-10T20:30:00Z", // scanned TIMESTAMP column
		"2026-09-10T20:30:00+00:00",
	}
	for _, in := range inputs {
		if got := TSRFC3339(in); got != "2026-09-11T02:00:00+05:30" {
			t.Errorf("TSRFC3339(%q) = %q", in, got)
		}
	}
	if _, err := ParseUTC("not a date"); err == nil {
		t.Error("expected error for invalid timestamp")
	}
}
