//! `SQLite` per-user API-keys repository, implementing
//! `crate::repo::traits::UserKeyRepo` over an `sqlx::SqlitePool`.

use async_trait::async_trait;
use sqlx::SqlitePool;
use uuid::Uuid;

use crate::error::Result;
use crate::repo::traits::user_key::{UserKeyRepo as UserKeyRepoTrait, UserKeyRow};

pub struct SqliteUserKeyRepo {
    pool: SqlitePool,
}

impl SqliteUserKeyRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    async fn create_inner(
        &self,
        user_id: &str,
        name: &str,
        key_hash: &str,
        key_prefix: &str,
        expires_at: Option<String>,
        scope: &str,
    ) -> Result<String> {
        let id = Uuid::new_v4().to_string();
        let now = crate::utils::timex::now_go_ts();
        sqlx::query(
            "INSERT INTO user_keys (id, user_id, name, key_hash, key_prefix, created_at, expires_at, last_used_at, scope)
             VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?)",
        )
        .bind(&id)
        .bind(user_id)
        .bind(name)
        .bind(key_hash)
        .bind(key_prefix)
        .bind(&now)
        .bind(expires_at)
        .bind(scope)
        .execute(&self.pool)
        .await?;
        Ok(id)
    }

    async fn list_inner(&self, user_id: &str) -> Result<Vec<UserKeyRow>> {
        let rows = sqlx::query_as::<_, RawUserKey>(
            "SELECT id, name, key_prefix, created_at, expires_at, last_used_at, scope
             FROM user_keys WHERE user_id = ? ORDER BY created_at DESC",
        )
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    async fn count_inner(&self, user_id: &str) -> Result<i64> {
        let n: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM user_keys WHERE user_id = ?")
            .bind(user_id)
            .fetch_one(&self.pool)
            .await?;
        Ok(n)
    }

    async fn get_by_id_inner(&self, user_id: &str, id: &str) -> Result<Option<UserKeyRow>> {
        let row = sqlx::query_as::<_, RawUserKey>(
            "SELECT id, name, key_prefix, created_at, expires_at, last_used_at, scope
             FROM user_keys WHERE id = ? AND user_id = ?",
        )
        .bind(id)
        .bind(user_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    async fn delete_inner(&self, user_id: &str, id: &str) -> Result<bool> {
        let res = sqlx::query("DELETE FROM user_keys WHERE id = ? AND user_id = ?")
            .bind(id)
            .bind(user_id)
            .execute(&self.pool)
            .await?;
        Ok(res.rows_affected() > 0)
    }

    async fn by_key_hash_inner(&self, key_hash: &str) -> Result<Option<(String, String, String)>> {
        let row: Option<(String, String, String)> =
            sqlx::query_as("SELECT user_id, id, scope FROM user_keys WHERE key_hash = ?")
                .bind(key_hash)
                .fetch_optional(&self.pool)
                .await?;
        Ok(row)
    }

    async fn touch_last_used_inner(&self, id: &str) -> Result<()> {
        sqlx::query("UPDATE user_keys SET last_used_at = ? WHERE id = ?")
            .bind(crate::utils::timex::now_go_ts())
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }
}

#[async_trait]
impl UserKeyRepoTrait for SqliteUserKeyRepo {
    async fn create(
        &self,
        user_id: &str,
        name: &str,
        key_hash: &str,
        key_prefix: &str,
        expires_at: Option<String>,
        scope: &str,
    ) -> Result<String> {
        self.create_inner(user_id, name, key_hash, key_prefix, expires_at, scope)
            .await
    }
    async fn list(&self, user_id: &str) -> Result<Vec<UserKeyRow>> {
        self.list_inner(user_id).await
    }
    async fn count(&self, user_id: &str) -> Result<i64> {
        self.count_inner(user_id).await
    }
    async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<UserKeyRow>> {
        self.get_by_id_inner(user_id, id).await
    }
    async fn delete(&self, user_id: &str, id: &str) -> Result<bool> {
        self.delete_inner(user_id, id).await
    }
    async fn by_key_hash(&self, key_hash: &str) -> Result<Option<(String, String, String)>> {
        self.by_key_hash_inner(key_hash).await
    }
    async fn touch_last_used(&self, id: &str) -> Result<()> {
        self.touch_last_used_inner(id).await
    }
}

#[derive(sqlx::FromRow)]
struct RawUserKey {
    id: String,
    name: String,
    key_prefix: String,
    created_at: String,
    expires_at: Option<String>,
    last_used_at: Option<String>,
    scope: String,
}

impl From<RawUserKey> for UserKeyRow {
    fn from(r: RawUserKey) -> Self {
        Self {
            id: r.id,
            name: r.name,
            key_prefix: r.key_prefix,
            created_at: r.created_at,
            expires_at: r.expires_at,
            last_used_at: r.last_used_at,
            scope: r.scope,
        }
    }
}
