package services

import (
	"testing"
	"time"
)

func TestPresetPeriodWeekend(t *testing.T) {
	// Sunday 2026-08-09 -> Friday 2026-08-07 full session
	sunday := time.Date(2026, 8, 9, 10, 30, 0, 0, time.UTC)
	p1, p2 := presetPeriod("1d", priceHistoryRanges["1d"], sunday)
	if p1 != time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("sunday p1 = %v, want Friday 00:00", time.Unix(p1, 0).UTC())
	}
	if p2 != time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("sunday p2 = %v, want Saturday 00:00", time.Unix(p2, 0).UTC())
	}

	// Saturday 2026-08-08 -> Friday 2026-08-07 full session
	saturday := time.Date(2026, 8, 8, 10, 30, 0, 0, time.UTC)
	p1, p2 = presetPeriod("1d", priceHistoryRanges["1d"], saturday)
	if p1 != time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("saturday p1 = %v, want Friday 00:00", time.Unix(p1, 0).UTC())
	}

	// Weekday -> trailing 24h
	wednesday := time.Date(2026, 8, 5, 10, 30, 0, 0, time.UTC)
	p1, p2 = presetPeriod("1d", priceHistoryRanges["1d"], wednesday)
	if p2 != wednesday.Unix() {
		t.Fatalf("weekday p2 = %v, want now", time.Unix(p2, 0).UTC())
	}
	if time.Unix(p2, 0).UTC().Sub(time.Unix(p1, 0).UTC()) != 24*time.Hour {
		t.Fatalf("weekday span = %v, want 24h", time.Unix(p2, 0).UTC().Sub(time.Unix(p1, 0).UTC()))
	}

	// Non-1d ranges unaffected
	p1, p2 = presetPeriod("1m", priceHistoryRanges["1m"], sunday)
	if time.Unix(p2, 0).UTC().Sub(time.Unix(p1, 0).UTC()) != 30*24*time.Hour {
		t.Fatalf("1m span = %v, want 30d", time.Unix(p2, 0).UTC().Sub(time.Unix(p1, 0).UTC()))
	}
}
