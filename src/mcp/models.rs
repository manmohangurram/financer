//! MCP tool request/parameter structs. Each is the `#[tool]` argument payload,
//! deserialized by rmcp and validated against its JSON schema.

use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

use crate::repo::traits::rule::{RuleAction, RuleCondition, RuleLogic};

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct ListTransactionsReq {
    #[serde(default)]
    pub category_ids: Vec<String>,
    pub ty: Option<String>,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct RuleReq {
    pub name: String,
    #[serde(default)]
    pub priority: i64,
    #[serde(default)]
    pub logic: RuleLogic,
    #[serde(default)]
    pub conditions: Vec<RuleCondition>,
    #[serde(default)]
    pub actions: Vec<RuleAction>,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct UpdateRuleReq {
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub priority: i64,
    #[serde(default)]
    pub logic: RuleLogic,
    #[serde(default)]
    pub conditions: Vec<RuleCondition>,
    #[serde(default)]
    pub actions: Vec<RuleAction>,
}

#[derive(Serialize, Deserialize, JsonSchema)]
pub struct CreateCategoryReq {
    pub names: Vec<String>,
}

#[derive(Serialize, Deserialize, JsonSchema)]
pub struct DeleteReq {
    pub id: String,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct TxnPayload {
    #[serde(default)]
    pub id: String,
    pub name: String,
    pub amount: f64,
    #[serde(rename = "type")]
    pub transaction_type: crate::repo::traits::transaction::TransactionType,
    #[serde(default)]
    pub occurred_at: String,
    #[serde(default)]
    pub account_id: String,
    #[serde(default)]
    pub category_ids: Vec<String>,
    #[serde(default)]
    pub external_id: Option<String>,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct CreateTransactionsReq {
    pub transactions: Vec<TxnPayload>,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct UpdateTransactionsReq {
    pub transactions: Vec<TxnPayload>,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct DeleteTransactionsReq {
    pub ids: Vec<String>,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct CreateAccountReq {
    pub bank_name: String,
    #[serde(default)]
    pub nickname: String,
    #[serde(rename = "type")]
    pub account_type: crate::repo::traits::account::AccountType,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct UpdateAccountReq {
    pub id: String,
    #[serde(default)]
    pub bank_name: String,
    #[serde(default)]
    pub nickname: String,
    #[serde(rename = "type")]
    pub account_type: crate::repo::traits::account::AccountType,
}

/// One transfer link (debit/credit pair).
#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct TransferLink {
    #[serde(default)]
    pub debit_transaction_id: String,
    #[serde(default)]
    pub credit_transaction_id: String,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct TransfersReq {
    #[serde(default)]
    pub ids: Vec<String>,
    #[serde(default)]
    pub links: Vec<TransferLink>,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct CreateTransferCounterpartReq {
    #[serde(default)]
    pub transaction_id: String,
    #[serde(default)]
    pub to_account_id: String,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct PreviewRuleReq {
    #[serde(default)]
    pub logic: crate::repo::traits::rule::RuleLogic,
    #[serde(default)]
    pub conditions: Vec<crate::repo::traits::rule::RuleCondition>,
    #[serde(default)]
    pub limit: i64,
}

#[derive(Serialize, Deserialize, JsonSchema)]
pub struct RunRuleReq {
    pub id: String,
}

/// One category update item.
#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct CatItem {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct CategoriesReq {
    #[serde(default)]
    pub ids: Vec<String>,
    #[serde(default)]
    pub categories: Vec<CatItem>,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct InvestmentReq {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub symbol: String,
    #[serde(default)]
    pub name: String,
    #[serde(rename = "investmentType")]
    pub investment_type: crate::repo::traits::investment::InvestmentType,
    #[serde(default)]
    pub manual_nav: f64,
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct LotInputReq {
    #[serde(default)]
    pub investment_id: String,
    #[serde(default)]
    pub lot_id: String,
    #[serde(default)]
    pub side: Option<i64>,
    #[serde(default)]
    pub quantity: f64,
    #[serde(default)]
    pub price: f64,
    #[serde(default)]
    pub occurred_at: String,
}
