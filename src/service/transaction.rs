//! Transactions service — business logic mirroring Go's `services/transaction.go`.
//! Rule overlay is Phase 4 and intentionally not wired here.

use std::collections::{HashMap, HashSet};
use std::sync::Arc;

use chrono::Datelike;
use utoipa::ToSchema;

use crate::error::{ApiError, Result};
use crate::repo::traits::{AccountRepo, CategoryRepo, TransactionRepo};
use crate::repo::traits::transaction::{CreateOutcome, CreateTransactionInput, ListTransactionResult, Transaction, TransactionListFilter, TransactionType, UpdateTransactionInput};
use crate::service::rule::{RuleService, TransactionView};
use crate::service::transfer_rule::TransferRuleService;
use crate::utils::math::round2;
use crate::utils::timex::go_ts;

#[derive(Clone)]
pub struct TransactionService {
    transaction_repo: Arc<dyn TransactionRepo>,
    account_repo: Arc<dyn AccountRepo>,
    category_repo: Arc<dyn CategoryRepo>,
    transfer_rule: Option<TransferRuleService>,
    rule: Option<Arc<RuleService>>,
}

/// A transaction ready for create/update, mirroring Go's `TransactionResponse`.
pub struct TransactionReq {
    pub id: String,
    pub name: String,
    pub amount: f64,
    pub transaction_type: TransactionType,
    pub occurred_at: String, // empty = now / unchanged
    pub account_id: String,
    pub category_ids: Vec<String>,
    pub external_id: Option<String>,
}

pub struct BulkResult {
    pub success: bool,
    pub message: String,
    pub failed_ids: Vec<String>,
    pub skipped: i64,
}

impl TransactionService {
    pub fn new(transaction_repo: Arc<dyn TransactionRepo>, account_repo: Arc<dyn AccountRepo>, category_repo: Arc<dyn CategoryRepo>) -> Self {
        Self { transaction_repo, account_repo, category_repo, transfer_rule: None, rule: None }
    }

    pub fn with_transfer_rule(mut self, transfer_rule: TransferRuleService) -> Self {
        self.transfer_rule = Some(transfer_rule);
        self
    }

    /// Enable write-time rule application: on create, set `clean_name` (and
    /// category) from matching rules. The raw `name` is never overwritten.
    pub fn with_rule(mut self, rule: Arc<RuleService>) -> Self {
        self.rule = Some(rule);
        self
    }

    fn delta(transaction_type: TransactionType, amount: f64) -> f64 {
        if transaction_type == TransactionType::Credit {
            amount
        } else {
            -amount
        }
    }

    fn totals(transaction_type: TransactionType, amount: f64) -> (f64, f64) {
        if transaction_type == TransactionType::Credit {
            (amount, 0.0)
        } else {
            (0.0, amount)
        }
    }

    /// Reject the request with 400 unless the account and every category
    /// referenced by `t` belong to `user_id`.
    async fn verify_ownership(&self, user_id: &str, t: &TransactionReq) -> Result<()> {
        if self.account_repo.get_by_id(user_id, &t.account_id).await?.is_none() {
            return Err(ApiError::bad_request("account does not belong to user"));
        }
        for cat in &t.category_ids {
            if self.category_repo.get_by_id(user_id, cat).await?.is_none() {
                return Err(ApiError::bad_request("category does not belong to user"));
            }
        }
        Ok(())
    }

    pub async fn list(&self, user_id: &str, f: TransactionListFilter) -> Result<ListTransactionResult> {
        self.transaction_repo.list(user_id, &f).await
    }

    // Snapshot rule-resolved name into clean_name + category at write; raw name is untouched.
    // No rule wired, or no match, leaves clean_name None.
    async fn apply_rules_on_create(&self, user_id: &str, inputs: &mut [CreateTransactionInput]) -> Result<()> {
        let Some(rule) = &self.rule else { return Ok(()) };
        let mut views: Vec<TransactionView> = inputs.iter().map(txn_to_view).collect();
        rule.overlay(user_id, &mut views).await?;
        for (input, view) in inputs.iter_mut().zip(views) {
            // Only adopt when a rule actually changed the display name.
            if view.name != input.txn.name {
                input.txn.clean_name = Some(view.name);
            }
            input.category_ids = view.category_ids;
        }
        Ok(())
    }

