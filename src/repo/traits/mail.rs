//! Shares mail-ingest types + the backend-agnostic `MailRepo` trait.

use async_trait::async_trait;

use crate::error::Result;

/// A linked mailbox. `secret_enc` is ciphertext, never a plain credential.
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
    pub sender_allowlist: Vec<String>,
    pub enabled: bool,
    pub last_polled_at: Option<String>,
    pub last_error: String,
    pub created_at: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, strum::Display, strum::EnumString)]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
pub enum MailStatus {
    /// Extracted, awaiting review.
    Pending,
    /// Sender matched but nothing extractable came out.
    Unparsed,
    /// Deliberately not a transaction.
    Ignored,
    /// Created by an `auto` rule.
    Auto,
    /// Rejected by the user; kept for audit.
    Rejected,
}

/// Fields a caller supplies when linking a mailbox.
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
}

#[async_trait]
pub trait MailRepo: Send + Sync {
    async fn create_account(&self, user_id: &str, input: &NewMailAccount)
    -> Result<MailAccountRow>;
    async fn list_accounts(&self, user_id: &str) -> Result<Vec<MailAccountRow>>;
    async fn get_account(&self, user_id: &str, id: &str) -> Result<Option<MailAccountRow>>;
    async fn delete_account(&self, user_id: &str, id: &str) -> Result<bool>;
    /// Every enabled mailbox, across users — what the poll tick iterates.
    async fn list_pollable_accounts(&self) -> Result<Vec<MailAccountRow>>;
    /// Record a poll attempt: `error` empty on success.
    async fn mark_polled(&self, account_id: &str, error: &str) -> Result<()>;
    /// Which of `ids` this user has already seen.
    async fn known_message_ids(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>>;
    /// Store a message; `false` when its `message_id` was already present.
    async fn insert_message(&self, user_id: &str, msg: &NewMailMessage) -> Result<bool>;
    /// Drop bodies of resolved rows; returns how many were cleared.
    async fn sweep_bodies(&self) -> Result<u64>;
}
