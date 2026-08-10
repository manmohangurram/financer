//! Rules repository — enums, SQL, and read-time overlay data, mirroring the Go
//! backend's `repository/rule.go` + `services/rule.go` match engine.

use serde::{Deserialize, Serialize};
use sqlx::SqlitePool;
use uuid::Uuid;

use crate::error::Result;
use crate::timex::ts_rfc3339;
use crate::timex::go_ts;

/// Rule logic: how conditions combine. Wire + DB string.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, strum::Display, strum::EnumString,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
pub enum RuleLogic {
    Or,
    And,
}

/// Condition match field. Wire + DB string (Go kept `RULE_MATCH_FIELD_*`;
/// renamed to clean uppercase).
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, strum::Display, strum::EnumString,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
pub enum MatchField {
    Name,
    Amount,
    Type,
    Category,
    Account,
}

/// Condition operator. Wire + DB string.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, strum::Display, strum::EnumString,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
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
    Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, strum::Display, strum::EnumString,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
pub enum ActionOp {
    Rename,
    AddPrefix,
    AddSuffix,
}

/// The action's kind, stored in the `action_type` column. Variants mirror
/// Go's SET_* names (the wire/db contract).
#[allow(clippy::enum_variant_names)]
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, strum::Display, strum::EnumString,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
pub enum ActionType {
    SetName,
    SetCategory,
    SetTransferAccount,
}

/// One rule condition (wire shape).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RuleCondition {
    #[serde(rename = "matchField")]
    pub match_field: MatchField,
    pub operator: MatchOperator,
    pub pattern: String,
}

/// One rule output action (wire shape). The `set_` prefix matches the wire
/// keys (`setName`, `setCategoryId`, ...).
#[allow(clippy::struct_field_names)]
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
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
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
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
pub struct RuleRepo {
    pub pool: SqlitePool,
}

