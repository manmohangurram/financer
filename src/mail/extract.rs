//! Turns a fetched alert into a transaction draft.
//!
//! Deterministic first: bank alerts are templated, so a regex extractor covers
//! the common senders with output you can read and debug. The model-backed
//! extractor implements the same trait later.

use std::sync::LazyLock;

use regex::Regex;

#[derive(Debug, Clone, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Extraction {
    /// As written in the alert; the currency is the account's.
    pub amount: f64,
    pub direction: Direction,
    /// `YYYY-MM-DD`, or `None` when the alert names no date.
    pub occurred_at: Option<String>,
    /// Best-effort; `Unknown` when no "at/to/from" phrase was found.
    pub merchant: String,
    /// Trailing digits of the account or card named in the alert.
    pub account_hint: Option<String>,
    /// `1.0` only for a full match; anything lower must not auto-create.
    pub confidence: f32,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum Direction {
    Debit,
    Credit,
}

pub trait Extractor: Send + Sync {
    /// `None` when the mail is not a transaction (a statement, a promo, an OTP).
    /// Never a guess: a missing amount or direction means no extraction.
    fn extract(&self, subject: &str, body: &str) -> Option<Extraction>;
    fn source(&self) -> &'static str;
}

static DEBIT_VERB: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?i)\b(?:debited|withdrawn|withdrawal|spent|paid|purchase[d]?)\b")
        .expect("debit verb")
});
static CREDIT_VERB: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?i)\b(?:credited|deposited|received|refund(?:ed)?)\b").expect("credit verb")
});
/// Amounts appear before *or* after the verb depending on the bank, so the
static AMOUNT: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?i)(?:INR|Rs\.?|₹)\s*([\d,]+(?:\.\d{1,2})?)").expect("amount pattern")
});
static ACCOUNT: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?i)(?:\ba/c|\baccount|\bcard)\b[^.\n]{0,24}?([*xX]{0,4})(\d{4})\b")
        .expect("account pattern")
});
static MERCHANT: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?i)\b(?:at|to|towards|from)\s+([A-Za-z0-9][A-Za-z0-9 .&_'-]{1,40})")
        .expect("merchant pattern")
});
static DATE_DMY: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"\b(\d{1,2})[-/](\d{1,2})[-/](\d{2,4})\b").expect("date pattern"));

pub struct TemplateExtractor;

impl Extractor for TemplateExtractor {
    fn extract(&self, subject: &str, body: &str) -> Option<Extraction> {
        let text = format!("{subject}\n{body}");
        let debit = DEBIT_VERB.is_match(&text);
        let credit = CREDIT_VERB.is_match(&text);
        if debit == credit {
            // Both or neither: the direction is ambiguous, so do not guess.
            return None;
        }
        let direction = if debit {
            Direction::Debit
        } else {
            Direction::Credit
        };
        let amount = parse_amount(&AMOUNT.captures(&text)?[1])?;
        if amount <= 0.0 {
            return None;
        }

        let account_hint = ACCOUNT.captures(&text).map(|c| c[2].to_string());
        let merchant = MERCHANT
            .captures(&text)
            .map(|c| trim_merchant(&c[1]))
            .filter(|m| m.len() >= 2)
            .unwrap_or_else(|| "Unknown".to_string());
        let occurred_at = DATE_DMY
            .captures(&text)
            .and_then(|c| build_date(&c[1], &c[2], &c[3]));

        let confidence = match (account_hint.is_some(), occurred_at.is_some()) {
            (true, true) => 1.0,
            (true, false) | (false, true) => 0.6,
            (false, false) => 0.3,
        };

        Some(Extraction {
            amount,
            direction,
            occurred_at,
            merchant,
            account_hint,
            confidence,
        })
    }

    fn source(&self) -> &'static str {
        "template"
    }
}

