//! Transactions service — business logic mirroring Go's `services/transaction.go`.
//! Rule overlay is Phase 4 and intentionally not wired here.

use std::collections::HashMap;

use crate::error::{ApiError, Result};
use crate::repo::account::AccountRepo;
use crate::repo::transaction::{CreateOutcome, CreateTransactionInput, ListTransactionResult, TransactionRepo, Transaction, TransactionListFilter, TransactionType, UpdateTransactionInput};
use crate::service::transfer_rule::TransferRuleService;
use crate::timex::go_ts;

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
}

fn round2(v: f64) -> f64 {
    (v * 100.0).round() / 100.0
}
