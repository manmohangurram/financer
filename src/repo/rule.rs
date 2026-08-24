//! Rules repository — enums, `SurrealDB` storage, and read-time overlay data,
//! mirroring the Go backend's `repository/rule.go` + `services/rule.go` match
//! engine. Conditions and actions are stored as JSON on the rule record.

use surrealdb::Connection;
use utoipa::ToSchema;

use crate::error::Result;
use crate::repo::surreal::{rid, take_json, DbClient, RepoConn};
use crate::timex::{go_ts, ts_rfc3339};

/// Rule logic: how conditions combine. Wire + DB string.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize, strum::Display, strum::EnumString, ToSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum RuleLogic {
    Or,
    And,
}

/// Condition match field. Wire + DB string.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize, strum::Display, strum::EnumString, ToSchema,
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

/// Condition operator. Wire + DB string.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize, strum::Display, strum::EnumString, ToSchema,
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

/// Name output operation. Wire + DB string.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize, strum::Display, strum::EnumString, ToSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum ActionOp {
    Rename,
    AddPrefix,
    AddSuffix,
}

/// The action's kind. Variants mirror Go's SET_* names (the wire/db contract).
#[allow(clippy::enum_variant_names)]
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize, strum::Display, strum::EnumString, ToSchema,
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
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize, ToSchema)]
pub struct RuleCondition {
    #[serde(rename = "matchField")]
    pub match_field: MatchField,
    pub operator: MatchOperator,
    pub pattern: String,
}

/// One rule output action (wire shape).
#[allow(clippy::struct_field_names)]
#[derive(Debug, Clone, Default, serde::Serialize, serde::Deserialize, ToSchema)]
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

/// A rule as returned to the frontend (wire shape). Also deserializable for
/// the `SurrealDB` JSON round-trip.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize, ToSchema)]
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

/// Conditions flattened for the match engine (Go's `RuleConditionData`).
#[derive(Debug, Clone)]
pub struct ConditionData {
    pub match_field: MatchField,
    pub operator: MatchOperator,
    pub pattern: String,
}

/// A rule loaded for read-time overlay, priority-ordered.
#[derive(Debug, Clone)]
pub struct OverlayRule {
    pub logic: RuleLogic,
    pub conditions: Vec<ConditionData>,
    pub actions: Vec<RuleAction>,
}

#[derive(Clone)]
pub struct RuleRepo<C: Connection = DbClient> {
    db: RepoConn<C>,
}

impl<C: Connection> RuleRepo<C> {
    pub fn new(db: RepoConn<C>) -> Self {
        Self { db }
    }

    /// Create a rule with its conditions and actions stored as JSON.
    pub async fn create(
        &self,
        user_id: &str,
        name: &str,
        priority: i64,
        logic: RuleLogic,
        conditions: &[RuleCondition],
        actions: &[RuleAction],
    ) -> Result<Rule> {
        let now = go_ts(chrono::Utc::now());
        let mut res = self
            .db
            .query(
                "CREATE rule CONTENT {
                    user: $uid, name: $name, priority: $priority, logic: $logic,
                    conditions: $conditions, actions: $actions, createdAt: $created
                } RETURN meta::id(id) AS id, name, priority, logic, conditions, actions, createdAt",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("name", name.to_string()))
            .bind(("priority", priority))
            .bind(("logic", logic.to_string()))
            .bind(("conditions", serde_json::to_value(conditions).unwrap()))
            .bind(("actions", serde_json::to_value(actions).unwrap()))
            .bind(("created", now.clone()))
            .await?
            .check()?;
        let mut rule = take_json::<Rule>(&mut res, 0)?
            .into_iter()
            .next()
            .ok_or_else(|| crate::error::ApiError::internal("rule create returned no row"))?;
        rule.created_at = ts_rfc3339(&rule.created_at);
        Ok(rule)
    }

    #[allow(clippy::too_many_arguments)]
    pub async fn update(
        &self,
        user_id: &str,
        id: &str,
        name: &str,
        priority: i64,
        logic: RuleLogic,
        conditions: &[RuleCondition],
        actions: &[RuleAction],
    ) -> Result<bool> {
        let mut res = self
            .db
            .query(
                "UPDATE $rid SET
                    name = IF $name != '' THEN $name ELSE name END,
                    priority = $priority,
                    logic = $logic,
                    conditions = $conditions,
                    actions = $actions
                 WHERE user = $uid RETURN meta::id(id) AS id",
            )
            .bind(("rid", rid("rule", id)))
            .bind(("uid", rid("user", user_id)))
            .bind(("name", name.to_string()))
            .bind(("priority", priority))
            .bind(("logic", logic.to_string()))
            .bind(("conditions", serde_json::to_value(conditions).unwrap()))
            .bind(("actions", serde_json::to_value(actions).unwrap()))
            .await?;
        Ok(!take_json::<serde_json::Value>(&mut res, 0)?.is_empty())
    }

