//! Rules service — validation, read-time overlay, and preview, mirroring Go's
//! `services/rule.go`.

use std::sync::Arc;

use crate::error::{ApiError, Result};
use crate::repo::traits::rule::{ActionOp, ConditionData, MatchField, MatchOperator, Rule, RuleAction, RuleCondition, RuleLogic};
use crate::repo::traits::{CategoryRepo as CatTrait, RuleRepo, TransactionRepo as TxnTrait};
use crate::repo::traits::transaction::{ListRow, TransactionListFilter, TransactionType};

#[derive(Clone)]
pub struct RuleService {
    rules: Arc<dyn RuleRepo>,
    categories: Arc<dyn CatTrait>,
    transactions: Arc<dyn TxnTrait>,
}

/// A transaction's matchable view, mutable so overlay can rename/re-categorize.
#[derive(Debug, Clone)]
pub struct TransactionView {
    pub name: String,
    pub amount: f64,
    pub transaction_type: TransactionType,
    pub account_id: String,
    pub category_ids: Vec<String>,
}

impl RuleService {
    pub fn new(rule_repo: Arc<dyn RuleRepo>, cat_repo: Arc<dyn CatTrait>, txn_repo: Arc<dyn TxnTrait>) -> Self {
        Self { rules: rule_repo, categories: cat_repo, transactions: txn_repo }
    }

    pub async fn create(&self, user_id: &str, name: &str, priority: i64, logic: RuleLogic, conditions: &[RuleCondition], actions: &[RuleAction]) -> Result<Rule> {
        if name.is_empty() {
            return Err(ApiError::bad_request("name is required"));
        }
        if conditions.is_empty() {
            return Err(ApiError::bad_request("at least one condition is required"));
        }
        self.rules.create(user_id, name, priority, logic, conditions, actions).await
    }

    #[allow(clippy::too_many_arguments)]
    pub async fn update(&self, user_id: &str, id: &str, name: &str, priority: i64, logic: RuleLogic, conditions: &[RuleCondition], actions: &[RuleAction]) -> Result<Rule> {
        let updated = self.rules.update(user_id, id, name, priority, logic, conditions, actions).await?;
        if !updated {
            return Err(ApiError::not_found(format!("rule {id} not found")));
        }
        self.rules.get_by_id(user_id, id).await?.ok_or_else(|| ApiError::internal("fetching updated rule"))
    }

