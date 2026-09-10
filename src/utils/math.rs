//! Numeric helpers.

/// Round a monetary value to 2 decimals (Go's `round2f`/`cents`).
pub fn round2(v: f64) -> f64 {
    (v * 100.0).round() / 100.0
}
