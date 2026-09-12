//! `SurrealDB` per-user API-keys repository, implementing
//! `crate::repo::traits::UserKeyRepo`.

use surrealdb::Connection;

use crate::error::Result;
use crate::repo::surreal::{rid, DbClient, RepoConn};

pub use crate::repo::traits::user_key::UserKeyRow;

#[derive(Clone)]
pub struct UserKeyRepo<C: Connection = DbClient> {
    db: RepoConn<C>,
}

impl<C: Connection> UserKeyRepo<C> {
    pub fn new(db: RepoConn<C>) -> Self {
        Self { db }
    }

    pub async fn create(
        &self,
        user_id: &str,
        name: &str,
        key_hash: &str,
        key_prefix: &str,
        expires_at: Option<String>,
        scope: &str,
    ) -> Result<String> {
        let now = crate::utils::timex::now_go_ts();
        let mut res = self
            .db
            .query(
                "CREATE user_key CONTENT {
                    user: $uid, name: $name, keyHash: $hash, keyPrefix: $prefix,
                    createdAt: $created, expiresAt: $expires, lastUsedAt: NONE, scope: $scope
                } RETURN meta::id(id) AS id",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("name", name))
            .bind(("hash", key_hash))
            .bind(("prefix", key_prefix))
            .bind(("created", now))
            .bind(("expires", expires_at))
            .bind(("scope", scope))
            .await?
            .check()?;
        Ok(super::take_json::<IdRow>(&mut res, 0)?
            .into_iter()
            .next()
            .map(|r| r.id)
            .unwrap_or_default())
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<UserKeyRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, name, keyPrefix, createdAt, expiresAt, lastUsedAt, scope
                 FROM user_key WHERE user = $uid ORDER BY createdAt DESC",
            )
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(super::take_json(&mut res, 0)?)
    }

    pub async fn count(&self, user_id: &str) -> Result<i64> {
        let mut res = self
            .db
            .query("SELECT math::count() AS n FROM user_key WHERE user = $uid GROUP ALL")
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(super::take_json::<CountRow>(&mut res, 0)?
            .first()
            .map_or(0, |r| r.n))
    }

    pub async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<UserKeyRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, name, keyPrefix, createdAt, expiresAt, lastUsedAt, scope
                 FROM user_key WHERE id = $rid AND user = $uid LIMIT 1",
            )
            .bind(("rid", rid("user_key", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(super::take_json(&mut res, 0)?.into_iter().next())
    }

    pub async fn delete(&self, user_id: &str, id: &str) -> Result<bool> {
        let mut res = self
            .db
            .query("DELETE $rid WHERE user = $uid RETURN BEFORE")
            .bind(("rid", rid("user_key", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(!super::take_json::<serde_json::Value>(&mut res, 0)?.is_empty())
    }

    pub async fn by_key_hash(&self, key_hash: &str) -> Result<Option<(String, String, String)>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(user) AS uid, meta::id(id) AS id, scope
                 FROM user_key WHERE keyHash = $hash LIMIT 1",
            )
            .bind(("hash", key_hash))
            .await?;
        let row = super::take_json::<AuthRow>(&mut res, 0)?.into_iter().next();
        Ok(row.map(|r| (r.uid, r.id, r.scope)))
    }

    pub async fn touch_last_used(&self, id: &str) -> Result<()> {
        self.db
            .query("UPDATE $rid SET lastUsedAt = $at")
            .bind(("rid", rid("user_key", id)))
            .bind(("at", crate::utils::timex::now_go_ts()))
            .await?
            .check()?;
        Ok(())
    }
}

#[derive(serde::Deserialize)]
struct IdRow {
    id: String,
}

#[derive(serde::Deserialize)]
struct CountRow {
    n: i64,
}

#[derive(serde::Deserialize)]
struct AuthRow {
    uid: String,
    id: String,
    scope: String,
}

// Backend-agnostic `UserKeyRepo` trait impl (forwarders → inherent methods).
#[async_trait::async_trait]
impl crate::repo::traits::UserKeyRepo for UserKeyRepo<DbClient> {
    async fn create(&self, user_id: &str, name: &str, key_hash: &str, key_prefix: &str, expires_at: Option<String>, scope: &str) -> Result<String> {
        self.create(user_id, name, key_hash, key_prefix, expires_at, scope).await
    }
    async fn list(&self, user_id: &str) -> Result<Vec<UserKeyRow>> {
        self.list(user_id).await
    }
    async fn count(&self, user_id: &str) -> Result<i64> {
        self.count(user_id).await
    }
    async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<UserKeyRow>> {
        self.get_by_id(user_id, id).await
    }
    async fn delete(&self, user_id: &str, id: &str) -> Result<bool> {
        self.delete(user_id, id).await
    }
    async fn by_key_hash(&self, key_hash: &str) -> Result<Option<(String, String, String)>> {
        self.by_key_hash(key_hash).await
    }
    async fn touch_last_used(&self, id: &str) -> Result<()> {
        self.touch_last_used(id).await
    }
}