    pub async fn delete(&self, user_id: &str, id: &str) -> Result<()> {
        if !self.rules.delete(user_id, id).await? {
            return Err(ApiError::not_found(format!("rule {id} not found")));
        }
        Ok(())
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<Rule>> {
        self.rules.list(user_id).await
    }

    /// Read-time overlay: apply every matching rule's actions to each txn, in
    /// priority order (highest first); later rules override earlier setters.
    /// Nothing is persisted. `SET_TRANSFER_ACCOUNT` is a write-time action with
    /// no view change, so it is skipped.
    pub async fn overlay(&self, user_id: &str, txns: &mut [TransactionView]) -> Result<()> {
        if txns.is_empty() {
            return Ok(());
        }
        let rules = self.rules.list_for_overlay(user_id).await?;
        if rules.is_empty() {
            return Ok(());
        }
        let cat_names = self.category_name_by_id(user_id).await?;

        for txn in txns {
            let data = Self::txn_match_data(txn, &cat_names);
            for rule in &rules {
                if !match_rule_data(&data, rule.logic, &rule.conditions) {
                    continue;
                }
                apply_rule_actions(txn, &rule.actions);
            }
        }
        Ok(())
    }

    /// Preview: match rules' conditions against existing transactions
    /// (no type filter — mirrors Go's `SearchByRule`).
    pub async fn preview(&self, user_id: &str, logic: RuleLogic, conditions: &[RuleCondition], limit: i64) -> Result<Vec<TransactionView>> {
        let conds: Vec<ConditionData> = conditions.iter().map(|c| ConditionData {
            match_field: c.match_field,
            operator: c.operator,
            pattern: c.pattern.clone(),
        }).collect();
        for c in &conds {
            if c.operator == MatchOperator::Regex {
                compile_rule_regex(&c.pattern).map_err(|e| ApiError::bad_request(format!("invalid regex in condition: {e}")))?;
            }
        }
        let limit = if limit <= 0 || limit > 20 { 20 } else { limit };
        let rows = self.transactions.list(user_id, &TransactionListFilter::default()).await?;
        let cat_names = self.category_name_by_id(user_id).await?;
        let mut out = Vec::new();
        for row in &rows.rows {
            let view = Self::row_to_view(row);
            let data = Self::txn_match_data(&view, &cat_names);
            if match_rule_data(&data, logic, &conds) {
                out.push(view);
                if i64::try_from(out.len()).unwrap_or(i64::MAX) >= limit {
                    break;
                }
            }
        }
        Ok(out)
    }

    async fn category_name_by_id(&self, user_id: &str) -> Result<std::collections::HashMap<String, String>> {
        let res = self.categories.list(user_id, 0, "").await?;
        Ok(res.categories.into_iter().map(|c| (c.id, c.name)).collect())
    }

    fn txn_match_data(txn: &TransactionView, cat_names: &std::collections::HashMap<String, String>) -> TransactionMatchData {
        let categories: Vec<String> = txn
            .category_ids
            .iter()
            .filter_map(|id| cat_names.get(id).cloned())
            .collect();
        TransactionMatchData {
            name: txn.name.clone(),
            amount: txn.amount,
            transaction_type: txn.transaction_type,
            account_id: txn.account_id.clone(),
            categories,
        }
    }

    fn row_to_view(row: &ListRow) -> TransactionView {
        TransactionView {
            name: row.txn.name.clone(),
            amount: row.txn.amount,
            transaction_type: row.txn.transaction_type,
            account_id: row.txn.account_id.clone(),
            category_ids: row.category_ids.clone(),
        }
    }
}

/// Everything a rule condition can match against.
pub struct TransactionMatchData {
    pub name: String,
    pub amount: f64,
    pub transaction_type: TransactionType,
    pub account_id: String,
    pub categories: Vec<String>,
}

/// Apply a rule's output actions to a txn in order.
fn apply_rule_actions(txn: &mut TransactionView, actions: &[RuleAction]) {
    for act in actions {
        if !act.set_transfer_account_id.is_empty() {
            continue;
        }
        if !act.set_category_id.is_empty() {
            txn.category_ids = vec![act.set_category_id.clone()];
            continue;
        }
        if act.set_name.is_empty() {
            continue;
        }
        txn.name = apply_name_op(&txn.name, &act.set_name, act.set_name_op.unwrap_or(ActionOp::Rename));
    }
}

fn apply_name_op(current: &str, value: &str, op: ActionOp) -> String {
    match op {
        ActionOp::AddPrefix => format!("{value}{current}"),
        ActionOp::AddSuffix => format!("{current}{value}"),
        ActionOp::Rename => value.to_string(),
    }
}

pub fn match_rule_data(txn: &TransactionMatchData, logic: RuleLogic, conditions: &[ConditionData]) -> bool {
    if conditions.is_empty() {
        return false;
    }
    let is_or = logic == RuleLogic::Or;
    for cond in conditions {
        let matched = match_condition(txn, cond);
        if is_or && matched {
            return true;
        }
        if !is_or && !matched {
            return false;
        }
    }
    !is_or
}

fn match_condition(txn: &TransactionMatchData, cond: &ConditionData) -> bool {
    for val in field_values(txn, cond.match_field) {
        if evaluate(&val, cond.operator, &cond.pattern) {
            return true;
        }
    }
    false
}

fn field_values(txn: &TransactionMatchData, field: MatchField) -> Vec<String> {
    match field {
        MatchField::Name => vec![txn.name.clone()],
        MatchField::Amount => vec![format_amount(txn.amount)],
        MatchField::Type => vec![txn.transaction_type.to_string()],
        MatchField::Category => txn.categories.clone(),
        MatchField::Account => vec![txn.account_id.clone()],
    }
}

/// Go formats amount with `strconv.FormatFloat(v, 'f', -1, 64)`.
fn format_amount(v: f64) -> String {
    let mut s = format!("{v}");
    if let Some(stripped) = s.strip_suffix(".0") {
        s = stripped.to_string();
    }
    s
}

fn evaluate(value: &str, op: MatchOperator, pattern: &str) -> bool {
    match op {
        MatchOperator::Contains => value.to_lowercase().contains(&pattern.to_lowercase()),
        MatchOperator::StartsWith => value.to_lowercase().starts_with(&pattern.to_lowercase()),
        MatchOperator::EndsWith => value.to_lowercase().ends_with(&pattern.to_lowercase()),
        MatchOperator::Equals => value.eq_ignore_ascii_case(pattern),
        MatchOperator::GreaterThan => cmp_float(value, pattern, |a, b| a > b),
        MatchOperator::LessThan => cmp_float(value, pattern, |a, b| a < b),
        MatchOperator::Regex => compile_rule_regex(pattern).is_ok_and(|re| re.is_match(value)),
    }
}

fn cmp_float(value: &str, pattern: &str, f: fn(f64, f64) -> bool) -> bool {
    match (value.parse::<f64>(), pattern.parse::<f64>()) {
        (Ok(v), Ok(p)) => f(v, p),
        _ => false,
    }
}

/// Compile a rule regex case-insensitively unless it sets its own flags.
fn compile_rule_regex(pattern: &str) -> std::result::Result<regex::Regex, regex::Error> {
    let pat = if pattern.starts_with("(?") { pattern.to_string() } else { format!("(?i){pattern}") };
    regex::Regex::new(&pat)
}

#[cfg(test)]
mod tests {
    use std::str::FromStr;

