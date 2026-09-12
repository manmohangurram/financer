//! Temporary storage for uploaded import files.
//!
//! Files live in `<temp>/financer-uploads/<user_id>_<uuid>.<ext>`. Encoding the
//! owner in the filename is what scopes an id to a user — a later load for
//! someone else's id simply does not resolve, and there is no sidecar to write
//! or sweep.

use crate::error::{ApiError, Result};
use sha2::{Digest, Sha256};
use std::path::{Path, PathBuf};

fn dir() -> PathBuf {
    std::env::temp_dir().join("financer-uploads")
}

/// Persist an upload for `user_id`; returns the opaque id and the file's sha256.
pub fn save(user_id: &str, ext: &str, bytes: &[u8]) -> Result<(String, String)> {
    std::fs::create_dir_all(dir()).map_err(|e| ApiError::internal(format!("could not create upload dir: {e}")))?;
    let id = uuid::Uuid::new_v4().to_string();
    let path = dir().join(format!("{user_id}_{id}.{ext}"));
    std::fs::write(&path, bytes).map_err(|e| ApiError::internal(format!("could not store upload: {e}")))?;
    Ok((id, sha256_hex(bytes)))
}

/// Hex-encoded sha256 of the bytes — the re-import dedupe prefix.
pub fn sha256_hex(bytes: &[u8]) -> String {
    use std::fmt::Write;
    let digest = Sha256::digest(bytes);
    let mut out = String::with_capacity(64);
    for b in digest {
        let _ = write!(out, "{b:02x}");
    }
    out
}

/// The lowercase extension of a filename, defaulting to `bin`.
pub fn extension_of(filename: &str) -> String {
    Path::new(filename)
        .extension()
        .and_then(|e| e.to_str())
        .map_or_else(|| "bin".to_string(), str::to_ascii_lowercase)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn save_stores_the_file_under_the_owner_prefixed_name() {
        let (id, hash) = save("u-scope-test", "csv", b"a,b\n1,2\n").unwrap();
        assert_eq!(hash.len(), 64, "sha256 hex");
        let path = dir().join(format!("u-scope-test_{id}.csv"));
        assert!(path.exists(), "stored at the owner-prefixed path");
        let _ = std::fs::remove_file(path);
    }

    #[test]
    fn extension_is_lowercased() {
        assert_eq!(extension_of("Statement.XLSX"), "xlsx");
        assert_eq!(extension_of("noext"), "bin");
    }
}