/// Banks write "at SWIGGY on 12/09/2026"; keep only the merchant itself.
fn trim_merchant(raw: &str) -> String {
    static TAIL: LazyLock<Regex> =
        LazyLock::new(|| Regex::new(r"(?i)\s+(?:on|dated|dt|at|for)\s+.*$").expect("tail pattern"));
    let cut = TAIL.replace(raw, "");
    cut.trim().trim_end_matches(['.', ',', ' ']).to_string()
}

/// The account whose stored trailing digits match the alert, or `None` when
/// none matches or more than one does — two accounts ending 1234 is exactly
/// where guessing would post to the wrong one.
pub fn resolve_account(
    hint: Option<&str>,
    accounts: &[(String, Option<String>)],
) -> Option<String> {
    let hint = hint?;
    let mut matches = accounts
        .iter()
        .filter(|(_, ending)| ending.as_deref() == Some(hint))
        .map(|(id, _)| id.clone());
    let first = matches.next()?;
    matches.next().is_none().then_some(first)
}

fn parse_amount(raw: &str) -> Option<f64> {
    raw.replace(',', "").parse::<f64>().ok()
}

fn build_date(d: &str, m: &str, y: &str) -> Option<String> {
    let day: u32 = d.parse().ok()?;
    let month: u32 = m.parse().ok()?;
    let mut year: i32 = y.parse().ok()?;
    if year < 100 {
        year += 2000;
    }
    if !(1..=31).contains(&day) || !(1..=12).contains(&month) || !(2000..=2100).contains(&year) {
        return None;
    }
    Some(format!("{year:04}-{month:02}-{day:02}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn extract(subject: &str, body: &str) -> Option<Extraction> {
        TemplateExtractor.extract(subject, body)
    }

    #[test]
    fn debits_and_credits_with_every_field() {
        let debit = extract(
            "Transaction alert",
            "Your A/c XX1234 has been debited with INR 1,249.50 at SWIGGY on 12/09/2026.",
        )
        .expect("should extract");
        assert_eq!(debit.direction, Direction::Debit);
        assert_eq!(debit.amount, 1249.50);
        assert_eq!(debit.account_hint.as_deref(), Some("1234"));
        assert_eq!(debit.occurred_at.as_deref(), Some("2026-09-12"));
        assert_eq!(debit.confidence, 1.0);
        assert_eq!(debit.merchant, "SWIGGY");

        let credit = extract(
            "Credit alert",
            "INR 50,000.00 credited to your account 9876 from ACME PAYROLL on 01-10-2026",
        )
        .expect("should extract");
        assert_eq!(credit.direction, Direction::Credit);
        assert_eq!(credit.amount, 50_000.0);
        assert_eq!(credit.account_hint.as_deref(), Some("9876"));
    }

    #[test]
    fn a_missing_field_lowers_confidence_but_still_extracts() {
        let e = extract("Debited", "Rs. 500 debited at a shop").expect("still a transaction");
        assert_eq!(e.amount, 500.0);
        assert_eq!(e.confidence, 0.3, "no account and no date");
        assert_eq!(e.account_hint, None);
    }

    #[test]
    fn non_transactions_return_none() {
        assert!(extract("Your OTP", "Use 123456 to log in").is_none());
        assert!(extract("Statement ready", "Your statement for August is attached").is_none());
        assert!(extract("Alert", "INR 500 at some shop on 12/09/2026").is_none());
    }

    #[test]
    fn both_directions_is_ambiguous_and_skipped() {
        assert!(extract("Alert", "INR 500 debited and INR 200 credited").is_none());
    }

    #[test]
    fn dates_normalise_and_reject_impossible_values() {
        assert_eq!(build_date("1", "9", "26").as_deref(), Some("2026-09-01"));
        assert_eq!(
            build_date("31", "12", "2026").as_deref(),
            Some("2026-12-31")
        );
        assert_eq!(build_date("45", "1", "2026"), None);
        assert_eq!(build_date("1", "13", "2026"), None);
        assert_eq!(build_date("1", "1", "1899"), None);
    }
}