    use super::*;
    use crate::repo::traits::rule::{MatchField, MatchOperator, RuleLogic};

    fn match_data() -> TransactionMatchData {
        TransactionMatchData {
            name: "Netflix".to_string(),
            amount: 15.99,
            transaction_type: crate::repo::traits::transaction::TransactionType::Debit,
            account_id: "acc-1".to_string(),
            categories: vec!["Entertainment".to_string()],
        }
    }

    #[test]
    fn name_ops() {
        assert_eq!(apply_name_op("Netflix", "Fee", ActionOp::Rename), "Fee");
        assert_eq!(apply_name_op("Netflix", "Fee", ActionOp::AddPrefix), "FeeNetflix");
        assert_eq!(apply_name_op("Netflix", "-Sub", ActionOp::AddSuffix), "Netflix-Sub");
        assert_eq!(apply_name_op("Netflix", "", ActionOp::AddSuffix), "Netflix");
    }

    #[test]
    fn match_conditions() {
        let data = match_data();
        let contains = ConditionData { match_field: MatchField::Name, operator: MatchOperator::Contains, pattern: "netflix".to_string() };
        assert!(match_rule_data(&data, RuleLogic::Or, &[contains.clone()]));

        let gt = ConditionData { match_field: MatchField::Amount, operator: MatchOperator::GreaterThan, pattern: "100".to_string() };
        assert!(!match_rule_data(&data, RuleLogic::Or, &[gt]));

        let regex = ConditionData { match_field: MatchField::Name, operator: MatchOperator::Regex, pattern: "NETFLIX".to_string() };
        assert!(match_rule_data(&data, RuleLogic::Or, &[regex]));

        let cat_eq = ConditionData { match_field: MatchField::Category, operator: MatchOperator::Equals, pattern: "Entertainment".to_string() };
        assert!(match_rule_data(&data, RuleLogic::And, &[cat_eq, contains]));
    }

    #[test]
    fn apply_actions_in_order() {
        let mut txn = TransactionView {
            name: "Netflix".to_string(),
            amount: 15.99,
            transaction_type: crate::repo::traits::transaction::TransactionType::Debit,
            account_id: "acc-1".to_string(),
            category_ids: vec![],
        };
        apply_rule_actions(&mut txn, &[
            RuleAction { set_name: "Netflix Subscription".to_string(), set_name_op: Some(ActionOp::Rename), ..Default::default() },
            RuleAction { set_category_id: "cat-1".to_string(), ..Default::default() },
        ]);
        assert_eq!(txn.name, "Netflix Subscription");
        assert_eq!(txn.category_ids, vec!["cat-1"]);
    }

    #[test]
    fn rejects_unknown_rule_enums() {
        assert!(RuleLogic::from_str("BOGUS").is_err());
        assert!(MatchField::from_str("BOGUS").is_err());
        assert!(MatchOperator::from_str("BOGUS").is_err());
    }
}
