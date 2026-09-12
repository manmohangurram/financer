//! Shares the user row type + the backend-agnostic `UserRepo` trait.

use async_trait::async_trait;

use crate::error::Result;

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UserRow {
    pub id: String,
    pub email: String,
    pub name: Option<String>,
    pub password_hash: Option<String>,
    pub token_version: i64,
    pub avatar_url: Option<String>,
}

#[async_trait]
pub trait UserRepo: Send + Sync {
    async fn create(&self, email: &str, password_hash: &str, name: &str) -> Result<String>;
    async fn by_email(&self, email: &str) -> Result<Option<UserRow>>;
    async fn by_id(&self, id: &str) -> Result<Option<UserRow>>;
    async fn update_profile(&self, id: &str, name: &str, email: &str, avatar_url: &str) -> Result<()>;
    async fn change_password(&self, id: &str, password_hash: &str) -> Result<i64>;
    async fn logout_all(&self, id: &str) -> Result<()>;
}
