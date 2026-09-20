//! Shared time helpers: the process-wide timezone (initialised once at startup)
//! and timestamp formatting/parsing matching Go's sqlite driver wire format.
//!
//! Storage is always UTC (`"YYYY-MM-DD HH:MM:SS +0000 UTC"`); the configured
//! timezone is only used to interpret those instants (day/month buckets, ranges,
//! `now()`).

use chrono::{DateTime, NaiveDate, NaiveDateTime, SecondsFormat, TimeZone, Utc};
use chrono_tz::Tz;
use std::sync::OnceLock;

/// The process timezone. Set once from config via [`init_tz`]; UTC until then.
static TIMEZONE: OnceLock<Tz> = OnceLock::new();

/// Initialise the process timezone from an IANA name (e.g. `Asia/Kolkata`).
/// Unknown/invalid names fall back to `UTC`. Call once at startup.
pub fn init_tz(name: &str) {
    let tz = name.trim().parse::<Tz>().unwrap_or(Tz::UTC);
    let _ = TIMEZONE.set(tz);
}

/// The configured timezone (`UTC` if never initialised).
pub fn tz() -> Tz {
    TIMEZONE.get().copied().unwrap_or(Tz::UTC)
}

/// Current instant in UTC — for storage.
pub fn now_utc() -> DateTime<Utc> {
    Utc::now()
}

/// `go_ts` for the current instant (what repos store in `created_at`).
pub fn now_go_ts() -> String {
    go_ts(Utc::now())
}

/// Format a timestamp the way the Go sqlite driver stores `time.Time` in the
/// shared DB file, so Rust and Go agree byte-for-byte on stored columns.
pub fn go_ts(utc: DateTime<Utc>) -> String {
    utc.format("%Y-%m-%d %H:%M:%S +0000 UTC").to_string()
}

/// Parse a stored timestamp (Go driver format, plain `YYYY-MM-DD HH:MM:SS`, or
/// RFC3339) into a UTC instant.
pub fn parse_utc(stored: &str) -> Option<DateTime<Utc>> {
    let s = stored.trim();
    // Go's driver stores "YYYY-MM-DD HH:MM:SS[.ffff] +0000 UTC"; the trailing
    // zone suffix isn't part of the wall clock, so drop it for naive parsing.
    let naive_str = s.split(' ').take(2).collect::<Vec<_>>().join(" ");
    for fmt in ["%Y-%m-%d %H:%M:%S%.f", "%Y-%m-%d %H:%M:%S"] {
        if let Ok(naive) = NaiveDateTime::parse_from_str(&naive_str, fmt) {
            return Some(naive.and_utc());
        }
    }
    DateTime::parse_from_rfc3339(s)
        .ok()
        .map(|dt| dt.with_timezone(&Utc))
}

/// Local bucket key for a stored UTC timestamp: `YYYY-MM` (month) or
/// `YYYY-MM-DD` (day), in the configured timezone.
pub fn local_key(stored: &str, granularity: &str) -> String {
    local_key_in(tz(), stored, granularity)
}

/// UTC instant of local midnight for `date` in the configured timezone. Used to
/// turn an inclusive local date range into UTC bounds.
pub fn local_date_start_utc(date: NaiveDate) -> Option<DateTime<Utc>> {
    let naive = date.and_hms_opt(0, 0, 0)?;
    tz().from_local_datetime(&naive)
        .earliest()
        .map(|dt| dt.with_timezone(&Utc))
}

/// `go_ts` of the local day start for `YYYY-MM-DD` (inclusive lower bound).
pub fn local_day_start(date: &str) -> Option<String> {
    NaiveDate::parse_from_str(date, "%Y-%m-%d")
        .ok()
        .and_then(local_date_start_utc)
        .map(go_ts)
}

/// `go_ts` of the local day start of the day AFTER `YYYY-MM-DD` (exclusive upper
/// bound, since the range end is inclusive).
pub fn local_day_end(date: &str) -> Option<String> {
    NaiveDate::parse_from_str(date, "%Y-%m-%d")
        .ok()
        .and_then(|d| local_date_start_utc(d + chrono::Duration::days(1)))
        .map(go_ts)
}

/// [`local_key`] against an explicit timezone (testable, no global state).
pub fn local_key_in(tz: Tz, stored: &str, granularity: &str) -> String {
    let fmt = if granularity == "month" {
        "%Y-%m"
    } else {
        "%Y-%m-%d"
    };
    match parse_utc(stored) {
        Some(dt) => dt.with_timezone(&tz).format(fmt).to_string(),
        None => String::new(),
    }
}

/// Format a stored UTC timestamp for the wire: the instant rendered in the
/// configured timezone (`2026-09-11T02:00:00+05:30`). Storage stays UTC; only
/// the return value is converted.
pub fn ts_rfc3339(stored: &str) -> String {
    ts_rfc3339_in(tz(), stored)
}

/// [`ts_rfc3339`] against an explicit timezone (testable, no global state).
pub fn ts_rfc3339_in(tz: Tz, stored: &str) -> String {
    if let Some(dt) = parse_utc(stored) {
        return dt
            .with_timezone(&tz)
            .to_rfc3339_opts(SecondsFormat::Secs, false);
    }
    stored.trim().to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn tz(name: &str) -> Tz {
        name.parse().unwrap()
    }

    #[test]
    fn parse_utc_accepts_stored_and_rfc3339() {
        assert_eq!(
            parse_utc("2026-09-10 20:30:00 +0000 UTC")
                .unwrap()
                .to_rfc3339(),
            "2026-09-10T20:30:00+00:00"
        );
        assert_eq!(
            parse_utc("2026-09-10T20:30:00Z").unwrap().to_rfc3339(),
            "2026-09-10T20:30:00+00:00"
        );
        assert!(parse_utc("not a date").is_none());
    }

    #[test]
    fn ts_rfc3339_renders_the_configured_zone() {
        // Storage stays UTC; the wire value carries the local offset.
        assert_eq!(
            ts_rfc3339_in(Tz::UTC, "2026-09-10 20:30:00 +0000 UTC"),
            "2026-09-10T20:30:00+00:00"
        );
        let ist = tz("Asia/Kolkata");
        assert_eq!(
            ts_rfc3339_in(ist, "2026-09-10 20:30:00 +0000 UTC"),
            "2026-09-11T02:00:00+05:30"
        );
    }

    #[test]
    fn local_key_shifts_across_midnight_for_ist() {
        let ist = tz("Asia/Kolkata");
        // 20:30 UTC on Sep 10 == 02:00 IST on Sep 11.
        assert_eq!(
            local_key_in(ist, "2026-09-10 20:30:00 +0000 UTC", "day"),
            "2026-09-11"
        );
        assert_eq!(
            local_key_in(ist, "2026-09-10 20:30:00 +0000 UTC", "month"),
            "2026-09"
        );
        // UTC bucket is unchanged.
        assert_eq!(
            local_key_in(Tz::UTC, "2026-09-10 20:30:00 +0000 UTC", "day"),
            "2026-09-10"
        );
    }

    #[test]
    fn local_key_is_dst_aware() {
        let ny = tz("America/New_York");
        // Mar 8 2026 is the US spring-forward day (EST -5 → EDT -4).
        assert_eq!(
            local_key_in(ny, "2026-03-08 07:30:00 +0000 UTC", "day"),
            "2026-03-08"
        );
        assert_eq!(
            local_key_in(ny, "2026-03-09 08:30:00 +0000 UTC", "day"),
            "2026-03-09"
        );
    }
}
