//! Shares user-key types + the backend-agnostic `UserKeyRepo` trait.

use async_trait::async_trait;

use crate::error::Result;

/// A per-user API key row (masked view for the UI; never contains the secret).
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize, utoipa::ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct UserKeyRow {
    pub id: String,
    pub name: String,
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

#[async_trait]
pub trait UserKeyRepo: Send + Sync {
    /// Insert a key; returns the row id.
    async fn create(
        &self,
        user_id: &str,
        name: &str,
        key_hash: &str,
        key_prefix: &str,
        expires_at: Option<String>,
        scope: &str,
    ) -> Result<String>;
    /// List a user's keys (masked), newest first.
    async fn list(&self, user_id: &str) -> Result<Vec<UserKeyRow>>;
    /// Count a user's keys (for the max-50 cap).
    async fn count(&self, user_id: &str) -> Result<i64>;
    /// Fetch one key by id scoped to a user.
    async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<UserKeyRow>>;
    /// Delete one key scoped to a user; returns whether a row was removed.
    async fn delete(&self, user_id: &str, id: &str) -> Result<bool>;
    /// Auth lookup by key hash; returns `(user_id, key id, scope)`.
    async fn by_key_hash(&self, key_hash: &str) -> Result<Option<(String, String, String)>>;
    /// Record a successful auth (update `last_used_at`).
    async fn touch_last_used(&self, id: &str) -> Result<()>;
}
