//! Shares mail-ingest types + the backend-agnostic `MailRepo` trait.

use async_trait::async_trait;

use crate::error::Result;

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MailAccountRow {
    pub id: String,
    pub user_id: String,
    pub label: String,
    pub imap_host: String,
    pub imap_port: i64,
    pub username: String,
    pub secret_enc: String,
    /// Exact addresses; the IMAP `FROM` search is built from these.
    pub sender_allowlist: Vec<String>,
    pub enabled: bool,
    pub last_polled_at: Option<String>,
    /// Last poll failure, empty on success.
    pub last_error: String,
    pub created_at: String,
}

#[derive(
    Debug,
    Clone,
    Copy,
    PartialEq,
    Eq,
    serde::Serialize,
    serde::Deserialize,
    strum::Display,
    strum::EnumString,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
pub enum MailStatus {
    Pending,
    Unparsed,
    Ignored,
    Auto,
    Rejected,
}

#[derive(Debug, Clone)]
pub struct NewMailAccount {
    pub label: String,
    pub imap_host: String,
    pub imap_port: i64,
    pub username: String,
    pub secret_enc: String,
    pub sender_allowlist: Vec<String>,
}

/// A message about to be stored. `message_id` is the idempotency key.
#[derive(Debug, Clone)]
pub struct NewMailMessage {
    pub mail_account_id: String,
    pub message_id: String,
    pub from_addr: String,
    pub received_at: Option<String>,
    pub subject: String,
    pub raw_text: String,
    pub status: MailStatus,
    pub parsed_json: Option<String>,
    pub confidence: f64,
    pub extractor_source: String,
}

/// A stored message as the review UI sees it: no secret, body optional
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MailMessageRow {
    pub id: String,
    pub message_id: String,
    pub from_addr: String,
    pub subject: String,
    pub received_at: Option<String>,
    pub status: MailStatus,
    /// Serialised [`crate::mail::extract::Extraction`]; `None` when unparsed.
    pub parsed_json: Option<String>,
    pub confidence: f64,
    /// Pruned once the row is resolved.
    pub raw_text: Option<String>,
    pub transaction_id: Option<String>,
}

#[async_trait]
pub trait MailRepo: Send + Sync {
    async fn create_account(&self, user_id: &str, input: &NewMailAccount)
    -> Result<MailAccountRow>;
    async fn list_accounts(&self, user_id: &str) -> Result<Vec<MailAccountRow>>;
    async fn delete_account(&self, user_id: &str, id: &str) -> Result<bool>;
    async fn list_pollable_accounts(&self) -> Result<Vec<MailAccountRow>>;
    async fn mark_polled(&self, account_id: &str, error: &str) -> Result<()>;
    async fn known_message_ids(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>>;
    /// Store a message; `false` when its `message_id` was already present.
    async fn insert_message(&self, user_id: &str, msg: &NewMailMessage) -> Result<bool>;
    async fn list_messages(
        &self,
        user_id: &str,
        status: Option<MailStatus>,
        limit: i64,
    ) -> Result<Vec<MailMessageRow>>;
    async fn get_message(&self, user_id: &str, id: &str) -> Result<Option<MailMessageRow>>;
    /// Record a decision: the new status, the values approved, and the
    /// transaction it produced. `false` when the row is not this user's.
    async fn resolve_message(
        &self,
        user_id: &str,
        id: &str,
        status: MailStatus,
        approved_json: Option<&str>,
        transaction_id: Option<&str>,
    ) -> Result<bool>;
    async fn sweep_bodies(&self) -> Result<u64>;
}
