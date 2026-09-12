//! Investment import: locate the holdings/transactions table inside a broker
//! export and map its columns onto create rows.

use crate::repo::traits::investment::InvestmentType;
use crate::service::investment::ImportRow;
use crate::utils::math::round2;
use crate::utils::timex::go_ts;

use super::mapping::{build_date, parse_amount};

/// Keyword groups used to locate the table (≥2 groups in one row = header row)
/// and, in the UI, to guess a column's field.
pub(crate) const KEYWORD_GROUPS: &[(&str, &[&str])] = &[
    ("symbol", &["symbol", "ticker", "code", "isin"]),
    ("name", &["name", "scheme", "fund", "stock", "company"]),
    ("quantity", &["units", "quantity", "qty", "unit"]),
    ("price", &["price", "nav", "rate", "amount", "amt", "value"]),
    ("side", &["side", "buy", "sell", "transaction type", "debit", "credit"]),
    ("date", &["date", "trade date", "transaction date"]),
];

/// A located table: its header row and the data rows below it.
pub struct LocatedTable {
    pub headers: Vec<String>,
    pub rows: Vec<Vec<String>>,
}

/// Find the holdings table inside a broker export. Sheets carry junk above and
/// below: the table starts at the first row matching ≥2 keyword groups, and ends
/// at the first sparse row (<2 populated cells).
pub fn locate_table(rows: &[Vec<String>]) -> Option<LocatedTable> {
    for (i, row) in rows.iter().enumerate() {
        let matched = KEYWORD_GROUPS
            .iter()
            .filter(|(_, kws)| row.iter().any(|c| kws.iter().any(|k| c.to_lowercase().contains(k))))
            .count();
        if matched < 2 {
            continue;
        }
        let data: Vec<Vec<String>> = rows[i + 1..]
            .iter()
            .take_while(|r| r.iter().filter(|c| !c.trim().is_empty()).count() >= 2)
            .cloned()
            .collect();
        if data.is_empty() {
            continue;
        }
        return Some(LocatedTable { headers: row.clone(), rows: data });
    }
    None
}

/// Column index per field; a negative index means "not mapped".
#[derive(Debug, Clone, Default, serde::Deserialize, utoipa::ToSchema)]
#[serde(rename_all = "camelCase", default)]
pub struct Mapping {
    pub symbol: i64,
    pub name: i64,
    #[serde(rename = "type")]
    pub type_col: i64,
    pub side: i64,
    pub quantity: i64,
    pub price: i64,
    pub date: i64,
}

/// Read a mapped cell, or `""` when the column is unmapped/missing.
fn cell(row: &[String], index: i64) -> &str {
    usize::try_from(index).ok().and_then(|c| row.get(c)).map_or("", String::as_str)
}

/// Map the located table onto create rows, tagging each with `<hash>:<index>`.
pub fn to_rows(table: &LocatedTable, m: &Mapping, hash: &str) -> Vec<ImportRow> {
    let mut out = Vec::with_capacity(table.rows.len());
    for (i, row) in table.rows.iter().enumerate() {
        let symbol = cell(row, m.symbol).trim();
        let quantity = parse_amount(cell(row, m.quantity));
        if symbol.is_empty() || quantity <= 0.0 {
            continue;
        }
        let name = match cell(row, m.name).trim() {
            "" => symbol.to_string(),
            s => s.to_string(),
        };
        let occurred_at = go_ts(build_date(cell(row, m.date)));
        out.push(ImportRow {
            symbol: symbol.to_string(),
            name,
            investment_type: resolve_type(cell(row, m.type_col)),
            side: resolve_side(cell(row, m.side)),
            quantity: round2(quantity),
            price: round2(parse_amount(cell(row, m.price))),
            occurred_at,
            external_id: format!("{hash}:{i}"),
        });
    }
    out
}

fn resolve_type(cell: &str) -> InvestmentType {
    if cell.to_lowercase().contains("mutual") || cell.to_lowercase().contains("fund") {
        InvestmentType::MutualFund
    } else {
        InvestmentType::Stock
    }
}

fn resolve_side(cell: &str) -> i64 {
    let s = cell.to_lowercase();
    if ["sell", "debit", "redeem"].iter().any(|k| s.contains(k)) || s.contains('-') {
        -1
    } else {
        1
    }
}

#[cfg(test)]
mod tests {
    #![allow(clippy::float_cmp)]
    use super::*;

    fn row(cells: &[&str]) -> Vec<String> {
        cells.iter().map(|c| (*c).to_string()).collect()
    }

    #[test]
    fn locates_the_table_past_junk() {
        let rows = vec![
            row(&["ICICI Direct", ""]),
            row(&["Report as on 01/04/2025", ""]),
            row(&["Symbol", "Scheme Name", "Units", "Nav", "Trade Date"]),
            row(&["INFY", "Infosys", "10", "1500.50", "01/04/2025"]),
            row(&["TCS", "Tata Consultancy", "5", "3800.00", "02/04/2025"]),
            row(&["Total", ""]),
        ];
        let t = locate_table(&rows).unwrap();
        assert_eq!(t.headers[0], "Symbol");
        assert_eq!(t.rows.len(), 2, "stops at the footer row");
    }

    #[test]
    fn maps_rows_and_skips_invalid_ones() {
        let t = locate_table(&[
            row(&["Symbol", "Scheme Name", "Units", "Nav", "Trade Date"]),
            row(&["INFY", "Infosys", "10", "1,500.50", "01/04/2025"]),
            row(&["", "no symbol", "5", "10", "01/04/2025"]),
            row(&["TCS", "Tata", "0", "10", "01/04/2025"]),
        ])
        .unwrap();
        let m = Mapping { symbol: 0, name: 1, quantity: 2, price: 3, date: 4, type_col: -1, side: -1 };
        let rows = to_rows(&t, &m, "hash");
        assert_eq!(rows.len(), 1);
        assert_eq!(rows[0].symbol, "INFY");
        assert_eq!(rows[0].quantity, 10.0);
        assert_eq!(rows[0].price, 1500.5);
        assert_eq!(rows[0].external_id, "hash:0");
    }
}