impl RuleRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    /// Create a rule with its conditions and actions in one tx.
    pub async fn create(
        &self,
        user_id: &str,
        name: &str,
        priority: i64,
        logic: RuleLogic,
        conditions: &[RuleCondition],
        actions: &[RuleAction],
    ) -> Result<Rule> {
        let id = Uuid::new_v4().to_string();
        let now = go_ts(chrono::Utc::now());
        let mut tx = self.pool.begin().await?;
        sqlx::query("INSERT INTO rules (id, name, priority, logic, created_at, user_id) VALUES (?, ?, ?, ?, ?, ?)")
            .bind(&id)
            .bind(name)
            .bind(priority)
            .bind(logic.to_string())
            .bind(&now)
            .bind(user_id)
            .execute(&mut *tx)
            .await?;
        insert_conditions(&mut tx, &id, conditions).await?;
        insert_actions(&mut tx, &id, actions).await?;
        tx.commit().await?;
        Ok(Rule { id, name: name.to_string(), priority, logic, conditions: conditions.to_vec(), actions: actions.to_vec(), created_at: ts_rfc3339(&now) })
    }

    pub async fn update(
        &self,
        id: &str,
        name: &str,
        priority: i64,
        logic: RuleLogic,
        conditions: &[RuleCondition],
        actions: &[RuleAction],
    ) -> Result<bool> {
        let mut tx = self.pool.begin().await?;
        let result = sqlx::query("UPDATE rules SET name = COALESCE(NULLIF(?, ''), name), priority = ?, logic = ? WHERE id = ?")
            .bind(name)
            .bind(priority)
            .bind(logic.to_string())
            .bind(id)
            .execute(&mut *tx)
            .await?;
        if result.rows_affected() == 0 {
            return Ok(false);
        }
        sqlx::query("DELETE FROM rule_conditions WHERE rule_id = ?").bind(id).execute(&mut *tx).await?;
        sqlx::query("DELETE FROM rule_actions WHERE rule_id = ?").bind(id).execute(&mut *tx).await?;
        insert_conditions(&mut tx, id, conditions).await?;
        insert_actions(&mut tx, id, actions).await?;
        tx.commit().await?;
        Ok(true)
    }

    pub async fn delete(&self, id: &str) -> Result<bool> {
        let result = sqlx::query("DELETE FROM rules WHERE id = ?").bind(id).execute(&self.pool).await?;
        Ok(result.rows_affected() > 0)
    }

    pub async fn get_by_id(&self, id: &str) -> Result<Option<Rule>> {
        let row = sqlx::query_as::<_, RawRule>("SELECT id, name, priority, logic, created_at FROM rules WHERE id = ?")
            .bind(id)
            .fetch_optional(&self.pool)
            .await?;
        let Some(raw) = row else { return Ok(None) };
        let conditions = self.conditions_for_rule(id).await?;
        let actions = self.actions_for_rule(id).await?;
        Ok(Some(raw.into_rule(conditions, actions)))
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<Rule>> {
        let rows = sqlx::query_as::<_, RawRule>(
            "SELECT id, name, priority, logic, created_at FROM rules WHERE user_id = ? ORDER BY priority DESC, name ASC",
        )
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        let mut rules = Vec::new();
        for raw in rows {
            let conditions = self.conditions_for_rule(&raw.id).await?;
            let actions = self.actions_for_rule(&raw.id).await?;
            rules.push(raw.into_rule(conditions, actions));
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

    async fn conditions_for_rule(&self, rule_id: &str) -> Result<Vec<RuleCondition>> {
        let rows = sqlx::query_as::<_, RawCondition>(
            "SELECT match_field, operator, pattern FROM rule_conditions WHERE rule_id = ?",
        )
        .bind(rule_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    async fn actions_for_rule(&self, rule_id: &str) -> Result<Vec<RuleAction>> {
        let rows = sqlx::query_as::<_, RawAction>(
            "SELECT action_type, name_op, value, category_id, transfer_account_id FROM rule_actions WHERE rule_id = ? ORDER BY rowid",
        )
        .bind(rule_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }
}

fn cond_to_data(c: &RuleCondition) -> ConditionData {
    ConditionData { match_field: c.match_field, operator: c.operator, pattern: c.pattern.clone() }
}

async fn insert_conditions(tx: &mut sqlx::Transaction<'_, sqlx::Sqlite>, rule_id: &str, conditions: &[RuleCondition]) -> Result<()> {
    for cond in conditions {
        sqlx::query("INSERT INTO rule_conditions (id, rule_id, match_field, operator, pattern) VALUES (?, ?, ?, ?, ?)")
            .bind(Uuid::new_v4().to_string())
            .bind(rule_id)
            .bind(cond.match_field.to_string())
            .bind(cond.operator.to_string())
            .bind(&cond.pattern)
            .execute(&mut **tx)
            .await?;
    }
    Ok(())
}

async fn insert_actions(tx: &mut sqlx::Transaction<'_, sqlx::Sqlite>, rule_id: &str, actions: &[RuleAction]) -> Result<()> {
    for act in actions {
        let (action_type, name_op, value, cat_id, transfer_id) = classify_action(act);
        sqlx::query(
            "INSERT INTO rule_actions (id, rule_id, action_type, name_op, value, category_id, transfer_account_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
        )
        .bind(Uuid::new_v4().to_string())
        .bind(rule_id)
        .bind(action_type.to_string())
        .bind(name_op)
        .bind(value)
        .bind(cat_id)
        .bind(transfer_id)
        .execute(&mut **tx)
        .await?;
    }
    Ok(())
}

/// Map a `RuleAction` to its DB columns (mirrors Go's `insertActions`).
fn classify_action(act: &RuleAction) -> (ActionType, String, String, String, String) {
    if !act.set_transfer_account_id.is_empty() {
        (ActionType::SetTransferAccount, String::new(), String::new(), String::new(), act.set_transfer_account_id.clone())
    } else if !act.set_category_id.is_empty() {
        (ActionType::SetCategory, String::new(), String::new(), act.set_category_id.clone(), String::new())
    } else {
        let op = act.set_name_op.unwrap_or(ActionOp::Rename);
        (ActionType::SetName, op.to_string(), act.set_name.clone(), String::new(), String::new())
    }
}

#[derive(sqlx::FromRow)]
struct RawRule {
    id: String,
    name: String,
    priority: i64,
    logic: String,
    created_at: String,
}

impl RawRule {
    fn into_rule(self, conditions: Vec<RuleCondition>, actions: Vec<RuleAction>) -> Rule {
        Rule {
            id: self.id,
            name: self.name,
            priority: self.priority,
            logic: parse_logic(&self.logic),
            conditions,
            actions,
            created_at: ts_rfc3339(&self.created_at),
        }
    }
}

fn parse_logic(s: &str) -> RuleLogic {
    match s {
        "AND" => RuleLogic::And,
        _ => RuleLogic::Or,
    }
}

#[derive(sqlx::FromRow)]
struct RawCondition {
    match_field: String,
    operator: String,
    pattern: String,
}

impl From<RawCondition> for RuleCondition {
    fn from(r: RawCondition) -> Self {
        Self {
            match_field: r.match_field.parse().unwrap_or(MatchField::Name),
            operator: r.operator.parse().unwrap_or(MatchOperator::Contains),
            pattern: r.pattern,
        }
    }
}

#[derive(sqlx::FromRow)]
struct RawAction {
    action_type: String,
    name_op: String,
    value: String,
    category_id: String,
    transfer_account_id: String,
}

impl From<RawAction> for RuleAction {
    fn from(r: RawAction) -> Self {
        match r.action_type.parse::<ActionType>().unwrap_or(ActionType::SetName) {
            ActionType::SetTransferAccount => RuleAction { set_transfer_account_id: r.transfer_account_id, ..Default::default() },
            ActionType::SetCategory => RuleAction { set_category_id: r.category_id, ..Default::default() },
            ActionType::SetName => RuleAction {
                set_name: r.value,
                set_name_op: r.name_op.parse().ok(),
                ..Default::default()
            },
        }
    }
}

#[cfg(test)]
mod tests {
    use std::str::FromStr;

    use super::*;

    async fn test_pool() -> SqlitePool {
        let dir = tempfile::tempdir().unwrap();
        // Keep the temp dir alive for the pool's lifetime (auto-delete on drop
        // would unlink the DB file before the lazy pool opens it).
        let path = dir.into_path().join("test.db");
        let opt = sqlx::sqlite::SqliteConnectOptions::from_str(path.to_str().unwrap())
            .unwrap()
            .create_if_missing(true)
            .journal_mode(sqlx::sqlite::SqliteJournalMode::Wal)
            .foreign_keys(true);
        let pool = SqlitePool::connect_with(opt).await.unwrap();
        crate::db::run_migrations(&pool, std::path::Path::new("db/migrations")).await.unwrap();
        pool
    }

    #[tokio::test]
    async fn rule_crud_roundtrip() {
        use std::str::FromStr;
        let pool = test_pool().await;
        sqlx::query("INSERT INTO users (id, email, password_hash, name) VALUES ('u1','u1@x.com','h','u1')")
            .execute(&pool).await.unwrap();

        let repo = RuleRepo::new(pool.clone());
        let cond = RuleCondition {
            match_field: "NAME".parse().unwrap(),
            operator: "CONTAINS".parse().unwrap(),
            pattern: "netflix".to_string(),
        };
        let action = RuleAction {
            set_name: "Netflix Sub".to_string(),
            set_name_op: Some(ActionOp::Rename),
            ..Default::default()
        };
        let created = repo.create("u1", "Renamer", 5, RuleLogic::And, &[cond.clone()], &[action.clone()]).await.unwrap();
        assert_eq!(created.name, "Renamer");
        assert_eq!(created.logic, RuleLogic::And);

        let fetched = repo.get_by_id(&created.id).await.unwrap().unwrap();
        assert_eq!(fetched.conditions.len(), 1);
        assert_eq!(fetched.conditions[0].match_field, MatchField::Name);
        assert_eq!(fetched.conditions[0].operator, MatchOperator::Contains);
        assert_eq!(fetched.conditions[0].pattern, "netflix");
        assert_eq!(fetched.actions[0].set_name, "Netflix Sub");
        assert_eq!(fetched.actions[0].set_name_op, Some(ActionOp::Rename));

        let updated = repo.update(&created.id, "Renamer2", 9, RuleLogic::Or, &[cond], &[]).await.unwrap();
        assert!(updated);
        let after = repo.get_by_id(&created.id).await.unwrap().unwrap();
        assert_eq!(after.name, "Renamer2");
        assert_eq!(after.logic, RuleLogic::Or);
        assert!(after.actions.is_empty());

        assert!(repo.delete(&created.id).await.unwrap());
        assert!(repo.get_by_id(&created.id).await.unwrap().is_none());
    }
}
