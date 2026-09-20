//! Accounts service — business logic mirroring Go's `services/account.go`.

use std::sync::Arc;

use serde::Serialize;
use utoipa::ToSchema;

use crate::error::{ApiError, Result};
use crate::repo::traits::account::{AccountRow, AccountType};
use crate::repo::traits::AccountRepo;
use crate::utils::timex::ts_rfc3339;

#[derive(Clone)]
pub struct AccountService {
    repo: Arc<dyn AccountRepo>,
}

impl AccountService {
    pub fn new(repo: Arc<dyn AccountRepo>) -> Self {
        Self { repo }
    }

    pub async fn create(
        &self,
        user_id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: AccountType,
        ending_numbers: &str,
    ) -> Result<AccountResponse> {
        if bank_name.is_empty() {
            return Err(ApiError::bad_request("bank_name is required"));
        }
        validate_ending_numbers(ending_numbers)?;
        let row = self.repo.create(user_id, bank_name, nickname, account_type, ending_numbers).await?;
        Ok(AccountResponse::from_row(row))
    }

    pub async fn update(
        &self,
        user_id: &str,
        id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: AccountType,
        ending_numbers: &str,
    ) -> Result<AccountResponse> {
        if !ending_numbers.is_empty() {
            validate_ending_numbers(ending_numbers)?;
        }
        let row = self
            .repo
            .update(user_id, id, bank_name, nickname, account_type, ending_numbers)
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

/// Trailing digits of the account or card number: exactly 4, digits only.
/// Card alerts name the card, not the account, so this is not account-specific.
fn validate_ending_numbers(v: &str) -> Result<()> {
    if v.len() == 4 && v.bytes().all(|b| b.is_ascii_digit()) {
        Ok(())
    } else {
        Err(ApiError::bad_request("endingNumbers must be exactly 4 digits"))
    }
}

/// Wire response, serialized directly (Rust serde covers Go's wire mapping).
#[derive(Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct AccountResponse {
    pub id: String,
    pub bank_name: String,
    pub nickname: String,
    #[serde(rename = "type")]
    pub account_type: AccountType,
    #[serde(serialize_with = "round2")]
    pub balance: f64,
    pub ending_numbers: Option<String>,
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
            ending_numbers: r.ending_numbers,
            created_at: ts_rfc3339(&r.created_at),
        }
    }
}
