//! Users repository — auth + profile SQL, mirroring the Go backend's queries.

pub mod account;
pub mod category;
pub mod investment;
pub mod rule;
pub mod surreal;
pub mod transaction;

use sqlx::SqlitePool;

use crate::error::Result;

#[derive(Clone)]
pub struct UserRepo {
    pub pool: SqlitePool,
}

pub struct UserRow {
    pub id: String,
    pub email: String,
    pub name: Option<String>,
    pub password_hash: Option<String>,
    pub token_version: i64,
    pub avatar_url: Option<String>,
}

impl UserRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    pub async fn create(&self, id: &str, email: &str, password_hash: &str, name: &str) -> Result<()> {
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

    pub async fn by_email(&self, email: &str) -> Result<Option<UserRow>> {
        let row = sqlx::query_as::<_, RawUser>(
            "SELECT id, email, name, password_hash, token_version, avatar_url FROM users WHERE email = ?",
        )
        .bind(email)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    pub async fn by_id(&self, id: &str) -> Result<Option<UserRow>> {
        let row = sqlx::query_as::<_, RawUser>(
            "SELECT id, email, name, password_hash, token_version, avatar_url FROM users WHERE id = ?",
        )
        .bind(id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    /// Update `name`/`email`/`avatar_url` (`COALESCE NULLIF` — empty means unchanged).
    pub async fn update_profile(&self, id: &str, name: &str, email: &str, avatar_url: &str) -> Result<()> {
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

    /// Update the password hash and bump `token_version` (revokes other sessions).
    pub async fn change_password(&self, id: &str, password_hash: &str) -> Result<i64> {
        sqlx::query("UPDATE users SET password_hash = ?, token_version = token_version + 1 WHERE id = ?")
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

    /// Bump `token_version` to revoke every refresh token.
    pub async fn logout_all(&self, id: &str) -> Result<()> {
        sqlx::query("UPDATE users SET token_version = token_version + 1 WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }
}

// sqlx sqlite maps NULLable columns to Option<String>; name/password_hash/avatar_url
// can be NULL in the schema, so read them as optional.
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