    pub async fn create(&self, user_id: &str, reqs: &[TransactionReq]) -> Result<BulkResult> {
        if reqs.len() > 1000 {
            return Err(ApiError::bad_request("too many transactions in one request (max 1000)"));
        }
        let now = go_ts(chrono::Utc::now());
        let mut txns: Vec<Transaction> = Vec::new();
        let mut inputs: Vec<CreateTransactionInput> = Vec::new();
        for t in reqs {
            if t.name.is_empty() {
                return Err(ApiError::bad_request("invalid transaction: name is required"));
            }
            if t.amount <= 0.0 {
                return Err(ApiError::bad_request("invalid transaction: amount must be greater than zero"));
            }
            if t.account_id.is_empty() {
                return Err(ApiError::bad_request("invalid transaction: account_id is required"));
            }
            self.verify_ownership(user_id, t).await?;
            let occurred_at = if t.occurred_at.is_empty() { now.clone() } else { t.occurred_at.clone() };
            let txn = Transaction {
                id: t.id.clone(),
                name: t.name.clone(),
                clean_name: None,
                amount: round2(t.amount),
                transaction_type: t.transaction_type,
                occurred_at,
                account_id: t.account_id.clone(),
                created_at: now.clone(),
                external_id: t.external_id.clone(),
                transfer_linked: false,
            };
            inputs.push(CreateTransactionInput { txn: txn.clone(), category_ids: t.category_ids.clone() });
            txns.push(txn);
        }

        self.apply_rules_on_create(user_id, &mut inputs).await?;

        let outcome: CreateOutcome = self.transaction_repo.create(user_id, &inputs).await?;

        let mut inserted_txns: Vec<Transaction> = Vec::new();
        let mut skipped = 0i64;
        let mut deltas: HashMap<String, (f64, f64, f64)> = HashMap::new(); // account -> (balance, credit, debit)
        for t in &txns {
            if !outcome.inserted.contains(&t.id) {
                skipped += 1;
                continue;
            }
            inserted_txns.push(t.clone());
            let (c, d) = Self::totals(t.transaction_type, t.amount);
            let e = deltas.entry(t.account_id.clone()).or_insert((0.0, 0.0, 0.0));
            e.0 += Self::delta(t.transaction_type, t.amount);
            e.1 += c;
            e.2 += d;
        }
        for (acc_id, (b, c, d)) in deltas {
            if b != 0.0 {
                self.account_repo.update_balance(&acc_id, b).await?;
            }
            if c != 0.0 || d != 0.0 {
                self.account_repo.apply_totals(&acc_id, c, d).await?;
            }
        }

        if let Some(tr) = &self.transfer_rule {
            let uid_owned = user_id.to_string();
            let _ = tr.apply_to_transactions(&uid_owned, &inserted_txns).await;
        }

        let failed_ids: Vec<String> = outcome.errors.clone();
        if !failed_ids.is_empty() {
            return Ok(BulkResult { success: false, message: "some transactions failed".to_string(), failed_ids, skipped });
        }
        Ok(BulkResult { success: true, message: "transactions created successfully".to_string(), failed_ids: Vec::new(), skipped })
    }

    pub async fn update(&self, user_id: &str, reqs: &[TransactionReq]) -> Result<BulkResult> {
        if reqs.len() > 1000 {
            return Err(ApiError::bad_request("too many transactions in one request (max 1000)"));
        }
        for t in reqs {
            if !t.account_id.is_empty() && self.account_repo.get_by_id(user_id, &t.account_id).await?.is_none() {
                return Err(ApiError::bad_request("account does not belong to user"));
            }
            for cat in &t.category_ids {
                if self.category_repo.get_by_id(user_id, cat).await?.is_none() {
                    return Err(ApiError::bad_request("category does not belong to user"));
                }
            }
        }
        let ids: Vec<String> = reqs.iter().map(|t| t.id.clone()).collect();
        let old_txns = self.transaction_repo.get_by_id_batch(user_id, &ids).await?;
        let old_map: HashMap<String, Transaction> = old_txns.into_iter().map(|t| (t.id.clone(), t)).collect();

        let mut inputs: Vec<UpdateTransactionInput> = Vec::new();
        for t in reqs {
            inputs.push(UpdateTransactionInput {
                txn: Transaction {
                    id: t.id.clone(),
                    name: t.name.clone(),
                    clean_name: None,
                    amount: round2(t.amount),
                    transaction_type: t.transaction_type,
                    occurred_at: t.occurred_at.clone(),
                    account_id: t.account_id.clone(),
                    created_at: String::new(),
                    external_id: None,
                    transfer_linked: false,
                },
                category_ids: t.category_ids.clone(),
            });
        }
        let errs = self.transaction_repo.update(user_id, &inputs).await?;
        let failed: HashSet<String> = failed_ids(&errs);

        // Reverse old, apply new per account (balance + cached totals). Only for
        // txns the repo actually updated — a failed row must not shift balances.
        let mut deltas: HashMap<String, (f64, f64, f64)> = HashMap::new(); // account -> (balance, credit, debit)
        for t in reqs {
            if failed.contains(&t.id) {
                continue;
            }
            let Some(old) = old_map.get(&t.id) else { continue };
            let acc_id = if t.account_id.is_empty() { old.account_id.clone() } else { t.account_id.clone() };
            let (oc, od) = Self::totals(old.transaction_type, old.amount);
            let e = deltas.entry(old.account_id.clone()).or_insert((0.0, 0.0, 0.0));
            e.0 -= Self::delta(old.transaction_type, old.amount);
            e.1 -= oc;
            e.2 -= od;
            let (nc, nd) = Self::totals(t.transaction_type, t.amount);
            let e = deltas.entry(acc_id).or_insert((0.0, 0.0, 0.0));
            e.0 += Self::delta(t.transaction_type, t.amount);
            e.1 += nc;
            e.2 += nd;
        }
        for (acc_id, (b, c, d)) in deltas {
            if b != 0.0 {
                self.account_repo.update_balance(&acc_id, b).await?;
            }
            if c != 0.0 || d != 0.0 {
                self.account_repo.apply_totals(&acc_id, c, d).await?;
            }
        }

        if !errs.is_empty() {
            return Ok(BulkResult { success: false, message: "some updates failed".to_string(), failed_ids: errs, skipped: 0 });
        }
        Ok(BulkResult { success: true, message: "transactions updated successfully".to_string(), failed_ids: Vec::new(), skipped: 0 })
    }

