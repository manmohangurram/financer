//! Accounts repository — CRUD SQL mirroring the Go backend's `repository/account.go`.

use sqlx::SqlitePool;
use uuid::Uuid;

use crate::error::Result;

#[derive(Clone)]
pub struct AccountRepo {
    pub pool: SqlitePool,
}

pub struct AccountRow {
    pub id: String,
    pub bank_name: String,
    pub account_nickname: String,
    pub balance: f64,
    pub account_type: i64,
    pub created_at: String,
}

impl AccountRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    /// Insert a new account; returns the row. A `0` account type is coerced to
    /// CHECKING (1), matching Go's default.
    #[allow(clippy::cast_precision_loss)]
    pub async fn create(
        &self,
        user_id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: i64,
    ) -> Result<AccountRow> {
        let id = Uuid::new_v4().to_string();
        let account_type = if account_type == 0 { 1 } else { account_type };
        let now = go_ts(chrono::Utc::now());

        sqlx::query(
            "INSERT INTO accounts (id, user_id, bank_name, account_nickname, balance, account_type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
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
            account_nickname: nickname.to_string(),
            balance: 0.0,
            account_type,
            created_at: now,
        })
    }

    pub async fn get_by_id(&self, id: &str) -> Result<Option<AccountRow>> {
        let row = sqlx::query_as::<_, RawAccount>(
            "SELECT id, bank_name, account_nickname, balance, account_type, created_at FROM accounts WHERE id = ?",
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
        account_type: i64,
    ) -> Result<Option<AccountRow>> {
        let result = sqlx::query(
            "UPDATE accounts SET
                bank_name = COALESCE(NULLIF(?, ''), bank_name),
                account_nickname = COALESCE(NULLIF(?, ''), account_nickname),
                account_type = CASE WHEN ? != 0 THEN ? ELSE account_type END
             WHERE id = ?",
        )
        .bind(bank_name)
        .bind(nickname)
        .bind(account_type)
        .bind(account_type)
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
            "SELECT id, bank_name, account_nickname, balance, account_type, created_at FROM accounts WHERE user_id = ? ORDER BY created_at DESC",
        )
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }
}

/// Format a timestamp the way the Go sqlite driver stores `time.Time` in the
/// shared DB file, so Rust and Go agree byte-for-byte on `created_at`.
fn go_ts(utc: chrono::DateTime<chrono::Utc>) -> String {
    utc.format("%Y-%m-%d %H:%M:%S +0000 UTC").to_string()
}

/// Parse a stored `created_at` back to RFC3339 UTC (matching Go's `tsRFC3339`).
/// Handles both the Go driver format and plain `YYYY-MM-DD HH:MM:SS`.
pub fn ts_rfc3339(stored: &str) -> String {
    let s = stored.trim();
    // Go's driver stores "YYYY-MM-DD HH:MM:SS +0000 UTC"; the trailing zone
    // suffix isn't part of the wall clock, so drop it for naive parsing.
    let naive_str = s.split(' ').take(2).collect::<Vec<_>>().join(" ");
    if let Ok(naive) = chrono::NaiveDateTime::parse_from_str(&naive_str, "%Y-%m-%d %H:%M:%S") {
        // Go's tsRFC3339 = UTC().Format(time.RFC3339) → "...T..Z", not "+00:00".
        return naive.and_utc().format("%Y-%m-%dT%H:%M:%SZ").to_string();
    }
    if let Ok(dt) = chrono::DateTime::parse_from_rfc3339(s) {
        return dt.with_timezone(&chrono::Utc).format("%Y-%m-%dT%H:%M:%SZ").to_string();
    }
    s.to_string()
}

#[derive(sqlx::FromRow)]
struct RawAccount {
    id: String,
    bank_name: String,
    account_nickname: String,
    balance: f64,
    account_type: i64,
    created_at: String,
}

impl From<RawAccount> for AccountRow {
    fn from(r: RawAccount) -> Self {
        Self {
            id: r.id,
            bank_name: r.bank_name,
            account_nickname: r.account_nickname,
            balance: r.balance,
            account_type: r.account_type,
            created_at: r.created_at,
        }
    }
}

#[cfg(test)]
mod tests {
    use std::str::FromStr;

    use super::*;

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
        let a = repo.create("u1", "Chase", "Main", 1).await.unwrap();
        repo.create("u1", "Amex", "", 3).await.unwrap();
        repo.create("u2", "Other", "", 2).await.unwrap();

        let list = repo.list("u1").await.unwrap();
        assert_eq!(list.len(), 2, "only u1's accounts");
        let mut types: Vec<i64> = list.iter().map(|a| a.account_type).collect();
        types.sort();
        assert_eq!(types, vec![1, 3], "both of u1's accounts");

        let upd = repo.update(&a.id, "Chase Blue", "New", 2).await.unwrap().unwrap();
        assert_eq!(upd.bank_name, "Chase Blue");
        assert_eq!(upd.account_nickname, "New");
        assert_eq!(upd.account_type, 2);

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
}

