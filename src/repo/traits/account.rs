//! Shares account types + the backend-agnostic `AccountRepo` trait.

use async_trait::async_trait;

use crate::error::Result;

/// Account type. Wire value is the uppercase string (`CURRENT`/`CREDIT_CARD`).
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize, strum::Display, strum::EnumString, utoipa::ToSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum AccountType {
    Current,
    Savings,
    Loan,
    CreditCard,
}

/// A stored account row (serde wire shape: camelCase).
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AccountRow {
    pub id: String,
    pub bank_name: String,
    pub nickname: String,
    pub balance: f64,
    #[serde(rename = "type")]
    pub account_type: AccountType,
    pub created_at: String,
}

#[async_trait]
pub trait AccountRepo: Send + Sync {
    async fn create(&self, user_id: &str, bank_name: &str, nickname: &str, account_type: AccountType) -> Result<AccountRow>;
    async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<AccountRow>>;
    async fn update(&self, user_id: &str, id: &str, bank_name: &str, nickname: &str, account_type: AccountType) -> Result<Option<AccountRow>>;
    async fn delete(&self, user_id: &str, id: &str) -> Result<bool>;
    async fn list(&self, user_id: &str) -> Result<Vec<AccountRow>>;
    async fn update_balance(&self, account_id: &str, delta: f64) -> Result<()>;
    async fn apply_totals(&self, account_id: &str, credit: f64, debit: f64) -> Result<()>;
    async fn sum_balance(&self, user_id: &str) -> Result<f64>;
    async fn sum_totals(&self, user_id: &str) -> Result<(f64, f64)>;
}
