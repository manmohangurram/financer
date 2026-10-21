//! `SurrealDB` mail-ingest repository, implementing
//! `crate::repo::traits::MailRepo`.

use surrealdb::Connection;

use crate::error::Result;
use crate::repo::surreal::{DbClient, RepoConn, rid, take_json};

pub use crate::repo::traits::mail::{MailAccountRow, NewMailAccount, NewMailMessage};

const ACCOUNT_FIELDS: &str = "meta::id(id) AS id, userId, label, imapHost, imapPort, username, secretEnc, \
                              senderAllowlist, enabled, lastPolledAt, lastError, createdAt";

#[derive(Clone)]
pub struct MailRepo<C: Connection = DbClient> {
    db: RepoConn<C>,
}

impl<C: Connection> MailRepo<C> {
    pub fn new(db: RepoConn<C>) -> Self {
        Self { db }
    }

    pub async fn create_account(
        &self,
        user_id: &str,
        input: &NewMailAccount,
    ) -> Result<MailAccountRow> {
        let now = crate::utils::timex::now_go_ts();
        let mut res = self
            .db
            .query(format!(
                "CREATE mail_account CONTENT {{
                    user: $uid, userId: $userid, label: $label, imapHost: $host, imapPort: $port,
                    username: $user, secretEnc: $secret, senderAllowlist: $allow,
                    enabled: true, lastPolledAt: NONE, lastError: '', createdAt: $created
                }} RETURN {ACCOUNT_FIELDS}"
            ))
            .bind(("uid", rid("user", user_id)))
            .bind(("userid", user_id.to_string()))
            .bind(("label", input.label.clone()))
            .bind(("host", input.imap_host.clone()))
            .bind(("port", input.imap_port))
            .bind(("user", input.username.clone()))
            .bind(("secret", input.secret_enc.clone()))
            .bind(("allow", input.sender_allowlist.clone()))
            .bind(("created", now))
            .await?
            .check()?;
        take_json::<MailAccountRow>(&mut res, 0)?
            .into_iter()
            .next()
            .ok_or_else(|| crate::error::ApiError::internal("mail account create returned no row"))
    }

    pub async fn list_accounts(&self, user_id: &str) -> Result<Vec<MailAccountRow>> {
        let mut res = self
            .db
            .query(format!(
                "SELECT {ACCOUNT_FIELDS} FROM mail_account WHERE user = $uid ORDER BY createdAt"
            ))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json(&mut res, 0)?)
    }

    pub async fn delete_account(&self, user_id: &str, id: &str) -> Result<bool> {
        let mut res = self
            .db
            .query("DELETE $rid WHERE user = $uid RETURN BEFORE")
            .bind(("rid", rid("mail_account", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(!take_json::<serde_json::Value>(&mut res, 0)?.is_empty())
    }

    pub async fn list_pollable_accounts(&self) -> Result<Vec<MailAccountRow>> {
        let mut res = self
            .db
            .query(format!(
                "SELECT {ACCOUNT_FIELDS} FROM mail_account WHERE enabled = true ORDER BY createdAt"
            ))
            .await?;
        Ok(take_json(&mut res, 0)?)
    }

    pub async fn mark_polled(&self, account_id: &str, error: &str) -> Result<()> {
        let now = crate::utils::timex::now_go_ts();
        self.db
            .query("UPDATE $rid SET lastPolledAt = $now, lastError = $err")
            .bind(("rid", rid("mail_account", account_id)))
            .bind(("now", now))
            .bind(("err", error.to_string()))
            .await?
            .check()?;
        Ok(())
    }

    pub async fn known_message_ids(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        if ids.is_empty() {
            return Ok(Vec::new());
        }
        let mut res = self
            .db
            .query("SELECT messageId FROM mail_message WHERE user = $uid AND messageId IN $ids")
            .bind(("uid", rid("user", user_id)))
            .bind(("ids", ids.to_vec()))
            .await?;
        let rows = take_json::<serde_json::Value>(&mut res, 0)?;
        Ok(rows
            .into_iter()
            .filter_map(|v| v["messageId"].as_str().map(str::to_string))
            .collect())
    }

    /// `false` when the `Message-ID` was already stored. The unique index on
    /// `(user, messageId)` is the backstop; this check makes the common case
    /// silent instead of an error.
    pub async fn insert_message(&self, user_id: &str, msg: &NewMailMessage) -> Result<bool> {
        let mut existing = self
            .db
            .query("SELECT meta::id(id) AS id FROM mail_message WHERE user = $uid AND messageId = $mid LIMIT 1")
            .bind(("uid", rid("user", user_id)))
            .bind(("mid", msg.message_id.clone()))
            .await?;
        if !take_json::<serde_json::Value>(&mut existing, 0)?.is_empty() {
            return Ok(false);
        }

        let now = crate::utils::timex::now_go_ts();
        self.db
            .query(
                "CREATE mail_message CONTENT {
                    user: $uid, account: $acc, messageId: $mid, fromAddr: $from,
                    receivedAt: $recv, subject: $subject, rawText: $body,
                    parsedJson: NONE, approvedJson: NONE, status: $status,
                    extractorSource: '', confidence: NONE, transaction: NONE, createdAt: $created
                }",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("acc", rid("mail_account", &msg.mail_account_id)))
            .bind(("mid", msg.message_id.clone()))
            .bind(("from", msg.from_addr.clone()))
            .bind(("recv", msg.received_at.clone()))
            .bind(("subject", msg.subject.clone()))
            .bind(("body", msg.raw_text.clone()))
            .bind(("status", msg.status.to_string()))
            .bind(("created", now))
            .await?
            .check()?;
        Ok(true)
    }

    pub async fn sweep_bodies(&self) -> Result<u64> {
        let mut res = self
            .db
            .query(
                "UPDATE mail_message SET rawText = NONE
                 WHERE rawText != NONE AND status NOT IN ['PENDING', 'UNPARSED']
                 RETURN AFTER",
            )
            .await?;
        Ok(take_json::<serde_json::Value>(&mut res, 0)?.len() as u64)
    }
}

#[async_trait::async_trait]
impl crate::repo::traits::MailRepo for MailRepo<DbClient> {
    async fn create_account(
        &self,
        user_id: &str,
        input: &NewMailAccount,
    ) -> Result<MailAccountRow> {
        self.create_account(user_id, input).await
    }
    async fn list_accounts(&self, user_id: &str) -> Result<Vec<MailAccountRow>> {
        self.list_accounts(user_id).await
    }
    async fn delete_account(&self, user_id: &str, id: &str) -> Result<bool> {
        self.delete_account(user_id, id).await
    }
    async fn list_pollable_accounts(&self) -> Result<Vec<MailAccountRow>> {
        self.list_pollable_accounts().await
    }
    async fn mark_polled(&self, account_id: &str, error: &str) -> Result<()> {
        self.mark_polled(account_id, error).await
    }
    async fn known_message_ids(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        self.known_message_ids(user_id, ids).await
    }
    async fn insert_message(&self, user_id: &str, msg: &NewMailMessage) -> Result<bool> {
        self.insert_message(user_id, msg).await
    }
    async fn sweep_bodies(&self) -> Result<u64> {
        self.sweep_bodies().await
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use super::*;
    use crate::repo::db;
    use crate::repo::traits::mail::MailStatus;

    async fn repo() -> MailRepo<surrealdb::engine::local::Db> {
        let db = Arc::new(db::surreal_connect_mem().await.unwrap());
        db.query(
            "CREATE user CONTENT { id: 'u1', email: 'u1@x.com', passwordHash: 'h', name: 'u1' }",
        )
        .await
        .unwrap()
        .check()
        .unwrap();
        MailRepo::new(db)
    }

    /// The mailbox-management surface arrives with the mailbox API; tests seed
    /// the row directly.
    async fn seed_account(repo: &MailRepo<surrealdb::engine::local::Db>) {
        repo.db
            .query(
                "CREATE mail_account CONTENT {
                    user: $uid, userId: 'u1', label: 'Personal', imapHost: 'imap.x.com',
                    imapPort: 993, username: 'a@x.com', secretEnc: 'enc',
                    senderAllowlist: ['alerts@bank.com'], enabled: true,
                    lastPolledAt: NONE, lastError: '', createdAt: '2026-01-01 00:00:00 +0000 UTC'
                }",
            )
            .bind(("uid", rid("user", "u1")))
            .await
            .unwrap()
            .check()
            .unwrap();
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
        }
    }

    #[tokio::test]
    async fn account_row_round_trips_through_the_repo() {
        let repo = repo().await;
        seed_account(&repo).await;
        let listed = repo.list_pollable_accounts().await.unwrap();
        assert_eq!(listed.len(), 1);
        assert_eq!(listed[0].user_id, "u1");
        assert_eq!(listed[0].sender_allowlist, vec!["alerts@bank.com"]);
        assert!(listed[0].enabled);

        repo.mark_polled(&listed[0].id, "auth failed")
            .await
            .unwrap();
        let after = &repo.list_pollable_accounts().await.unwrap()[0];
        assert_eq!(after.last_error, "auth failed");
        assert!(after.last_polled_at.is_some());
    }

    #[tokio::test]
    async fn duplicate_message_id_is_not_stored_twice() {
        let repo = repo().await;
        seed_account(&repo).await;
        let acc = repo.list_pollable_accounts().await.unwrap()[0].clone();

        assert!(
            repo.insert_message("u1", &message(&acc.id, "<m1>", MailStatus::Pending))
                .await
                .unwrap()
        );
        assert!(
            !repo
                .insert_message("u1", &message(&acc.id, "<m1>", MailStatus::Pending))
                .await
                .unwrap()
        );

        let known = repo
            .known_message_ids("u1", &["<m1>".to_string(), "<other>".to_string()])
            .await
            .unwrap();
        assert_eq!(known, vec!["<m1>".to_string()]);
    }

    #[tokio::test]
    async fn sweep_drops_bodies_only_for_resolved_rows() {
        let repo = repo().await;
        seed_account(&repo).await;
        let acc = repo.list_pollable_accounts().await.unwrap()[0].clone();
        for (mid, status) in [
            ("<pending>", MailStatus::Pending),
            ("<unparsed>", MailStatus::Unparsed),
            ("<rejected>", MailStatus::Rejected),
        ] {
            repo.insert_message("u1", &message(&acc.id, mid, status))
                .await
                .unwrap();
        }

        assert_eq!(
            repo.sweep_bodies().await.unwrap(),
            1,
            "only rejected is resolved"
        );
        assert_eq!(repo.sweep_bodies().await.unwrap(), 0, "idempotent");

        let mut res = repo
            .db
            .query("SELECT count() AS n FROM mail_message WHERE rawText != NONE GROUP ALL")
            .await
            .unwrap();
        let rows = take_json::<serde_json::Value>(&mut res, 0).unwrap();
        assert_eq!(rows[0]["n"], 2, "the unresolved bodies survive");
    }
}
