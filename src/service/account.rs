//! Accounts service — business logic mirroring Go's `services/account.go`.

use serde::Serialize;

use crate::error::{ApiError, Result};
use crate::repo::account::{AccountRepo, AccountRow, AccountType};
use crate::timex::ts_rfc3339;

/// Parse the `accountType` wire value: enum name string or numeric. Anything
/// else (missing, unknown, out of range) is a 400 — no implicit default.
pub fn type_value(v: &serde_json::Value) -> std::result::Result<AccountType, ApiError> {
    match v {
        serde_json::Value::Number(n) => n
            .as_i64()
            .and_then(|n| AccountType::try_from(n).ok())
            .ok_or_else(|| ApiError::bad_request("invalid account type")),
        serde_json::Value::String(s) => AccountType::from_wire(s)
            .ok_or_else(|| ApiError::bad_request(format!("unknown account type {s:?}"))),
        _ => Err(ApiError::bad_request("invalid account type")),
    }
}

#[derive(Clone)]
pub struct AccountService {
    repo: AccountRepo,
}

impl AccountService {
    pub fn new(repo: AccountRepo) -> Self {
        Self { repo }
    }

    pub async fn create(&self, user_id: &str, bank_name: &str, nickname: &str, r#type: AccountType) -> Result<AccountResponse> {
        if bank_name.is_empty() {
            return Err(ApiError::bad_request("bank_name is required"));
        }
        let row = self.repo.create(user_id, bank_name, nickname, r#type).await?;
        Ok(AccountResponse::from_row(row))
    }

    pub async fn update(&self, id: &str, bank_name: &str, nickname: &str, r#type: AccountType) -> Result<AccountResponse> {
        let row = self
            .repo
            .update(id, bank_name, nickname, r#type)
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

/// Wire response, serialized directly (Rust serde covers Go's wire mapping).
#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct AccountResponse {
    pub id: String,
    pub bank_name: String,
    pub account_nickname: String,
    #[serde(rename = "accountType")]
    pub r#type: AccountType,
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
            account_nickname: r.account_nickname,
            r#type: r.r#type,
            balance: r.balance,
            created_at: ts_rfc3339(&r.created_at),
        }
    }
}
