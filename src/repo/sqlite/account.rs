//! `SQLite` accounts repository — CRUD SQL, implementing
//! `crate::repo::traits::AccountRepo` over an `sqlx::SqlitePool`.

use async_trait::async_trait;
use sqlx::SqlitePool;
use std::str::FromStr;
use uuid::Uuid;

use crate::error::Result;
use crate::repo::traits::account::{AccountRow, AccountType};

pub struct SqliteAccountRepo {
    pool: SqlitePool,
}

impl SqliteAccountRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    #[allow(clippy::cast_precision_loss)]
    async fn create_inner(
        &self,
        user_id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: AccountType,
    ) -> Result<AccountRow> {
        let id = Uuid::new_v4().to_string();
        let now = crate::utils::timex::now_go_ts();
        sqlx::query(
            "INSERT INTO accounts (id, user_id, bank_name, nickname, balance, type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
        )
        .bind(&id)
        .bind(user_id)
        .bind(bank_name)
        .bind(nickname)
        .bind(0.0f64)
        .bind(account_type.to_string())
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

    async fn get_by_id_inner(&self, user_id: &str, id: &str) -> Result<Option<AccountRow>> {
        let row = sqlx::query_as::<_, RawAccount>(
            "SELECT id, bank_name, nickname, balance, type, created_at FROM accounts WHERE id = ? AND user_id = ?",
        )
        .bind(id)
        .bind(user_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    async fn update_inner(
        &self,
        user_id: &str,
        id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: AccountType,
    ) -> Result<Option<AccountRow>> {
        let result = sqlx::query(
            "UPDATE accounts SET
                bank_name = COALESCE(NULLIF(?, ''), bank_name),
                nickname = COALESCE(NULLIF(?, ''), nickname),
                type = ?
             WHERE id = ? AND user_id = ?",
        )
        .bind(bank_name)
        .bind(nickname)
        .bind(account_type.to_string())
        .bind(id)
        .bind(user_id)
        .execute(&self.pool)
        .await?;
        if result.rows_affected() == 0 {
            return Ok(None);
        }
        self.get_by_id_inner(user_id, id).await
    }

    async fn delete_inner(&self, user_id: &str, id: &str) -> Result<bool> {
        let result = sqlx::query("DELETE FROM accounts WHERE id = ? AND user_id = ?")
            .bind(id)
            .bind(user_id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    async fn list_inner(&self, user_id: &str) -> Result<Vec<AccountRow>> {
        let rows = sqlx::query_as::<_, RawAccount>(
            "SELECT id, bank_name, nickname, balance, type, created_at FROM accounts WHERE user_id = ? ORDER BY created_at DESC",
        )
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    async fn update_balance_inner(&self, account_id: &str, delta: f64) -> Result<()> {
        sqlx::query(
            "UPDATE accounts SET balance = ROUND(COALESCE(balance, 0) + ?, 2) WHERE id = ?",
        )
        .bind(delta)
        .bind(account_id)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    async fn apply_totals_inner(&self, account_id: &str, credit: f64, debit: f64) -> Result<()> {
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

    async fn sum_balance_inner(&self, user_id: &str) -> Result<f64> {
        let total: Option<f64> =
            sqlx::query_scalar("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE user_id = ?")
                .bind(user_id)
                .fetch_one(&self.pool)
                .await?;
        Ok(total.unwrap_or(0.0))
    }

    async fn sum_totals_inner(&self, user_id: &str) -> Result<(f64, f64)> {
        let row: (f64, f64) = sqlx::query_as(
            "SELECT COALESCE(SUM(total_credit), 0), COALESCE(SUM(total_debit), 0) FROM accounts WHERE user_id = ?",
        )
        .bind(user_id)
        .fetch_one(&self.pool)
        .await?;
        Ok(row)
    }
}

#[async_trait]
impl crate::repo::traits::AccountRepo for SqliteAccountRepo {
    async fn create(
        &self,
        user_id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: AccountType,
    ) -> Result<AccountRow> {
        self.create_inner(user_id, bank_name, nickname, account_type)
            .await
    }
    async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<AccountRow>> {
        self.get_by_id_inner(user_id, id).await
    }
    async fn update(
        &self,
        user_id: &str,
        id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: AccountType,
    ) -> Result<Option<AccountRow>> {
        self.update_inner(user_id, id, bank_name, nickname, account_type)
            .await
    }
    async fn delete(&self, user_id: &str, id: &str) -> Result<bool> {
        self.delete_inner(user_id, id).await
    }
    async fn list(&self, user_id: &str) -> Result<Vec<AccountRow>> {
        self.list_inner(user_id).await
    }
    async fn update_balance(&self, account_id: &str, delta: f64) -> Result<()> {
        self.update_balance_inner(account_id, delta).await
    }
    async fn apply_totals(&self, account_id: &str, credit: f64, debit: f64) -> Result<()> {
        self.apply_totals_inner(account_id, credit, debit).await
    }
    async fn sum_balance(&self, user_id: &str) -> Result<f64> {
        self.sum_balance_inner(user_id).await
    }
    async fn sum_totals(&self, user_id: &str) -> Result<(f64, f64)> {
        self.sum_totals_inner(user_id).await
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
