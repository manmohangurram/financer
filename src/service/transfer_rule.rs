//! Transfer-rule service — resolves a rule's "transfer to account" action at
//! write time, mirroring Go's `services/transfer_rule.go`.

use std::sync::Arc;

use crate::error::{ApiError, Result};
use crate::repo::traits::rule::ConditionData;
use crate::repo::traits::transaction::{TransactionListFilter, TransactionType};
use crate::repo::traits::{AccountRepo, RuleRepo, TransactionRepo};
use crate::service::rule::{TransactionMatchData, match_rule_data};
use crate::service::transfer::{ResolveOutcome, resolve_transfer};

#[derive(Clone)]
pub struct TransferRuleService {
    rules: Arc<dyn RuleRepo>,
    transactions: Arc<dyn TransactionRepo>,
    accounts: Arc<dyn AccountRepo>,
}

pub struct RunRuleResponse {
    pub matched: i64,
    pub linked: i64,
    pub created: i64,
}

impl TransferRuleService {
    pub fn new(
        rule_repo: Arc<dyn RuleRepo>,
        transaction_repo: Arc<dyn TransactionRepo>,
        account_repo: Arc<dyn AccountRepo>,
    ) -> Self {
        Self {
            rules: rule_repo,
            transactions: transaction_repo,
            accounts: account_repo,
        }
    }

    /// Run one rule against the user's (debit, unlinked) transactions.
    pub async fn run_rule(&self, user_id: &str, rule_id: &str) -> Result<RunRuleResponse> {
        let rule = self
            .rules
            .get_by_id(user_id, rule_id)
            .await?
            .ok_or_else(|| ApiError::not_found(format!("rule {rule_id} not found")))?;

        let mut transfer_target = String::new();
        for act in &rule.actions {
            if !act.set_transfer_account_id.is_empty() {
                transfer_target = act.set_transfer_account_id.clone();
            }
        }

        let conds: Vec<ConditionData> = rule
            .conditions
            .iter()
            .map(|c| ConditionData {
                match_field: c.match_field,
                operator: c.operator,
                pattern: c.pattern.clone(),
            })
            .collect();

        let rows = self
            .transactions
            .list(
                user_id,
                &TransactionListFilter {
                    transaction_type: Some(TransactionType::Debit),
                    ..Default::default()
                },
            )
            .await?;

        let mut matched = 0i64;
        let mut linked = 0i64;
        let mut created = 0i64;
        for row in &rows.rows {
            let txn = &row.txn;
            if !row.link_id.is_empty() {
                continue;
            }
            let data = TransactionMatchData {
                name: txn.name.clone(),
                amount: txn.amount,
                transaction_type: txn.transaction_type,
                account_id: txn.account_id.clone(),
                categories: Vec::new(),
            };
            if !match_rule_data(&data, rule.logic, &conds) {
                continue;
            }
            matched += 1;
            if transfer_target.is_empty() {
                continue;
            }
            match resolve_transfer(
                user_id,
                txn,
                &transfer_target,
                &*self.transactions,
                &*self.accounts,
            )
            .await?
            {
                ResolveOutcome::Linked { created: c, .. } => {
                    linked += 1;
                    if c {
                        created += 1;
                    }
                }
                ResolveOutcome::Noop => {}
            }
        }

        Ok(RunRuleResponse {
            matched,
            linked,
            created,
        })
    }

    /// Apply transfer actions of matching rules against given transactions
    /// (used on create/import).
    pub async fn apply_to_transactions(
        &self,
        user_id: &str,
        txns: &[crate::repo::traits::transaction::Transaction],
    ) -> Result<(i64, i64)> {
        if txns.is_empty() {
            return Ok((0, 0));
        }
        let rules = self.rules.list_for_overlay(user_id).await?;
        let mut linked = 0i64;
        let mut created = 0i64;
        for txn in txns {
            let data = TransactionMatchData {
                name: txn.name.clone(),
                amount: txn.amount,
                transaction_type: txn.transaction_type,
                account_id: txn.account_id.clone(),
                categories: Vec::new(),
            };
            for rule in &rules {
                if !match_rule_data(&data, rule.logic, &rule.conditions) {
                    continue;
                }
                for act in &rule.actions {
                    if act.set_transfer_account_id.is_empty() {
                        continue;
                    }
                    match resolve_transfer(
                        user_id,
                        txn,
                        &act.set_transfer_account_id,
                        &*self.transactions,
                        &*self.accounts,
                    )
                    .await?
                    {
                        ResolveOutcome::Linked { created: c, .. } => {
                            linked += 1;
                            if c {
                                created += 1;
                            }
                        }
                        ResolveOutcome::Noop => {}
                    }
                }
            }
        }
        Ok((linked, created))
    }
}