    pub async fn delete(&self, user_id: &str, id: &str) -> Result<bool> {
        let mut res = self
            .db
            .query("DELETE $rid WHERE user = $uid RETURN BEFORE")
            .bind(("rid", rid("rule", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(!take_json::<serde_json::Value>(&mut res, 0)?.is_empty())
    }

    pub async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<Rule>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, name, priority, logic, conditions, actions, createdAt
                 FROM rule WHERE id = $rid AND user = $uid LIMIT 1",
            )
            .bind(("rid", rid("rule", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        let mut rule = take_json::<Rule>(&mut res, 0)?.into_iter().next();
        if let Some(r) = &mut rule {
            r.created_at = ts_rfc3339(&r.created_at);
        }
        Ok(rule)
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<Rule>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, name, priority, logic, conditions, actions, createdAt
                 FROM rule WHERE user = $uid ORDER BY priority DESC, name ASC",
            )
            .bind(("uid", rid("user", user_id)))
            .await?;
        let mut rules = take_json::<Rule>(&mut res, 0)?;
        for r in &mut rules {
            r.created_at = ts_rfc3339(&r.created_at);
        }
        Ok(rules)
    }

    /// Rules for read-time overlay, priority DESC (highest first).
    pub async fn list_for_overlay(&self, user_id: &str) -> Result<Vec<OverlayRule>> {
        let rules = self.list(user_id).await?;
        Ok(rules
            .into_iter()
            .map(|r| OverlayRule {
                logic: r.logic,
                conditions: r.conditions.iter().map(cond_to_data).collect(),
                actions: r.actions,
            })
            .collect())
    }
}

fn cond_to_data(c: &RuleCondition) -> ConditionData {
    ConditionData { match_field: c.match_field, operator: c.operator, pattern: c.pattern.clone() }
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;
    use surrealdb::Surreal;

    use super::*;
    use crate::surreal_db;

    async fn repo() -> RuleRepo<surrealdb::engine::local::Db> {
        let db = Arc::new(surreal_db::connect_mem().await.unwrap());
        db.query("CREATE user CONTENT { email: 'u1@x.com', passwordHash: 'h', name: 'u1' }")
            .await
            .unwrap()
            .check()
            .unwrap();
        RuleRepo::new(db)
    }

    fn cond() -> RuleCondition {
        RuleCondition {
            match_field: "NAME".parse().unwrap(),
            operator: "CONTAINS".parse().unwrap(),
            pattern: "netflix".to_string(),
        }
    }

    #[tokio::test]
    async fn rule_crud_roundtrip() {
        let repo = repo().await;
        let action = RuleAction {
            set_name: "Netflix Sub".to_string(),
            set_name_op: Some(ActionOp::Rename),
            ..Default::default()
        };
        let created = repo.create("u1", "Renamer", 5, RuleLogic::And, &[cond()], &[action.clone()]).await.unwrap();
        assert_eq!(created.name, "Renamer");
        assert_eq!(created.logic, RuleLogic::And);
        assert_eq!(created.conditions.len(), 1);
        assert_eq!(created.actions.len(), 1);

        let fetched = repo.get_by_id("u1", &created.id).await.unwrap().unwrap();
        assert_eq!(fetched.conditions[0].pattern, "netflix");
        assert_eq!(fetched.actions[0].set_name, "Netflix Sub");

        let updated = repo.update("u1", &created.id, "Renamer2", 9, RuleLogic::Or, &[cond()], &[]).await.unwrap();
        assert!(updated);
        let after = repo.get_by_id("u1", &created.id).await.unwrap().unwrap();
        assert_eq!(after.name, "Renamer2");
        assert_eq!(after.logic, RuleLogic::Or);
        assert!(after.actions.is_empty());

        assert!(repo.delete("u1", &created.id).await.unwrap());
        assert!(repo.get_by_id("u1", &created.id).await.unwrap().is_none());

        // cross-user
        let r2 = repo.create("u1", "Other", 1, RuleLogic::Or, &[], &[]).await.unwrap();
        assert!(repo.get_by_id("u2", &r2.id).await.unwrap().is_none());
    }
}
