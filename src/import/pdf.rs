//! PDF parsing via `pdfsink-rs`: every table in the document, merged into one.
//!
//! Encrypted PDFs are rejected. Table detection uses `pdfsink-rs`'s default
//! line-based settings — the same ones the `pdfsink-rs` CLI uses.

use super::{ParsedFile, Table, merge_tables};
use crate::error::{ApiError, Result};
use pdfsink_rs::{TableSettings, open_pdf_bytes};

/// Parse the PDF's tables into one import-ready `{headers, rows}`.
pub fn parse(bytes: &[u8]) -> Result<ParsedFile> {
    merge_tables(extract_tables(bytes)?)
}

/// Extract every table in the document, in page order. Each table's first row
/// becomes its headers; blank header cells are named `column1`, `column2`, …
fn extract_tables(bytes: &[u8]) -> Result<Vec<Table>> {
    if is_encrypted(bytes) {
        return Err(ApiError::bad_request(
            "PDF is encrypted; password-protected PDFs are not supported",
        ));
    }
    let doc = open_pdf_bytes(bytes)
        .map_err(|e| ApiError::bad_request(format!("could not read PDF: {e}")))?;
    let mut tables = Vec::new();
    for number in 1..=doc.len() {
        let page = doc
            .page(number)
            .map_err(|e| ApiError::bad_request(format!("could not read PDF page {number}: {e}")))?;
        let found = page
            .extract_tables(TableSettings::default())
            .map_err(|e| ApiError::bad_request(format!("table extraction failed: {e}")))?;
        for table in found {
            let t = normalise(table);
            if !t.headers.is_empty() {
                tables.push(t);
            }
        }
    }
    if tables.is_empty() {
        return Err(ApiError::bad_request("no table found in the PDF"));
    }
    Ok(tables)
}

/// An encrypted PDF carries an `/Encrypt` entry in its trailer dictionary.
fn is_encrypted(bytes: &[u8]) -> bool {
    bytes.windows(b"/Encrypt".len()).any(|w| w == b"/Encrypt")
}

/// First row -> headers; remaining rows -> data, trimmed, newlines folded to
/// spaces and every row padded/truncated to the header width.
fn normalise(table: Vec<Vec<Option<String>>>) -> Table {
    let mut rows = table
        .into_iter()
        .map(|r| r.into_iter().map(cell).collect::<Vec<String>>());
    let mut headers = rows.next().unwrap_or_default();
    for (i, h) in headers.iter_mut().enumerate() {
        if h.is_empty() {
            *h = format!("column{}", i + 1);
        }
    }
    let width = headers.len();
    let data = rows
        .map(|mut r| {
            r.truncate(width);
            r.resize(width, String::new());
            r
        })
        .collect();
    Table {
        headers,
        rows: data,
    }
}

fn cell(c: Option<String>) -> String {
    c.unwrap_or_default().replace('\n', " ").trim().to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detects_the_encrypt_dictionary() {
        assert!(is_encrypted(b"trailer << /Encrypt 12 0 R >>"));
        assert!(!is_encrypted(b"trailer << /Size 12 >>"));
    }

    #[test]
    fn normalise_names_blank_headers_and_squares_rows() {
        let table = vec![
            vec![Some("Date".into()), Some("Amount".into()), None],
            vec![
                Some("1 Jan".into()),
                Some("10.00".into()),
                Some("a\nb".into()),
            ],
            vec![Some("2 Jan".into())], // short row is padded
        ];
        let out = normalise(table);
        assert_eq!(out.headers, vec!["Date", "Amount", "column3"]);
        assert_eq!(out.rows[0], vec!["1 Jan", "10.00", "a b"]);
        assert_eq!(out.rows[1], vec!["2 Jan", "", ""]);
    }

    #[test]
    fn extracts_every_table_from_a_pdf() {
        let tables = extract_tables(include_bytes!("fixtures/table.pdf")).unwrap();
        assert_eq!(tables.len(), 3, "fixture has three tables");
        assert_eq!(tables[0].headers, vec!["Date", "Description", "Amount"]);
        assert_eq!(tables[0].rows.len(), 2);
    }

    #[test]
    fn merges_all_tables_into_one() {
        let out = parse(include_bytes!("fixtures/table.pdf")).unwrap();
        assert_eq!(out.headers, vec!["Date", "Description", "Amount"]);
        // 2 rows from A + 1 from B (its repeated header is dropped) + 2 from C
        // (C has no header row, so its first row is kept as data).
        assert_eq!(out.rows.len(), 5);
        assert_eq!(out.rows[0], vec!["01/04/2021", "Coffee Shop", "120.50"]);
        assert_eq!(out.rows[2], vec!["03/04/2021", "Groceries", "800.00"]);
        assert_eq!(out.rows[4], vec!["05/04/2021", "Pharmacy", "150.00"]);
    }
}
