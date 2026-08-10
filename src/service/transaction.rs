//! Transactions service — business logic mirroring Go's `services/transaction.go`.
//! Rule overlay is Phase 4 and intentionally not wired here.

use std::collections::HashMap;

use chrono::Datelike;

use crate::error::{ApiError, Result};
use crate::repo::account::AccountRepo;
use crate::repo::transaction::{CreateOutcome, CreateTransactionInput, ListTransactionResult, TransactionRepo, Transaction, TransactionListFilter, TransactionType, UpdateTransactionInput};
use crate::service::transfer_rule::TransferRuleService;
use crate::timex::{go_ts, round2};

#[derive(Clone)]
pub struct TransactionService {
    transaction_repo: TransactionRepo,
    account_repo: AccountRepo,
    transfer_rule: Option<TransferRuleService>,
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
    pub fn new(transaction_repo: TransactionRepo, account_repo: AccountRepo) -> Self {
        Self { transaction_repo, account_repo, transfer_rule: None }
    }

    pub fn with_transfer_rule(mut self, transfer_rule: TransferRuleService) -> Self {
        self.transfer_rule = Some(transfer_rule);
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

    pub async fn list(&self, user_id: &str, f: TransactionListFilter) -> Result<ListTransactionResult> {
        self.transaction_repo.list(user_id, &f).await
    }

    pub async fn create(&self, user_id: &str, reqs: &[TransactionReq]) -> Result<BulkResult> {
        if reqs.len() > 1000 {
            return Err(ApiError::bad_request("too many transactions in one request (max 1000)"));
        }
        let now = go_ts(chrono::Utc::now());
        let mut txns: Vec<Transaction> = Vec::new();
        let mut inputs: Vec<CreateTransactionInput> = Vec::new();
        for t in reqs {
            if t.name.is_empty() || t.amount <= 0.0 || t.account_id.is_empty() {
                continue;
            }
            let occurred_at = if t.occurred_at.is_empty() { now.clone() } else { t.occurred_at.clone() };
            let txn = Transaction {
                id: t.id.clone(),
                name: t.name.clone(),
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

        let outcome: CreateOutcome = self.transaction_repo.create(user_id, &inputs).await?;

        let mut inserted_txns: Vec<Transaction> = Vec::new();
        let mut skipped = 0i64;
        for t in &txns {
            if !outcome.inserted.contains(&t.id) {
                skipped += 1;
                continue;
            }
            inserted_txns.push(t.clone());
            self.account_repo.update_balance(&t.account_id, Self::delta(t.transaction_type, t.amount)).await?;
            let (c, d) = Self::totals(t.transaction_type, t.amount);
            self.account_repo.apply_totals(&t.account_id, c, d).await?;
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

    pub async fn update(&self, reqs: &[TransactionReq]) -> Result<BulkResult> {
        if reqs.len() > 1000 {
            return Err(ApiError::bad_request("too many transactions in one request (max 1000)"));
        }
        let ids: Vec<String> = reqs.iter().map(|t| t.id.clone()).collect();
        let old_txns = self.transaction_repo.get_by_id_batch(&ids).await?;
        let old_map: HashMap<String, Transaction> = old_txns.into_iter().map(|t| (t.id.clone(), t)).collect();

        let mut inputs: Vec<UpdateTransactionInput> = Vec::new();
        for t in reqs {
            inputs.push(UpdateTransactionInput {
                txn: Transaction {
                    id: t.id.clone(),
                    name: t.name.clone(),
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
        let errs = self.transaction_repo.update(&inputs).await?;

        // Reverse old, apply new per account (balance + cached totals).
        let mut deltas: HashMap<String, (f64, f64, f64)> = HashMap::new(); // account -> (balance, credit, debit)
        for t in reqs {
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

    pub async fn delete(&self, ids: &[String]) -> Result<BulkResult> {
        if ids.is_empty() {
            return Err(ApiError::bad_request("no ids provided"));
        }
        let old_txns = self.transaction_repo.get_by_id_batch(ids).await?;
        let errs = self.transaction_repo.delete(ids).await?;

        let mut deltas: HashMap<String, (f64, f64, f64)> = HashMap::new();
        for t in &old_txns {
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

        let filter = crate::repo::transaction::SpendingFilter {
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

// `total_` prefix matches the wire keys (totalBalance, ...).
#[allow(clippy::struct_field_names)]
pub struct Dashboard {
    pub total_balance: f64,
    pub total_income: f64,
    pub total_expenses: f64,
}

pub struct SpendingBucket {
    pub key: String,
    pub label: String,
    pub amount: f64,
}

pub struct SpendingCategory {
    pub id: String,
    pub name: String,
    pub debit: f64,
    pub credit: f64,
    pub net: f64,
}

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



