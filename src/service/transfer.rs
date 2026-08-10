//! Transfers service — link/unlink and counterpart creation, mirroring Go's
//! `services/transfer.go`.

use uuid::Uuid;

use crate::error::{ApiError, Result};
use crate::repo::account::AccountRepo;
use crate::repo::transaction::{self, TransactionRepo, Transaction, TransactionType};
use crate::timex::go_ts;

#[derive(Clone)]
pub struct TransferService {
    transaction_repo: TransactionRepo,
    account_repo: AccountRepo,
}

pub struct CreateTransferResp {
    pub debit_transaction_id: String,
    pub credit_transaction_id: String,
}

pub type BulkResult = crate::service::transaction::BulkResult;

impl TransferService {
    pub fn new(transaction_repo: TransactionRepo, account_repo: AccountRepo) -> Self {
        Self { transaction_repo, account_repo }
    }

    /// Create the missing side of a transfer for an existing transaction and
    /// link it. If a matching counterpart already exists it is linked instead.
    pub async fn create_counterpart(&self, user_id: &str, txn_id: &str, to_account_id: &str) -> Result<CreateTransferResp> {
        let Some(src) = self.transaction_repo.get_full(txn_id).await? else {
            return Err(ApiError::not_found(format!("transaction {txn_id} not found")));
        };
        if to_account_id.is_empty() || to_account_id == src.account_id {
            return Err(ApiError::bad_request("to account must differ from the source account"));
        }

        match resolve_transfer(user_id, &src, to_account_id, &self.transaction_repo, &self.account_repo).await? {
            ResolveOutcome::Noop => Err(ApiError::bad_request("source transaction is already linked")),
            ResolveOutcome::Linked(counterpart_id) => {
                if src.transaction_type == TransactionType::Debit {
                    Ok(CreateTransferResp { debit_transaction_id: src.id, credit_transaction_id: counterpart_id })
                } else {
                    Ok(CreateTransferResp { debit_transaction_id: counterpart_id, credit_transaction_id: src.id })
                }
            }
        }
    }

    pub async fn link_transfers(&self, user_id: &str, links: &[(String, String)]) -> Result<BulkResult> {
        if links.is_empty() {
            return Err(ApiError::bad_request("no links provided"));
        }
        let mut inputs: Vec<(String, String)> = Vec::new();
        let mut failed: Vec<String> = Vec::new();
        for (i, (debit_id, credit_id)) in links.iter().enumerate() {
            if debit_id.is_empty() || credit_id.is_empty() {
                failed.push(format!("link {i}: both debit and credit transaction IDs are required"));
                continue;
            }
            if debit_id == credit_id {
                failed.push(format!("link {i}: debit and credit transactions must be different"));
                continue;
            }
            let Some((dt, _, da)) = self.transaction_repo.get_by_id_for_transfer(debit_id).await? else {
                failed.push(format!("debit transaction {debit_id} not found"));
                continue;
            };
            let Some((ct, _, ca)) = self.transaction_repo.get_by_id_for_transfer(credit_id).await? else {
                failed.push(format!("credit transaction {credit_id} not found"));
                continue;
            };
            if dt != 0 {
                failed.push(format!("transaction {debit_id} is not a DEBIT"));
                continue;
            }
            if ct != 1 {
                failed.push(format!("transaction {credit_id} is not a CREDIT"));
                continue;
            }
            if da == ca {
                failed.push(format!("link {i}: debit and credit must be from different accounts"));
                continue;
            }
            let debit_linked = transaction::is_transaction_linked(&self.transaction_repo.pool, debit_id).await?;
            let credit_linked = transaction::is_transaction_linked(&self.transaction_repo.pool, credit_id).await?;
            if debit_linked || credit_linked {
                failed.push(format!("one or both transactions in link {i} are already linked"));
                continue;
            }
            inputs.push((debit_id.clone(), credit_id.clone()));
        }

        let errs = transaction::create_links(&self.transaction_repo.pool, user_id, &inputs).await?;
        failed.extend(errs);
        if !failed.is_empty() {
            return Ok(BulkResult { success: false, message: "some transfers failed".to_string(), failed_ids: failed, skipped: 0 });
        }
        Ok(BulkResult { success: true, message: "transfers linked successfully".to_string(), failed_ids: Vec::new(), skipped: 0 })
    }