    pub async fn delete(&self, user_id: &str, ids: &[String]) -> Result<BulkResult> {
        if ids.is_empty() {
            return Err(ApiError::bad_request("no ids provided"));
        }
        let old_txns = self.transaction_repo.get_by_id_batch(user_id, ids).await?;
        let errs = self.transaction_repo.delete(user_id, ids).await?;
        let failed: HashSet<String> = failed_ids(&errs);

        // Only reverse balances for txns that were actually deleted.
        let mut deltas: HashMap<String, (f64, f64, f64)> = HashMap::new();
        for t in &old_txns {
            if failed.contains(&t.id) {
                continue;
            }
            let e = deltas.entry(t.account_id.clone()).or_insert((0.0, 0.0, 0.0));
            let (c, d) = Self::totals(t.transaction_type, t.amount);
            e.0 -= Self::delta(t.transaction_type, t.amount);
            e.1 -= c;
            e.2 -= d;
        }
        for (acc_id, (b, c, d)) in deltas {
            if b != 0.0 {
                self.account_repo.update_balance(&acc_id, b).await?;
            }
            if c != 0.0 || d != 0.0 {
                self.account_repo.apply_totals(&acc_id, c, d).await?;
            }
        }

        if !errs.is_empty() {
            return Ok(BulkResult { success: false, message: "some deletions failed".to_string(), failed_ids: errs, skipped: 0 });
        }
        Ok(BulkResult { success: true, message: "transactions deleted successfully".to_string(), failed_ids: Vec::new(), skipped: 0 })
    }

    /// Dashboard: total balance, income, expenses (from cached account totals).
    pub async fn dashboard(&self, user_id: &str) -> Result<Dashboard> {
        let balance = self.account_repo.sum_balance(user_id).await?;
        let (credit, debit) = self.account_repo.sum_totals(user_id).await?;
        Ok(Dashboard { total_balance: balance, total_income: credit, total_expenses: debit })
    }

