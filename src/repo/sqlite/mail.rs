//! `SQLite` mail-ingest repository, implementing `crate::repo::traits::MailRepo`
//! over an `sqlx::SqlitePool`.

use std::str::FromStr;

use async_trait::async_trait;
use sqlx::SqlitePool;
use uuid::Uuid;

use crate::error::Result;
use crate::repo::traits::mail::{
    MailAccountRow, MailMessageRow, MailRepo as MailRepoTrait, MailStatus, NewMailAccount,
    NewMailMessage,
};

pub struct SqliteMailRepo {
    pub(crate) pool: SqlitePool,
}

impl SqliteMailRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    async fn create_account_inner(
        &self,
        user_id: &str,
        input: &NewMailAccount,
    ) -> Result<MailAccountRow> {
        let id = Uuid::new_v4().to_string();
        let now = crate::utils::timex::now_go_ts();
        let allowlist =
            serde_json::to_string(&input.sender_allowlist).unwrap_or_else(|_| "[]".into());
        sqlx::query(
            "INSERT INTO mail_accounts
                (id, user_id, label, imap_host, imap_port, username, secret_enc, sender_allowlist, enabled, created_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)",
        )
        .bind(&id)
        .bind(user_id)
        .bind(&input.label)
        .bind(&input.imap_host)
        .bind(input.imap_port)
        .bind(&input.username)
        .bind(&input.secret_enc)
        .bind(&allowlist)
        .bind(&now)
        .execute(&self.pool)
        .await?;
        Ok(MailAccountRow {
            id,
            user_id: user_id.to_string(),
            label: input.label.clone(),
            imap_host: input.imap_host.clone(),
            imap_port: input.imap_port,
            username: input.username.clone(),
            secret_enc: input.secret_enc.clone(),
            sender_allowlist: input.sender_allowlist.clone(),
            enabled: true,
            last_polled_at: None,
            last_error: String::new(),
            created_at: now,
        })
    }

    async fn list_accounts_inner(&self, user_id: &str) -> Result<Vec<MailAccountRow>> {
        let rows = sqlx::query_as::<_, RawMailAccount>(
            "SELECT id, user_id, label, imap_host, imap_port, username, secret_enc, sender_allowlist,
                    enabled, last_polled_at, last_error, created_at
             FROM mail_accounts WHERE user_id = ? ORDER BY created_at",
        )
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    async fn delete_account_inner(&self, user_id: &str, id: &str) -> Result<bool> {
        let r = sqlx::query("DELETE FROM mail_accounts WHERE id = ? AND user_id = ?")
            .bind(id)
            .bind(user_id)
            .execute(&self.pool)
            .await?;
        Ok(r.rows_affected() > 0)
    }

    async fn list_pollable_accounts_inner(&self) -> Result<Vec<MailAccountRow>> {
        let rows = sqlx::query_as::<_, RawMailAccount>(
            "SELECT id, user_id, label, imap_host, imap_port, username, secret_enc, sender_allowlist,
                    enabled, last_polled_at, last_error, created_at
             FROM mail_accounts WHERE enabled = 1 ORDER BY created_at",
        )
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    async fn mark_polled_inner(&self, account_id: &str, error: &str) -> Result<()> {
        let now = crate::utils::timex::now_go_ts();
        sqlx::query("UPDATE mail_accounts SET last_polled_at = ?, last_error = ? WHERE id = ?")
            .bind(&now)
            .bind(error)
            .bind(account_id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    async fn known_message_ids_inner(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        if ids.is_empty() {
            return Ok(Vec::new());
        }
        let placeholders = vec!["?"; ids.len()].join(",");
        let sql = format!(
            "SELECT message_id FROM mail_messages WHERE user_id = ? AND message_id IN ({placeholders})"
        );
        let mut q = sqlx::query_scalar::<_, String>(&sql).bind(user_id);
        for id in ids {
            q = q.bind(id);
        }
        Ok(q.fetch_all(&self.pool).await?)
    }

    async fn insert_message_inner(&self, user_id: &str, msg: &NewMailMessage) -> Result<bool> {
        let id = Uuid::new_v4().to_string();
        let now = crate::utils::timex::now_go_ts();
        let r = sqlx::query(
            "INSERT OR IGNORE INTO mail_messages
                (id, user_id, mail_account_id, message_id, from_addr, received_at, subject,
                 raw_text, status, parsed_json, confidence, extractor_source, created_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
        )
        .bind(&id)
        .bind(user_id)
        .bind(&msg.mail_account_id)
        .bind(&msg.message_id)
        .bind(&msg.from_addr)
        .bind(&msg.received_at)
        .bind(&msg.subject)
        .bind(&msg.raw_text)
        .bind(msg.status.to_string())
        .bind(&msg.parsed_json)
        .bind(msg.confidence)
        .bind(&msg.extractor_source)
        .bind(&now)
        .execute(&self.pool)
        .await?;
        Ok(r.rows_affected() > 0)
    }

    async fn list_messages_inner(
        &self,
        user_id: &str,
        status: Option<MailStatus>,
        limit: i64,
    ) -> Result<Vec<MailMessageRow>> {
        let rows = sqlx::query_as::<_, RawMailMessage>(
            "SELECT id, message_id, from_addr, subject, received_at, status, parsed_json,
                    COALESCE(confidence, 0) AS confidence, raw_text, transaction_id
             FROM mail_messages
             WHERE user_id = ? AND (?1 IS NULL OR status = ?2)
             ORDER BY created_at DESC LIMIT ?3",
        )
        .bind(user_id)
        .bind(status.map(|s| s.to_string()))
        .bind(status.map(|s| s.to_string()))
        .bind(limit)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    async fn sweep_bodies_inner(&self) -> Result<u64> {
        let r = sqlx::query(
            "UPDATE mail_messages SET raw_text = NULL
             WHERE raw_text IS NOT NULL AND status NOT IN ('PENDING', 'UNPARSED')",
        )
        .execute(&self.pool)
        .await?;
        Ok(r.rows_affected())
    }
}

#[async_trait]
impl MailRepoTrait for SqliteMailRepo {
    async fn create_account(
        &self,
        user_id: &str,
        input: &NewMailAccount,
    ) -> Result<MailAccountRow> {
        self.create_account_inner(user_id, input).await
    }
    async fn list_accounts(&self, user_id: &str) -> Result<Vec<MailAccountRow>> {
        self.list_accounts_inner(user_id).await
    }
    async fn delete_account(&self, user_id: &str, id: &str) -> Result<bool> {
        self.delete_account_inner(user_id, id).await
    }
    async fn list_pollable_accounts(&self) -> Result<Vec<MailAccountRow>> {
        self.list_pollable_accounts_inner().await
    }
    async fn mark_polled(&self, account_id: &str, error: &str) -> Result<()> {
        self.mark_polled_inner(account_id, error).await
    }
    async fn known_message_ids(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        self.known_message_ids_inner(user_id, ids).await
    }
    async fn insert_message(&self, user_id: &str, msg: &NewMailMessage) -> Result<bool> {
        self.insert_message_inner(user_id, msg).await
    }
    async fn list_messages(
        &self,
        user_id: &str,
        status: Option<MailStatus>,
        limit: i64,
    ) -> Result<Vec<MailMessageRow>> {
        self.list_messages_inner(user_id, status, limit).await
    }
    async fn sweep_bodies(&self) -> Result<u64> {
        self.sweep_bodies_inner().await
    }
}

#[derive(sqlx::FromRow)]
struct RawMailMessage {
    id: String,
    message_id: String,
    from_addr: String,
    subject: String,
    received_at: Option<String>,
    status: String,
    parsed_json: Option<String>,
    confidence: f64,
    raw_text: Option<String>,
    transaction_id: Option<String>,
}

impl From<RawMailMessage> for MailMessageRow {
    fn from(r: RawMailMessage) -> Self {
        Self {
            id: r.id,
            message_id: r.message_id,
            from_addr: r.from_addr,
            subject: r.subject,
            received_at: r.received_at,
            status: MailStatus::from_str(&r.status).unwrap_or(MailStatus::Unparsed),
            parsed_json: r.parsed_json,
            confidence: r.confidence,
            raw_text: r.raw_text,
            transaction_id: r.transaction_id,
        }
    }
}

#[derive(sqlx::FromRow)]
struct RawMailAccount {
    id: String,
    user_id: String,
    label: String,
    imap_host: String,
    imap_port: i64,
    username: String,
    secret_enc: String,
    sender_allowlist: String,
    enabled: i64,
    last_polled_at: Option<String>,
    last_error: String,
    created_at: String,
}

impl From<RawMailAccount> for MailAccountRow {
    fn from(r: RawMailAccount) -> Self {
        Self {
            id: r.id,
            user_id: r.user_id,
            label: r.label,
            imap_host: r.imap_host,
            imap_port: r.imap_port,
            username: r.username,
            secret_enc: r.secret_enc,
            sender_allowlist: serde_json::from_str(&r.sender_allowlist).unwrap_or_default(),
            enabled: r.enabled != 0,
            last_polled_at: r.last_polled_at,
            last_error: r.last_error,
            created_at: r.created_at,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::repo::traits::MailRepo;
    use crate::repo::traits::mail::MailStatus;

    // One connection: an in-memory DB is per-connection, so a pool would give
    // each connection its own empty database.
    async fn pool() -> SqlitePool {
        sqlx::sqlite::SqlitePoolOptions::new()
            .max_connections(1)
            .connect("sqlite::memory:")
            .await
            .unwrap()
    }

    /// Minimal user row so the FK on `mail_accounts` holds.
    async fn seed_user(pool: &SqlitePool, id: &str) {
        sqlx::query("INSERT INTO users (id, email, password_hash, name, created_at) VALUES (?, ?, 'h', 'U', '2026-01-01 00:00:00 +0000 UTC')")
            .bind(id)
            .bind(format!("{id}@x.com"))
            .execute(pool)
            .await
            .unwrap();
    }

    async fn repo_with_user() -> (SqliteMailRepo, String) {
        let pool = pool().await;
        crate::repo::db::run_migrations(&pool).await.unwrap();
        seed_user(&pool, "u1").await;
        (SqliteMailRepo::new(pool), "u1".to_string())
    }

    fn account_input() -> NewMailAccount {
        NewMailAccount {
            label: "Personal".into(),
            imap_host: "imap.x.com".into(),
            imap_port: 993,
            username: "a@x.com".into(),
            secret_enc: "enc".into(),
            sender_allowlist: vec!["alerts@bank.com".into()],
        }
    }

    fn message(account_id: &str, message_id: &str, status: MailStatus) -> NewMailMessage {
        NewMailMessage {
            mail_account_id: account_id.into(),
            message_id: message_id.into(),
            from_addr: "alerts@bank.com".into(),
            received_at: None,
            subject: "Debited".into(),
            raw_text: "body".into(),
            status,
            parsed_json: None,
            confidence: 0.0,
            extractor_source: "template".into(),
        }
    }

    #[tokio::test]
    async fn account_roundtrip_keeps_the_allowlist() {
        let (repo, uid) = repo_with_user().await;
        let created = repo.create_account(&uid, &account_input()).await.unwrap();
        assert_eq!(created.sender_allowlist, vec!["alerts@bank.com"]);
        assert!(created.enabled);

        let listed = repo.list_accounts(&uid).await.unwrap();
        assert_eq!(listed.len(), 1);
        assert_eq!(listed[0].sender_allowlist, vec!["alerts@bank.com"]);
        assert_eq!(repo.list_pollable_accounts().await.unwrap().len(), 1);

        assert!(repo.delete_account(&uid, &created.id).await.unwrap());
        assert!(!repo.delete_account(&uid, &created.id).await.unwrap());
    }

    #[tokio::test]
    async fn accounts_are_scoped_to_their_user() {
        let (repo, uid) = repo_with_user().await;
        let other = repo.create_account("u2", &account_input()).await;
        assert!(other.is_err(), "u2 does not exist, FK must reject");
        repo.create_account(&uid, &account_input()).await.unwrap();
        assert!(repo.list_accounts("u2").await.unwrap().is_empty());
    }

    #[tokio::test]
    async fn duplicate_message_id_is_not_stored_twice() {
        let (repo, uid) = repo_with_user().await;
        let acc = repo.create_account(&uid, &account_input()).await.unwrap();

        assert!(
            repo.insert_message(&uid, &message(&acc.id, "<m1>", MailStatus::Pending))
                .await
                .unwrap()
        );
        assert!(
            !repo
                .insert_message(&uid, &message(&acc.id, "<m1>", MailStatus::Pending))
                .await
                .unwrap(),
            "same Message-ID must not insert twice"
        );
        assert!(
            repo.insert_message(&uid, &message(&acc.id, "<m2>", MailStatus::Pending))
                .await
                .unwrap()
        );
    }

    #[tokio::test]
    async fn known_message_ids_reports_only_the_seen_ones() {
        let (repo, uid) = repo_with_user().await;
        let acc = repo.create_account(&uid, &account_input()).await.unwrap();
        repo.insert_message(&uid, &message(&acc.id, "<seen>", MailStatus::Pending))
            .await
            .unwrap();

        let ids = ["<seen>".to_string(), "<unseen>".to_string()];
        let known = repo.known_message_ids(&uid, &ids).await.unwrap();
        assert_eq!(known, vec!["<seen>".to_string()]);
        assert!(repo.known_message_ids(&uid, &[]).await.unwrap().is_empty());
    }

    #[tokio::test]
    async fn sweep_drops_bodies_only_for_resolved_rows() {
        let (repo, uid) = repo_with_user().await;
        let acc = repo.create_account(&uid, &account_input()).await.unwrap();
        for (mid, status) in [
            ("<pending>", MailStatus::Pending),
            ("<unparsed>", MailStatus::Unparsed),
            ("<rejected>", MailStatus::Rejected),
            ("<auto>", MailStatus::Auto),
        ] {
            repo.insert_message(&uid, &message(&acc.id, mid, status))
                .await
                .unwrap();
        }

        assert_eq!(
            repo.sweep_bodies().await.unwrap(),
            2,
            "only rejected + auto"
        );
        let (bodies, _): (i64, i64) = sqlx::query_as(
            "SELECT COUNT(raw_text), (SELECT COUNT(*) FROM mail_messages) FROM mail_messages WHERE status IN ('PENDING','UNPARSED')",
        )
        .fetch_one(&repo.pool)
        .await
        .unwrap();
        assert_eq!(bodies, 2, "the unresolved bodies survive");

        assert_eq!(repo.sweep_bodies().await.unwrap(), 0, "idempotent");
    }

    #[tokio::test]
    async fn marking_a_poll_records_the_error_and_time() {
        let (repo, uid) = repo_with_user().await;
        let acc = repo.create_account(&uid, &account_input()).await.unwrap();
        repo.mark_polled(&acc.id, "auth failed").await.unwrap();
        let after = repo
            .list_accounts(&uid)
            .await
            .unwrap()
            .into_iter()
            .next()
            .unwrap();
        assert_eq!(after.last_error, "auth failed");
        assert!(after.last_polled_at.is_some());
    }
}
