//! Per-user API-key service: create (show-once), list (masked), revoke, and
//! authenticate for the downstream MCP server (issue #61).
//!
//! The HTTP stage wires this in; dead-code is allowed until then.
#![allow(dead_code)]

use std::sync::Arc;

use base64::{engine::general_purpose::URL_SAFE as B64URL, Engine as _};
use sha2::{Digest, Sha256};

use crate::error::{ApiError, Result};
use crate::repo::traits::UserKeyRepo;
use crate::repo::traits::user_key::UserKeyRow;
use crate::utils::timex::go_ts;

const MAX_KEYS_PER_USER: i64 = 50;
const KEY_PREFIX: &str = "fin_live_";

/// A key returned to the UI: the masked row, plus the plaintext key on the
/// create response only (never stored or re-displayed).
#[derive(Debug, Clone, serde::Serialize, utoipa::ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct UserKeyView {
    pub id: String,
    pub name: String,
    #[serde(rename = "key")]
    pub plaintext: String,
    #[serde(rename = "keyPrefix")]
    pub key_prefix: String,
    #[serde(rename = "createdAt")]
    pub created_at: String,
    #[serde(rename = "expiresAt")]
    pub expires_at: Option<String>,
    #[serde(rename = "lastUsedAt")]
    pub last_used_at: Option<String>,
    /// `read` | `read_write`
    pub scope: String,
}

#[derive(Clone)]
pub struct UserKeyService {
    repo: Arc<dyn UserKeyRepo>,
}

impl UserKeyService {
    pub fn new(repo: Arc<dyn UserKeyRepo>) -> Self {
        Self { repo }
    }

    /// Create a key. `expires_in_days` of 0 leaves it non-expiring. `scope` is
    /// `read` (default) or `read_write`.
    pub async fn create(&self, user_id: &str, name: &str, expires_in_days: i64, scope: &str) -> Result<UserKeyView> {
        if self.repo.count(user_id).await? >= MAX_KEYS_PER_USER {
            return Err(ApiError::bad_request(format!("max {MAX_KEYS_PER_USER} keys per user")));
        }
        let scope = if scope.eq_ignore_ascii_case("read_write") { "read_write" } else { "read" };
        let (key, hash, prefix) = generate_key();
        let expires_at = if expires_in_days > 0 {
            Some(go_ts(crate::utils::timex::now_utc() + chrono::Duration::days(expires_in_days)))
        } else {
            None
        };
        let id = self.repo.create(user_id, name, &hash, &prefix, expires_at.clone(), scope).await?;
        Ok(UserKeyView {
            id,
            name: name.to_string(),
            plaintext: key,
            key_prefix: prefix,
            created_at: crate::utils::timex::now_go_ts(),
            expires_at,
            last_used_at: None,
            scope: scope.to_string(),
        })
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<UserKeyRow>> {
        self.repo.list(user_id).await
    }

    pub async fn delete(&self, user_id: &str, id: &str) -> Result<()> {
        if !self.repo.delete(user_id, id).await? {
            return Err(ApiError::not_found(format!("key {id} not found")));
        }
        Ok(())
    }

    /// Authenticate a raw key for the MCP server. Returns `(user_id, scope)`.
    /// Rejects unknown and expired keys.
    pub async fn authenticate(&self, key: &str) -> Result<(String, String)> {
        let hash = hash_key(key);
        let Some((user_id, id, scope)) = self.repo.by_key_hash(&hash).await? else {
            return Err(ApiError::unauthorized("invalid API key"));
        };
        if let Some(expires) = self.repo.get_by_id(&user_id, &id).await?.and_then(|r| r.expires_at) {
            if is_expired(&expires) {
                return Err(ApiError::unauthorized("API key expired"));
            }
        }
        let _ = self.repo.touch_last_used(&id).await;
        Ok((user_id, scope))
    }
}

/// Generate `fin_live_<base64url(32 bytes)>` and its SHA-256 hex hash + display prefix.
fn generate_key() -> (String, String, String) {
    use rand::RngCore;

    let mut bytes = [0u8; 32];
    rand::rngs::OsRng.fill_bytes(&mut bytes);
    let b64 = B64URL.encode(bytes);
    let key = format!("{KEY_PREFIX}{b64}");
    let hash = hash_key(&key);
    let prefix = key.chars().take(14).collect(); // e.g. fin_live_<8 chars>
    (key, hash, prefix)
}

fn hash_key(key: &str) -> String {
    use std::fmt::Write;

    let digest = Sha256::digest(key.as_bytes());
    let mut out = String::with_capacity(digest.len() * 2);
    for b in digest {
        let _ = write!(out, "{b:02x}");
    }
    out
}

fn is_expired(expires_at: &str) -> bool {
    // Stored as go_ts "YYYY-MM-DD HH:MM:SS +0000 UTC"; compare against now.
    let now = crate::utils::timex::now_go_ts();
    expires_at < now.as_str()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn generate_key_shape_and_hash() {
        let (key, hash, prefix) = generate_key();
        assert!(key.starts_with(KEY_PREFIX));
        assert_eq!(hash_key(&key), hash);
        assert!(prefix.len() <= 14);
        assert_eq!(prefix, &key[..prefix.len()]);
    }

    #[test]
    fn expiry_parses_go_ts_string() {
        let past = go_ts(crate::utils::timex::now_utc() - chrono::Duration::days(1));
        let future = go_ts(crate::utils::timex::now_utc() + chrono::Duration::days(1));
        assert!(is_expired(&past));
        assert!(!is_expired(&future));
    }
}
