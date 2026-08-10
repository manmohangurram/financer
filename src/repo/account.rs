//! Accounts repository — CRUD SQL mirroring the Go backend's `repository/account.go`.

use serde::Serialize;
use sqlx::SqlitePool;
use uuid::Uuid;

use crate::error::Result;
use crate::timex::go_ts;

/// Account type. Stored as its integer discriminant; the wire name keeps Go's
/// `ACCOUNT_TYPE_*` shape via `#[serde(rename)]`. No "unspecified" sentinel —
/// a bad/missing value is rejected with 400 at the boundary.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[repr(i64)]
pub enum AccountType {
    #[serde(rename = "ACCOUNT_TYPE_CHECKING")]
    Checking = 1,
    #[serde(rename = "ACCOUNT_TYPE_SAVINGS")]
    Savings = 2,
    #[serde(rename = "ACCOUNT_TYPE_CREDIT_CARD")]
    CreditCard = 3,
    #[serde(rename = "ACCOUNT_TYPE_LOAN")]
    Loan = 4,
}

impl TryFrom<i64> for AccountType {
    type Error = ();
    fn try_from(v: i64) -> std::result::Result<Self, Self::Error> {
        match v {
            1 => Ok(Self::Checking),
            2 => Ok(Self::Savings),
            3 => Ok(Self::CreditCard),
            4 => Ok(Self::Loan),
            _ => Err(()),
        }
    }
}

impl AccountType {
    /// Look up a type by its wire name (`"ACCOUNT_TYPE_CHECKING"`).
    pub fn from_wire(s: &str) -> Option<Self> {
        match s {
            "ACCOUNT_TYPE_CHECKING" => Some(Self::Checking),
            "ACCOUNT_TYPE_SAVINGS" => Some(Self::Savings),
            "ACCOUNT_TYPE_CREDIT_CARD" => Some(Self::CreditCard),
            "ACCOUNT_TYPE_LOAN" => Some(Self::Loan),
            _ => None,
        }
    }

    /// Integer stored in the `type` column.
    pub fn db_value(self) -> i64 {
        self as i64
    }
}

#[derive(Clone)]
pub struct AccountRepo {
    pub pool: SqlitePool,
}

pub struct AccountRow {
    pub id: String,
    pub bank_name: String,
    pub account_nickname: String,
    pub balance: f64,
    pub r#type: AccountType,
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
        r#type: AccountType,
    ) -> Result<AccountRow> {
        let id = Uuid::new_v4().to_string();
        let now = go_ts(chrono::Utc::now());

        sqlx::query(
            "INSERT INTO accounts (id, user_id, bank_name, account_nickname, balance, type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
        )
        .bind(&id)
        .bind(user_id)
        .bind(bank_name)
        .bind(nickname)
        .bind(0.0f64)
        .bind(r#type.db_value())
        .bind(&now)
        .execute(&self.pool)
        .await?;

        Ok(AccountRow {
            id,
            bank_name: bank_name.to_string(),
            account_nickname: nickname.to_string(),
            balance: 0.0,
            r#type,
            created_at: now,
        })
    }

    pub async fn get_by_id(&self, id: &str) -> Result<Option<AccountRow>> {
        let row = sqlx::query_as::<_, RawAccount>(
            "SELECT id, bank_name, account_nickname, balance, type, created_at FROM accounts WHERE id = ?",
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
        r#type: AccountType,
    ) -> Result<Option<AccountRow>> {
        let v = r#type.db_value();
        let result = sqlx::query(
            "UPDATE accounts SET
                bank_name = COALESCE(NULLIF(?, ''), bank_name),
                account_nickname = COALESCE(NULLIF(?, ''), account_nickname),
                type = CASE WHEN ? != 0 THEN ? ELSE type END
             WHERE id = ?",
        )
        .bind(bank_name)
        .bind(nickname)
        .bind(v)
        .bind(v)
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
            "SELECT id, bank_name, account_nickname, balance, type, created_at FROM accounts WHERE user_id = ? ORDER BY created_at DESC",
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
}

#[derive(sqlx::FromRow)]
struct RawAccount {
    id: String,
    bank_name: String,
    account_nickname: String,
    balance: f64,
    #[sqlx(rename = "type")]
    r#type: i64,
    created_at: String,
}

impl From<RawAccount> for AccountRow {
    fn from(r: RawAccount) -> Self {
        Self {
            id: r.id,
            bank_name: r.bank_name,
            account_nickname: r.account_nickname,
            balance: r.balance,
            r#type: AccountType::try_from(r.r#type).unwrap_or(AccountType::Checking),
            created_at: r.created_at,
        }
    }
}

#[cfg(test)]
mod tests {
    use std::str::FromStr;

    use super::*;
    use crate::timex::ts_rfc3339;

    async fn test_pool() -> SqlitePool {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("test.db");
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
        let a = repo.create("u1", "Chase", "Main", AccountType::Checking).await.unwrap();
        repo.create("u1", "Amex", "", AccountType::CreditCard).await.unwrap();
        repo.create("u2", "Other", "", AccountType::Savings).await.unwrap();

        let list = repo.list("u1").await.unwrap();
        assert_eq!(list.len(), 2, "only u1's accounts");
        let mut types: Vec<i64> = list.iter().map(|a| a.r#type.db_value()).collect();
        types.sort();
        assert_eq!(types, vec![1, 3], "both of u1's accounts");

        let upd = repo.update(&a.id, "Chase Blue", "New", AccountType::Savings).await.unwrap().unwrap();
        assert_eq!(upd.bank_name, "Chase Blue");
        assert_eq!(upd.account_nickname, "New");
        assert_eq!(upd.r#type, AccountType::Savings);

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
        let a = repo.create("u1", "Chase", "", AccountType::Checking).await.unwrap();

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

