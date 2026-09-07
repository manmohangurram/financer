//! MCP tool request/parameter structs. Each is the `#[tool]` argument payload,
//! deserialized by rmcp and validated against its JSON schema.

use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

use crate::repo::traits::rule::{RuleAction, RuleCondition, RuleLogic};

/// List transactions filter.
#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct ListTransactionsReq {
    #[serde(default)]
    pub category_ids: Vec<String>,
    pub ty: Option<String>,
}

/// Create a rule payload.
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

/// Update a rule payload (id + the settable fields).
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

/// Create categories payload.
#[derive(Serialize, Deserialize, JsonSchema)]
pub struct CreateCategoryReq {
    pub names: Vec<String>,
}

/// Delete by id payload.
#[derive(Serialize, Deserialize, JsonSchema)]
pub struct DeleteReq {
    pub id: String,
}
