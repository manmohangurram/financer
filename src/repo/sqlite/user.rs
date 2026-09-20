//! `SQLite` users repository — auth + profile SQL, implementing
//! `crate::repo::traits::UserRepo` over an `sqlx::SqlitePool`.

use async_trait::async_trait;
use sqlx::SqlitePool;

use crate::error::Result;
use crate::repo::traits::UserRepo as UserRepoTrait;
use crate::repo::traits::user::UserRow;

pub struct SqliteUserRepo {
    pool: SqlitePool,
}

impl SqliteUserRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    async fn create(&self, id: &str, email: &str, password_hash: &str, name: &str) -> Result<()> {
        sqlx::query(
            "INSERT INTO users (id, email, password_hash, name, created_at) VALUES (?, ?, ?, ?, datetime('now'))",
        )
        .bind(id)
        .bind(email)
        .bind(password_hash)
        .bind(name)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    async fn by_email(&self, email: &str) -> Result<Option<UserRow>> {
        let row = sqlx::query_as::<_, RawUser>(
            "SELECT id, email, password_hash, name, token_version, avatar_url FROM users WHERE email = ?",
        )
        .bind(email)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    async fn by_id(&self, id: &str) -> Result<Option<UserRow>> {
        let row = sqlx::query_as::<_, RawUser>(
            "SELECT id, email, password_hash, name, token_version, avatar_url FROM users WHERE id = ?",
        )
        .bind(id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    async fn update_profile(
        &self,
        id: &str,
        name: &str,
        email: &str,
        avatar_url: &str,
    ) -> Result<()> {
        sqlx::query(
            "UPDATE users SET
                name = COALESCE(NULLIF(?, ''), name),
                email = COALESCE(NULLIF(?, ''), email),
                avatar_url = COALESCE(NULLIF(?, ''), avatar_url)
             WHERE id = ?",
        )
        .bind(name)
        .bind(email)
        .bind(avatar_url)
        .bind(id)
        .execute(&self.pool)
        .await?;
        Ok(())
    }

    async fn change_password(&self, id: &str, password_hash: &str) -> Result<i64> {
        sqlx::query(
            "UPDATE users SET password_hash = ?, token_version = token_version + 1 WHERE id = ?",
        )
        .bind(password_hash)
        .bind(id)
        .execute(&self.pool)
        .await?;
        let ver: i64 = sqlx::query_scalar("SELECT token_version FROM users WHERE id = ?")
            .bind(id)
            .fetch_one(&self.pool)
            .await?;
        Ok(ver)
    }

    async fn logout_all(&self, id: &str) -> Result<()> {
        sqlx::query("UPDATE users SET token_version = token_version + 1 WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }
}

#[async_trait]
impl UserRepoTrait for SqliteUserRepo {
    async fn create(&self, email: &str, password_hash: &str, name: &str) -> Result<String> {
        let id = uuid::Uuid::new_v4().to_string();
        // The trait's `create` takes email/hash/name and returns the id. We
        // generate the id here so it matches the old SQL (which used Uuid).
        self.create(&id, email, password_hash, name).await?;
        Ok(id)
    }
    async fn by_email(&self, email: &str) -> Result<Option<UserRow>> {
        self.by_email(email).await
    }
    async fn by_id(&self, id: &str) -> Result<Option<UserRow>> {
        self.by_id(id).await
    }
    async fn update_profile(
        &self,
        id: &str,
        name: &str,
        email: &str,
        avatar_url: &str,
    ) -> Result<()> {
        self.update_profile(id, name, email, avatar_url).await
    }
    async fn change_password(&self, id: &str, password_hash: &str) -> Result<i64> {
        self.change_password(id, password_hash).await
    }
    async fn logout_all(&self, id: &str) -> Result<()> {
        self.logout_all(id).await
    }
}

#[derive(sqlx::FromRow)]
struct RawUser {
    id: String,
    email: String,
    name: Option<String>,
    password_hash: Option<String>,
    token_version: i64,
    avatar_url: Option<String>,
}

impl From<RawUser> for UserRow {
    fn from(r: RawUser) -> Self {
        Self {
            id: r.id,
            email: r.email,
            name: r.name,
            password_hash: r.password_hash,
            token_version: r.token_version,
            avatar_url: r.avatar_url,
        }
    }
}
