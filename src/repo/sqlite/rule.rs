//! `SQLite` rules repository — SQL + read-time overlay data, implementing
//! `crate::repo::traits::RuleRepo` over an `sqlx::SqlitePool`.

use async_trait::async_trait;
use sqlx::SqlitePool;
use uuid::Uuid;

use crate::error::Result;
use crate::repo::traits::rule::{ActionOp, ActionType, ConditionData, MatchField, MatchOperator, OverlayRule, Rule, RuleAction, RuleCondition, RuleLogic};
use crate::utils::timex::{ts_rfc3339};

pub struct SqliteRuleRepo {
    pool: SqlitePool,
}

impl SqliteRuleRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    async fn create_inner(
        &self,
        user_id: &str,
        name: &str,
        priority: i64,
        logic: RuleLogic,
        conditions: &[RuleCondition],
        actions: &[RuleAction],
    ) -> Result<Rule> {
        let id = Uuid::new_v4().to_string();
        let now = crate::utils::timex::now_go_ts();
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

    #[allow(clippy::too_many_arguments)]
    async fn update_inner(
        &self,
        user_id: &str,
        id: &str,
        name: &str,
        priority: i64,
        logic: RuleLogic,
        conditions: &[RuleCondition],
        actions: &[RuleAction],
    ) -> Result<bool> {
        let mut tx = self.pool.begin().await?;
        let result = sqlx::query("UPDATE rules SET name = COALESCE(NULLIF(?, ''), name), priority = ?, logic = ? WHERE id = ? AND user_id = ?")
            .bind(name)
            .bind(priority)
            .bind(logic.to_string())
            .bind(id)
            .bind(user_id)
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

    async fn delete_inner(&self, user_id: &str, id: &str) -> Result<bool> {
        let result = sqlx::query("DELETE FROM rules WHERE id = ? AND user_id = ?")
            .bind(id)
            .bind(user_id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    async fn get_by_id_inner(&self, user_id: &str, id: &str) -> Result<Option<Rule>> {
        let row = sqlx::query_as::<_, RawRule>("SELECT id, name, priority, logic, created_at FROM rules WHERE id = ? AND user_id = ?")
            .bind(id)
            .bind(user_id)
            .fetch_optional(&self.pool)
            .await?;
        let Some(raw) = row else { return Ok(None) };
        let conditions = self.conditions_for_rule(id).await?;
        let actions = self.actions_for_rule(id).await?;
        Ok(Some(raw.into_rule(conditions, actions)))
    }

    async fn list_inner(&self, user_id: &str) -> Result<Vec<Rule>> {
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

    async fn list_for_overlay_inner(&self, user_id: &str) -> Result<Vec<OverlayRule>> {
        let rules = self.list_inner(user_id).await?;
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

#[async_trait]
impl crate::repo::traits::RuleRepo for SqliteRuleRepo {
    async fn create(&self, user_id: &str, name: &str, priority: i64, logic: RuleLogic, conditions: &[RuleCondition], actions: &[RuleAction]) -> Result<Rule> {
        self.create_inner(user_id, name, priority, logic, conditions, actions).await
    }
    async fn update(&self, user_id: &str, id: &str, name: &str, priority: i64, logic: RuleLogic, conditions: &[RuleCondition], actions: &[RuleAction]) -> Result<bool> {
        self.update_inner(user_id, id, name, priority, logic, conditions, actions).await
    }
    async fn delete(&self, user_id: &str, id: &str) -> Result<bool> {
        self.delete_inner(user_id, id).await
    }
    async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<Rule>> {
        self.get_by_id_inner(user_id, id).await
    }
    async fn list(&self, user_id: &str) -> Result<Vec<Rule>> {
        self.list_inner(user_id).await
    }
    async fn list_for_overlay(&self, user_id: &str) -> Result<Vec<OverlayRule>> {
        self.list_for_overlay_inner(user_id).await
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
