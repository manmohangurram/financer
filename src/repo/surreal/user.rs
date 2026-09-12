//! Users repository — auth + profile queries on `SurrealDB`.

use surrealdb::Connection;

use crate::error::{ApiError, Result};
use crate::repo::surreal::{DbClient, RepoConn, rid};

pub use crate::repo::traits::user::UserRow;

#[derive(Clone)]
pub struct UserRepo<C: Connection = DbClient> {
    db: RepoConn<C>,
}

impl<C: Connection> UserRepo<C> {
    pub fn new(db: RepoConn<C>) -> Self {
        Self { db }
    }

    /// Create a user; returns the generated record id (plain string, e.g. the
    /// random part of `user:<id>`).
    pub async fn create(&self, email: &str, password_hash: &str, name: &str) -> Result<String> {
        let mut res = self
            .db
            .query(
                "CREATE user CONTENT {
                    email: $email, passwordHash: $hash, name: $name, tokenVersion: 0, createdAt: time::now()
                } RETURN meta::id(id) AS id",
            )
            .bind(("email", email))
            .bind(("hash", password_hash))
            .bind(("name", name))
            .await?
            .check()
            .map_err(|e| map_repo_err(&e))?;
        let id = super::take_json::<IdRow>(&mut res, 0)?
            .into_iter()
            .next()
            .map(|r| r.id)
            .unwrap_or_default();
        Ok(id)
    }

    pub async fn by_email(&self, email: &str) -> Result<Option<UserRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, email, name, passwordHash, tokenVersion, avatarUrl
                 FROM user WHERE email = $email LIMIT 1",
            )
            .bind(("email", email))
            .await?;
        Ok(super::take_json(&mut res, 0)?.into_iter().next())
    }

    pub async fn by_id(&self, id: &str) -> Result<Option<UserRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, email, name, passwordHash, tokenVersion, avatarUrl
                 FROM user WHERE id = $rid LIMIT 1",
            )
            .bind(("rid", rid("user", id)))
            .await?;
        Ok(super::take_json(&mut res, 0)?.into_iter().next())
    }

    /// Update `name`/`email`/`avatar_url` (empty means unchanged).
    pub async fn update_profile(
        &self,
        id: &str,
        name: &str,
        email: &str,
        avatar_url: &str,
    ) -> Result<()> {
        self.db
            .query(
                "UPDATE $rid SET
                    name = IF $name != '' THEN $name ELSE name END,
                    email = IF $email != '' THEN $email ELSE email END,
                    avatarUrl = IF $avatar != '' THEN $avatar ELSE avatarUrl END",
            )
            .bind(("rid", rid("user", id)))
            .bind(("name", name))
            .bind(("email", email))
            .bind(("avatar", avatar_url))
            .await?
            .check()
            .map_err(|e| map_repo_err(&e))?;
        Ok(())
    }

    /// Update the password hash and bump `token_version` (revokes other sessions).
    pub async fn change_password(&self, id: &str, password_hash: &str) -> Result<i64> {
        let mut res = self
            .db
            .query(
                "UPDATE $rid SET passwordHash = $hash, tokenVersion = tokenVersion + 1
                 RETURN tokenVersion",
            )
            .bind(("rid", rid("user", id)))
            .bind(("hash", password_hash))
            .await?;
        let ver = super::take_json::<TokenVersion>(&mut res, 0)?
            .first()
            .map_or(0, |t| t.token_version);
        Ok(ver)
    }

    /// Bump `token_version` to revoke every refresh token.
    pub async fn logout_all(&self, id: &str) -> Result<()> {
        self.db
            .query("UPDATE $rid SET tokenVersion = tokenVersion + 1")
            .bind(("rid", rid("user", id)))
            .await?
            .check()
            .map_err(|e| map_repo_err(&e))?;
        Ok(())
    }
}

#[derive(serde::Deserialize)]
struct TokenVersion {
    #[serde(rename = "tokenVersion")]
    token_version: i64,
}

#[derive(serde::Deserialize)]
struct IdRow {
    id: String,
}

// Backend-agnostic `UserRepo` trait impl (forwarders → inherent methods).
#[async_trait::async_trait]
impl crate::repo::traits::UserRepo for UserRepo<DbClient> {
    async fn create(&self, email: &str, password_hash: &str, name: &str) -> Result<String> {
        self.create(email, password_hash, name).await
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

fn map_repo_err(e: &surrealdb::Error) -> ApiError {
    let s = e.to_string();
    if s.contains("index") || s.contains("unique") {
        ApiError::conflict("email already exists")
    } else {
        ApiError::internal("database error")
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::surreal_db;
    use std::sync::Arc;

    #[tokio::test]
    async fn user_crud() {
        let db = Arc::new(surreal_db::connect_mem().await.unwrap());
        let repo = UserRepo::new(db);

        let id = repo.create("a@b.c", "h1", "A").await.unwrap();
        assert!(!id.is_empty(), "generated record id");
        let by_email = repo.by_email("a@b.c").await.unwrap().unwrap();
        assert_eq!(by_email.id, id);
        assert_eq!(by_email.email, "a@b.c");
        assert_eq!(by_email.name.as_deref(), Some("A"));
        assert_eq!(by_email.token_version, 0);

        let by_id = repo.by_id(&id).await.unwrap().unwrap();
        assert_eq!(by_id.email, "a@b.c");

        repo.update_profile(&id, "B", "", "").await.unwrap();
        assert_eq!(
            repo.by_id(&id).await.unwrap().unwrap().name.as_deref(),
            Some("B")
        );
        assert_eq!(repo.by_id(&id).await.unwrap().unwrap().email, "a@b.c");

        let ver = repo.change_password(&id, "h2").await.unwrap();
        assert_eq!(ver, 1);
        repo.logout_all(&id).await.unwrap();
        assert_eq!(repo.by_id(&id).await.unwrap().unwrap().token_version, 2);

        assert!(repo.by_email("nope@x.y").await.unwrap().is_none());
    }
}
