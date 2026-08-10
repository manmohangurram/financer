//! Accounts service — business logic mirroring Go's `services/account.go`.

use serde::Serialize;

use crate::error::{ApiError, Result};
use crate::repo::account::{AccountRepo, AccountRow, AccountType};
use crate::timex::ts_rfc3339;

#[derive(Clone)]
pub struct AccountService {
    repo: AccountRepo,
}

impl AccountService {
    pub fn new(repo: AccountRepo) -> Self {
        Self { repo }
    }

    pub async fn create(&self, user_id: &str, bank_name: &str, nickname: &str, account_type: AccountType) -> Result<AccountResponse> {
        if bank_name.is_empty() {
            return Err(ApiError::bad_request("bank_name is required"));
        }
        let row = self.repo.create(user_id, bank_name, nickname, account_type).await?;
        Ok(AccountResponse::from_row(row))
    }

    pub async fn update(&self, user_id: &str, id: &str, bank_name: &str, nickname: &str, account_type: AccountType) -> Result<AccountResponse> {
        let row = self
            .repo
            .update(user_id, id, bank_name, nickname, account_type)
            .await?
            .ok_or_else(|| ApiError::not_found(format!("account {id} not found")))?;
        Ok(AccountResponse::from_row(row))
    }

    pub async fn delete(&self, user_id: &str, id: &str) -> Result<()> {
        if !self.repo.delete(user_id, id).await? {
            return Err(ApiError::not_found(format!("account {id} not found")));
        }
        Ok(())
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<AccountResponse>> {
        Ok(self.repo.list(user_id).await?.into_iter().map(AccountResponse::from_row).collect())
    }
}

/// Wire response, serialized directly (Rust serde covers Go's wire mapping).
#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct AccountResponse {
    pub id: String,
    pub bank_name: String,
    pub nickname: String,
    #[serde(rename = "type")]
    pub account_type: AccountType,
    #[serde(serialize_with = "round2")]
    pub balance: f64,
    pub created_at: String,
}

// serde's serialize_with requires fn(&T, S).
#[allow(clippy::trivially_copy_pass_by_ref)]
fn round2<S: serde::Serializer>(v: &f64, s: S) -> std::result::Result<S::Ok, S::Error> {
    s.serialize_f64((v * 100.0).round() / 100.0)
}

impl AccountResponse {
    fn from_row(r: AccountRow) -> Self {
        Self {
            id: r.id,
            bank_name: r.bank_name,
            nickname: r.nickname,
            account_type: r.account_type,
            balance: r.balance,
            created_at: ts_rfc3339(&r.created_at),
        }
    }
}
