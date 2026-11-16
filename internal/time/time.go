// Package time provides timestamp helpers matching the Rust backend's
// `timex` module: storage is always UTC in "YYYY-MM-DD HH:MM:SS +0000 UTC"
// form; the wire value renders in the configured timezone.
package time

import (
	"fmt"
	"strings"
	"time"

	// Embed the IANA timezone database so LoadLocation works on systems without
	// a system zoneinfo (notably Windows).
	_ "time/tzdata"
)

// storageLayout is the format stored in SQLite, matching Rust's `go_ts`:
// "2026-09-10 20:30:00 +0000 UTC".
const storageLayout = "2006-01-02 15:04:05 -0700 MST"

// Loc is the configured process timezone, set once at startup from config.
var Loc = time.UTC

// SetLoc sets the process timezone (called once at startup).
func SetLoc(name string) {
	if loc, err := time.LoadLocation(name); err == nil {
		Loc = loc
	}
}

// NowGoTS returns the current UTC instant in storage layout, matching
// Rust's `now_go_ts()` / `go_ts(Utc::now())`.
func NowGoTS() string {
	return time.Now().UTC().Format(storageLayout)
}

// ParseUTC parses a stored timestamp into a time.Time in UTC. It accepts every
// form a value can arrive in: the Go-driver layout "YYYY-MM-DD HH:MM:SS +0000
// UTC", a bare "YYYY-MM-DD HH:MM:SS", and RFC3339 (what the SQLite driver
// yields when a TIMESTAMP column is scanned into a string).
func ParseUTC(stored string) (time.Time, error) {
	s := strings.TrimSpace(stored)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	if i := strings.LastIndex(s, " +0000 UTC"); i > 0 {
		s = s[:i]
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04",
	} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", stored)
}

// TSRFC3339 renders a stored UTC timestamp as RFC3339 (seconds precision) in
// the configured location, matching Rust's `ts_rfc3339`.
func TSRFC3339(stored string) string {
	t, err := ParseUTC(stored)
	if err != nil {
		return stored
	}
	return t.In(Loc).Format(time.RFC3339)
}

// FromUTC formats a UTC instant in storage layout (Rust's `go_ts`).
func FromUTC(t time.Time) string {
	return t.UTC().Format(storageLayout)
}

// NowUTC is the current instant in UTC.
func NowUTC() time.Time {
	return time.Now().UTC()
}

// LocalDateStartUTC is the UTC instant of local midnight for a YYYY-MM-DD
// date in the configured timezone.
func LocalDateStartUTC(date string) (time.Time, bool) {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, Loc).UTC(), true
}

// LocalDayStart converts an inclusive local date (YYYY-MM-DD) to the stored
// UTC bound, matching Rust's `local_day_start`.
func LocalDayStart(date string) (string, bool) {
	t, ok := LocalDateStartUTC(date)
	if !ok {
		return "", false
	}
	return FromUTC(t), true
}

// LocalDayEnd converts the local day AFTER date to the stored UTC bound
// (exclusive upper bound, since range ends are inclusive), matching Rust's
// `local_day_end`.
func LocalDayEnd(date string) (string, bool) {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", false
	}
	next := d.AddDate(0, 0, 1)
	return FromUTC(time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, Loc).UTC()), true
}

// LocalKey is the local bucket key (YYYY-MM for "month", YYYY-MM-DD for "day")
// for a stored UTC timestamp, matching Rust's `local_key`.
func LocalKey(stored, granularity string) string {
	t, err := ParseUTC(stored)
	if err != nil {
		return ""
	}
	if granularity == "month" {
		return t.In(Loc).Format("2006-01")
	}
	return t.In(Loc).Format("2006-01-02")
}
