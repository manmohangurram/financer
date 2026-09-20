//! Shares rule types + the backend-agnostic `RuleRepo` trait.

use async_trait::async_trait;

use crate::error::Result;

#[derive(
    Debug,
    Clone,
    Copy,
    PartialEq,
    Eq,
    Default,
    serde::Serialize,
    serde::Deserialize,
    strum::Display,
    strum::EnumString,
    utoipa::ToSchema,
    schemars::JsonSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum RuleLogic {
    #[default]
    Or,
    And,
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
    utoipa::ToSchema,
    schemars::JsonSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum MatchField {
    Name,
    Amount,
    Type,
    Category,
    Account,
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
    utoipa::ToSchema,
    schemars::JsonSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum MatchOperator {
    Contains,
    StartsWith,
    EndsWith,
    Equals,
    GreaterThan,
    LessThan,
    Regex,
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
    utoipa::ToSchema,
    schemars::JsonSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum ActionOp {
    Rename,
    AddPrefix,
    AddSuffix,
}

#[allow(clippy::enum_variant_names)]
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
    utoipa::ToSchema,
    schemars::JsonSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum ActionType {
    SetName,
    SetCategory,
    SetTransferAccount,
}

/// One rule condition (wire shape).
#[derive(
    Debug, Clone, serde::Serialize, serde::Deserialize, utoipa::ToSchema, schemars::JsonSchema,
)]
pub struct RuleCondition {
    #[serde(rename = "matchField")]
    pub match_field: MatchField,
    pub operator: MatchOperator,
    pub pattern: String,
}

/// One rule output action (wire shape).
#[allow(clippy::struct_field_names)]
#[derive(
    Debug,
    Clone,
    Default,
    serde::Serialize,
    serde::Deserialize,
    utoipa::ToSchema,
    schemars::JsonSchema,
)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct RuleAction {
    #[serde(default)]
    pub set_name: String,
    #[serde(default)]
    pub set_name_op: Option<ActionOp>,
    #[serde(default)]
    pub set_category_id: String,
    #[serde(default)]
    pub set_transfer_account_id: String,
}

/// A rule as returned to the frontend (wire shape).
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize, utoipa::ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct Rule {
    pub id: String,
    pub name: String,
    pub priority: i64,
    pub logic: RuleLogic,
    pub conditions: Vec<RuleCondition>,
    pub actions: Vec<RuleAction>,
    #[serde(rename = "createdAt")]
    pub created_at: String,
}

#[derive(Debug, Clone)]
pub struct ConditionData {
    pub match_field: MatchField,
    pub operator: MatchOperator,
    pub pattern: String,
}

#[derive(Debug, Clone)]
pub struct OverlayRule {
    pub logic: RuleLogic,
    pub conditions: Vec<ConditionData>,
    pub actions: Vec<RuleAction>,
}

#[async_trait]
pub trait RuleRepo: Send + Sync {
    #[allow(clippy::too_many_arguments)]
    async fn create(
        &self,
        user_id: &str,
        name: &str,
        priority: i64,
        logic: RuleLogic,
        conditions: &[RuleCondition],
        actions: &[RuleAction],
    ) -> Result<Rule>;
    #[allow(clippy::too_many_arguments)]
    async fn update(
        &self,
        user_id: &str,
        id: &str,
        name: &str,
        priority: i64,
        logic: RuleLogic,
        conditions: &[RuleCondition],
        actions: &[RuleAction],
    ) -> Result<bool>;
    async fn delete(&self, user_id: &str, id: &str) -> Result<bool>;
    async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<Rule>>;
    async fn list(&self, user_id: &str) -> Result<Vec<Rule>>;
    async fn list_for_overlay(&self, user_id: &str) -> Result<Vec<OverlayRule>>;
}
