//! Shares transaction types + cursor helpers + the backend-agnostic
//! `TransactionRepo` trait.

use async_trait::async_trait;
use base64::{engine::general_purpose::URL_SAFE as B64URL, Engine as _};
use serde_json::json;

use crate::error::Result;
use crate::timex::go_ts;

#[derive(
    Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize, strum::Display, strum::EnumString, utoipa::ToSchema, schemars::JsonSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum TransactionType {
    Debit,
    Credit,
}

/// A transaction as stored/returned by the repository layer.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Transaction {
    pub id: String,
    pub name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub clean_name: Option<String>,
    pub amount: f64,
    #[allow(clippy::struct_field_names)]
    #[serde(rename = "type")]
    pub transaction_type: TransactionType,
    pub occurred_at: String,
    pub account_id: String,
    pub created_at: String,
    pub external_id: Option<String>,
    pub transfer_linked: bool,
}

/// One transaction to create: the txn plus its category links.
pub struct CreateTransactionInput {
    pub txn: Transaction,
    pub category_ids: Vec<String>,
}

/// One transaction to update: the txn plus its full category set.
pub struct UpdateTransactionInput {
    pub txn: Transaction,
    pub category_ids: Vec<String>,
}

/// Filters for listing transactions, mirroring Go's `TransactionListFilter`.
#[derive(Debug, Default, Clone)]
pub struct TransactionListFilter {
    pub account_id: String,
    pub category_ids: Vec<String>,
    pub transaction_type: Option<TransactionType>,
    pub date_from: String,
    pub date_to: String,
    pub min_amount: f64,
    pub max_amount: f64,
    pub names: Vec<String>,
    pub page_size: i64,
    pub page_token: String,
    pub sort_by: String,
    pub sort_dir: String,
    pub offset: i64,
}

pub struct ListTransactionResult {
    pub rows: Vec<ListRow>,
    pub next_page_token: String,
    pub total_count: i64,
}

/// A listed row: the transaction plus its transfer link id and category ids.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ListRow {
    #[serde(flatten)]
    pub txn: Transaction,
    #[serde(default)]
    pub link_id: String,
    #[serde(default)]
    pub category_ids: Vec<String>,
}

pub struct SpendingFilter {
    pub granularity: String,
    pub from: String,
    pub to: String,
    pub account_id: String,
}
#[derive(Debug, Clone, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SpendingBucketRow {
    pub key: String,
    pub amount: f64,
}

#[derive(Debug)]
pub struct SpendingCategoryRow {
    pub id: String,
    pub name: String,
    pub debit: f64,
    pub credit: f64,
}

/// A decoded page cursor. `occurred_at` is kept in the stored Go-driver format.
pub struct TransactionCursor {
    pub occurred_at: String,
    pub id: String,
}

pub fn encode_transaction_cursor(occurred_at: &str, id: &str) -> String {
    let payload = json!({ "OccurredAt": crate::timex::ts_rfc3339(occurred_at), "ID": id }).to_string();
    B64URL.encode(payload.as_bytes())
}

pub fn decode_transaction_cursor(token: &str) -> Option<TransactionCursor> {
    let raw = B64URL.decode(token.as_bytes()).ok()?;
    let v: serde_json::Value = serde_json::from_slice(&raw).ok()?;
    let rfc = v["OccurredAt"].as_str()?;
    let stored = chrono::DateTime::parse_from_rfc3339(rfc)
        .map_or_else(|_| rfc.to_string(), |dt| go_ts(dt.with_timezone(&chrono::Utc)));
    Some(TransactionCursor { occurred_at: stored, id: v["ID"].as_str()?.to_string() })
}

pub struct CreateOutcome {
    pub inserted: std::collections::HashSet<String>,
    pub errors: Vec<String>,
}

#[async_trait]
pub trait TransactionRepo: Send + Sync {
    async fn get_by_id_for_transfer(&self, user_id: &str, id: &str) -> Result<Option<(String, f64, String)>>;
    async fn get_full(&self, user_id: &str, id: &str) -> Result<Option<Transaction>>;
    async fn find_transfer_counterpart(
        &self,
        user_id: &str,
        account_id: &str,
        transaction_type: TransactionType,
        amount: f64,
        around: &str,
    ) -> Result<Option<Transaction>>;
    async fn is_transfer_linked(&self, txn_id: &str) -> Result<bool>;
    async fn create(&self, user_id: &str, inputs: &[CreateTransactionInput]) -> Result<CreateOutcome>;
    async fn update(&self, user_id: &str, inputs: &[UpdateTransactionInput]) -> Result<Vec<String>>;
    async fn delete(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>>;
    async fn get_by_id_batch(&self, user_id: &str, ids: &[String]) -> Result<Vec<Transaction>>;
    async fn list(&self, user_id: &str, f: &TransactionListFilter) -> Result<ListTransactionResult>;
    async fn spending_buckets(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingBucketRow>>;
    async fn spending_categories(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingCategoryRow>>;
    async fn create_links(&self, user_id: &str, links: &[(String, String)]) -> Result<Vec<String>>;
    async fn delete_links(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>>;
    async fn is_transaction_linked(&self, txn_id: &str) -> Result<bool>;
}
