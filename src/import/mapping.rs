//! Turn parsed rows into transactions using the column mapping from the UI.
//!
//! Mirrors the client-side mapping that used to live in `csv.ts`:
//! `single` uses one amount column (optionally with a type column), `split`
//! uses separate debit/credit columns. Amounts may carry thousands separators
//! and a currency symbol.

use super::ParsedFile;
use crate::repo::traits::transaction::TransactionType;
use crate::service::transaction::TransactionReq;
use crate::utils::math::round2;
use crate::utils::timex::{go_ts, now_utc};
use chrono::{DateTime, NaiveDate, Utc};

/// Column index per field; a negative index (or absent) means "not mapped".
#[derive(Debug, Clone, Default, serde::Deserialize, utoipa::ToSchema)]
#[serde(rename_all = "camelCase", default)]
pub struct Mapping {
    pub date: i64,
    pub description: i64,
    pub amount: i64,
    #[serde(rename = "type")]
    pub type_col: i64,
    pub debit: i64,
    pub credit: i64,
}

impl Mapping {
    fn col(index: i64) -> Option<usize> {
        usize::try_from(index).ok()
    }
}

/// Build one create request per usable row, tagging each with `<hash>:<index>`
/// so re-importing the same file skips duplicates.
pub fn to_transactions(parsed: &ParsedFile, m: &Mapping, account_id: &str, hash: &str) -> Vec<TransactionReq> {
    let mut out = Vec::with_capacity(parsed.rows.len());
    for (i, row) in parsed.rows.iter().enumerate() {
        let cell = |idx: i64| Mapping::col(idx).and_then(|c| row.get(c)).map_or("", String::as_str);
        let (amount, transaction_type) = amount_and_type(cell(m.debit), cell(m.credit), cell(m.type_col), cell(m.amount));
        if amount <= 0.0 {
            continue;
        }
        let name = match cell(m.description).trim() {
            "" => "Imported".to_string(),
            s => s.to_string(),
        };
        let occurred_at = if cell(m.date).trim().is_empty() { now_utc() } else { build_date(cell(m.date)) };
        out.push(TransactionReq {
            id: uuid::Uuid::new_v4().to_string(),
            name,
            amount: round2(amount),
            transaction_type,
            occurred_at: go_ts(occurred_at),
            account_id: account_id.to_string(),
            category_ids: Vec::new(),
            external_id: Some(format!("{hash}:{i}")),
        });
    }
    out
}

/// Resolve the amount and type from the mapped columns.
fn amount_and_type(debit: &str, credit: &str, kind: &str, amount: &str) -> (f64, TransactionType) {
    let debit_v = parse_amount(debit);
    let credit_v = parse_amount(credit);
    let mut hint = if kind.is_empty() { None } else { parse_type_value(kind) };

    let amt;
    if debit_v != 0.0 || credit_v != 0.0 {
        if debit_v != 0.0 && credit_v != 0.0 {
            let is_credit = hint.map_or(credit_v >= debit_v, |t| t == TransactionType::Credit);
            amt = if is_credit { credit_v } else { debit_v };
            hint = Some(if is_credit { TransactionType::Credit } else { TransactionType::Debit });
        } else if credit_v != 0.0 {
            amt = credit_v;
            hint = Some(TransactionType::Credit);
        } else {
            amt = debit_v;
            hint = Some(TransactionType::Debit);
        }
    } else {
        amt = parse_amount(amount);
    }
    let transaction_type = hint.unwrap_or(if amt < 0.0 { TransactionType::Debit } else { TransactionType::Credit });
    (amt.abs(), transaction_type)
}

/// `CREDIT`/`DEBIT` hint from a free-text type column.
fn parse_type_value(v: &str) -> Option<TransactionType> {
    let s = v.to_lowercase();
    if ["credit", "cr", "deposit", "received", "income", "refund"].iter().any(|k| s.contains(k)) || s.contains('+') {
        return Some(TransactionType::Credit);
    }
    if ["debit", "dr", "withdraw", "payment", "expense", "fee"].iter().any(|k| s.contains(k)) || s.contains('-') {
        return Some(TransactionType::Debit);
    }
    None
}

