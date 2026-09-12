//! CSV parsing via the `csv` crate.

use super::{ParsedFile, split_rows};
use crate::error::{ApiError, Result};

/// Parse CSV bytes; the first record is the header row.
pub fn parse(bytes: &[u8]) -> Result<ParsedFile> {
    let mut reader = csv::ReaderBuilder::new()
        .has_headers(false)
        .flexible(true)
        .from_reader(bytes);
    let mut rows: Vec<Vec<String>> = Vec::new();
    for record in reader.records() {
        let record =
            record.map_err(|e| ApiError::bad_request(format!("could not read CSV: {e}")))?;
        rows.push(record.iter().map(|c| c.trim().to_string()).collect());
    }
    split_rows(rows)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_headers_and_rows() {
        let out =
            parse(b"Date,Description,Amount\n01/04/2021,Coffee,120.50\n02/04/2021,Salary,5000\n")
                .unwrap();
        assert_eq!(out.headers, vec!["Date", "Description", "Amount"]);
        assert_eq!(out.rows.len(), 2);
        assert_eq!(out.rows[0], vec!["01/04/2021", "Coffee", "120.50"]);
    }

    #[test]
    fn rejects_a_header_only_file() {
        assert!(parse(b"Date,Amount\n").is_err());
    }
}
