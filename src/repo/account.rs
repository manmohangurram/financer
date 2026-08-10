//! Accounts repository — CRUD SQL mirroring the Go backend's `repository/account.go`.

use serde::{Deserialize, Serialize};
use sqlx::SqlitePool;
use std::str::FromStr;
use uuid::Uuid;

use crate::error::Result;
use crate::timex::go_ts;

/// Account type. Single source of truth: serde emits the uppercase wire value,
/// sqlx stores the same string in the `type` TEXT column. No "unspecified"
/// sentinel — a bad/missing value is rejected with 400 at the boundary.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type, strum::Display, strum::EnumString,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[sqlx(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
pub enum AccountType {
    Current,
    Savings,
    Loan,
    CreditCard,
}

#[derive(Clone)]
pub struct AccountRepo {
    pub pool: SqlitePool,
}

pub struct AccountRow {
    pub id: String,
    pub bank_name: String,
    pub nickname: String,
    pub balance: f64,
    pub account_type: AccountType,
    pub created_at: String,
}

impl AccountRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    /// Insert a new account; returns the row.
    #[allow(clippy::cast_precision_loss)]
    pub async fn create(
        &self,
        user_id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: AccountType,
    ) -> Result<AccountRow> {
        let id = Uuid::new_v4().to_string();
        let now = go_ts(chrono::Utc::now());

        sqlx::query(
            "INSERT INTO accounts (id, user_id, bank_name, nickname, balance, type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
        )
        .bind(&id)
        .bind(user_id)
        .bind(bank_name)
        .bind(nickname)
        .bind(0.0f64)
        .bind(account_type)
        .bind(&now)
        .execute(&self.pool)
        .await?;

        Ok(AccountRow {
            id,
            bank_name: bank_name.to_string(),
            nickname: nickname.to_string(),
            balance: 0.0,
            account_type,
            created_at: now,
        })
    }

    pub async fn get_by_id(&self, id: &str) -> Result<Option<AccountRow>> {
        let row = sqlx::query_as::<_, RawAccount>(
            "SELECT id, bank_name, nickname, balance, type, created_at FROM accounts WHERE id = ?",
        )
        .bind(id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    pub async fn update(
        &self,
        id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: AccountType,
    ) -> Result<Option<AccountRow>> {
        let v = account_type.to_string();
        let result = sqlx::query(
            "UPDATE accounts SET
                bank_name = COALESCE(NULLIF(?, ''), bank_name),
                nickname = COALESCE(NULLIF(?, ''), nickname),
                type = ?
             WHERE id = ?",
        )
        .bind(bank_name)
        .bind(nickname)
        .bind(&v)
        .bind(id)
        .execute(&self.pool)
        .await?;

        if result.rows_affected() == 0 {
            return Ok(None);
        }
        self.get_by_id(id).await
    }

    pub async fn delete(&self, id: &str) -> Result<bool> {
        let result = sqlx::query("DELETE FROM accounts WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<AccountRow>> {
        let rows = sqlx::query_as::<_, RawAccount>(
            "SELECT id, bank_name, nickname, balance, type, created_at FROM accounts WHERE user_id = ? ORDER BY created_at DESC",
        )
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    /// Apply a signed delta to an account's stored balance.
    pub async fn update_balance(&self, account_id: &str, delta: f64) -> Result<()> {
        sqlx::query(
            "UPDATE accounts SET balance = ROUND(COALESCE(balance, 0) + ?, 2) WHERE id = ?",
        )
        .bind(delta)
        .bind(account_id)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    /// Apply signed deltas to an account's cached credit/debit totals.
    pub async fn apply_totals(&self, account_id: &str, credit: f64, debit: f64) -> Result<()> {
        sqlx::query(
            "UPDATE accounts SET
                total_credit = ROUND(COALESCE(total_credit, 0) + ?, 2),
                total_debit  = ROUND(COALESCE(total_debit, 0) + ?, 2)
             WHERE id = ?",
        )
        .bind(credit)
        .bind(debit)
        .bind(account_id)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    /// Total stored balance across a user's accounts.
    pub async fn sum_balance(&self, user_id: &str) -> Result<f64> {
        let total: Option<f64> = sqlx::query_scalar("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE user_id = ?")
            .bind(user_id)
            .fetch_one(&self.pool)
            .await?;
        Ok(total.unwrap_or(0.0))
    }

    /// Cached total credits and debits across a user's accounts.
    pub async fn sum_totals(&self, user_id: &str) -> Result<(f64, f64)> {
        let row: (f64, f64) = sqlx::query_as(
            "SELECT COALESCE(SUM(total_credit), 0), COALESCE(SUM(total_debit), 0) FROM accounts WHERE user_id = ?",
        )
        .bind(user_id)
        .fetch_one(&self.pool)
        .await?;
        Ok(row)
    }
}

#[derive(sqlx::FromRow)]
struct RawAccount {
    id: String,
    bank_name: String,
    nickname: String,
    balance: f64,
    #[sqlx(rename = "type")]
    account_type: String,
    created_at: String,
}

impl From<RawAccount> for AccountRow {
    fn from(r: RawAccount) -> Self {
        Self {
            id: r.id,
            bank_name: r.bank_name,
            nickname: r.nickname,
            balance: r.balance,
            account_type: AccountType::from_str(&r.account_type).unwrap_or(AccountType::Current),
            created_at: r.created_at,
        }
    }
}

#[cfg(test)]
mod tests {
    use std::str::FromStr;

    use super::*;
    use crate::timex::ts_rfc3339;

    #[test]
    fn rejects_unknown_type() {
        assert!(AccountType::from_str("bogus").is_err());
        assert_eq!(AccountType::from_str("CURRENT"), Ok(AccountType::Current));
        assert_eq!(AccountType::from_str("CREDIT_CARD"), Ok(AccountType::CreditCard));
    }

    async fn test_pool() -> SqlitePool {
        let dir = tempfile::tempdir().unwrap();
        // Keep the temp dir alive for the pool's lifetime (auto-delete on drop
        // would unlink the DB file before the lazy pool opens it).
        let path = dir.into_path().join("test.db");
        let opt = sqlx::sqlite::SqliteConnectOptions::from_str(path.to_str().unwrap())
            .unwrap()
            .create_if_missing(true)
            .journal_mode(sqlx::sqlite::SqliteJournalMode::Wal)
            .foreign_keys(true);
        let pool = SqlitePool::connect_with(opt).await.unwrap();
        crate::db::run_migrations(&pool, std::path::Path::new("db/migrations"))
            .await
            .unwrap();
        pool
    }

    #[tokio::test]
    async fn crud_scoped_to_user() {
        let pool = test_pool().await;
        let repo = AccountRepo::new(pool.clone());
        for uid in ["u1", "u2"] {
            sqlx::query("INSERT INTO users (id, email, password_hash, name) VALUES (?, ?, ?, ?)")
                .bind(uid)
                .bind(format!("{uid}@x.com"))
                .bind("hash")
                .bind(uid)
                .execute(&pool)
                .await
                .unwrap();
        }
        let a = repo.create("u1", "Chase", "Main", AccountType::Current).await.unwrap();
        repo.create("u1", "Amex", "", AccountType::CreditCard).await.unwrap();
        repo.create("u2", "Other", "", AccountType::Savings).await.unwrap();

        let list = repo.list("u1").await.unwrap();
        assert_eq!(list.len(), 2, "only u1's accounts");
        let mut types: Vec<String> = list.iter().map(|a| a.account_type.to_string()).collect();
        types.sort();
        assert_eq!(types, vec!["CREDIT_CARD", "CURRENT"], "both of u1's accounts");

        let upd = repo.update(&a.id, "Chase Blue", "New", AccountType::Savings).await.unwrap().unwrap();
        assert_eq!(upd.bank_name, "Chase Blue");
        assert_eq!(upd.nickname, "New");
        assert_eq!(upd.account_type, AccountType::Savings);

        assert!(repo.delete(&a.id).await.unwrap());
        assert!(!repo.delete(&a.id).await.unwrap());
        assert_eq!(repo.list("u1").await.unwrap().len(), 1);
    }

    #[test]
    fn ts_conversion() {
        assert_eq!(ts_rfc3339("2024-01-02 03:04:05 +0000 UTC"), "2024-01-02T03:04:05Z");
        assert_eq!(ts_rfc3339("2024-01-02 03:04:05"), "2024-01-02T03:04:05Z");
        assert_eq!(ts_rfc3339("2024-01-02T03:04:05Z"), "2024-01-02T03:04:05Z");
    }

    #[tokio::test]
    async fn balance_and_totals_deltas() {
        let pool = test_pool().await;
        let repo = AccountRepo::new(pool.clone());
        sqlx::query("INSERT INTO users (id, email, password_hash, name) VALUES ('u1', 'u1@x.com', 'h', 'u1')")
            .execute(&pool)
            .await
            .unwrap();
        let a = repo.create("u1", "Chase", "", AccountType::Current).await.unwrap();

        repo.update_balance(&a.id, -5.5).await.unwrap();
        repo.update_balance(&a.id, 10.0).await.unwrap();
        repo.apply_totals(&a.id, 10.0, 5.5).await.unwrap();

        let row: (f64, f64, f64) = sqlx::query_as("SELECT balance, total_credit, total_debit FROM accounts WHERE id = ?")
            .bind(&a.id)
            .fetch_one(&pool)
            .await
            .unwrap();
        assert_eq!(row.0, 4.5);
        assert_eq!(row.1, 10.0);
        assert_eq!(row.2, 5.5);
    }
}