    pub async fn unlink_transfers(&self, ids: &[String]) -> Result<BulkResult> {
        if ids.is_empty() {
            return Err(ApiError::bad_request("no ids provided"));
        }
        let errs = transaction::delete_links(&self.transaction_repo.pool, ids).await?;
        if !errs.is_empty() {
            return Ok(BulkResult { success: false, message: "some unlinks failed".to_string(), failed_ids: errs, skipped: 0 });
        }
        Ok(BulkResult { success: true, message: "transfer links removed successfully".to_string(), failed_ids: Vec::new(), skipped: 0 })
    }
}

enum ResolveOutcome {
    Noop,
    Linked(String),
}

/// Find or create the counterpart of `src` in `target_account_id` and link
/// them. Same-account and already-linked transactions are no-ops.
async fn resolve_transfer(
    user_id: &str,
    src: &Transaction,
    target_account_id: &str,
    transaction_repo: &TransactionRepo,
    account_repo: &AccountRepo,
) -> Result<ResolveOutcome> {
    if src.account_id == target_account_id || src.transfer_linked {
        return Ok(ResolveOutcome::Noop);
    }
    let opposite = if src.transaction_type == TransactionType::Credit { TransactionType::Debit } else { TransactionType::Credit };

    if let Some(cand) = transaction_repo
        .find_transfer_counterpart(user_id, target_account_id, opposite, src.amount, &src.occurred_at)
        .await?
    {
        let (debit_id, credit_id) = if src.transaction_type == TransactionType::Debit {
            (src.id.clone(), cand.id.clone())
        } else {
            (cand.id.clone(), src.id.clone())
        };
        let errs = transaction::create_links(&transaction_repo.pool, user_id, &[(debit_id, credit_id)]).await?;
        if let Some(e) = errs.first() {
            return Err(ApiError::internal(e.clone()));
        }
        return Ok(ResolveOutcome::Linked(cand.id));
    }

    // No unlinked counterpart found. The other side may have just been linked
    // while we were holding a stale row — check before creating a duplicate.
    if transaction_repo.is_transfer_linked(&src.id).await? {
        return Ok(ResolveOutcome::Noop);
    }

    let counter_txn = Transaction {
        id: Uuid::new_v4().to_string(),
        name: format!("Transfer from {}", account_display(&src.account_id, account_repo).await),
        amount: src.amount,
        transaction_type: opposite,
        account_id: target_account_id.to_string(),
        occurred_at: src.occurred_at.clone(),
        created_at: go_ts(chrono::Utc::now()),
        external_id: None,
        transfer_linked: false,
    };
    let outcome = transaction_repo
        .create(user_id, &[transaction::CreateTransactionInput { txn: counter_txn.clone(), category_ids: Vec::new() }])
        .await?;
    if let Some(e) = outcome.errors.first() {
        return Err(ApiError::internal(e.clone()));
    }
    if !outcome.inserted.contains(&counter_txn.id) {
        return Ok(ResolveOutcome::Noop);
    }
    account_repo.update_balance(&counter_txn.account_id, if opposite == TransactionType::Credit { src.amount } else { -src.amount }).await?;
    let (c, d) = if opposite == TransactionType::Credit { (src.amount, 0.0) } else { (0.0, src.amount) };
    account_repo.apply_totals(&counter_txn.account_id, c, d).await?;

    let (debit_id, credit_id) = if src.transaction_type == TransactionType::Debit {
        (src.id.clone(), counter_txn.id.clone())
    } else {
        (counter_txn.id.clone(), src.id.clone())
    };
    let errs = transaction::create_links(&transaction_repo.pool, user_id, &[(debit_id, credit_id)]).await?;
    if let Some(e) = errs.first() {
        return Err(ApiError::internal(e.clone()));
    }
    Ok(ResolveOutcome::Linked(counter_txn.id))
}

async fn account_display(account_id: &str, account_repo: &AccountRepo) -> String {
    match account_repo.get_by_id(account_id).await {
        Ok(Some(a)) if !a.account_nickname.is_empty() => a.account_nickname,
        Ok(Some(a)) => a.bank_name,
        _ => account_id.to_string(),
    }
}
