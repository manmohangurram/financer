//! Shared timestamp helpers matching Go's sqlite driver wire format.
//!
//! The Go driver stores `time.Time` as `"YYYY-MM-DD HH:MM:SS +0000 UTC"`, and
//! both backends share the same `SQLite` file. These helpers format for storage
//! and parse back to RFC3339 for the wire (`tsRFC3339`).

use chrono::{DateTime, NaiveDateTime, Utc};

/// Format a timestamp the way the Go sqlite driver stores `time.Time` in the
/// shared DB file, so Rust and Go agree byte-for-byte on stored columns.
pub fn go_ts(utc: DateTime<Utc>) -> String {
    utc.format("%Y-%m-%d %H:%M:%S +0000 UTC").to_string()
}

/// Parse a stored timestamp back to RFC3339 UTC (matching Go's `tsRFC3339`).
/// Handles the Go driver format (with optional fractional seconds), plain
/// `YYYY-MM-DD HH:MM:SS`, and RFC3339.
pub fn ts_rfc3339(stored: &str) -> String {
    let s = stored.trim();
    // Go's driver stores "YYYY-MM-DD HH:MM:SS[.ffff] +0000 UTC"; the trailing
    // zone suffix isn't part of the wall clock, so drop it for naive parsing.
    let naive_str = s.split(' ').take(2).collect::<Vec<_>>().join(" ");
    for fmt in ["%Y-%m-%d %H:%M:%S%.f", "%Y-%m-%d %H:%M:%S"] {
        if let Ok(naive) = NaiveDateTime::parse_from_str(&naive_str, fmt) {
            // Go's tsRFC3339 = UTC().Format(time.RFC3339) → "...T..Z" (seconds).
            return naive.and_utc().format("%Y-%m-%dT%H:%M:%SZ").to_string();
        }
    }
    if let Ok(dt) = DateTime::parse_from_rfc3339(s) {
        return dt.with_timezone(&Utc).format("%Y-%m-%dT%H:%M:%SZ").to_string();
    }
    s.to_string()
}
