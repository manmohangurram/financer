//! File import: parse an uploaded CSV / XLSX / PDF into `{ headers, rows }`.
//!
//! The first row is the header row; the remaining rows are data. Parsing lives
//! here (not in the browser) so the API and MCP can reuse it.

pub mod csv;
pub mod investment;
pub mod mapping;
pub mod pdf;
pub mod store;
pub mod xlsx;

use crate::error::{ApiError, Result};

/// A parsed file: `headers` from the first row, `rows` are the data rows.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ParsedFile {
    pub headers: Vec<String>,
    pub rows: Vec<Vec<String>>,
}

/// Which parser to use, chosen by file extension.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FileKind {
    Csv,
    Xlsx,
    Pdf,
}

/// Pick a parser from the uploaded filename's extension (case-insensitive).
pub fn kind_for(filename: &str) -> Option<FileKind> {
    let ext = std::path::Path::new(filename).extension()?.to_str()?.to_ascii_lowercase();
    match ext.as_str() {
        "csv" => Some(FileKind::Csv),
        "xlsx" | "xls" | "ods" => Some(FileKind::Xlsx),
        "pdf" => Some(FileKind::Pdf),
        _ => None,
    }
}

/// Parse `bytes` with the parser for `kind`.
pub fn parse(bytes: &[u8], kind: FileKind) -> Result<ParsedFile> {
    match kind {
        FileKind::Csv => csv::parse(bytes),
        FileKind::Xlsx => xlsx::parse(bytes),
        FileKind::Pdf => pdf::parse(bytes),
    }
}

/// A single detected table: the first row is `headers`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct Table {
    pub headers: Vec<String>,
    pub rows: Vec<Vec<String>>,
}

/// Merge several tables into one `{headers, rows}` (statements emit one table
/// per page, with the header row only on the first page).
///
/// A table's first row is only a repeated header when it matches the merged
/// headers; otherwise it is real data and is kept as a data row. Rows are
/// padded/truncated to the header width.
pub(crate) fn merge_tables(mut tables: Vec<Table>) -> Result<ParsedFile> {
    if tables.is_empty() {
        return Err(ApiError::bad_request("no table found in the file"));
    }
    let mut merged = tables.remove(0);
    let width = merged.headers.len();
    for t in tables {
        let mut rows = t.rows;
        if t.headers != merged.headers {
            rows.insert(0, t.headers);
        }
        for mut row in rows {
            row.truncate(width);
            row.resize(width, String::new());
            merged.rows.push(row);
        }
    }
    Ok(ParsedFile { headers: merged.headers, rows: merged.rows })
}

/// Drop fully-blank rows, then split the first row off as headers.
pub(crate) fn split_rows(mut rows: Vec<Vec<String>>) -> Result<ParsedFile> {
    rows.retain(|r| r.iter().any(|c| !c.is_empty()));
    if rows.len() < 2 {
        return Err(ApiError::bad_request("need a header row and at least one data row"));
    }
    let headers = rows.remove(0);
    Ok(ParsedFile { headers, rows })
}
