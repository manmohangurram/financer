//! XLSX / XLS / ODS parsing via `calamine` (first sheet).

use super::{merge_tables, ParsedFile, Table};
use crate::error::{ApiError, Result};
use calamine::{open_workbook_from_rs, Data, Reader};

/// Parse every sheet of a workbook, merged into one `{headers, rows}`.
///
/// Statement exports often put one table per sheet (one per page) with the
/// header row only on the first — see `merge_tables`.
pub fn parse(bytes: &[u8]) -> Result<ParsedFile> {
    let cursor = std::io::Cursor::new(bytes);
    let mut workbook = open_workbook_from_rs::<calamine::Xlsx<_>, _>(cursor)
        .map_err(|e| ApiError::bad_request(format!("could not read workbook: {e}")))?;
    let sheets = workbook.sheet_names().clone();
    if sheets.is_empty() {
        return Err(ApiError::bad_request("workbook has no sheets"));
    }
    let mut tables: Vec<Table> = Vec::new();
    for sheet in sheets {
        let Ok(range) = workbook.worksheet_range(&sheet) else { continue };
        let mut rows: Vec<Vec<String>> = range.rows().map(|r| r.iter().map(cell).collect()).collect();
        rows.retain(|r| r.iter().any(|c| !c.is_empty()));
        if rows.is_empty() {
            continue;
        }
        let headers = rows.remove(0);
        if !headers.is_empty() {
            tables.push(Table { headers, rows });
        }
    }
    merge_tables(tables)
}

fn cell(c: &Data) -> String {
    match c {
        Data::Empty => String::new(),
        Data::String(s) => s.trim().to_string(),
        Data::Int(i) => i.to_string(),
        // Display prints 1500.0 as "1500" and 15.5 as "15.5".
        Data::Float(f) => f.to_string(),
        Data::Bool(b) => b.to_string(),
        Data::DateTime(d) => d.as_datetime().map_or_else(String::new, |dt| dt.format("%Y-%m-%d").to_string()),
        other => other.to_string(),
    }
}
