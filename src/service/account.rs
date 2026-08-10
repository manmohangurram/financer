//! Accounts service — business logic mirroring Go's `services/account.go`.

use crate::error::{ApiError, Result};
use crate::repo::account::{ts_rfc3339, AccountRepo, AccountRow};

#[derive(Clone)]
pub struct AccountService {
    repo: AccountRepo,
}

impl AccountService {
    pub fn new(repo: AccountRepo) -> Self {
        Self { repo }
    }

    pub async fn create(&self, user_id: &str, bank_name: &str, nickname: &str, account_type: i64) -> Result<AccountResponse> {
        if bank_name.is_empty() {
            return Err(ApiError::bad_request("bank_name is required"));
        }
        let row = self.repo.create(user_id, bank_name, nickname, account_type).await?;
        Ok(AccountResponse::from_row(row))
    }

    pub async fn update(&self, id: &str, bank_name: &str, nickname: &str, account_type: i64) -> Result<AccountResponse> {
        let row = self
            .repo
            .update(id, bank_name, nickname, account_type)
            .await?
            .ok_or_else(|| ApiError::not_found(format!("account {id} not found")))?;
        Ok(AccountResponse::from_row(row))
    }

    pub async fn delete(&self, id: &str) -> Result<()> {
        if !self.repo.delete(id).await? {
            return Err(ApiError::not_found(format!("account {id} not found")));
        }
        Ok(())
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<AccountResponse>> {
        Ok(self.repo.list(user_id).await?.into_iter().map(AccountResponse::from_row).collect())
    }
}

pub struct AccountResponse {
    pub id: String,
    pub bank_name: String,
    pub account_nickname: String,
    pub account_type: i64,
    pub balance: f64,
    pub created_at: String,
}

impl AccountResponse {
    fn from_row(r: AccountRow) -> Self {
        Self {
            id: r.id,
            bank_name: r.bank_name,
            account_nickname: r.account_nickname,
            account_type: r.account_type,
            balance: r.balance,
            created_at: ts_rfc3339(&r.created_at),
        }
    }
}