    /// Spending buckets + categories for a range, computed in SQL.
    pub async fn spending(&self, user_id: &str, range: &str, from: &str, to: &str, account_id: &str) -> Result<SpendingResult> {
        let now = chrono::Utc::now();
        let mut gran = "day";
        let mut from_ts: Option<String> = None;
        let mut to_ts: Option<String> = None;

        match range {
            "7D" => {
                from_ts = Some(go_ts(now - chrono::Duration::days(7)));
                to_ts = Some(go_ts(now));
            }
            "1M" => {
                from_ts = Some(go_ts(now - chrono::Months::new(1)));
                to_ts = Some(go_ts(now));
            }
            "6M" => {
                from_ts = Some(go_ts(now - chrono::Months::new(6)));
                to_ts = Some(go_ts(now));
                gran = "month";
            }
            "1Y" => {
                from_ts = Some(go_ts(now - chrono::Months::new(12)));
                to_ts = Some(go_ts(now));
                gran = "month";
            }
            _ => {}
        }
        if !from.is_empty() || !to.is_empty() {
            let f = chrono::NaiveDate::parse_from_str(from, "%Y-%m-%d").ok();
            let t = chrono::NaiveDate::parse_from_str(to, "%Y-%m-%d").ok();
            from_ts = f.map(|d| go_ts(d.and_hms_opt(0, 0, 0).unwrap().and_utc()));
            to_ts = t.map(|d| go_ts((d + chrono::Duration::days(1)).and_hms_opt(0, 0, 0).unwrap().and_utc()));
            if let (Some(ft), Some(tt)) = (&from_ts, &to_ts) {
                // compare via date strings
                let f_day = &ft[..10];
                let t_day = &tt[..10];
                let fd = chrono::NaiveDate::parse_from_str(f_day, "%Y-%m-%d").unwrap();
                let td = chrono::NaiveDate::parse_from_str(t_day, "%Y-%m-%d").unwrap();
                if (td - fd).num_days() > 366 {
                    return Err(ApiError::bad_request("custom range must be at most 1 year"));
                }
                if (td - fd).num_days() > 30 {
                    gran = "month";
                }
            }
        }

        let filter = crate::repo::traits::transaction::SpendingFilter {
            granularity: gran.to_string(),
            from: from_ts.unwrap_or_default(),
            to: to_ts.unwrap_or_default(),
            account_id: account_id.to_string(),
        };
        let buckets = self.transaction_repo.spending_buckets(user_id, &filter).await?;
        let cats = self.transaction_repo.spending_categories(user_id, &filter).await?;

        let bucket_items: Vec<SpendingBucket> = buckets.iter().map(|b| SpendingBucket {
            key: b.key.clone(),
            label: bucket_label(&b.key, gran),
            amount: b.amount,
        }).collect();
        let category_items: Vec<SpendingCategory> = cats.iter().map(|c| SpendingCategory {
            id: c.id.clone(),
            name: c.name.clone(),
            debit: c.debit,
            credit: c.credit,
            net: c.debit - c.credit,
        }).collect();
        Ok(SpendingResult { buckets: bucket_items, categories: category_items })
    }
}

/// Extract the failed transaction ids from the repo's sanitized error messages
/// (`failed to update {id}`, `failed to delete {id}`, `transaction {id} not found`).
fn failed_ids(errs: &[String]) -> HashSet<String> {
    errs.iter()
        .filter_map(|e| {
            for prefix in ["failed to update ", "failed to delete ", "failed to create ", "transaction "] {
                if let Some(rest) = e.strip_prefix(prefix) {
                    return Some(rest.split(' ').next().unwrap_or("").to_string());
                }
            }
            None
        })
        .collect()
}

// `total_` prefix matches the wire keys (totalBalance, ...).
#[allow(clippy::struct_field_names)]
#[derive(ToSchema, serde::Serialize)]
#[schema(rename_all = "camelCase")]
pub struct Dashboard {
    pub total_balance: f64,
    pub total_income: f64,
    pub total_expenses: f64,
}

#[derive(ToSchema)]
#[schema(rename_all = "camelCase")]
pub struct SpendingBucket {
    pub key: String,
    pub label: String,
    pub amount: f64,
}

#[derive(ToSchema)]
#[schema(rename_all = "camelCase")]
pub struct SpendingCategory {
    pub id: String,
    pub name: String,
    pub debit: f64,
    pub credit: f64,
    pub net: f64,
}

#[derive(ToSchema)]
#[schema(rename_all = "camelCase")]
pub struct SpendingResult {
    pub buckets: Vec<SpendingBucket>,
    pub categories: Vec<SpendingCategory>,
}

/// Human label for a bucket key (Go's `bucketLabel`).
fn bucket_label(key: &str, gran: &str) -> String {
    if gran == "month" {
        if let Ok(d) = chrono::NaiveDate::parse_from_str(key, "%Y-%m") {
            return d.format("%b %y").to_string();
        }
        return key.to_string();
    }
    if let Ok(d) = chrono::NaiveDate::parse_from_str(key, "%Y-%m-%d") {
        return format!("{} {}", d.day(), d.format("%b"));
    }
    key.to_string()
}

// Mirror a pending create input as the matchable view the rule overlay expects.
fn txn_to_view(input: &CreateTransactionInput) -> TransactionView {
    let t = &input.txn;
    TransactionView {
        name: t.name.clone(),
        amount: t.amount,
        transaction_type: t.transaction_type,
        account_id: t.account_id.clone(),
        category_ids: input.category_ids.clone(),
    }
}