/// Parse a money cell: strips thousands separators, spaces and a leading
/// currency symbol; blank/garbage is `0`.
pub(crate) fn parse_amount(cell: &str) -> f64 {
    let cleaned: String = cell
        .trim()
        .trim_start_matches(['₹', '$', '£', '€'])
        .chars()
        .filter(|c| !matches!(c, ',' | ' ' | '\u{a0}'))
        .collect();
    cleaned.parse::<f64>().unwrap_or(0.0)
}

/// Day-first `DD/MM/YYYY` (also `-` separated, 2- or 4-digit year), else ISO
/// `YYYY-MM-DD`, else "now" — same order as the old client parser.
pub(crate) fn build_date(cell: &str) -> DateTime<Utc> {
    if let Some(d) = parse_dmy(cell) {
        return d.and_hms_opt(0, 0, 0).unwrap().and_utc();
    }
    if let Ok(d) = NaiveDate::parse_from_str(cell.trim(), "%Y-%m-%d") {
        return d.and_hms_opt(0, 0, 0).unwrap().and_utc();
    }
    now_utc()
}

fn parse_dmy(cell: &str) -> Option<NaiveDate> {
    let parts: Vec<&str> = cell.trim().split(['/', '-']).collect();
    if parts.len() != 3 {
        return None;
    }
    let day: u32 = parts[0].parse().ok()?;
    let month: u32 = parts[1].parse().ok()?;
    let mut year: i32 = parts[2].parse().ok()?;
    if parts[2].len() <= 2 {
        year += 2000;
    }
    NaiveDate::from_ymd_opt(year, month, day)
}

#[cfg(test)]
mod tests {
    #![allow(clippy::float_cmp)]
    use super::*;

    fn parsed(rows: Vec<Vec<&str>>) -> ParsedFile {
        ParsedFile {
            headers: vec!["Date".into(), "Narration".into(), "Withdrawal".into(), "Deposit".into()],
            rows: rows.into_iter().map(|r| r.into_iter().map(str::to_string).collect()).collect(),
        }
    }

    #[test]
    fn split_columns_become_the_matching_type() {
        let m = Mapping { date: 0, description: 1, debit: 2, credit: 3, amount: -1, type_col: -1 };
        let p = parsed(vec![vec!["24/08/2021", "Coffee", "0.00", "4,000.00"], vec!["25/08/2021", "Rent", "599.00", "0.00"]]);
        let txns = to_transactions(&p, &m, "acc", "hash");
        assert_eq!(txns.len(), 2);
        assert_eq!(txns[0].transaction_type, TransactionType::Credit);
        assert_eq!(txns[0].amount, 4000.0, "thousands separators are stripped");
        assert_eq!(txns[1].transaction_type, TransactionType::Debit);
        assert_eq!(txns[0].external_id.as_deref(), Some("hash:0"));
        assert_eq!(txns[0].name, "Coffee");
    }

    #[test]
    fn single_amount_column_uses_the_type_hint() {
        let m = Mapping { date: 0, description: 1, amount: 2, type_col: -1, debit: -1, credit: -1 };
        let p = parsed(vec![vec!["24/08/2021", "Coffee", "-120.50"]]);
        let txns = to_transactions(&p, &m, "acc", "h");
        assert_eq!(txns[0].transaction_type, TransactionType::Debit);
        assert_eq!(txns[0].amount, 120.5);
    }

    #[test]
    fn dates_are_day_first() {
        assert_eq!(parse_dmy("31/12/2026"), NaiveDate::from_ymd_opt(2026, 12, 31));
        assert_eq!(parse_dmy("01-04-21"), NaiveDate::from_ymd_opt(2021, 4, 1));
        assert_eq!(parse_dmy("nonsense"), None);
    }
}
