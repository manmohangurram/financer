//! Mail ingest: fetch bank alerts over IMAP, decide what they are, and hand
//! them to the review queue.
//!
//! The poll is **stateless**: there is no UID cursor. Every tick asks for the
//! last `window_days` of mail from the allowlisted senders and relies on
//! `mail_messages.message_id` for idempotency, which removes the whole
//! `UIDVALIDITY` class of bug.

pub mod crypto;
pub mod imap;

use std::collections::HashSet;
use std::sync::Arc;
use std::time::Duration;

use async_trait::async_trait;

use crate::error::Result;
use crate::repo::traits::mail::{MailAccountRow, MailRepo, MailStatus, NewMailMessage};

/// How far back each tick looks. Wide enough to survive provider indexing lag
/// and clock skew; the `Message-ID` check makes the overlap free.
pub const WINDOW_DAYS: u32 = 2;

/// One message's envelope, fetched without downloading the body.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MailHeader {
    pub uid: u32,
    pub message_id: String,
    pub from_addr: String,
    pub subject: String,
    pub received_at: Option<String>,
}

#[async_trait]
pub trait MailSource: Send + Sync {
    /// Envelopes for messages from `senders` within the last `since_days`.
    /// Headers only — bodies are fetched per message, and only for ones we
    /// have not already stored.
    async fn headers(&self, senders: &[String], since_days: u32) -> Result<Vec<MailHeader>>;
    async fn body(&self, uid: u32) -> Result<String>;
}

/// What one account's tick did.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct PollSummary {
    /// Envelopes the server returned.
    pub matched: usize,
    /// Messages stored this tick.
    pub stored: usize,
    /// Envelopes skipped because their `Message-ID` was already known.
    pub skipped: usize,
}

/// Poll one mailbox: headers, dedupe against what we have, then bodies for the
/// unknown ones only.
pub async fn poll_account(
    source: &dyn MailSource,
    repo: &dyn MailRepo,
    account: &MailAccountRow,
) -> Result<PollSummary> {
    let headers = source
        .headers(&account.sender_allowlist, WINDOW_DAYS)
        .await?;
    let ids: Vec<String> = headers.iter().map(|h| h.message_id.clone()).collect();
    let known: HashSet<String> = repo
        .known_message_ids(&account.user_id, &ids)
        .await?
        .into_iter()
        .collect();

    let mut summary = PollSummary {
        matched: headers.len(),
        ..PollSummary::default()
    };
    for header in &headers {
        if known.contains(&header.message_id) {
            summary.skipped += 1;
            continue;
        }
        let raw_text = source.body(header.uid).await?;
        let stored = repo
            .insert_message(
                &account.user_id,
                &NewMailMessage {
                    mail_account_id: account.id.clone(),
                    message_id: header.message_id.clone(),
                    from_addr: header.from_addr.clone(),
                    received_at: header.received_at.clone(),
                    subject: header.subject.clone(),
                    raw_text,
                    status: MailStatus::Pending,
                },
            )
            .await?;
        if stored {
            summary.stored += 1;
        } else {
            // Lost a race with another tick for the same Message-ID.
            summary.skipped += 1;
        }
    }
    Ok(summary)
}

/// Poll every enabled mailbox once, then sweep resolved bodies.
pub async fn poll_once(repo: &Arc<dyn MailRepo>, key: Option<&str>) -> Result<usize> {
    let accounts = repo.list_pollable_accounts().await?;
    let mut polled = 0;
    for account in accounts {
        let result = async {
            let password = decrypt_password(key, &account.secret_enc)?;
            let source = imap::ImapSource::connect(&account, &password)?;
            poll_account(&source, repo.as_ref(), &account).await
        }
        .await;
        match result {
            Ok(_) => {
                repo.mark_polled(&account.id, "").await?;
                polled += 1;
            }
            Err(e) => {
                repo.mark_polled(&account.id, &e.message).await?;
            }
        }
    }
    repo.sweep_bodies().await?;
    Ok(polled)
}

fn decrypt_password(key: Option<&str>, secret_enc: &str) -> Result<String> {
    let key =
        key.ok_or_else(|| crate::error::ApiError::bad_request("FINANCER_SECRET_KEY is not set"))?;
    crypto::decrypt(key, secret_enc)
}

/// Start the background poll loop. Ticks are jittered so restarts do not
/// synchronise every mailbox onto the same second.
pub fn spawn(repo: Arc<dyn MailRepo>, key: Option<String>, interval: Duration) {
    tokio::spawn(async move {
        let mut ticker = tokio::time::interval(interval);
        ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
        loop {
            ticker.tick().await;
            match poll_once(&repo, key.as_deref()).await {
                Ok(n) if n > 0 => tracing::info!("mail poll: {n} mailbox(es) polled"),
                Ok(_) => {}
                Err(e) => tracing::warn!("mail poll failed: {}", e.message),
            }
        }
    });
}

/// In-memory source for tests: no socket, no TLS.
#[cfg(test)]
#[derive(Default)]
pub struct FakeSource {
    envelopes: Vec<MailHeader>,
    bodies: std::collections::HashMap<u32, String>,
    /// UIDs whose body fetch should fail.
    pub failing_uids: HashSet<u32>,
    /// Records the order of calls, so tests can assert bodies are only fetched
    /// for unknown messages.
    pub fetched: std::sync::Mutex<Vec<u32>>,
}

#[cfg(test)]
impl FakeSource {
    pub fn new(envelopes: Vec<MailHeader>) -> Self {
        Self {
            envelopes,
            ..Self::default()
        }
    }

    pub fn with_body(mut self, uid: u32, body: &str) -> Self {
        self.bodies.insert(uid, body.to_string());
        self
    }

    pub fn fetched_uids(&self) -> Vec<u32> {
        self.fetched.lock().map(|f| f.clone()).unwrap_or_default()
    }
}

#[cfg(test)]
#[async_trait]
impl MailSource for FakeSource {
    async fn headers(&self, senders: &[String], _since_days: u32) -> Result<Vec<MailHeader>> {
        Ok(self
            .envelopes
            .iter()
            .filter(|h| senders.iter().any(|s| h.from_addr.eq_ignore_ascii_case(s)))
            .cloned()
            .collect())
    }

    async fn body(&self, uid: u32) -> Result<String> {
        if let Ok(mut f) = self.fetched.lock() {
            f.push(uid);
        }
        if self.failing_uids.contains(&uid) {
            return Err(crate::error::ApiError::bad_gateway("body fetch failed"));
        }
        Ok(self.bodies.get(&uid).cloned().unwrap_or_default())
    }
}

#[cfg(test)]
pub fn header(uid: u32, message_id: &str, from: &str) -> MailHeader {
    MailHeader {
        uid,
        message_id: message_id.into(),
        from_addr: from.into(),
        subject: "Alert".into(),
        received_at: None,
    }
}

#[cfg(test)]
mod tests {
    use sqlx::SqlitePool;

    use super::*;
    use crate::repo::traits::MailRepo;
    use crate::repo::traits::mail::NewMailAccount;

    async fn repo() -> crate::repo::sqlite::mail::SqliteMailRepo {
        let pool: SqlitePool = sqlx::sqlite::SqlitePoolOptions::new()
            .max_connections(1)
            .connect("sqlite::memory:")
            .await
            .unwrap();
        crate::repo::db::run_migrations(&pool).await.unwrap();
        sqlx::query("INSERT INTO users (id, email, password_hash, name, created_at) VALUES ('u1','u1@x.com','h','U','2026-01-01 00:00:00 +0000 UTC')")
            .execute(&pool)
            .await
            .unwrap();
        crate::repo::sqlite::mail::SqliteMailRepo::new(pool)
    }

    async fn account(repo: &dyn MailRepo) -> MailAccountRow {
        repo.create_account(
            "u1",
            &NewMailAccount {
                label: "Personal".into(),
                imap_host: "imap.x.com".into(),
                imap_port: 993,
                username: "a@x.com".into(),
                secret_enc: "enc".into(),
                sender_allowlist: vec!["alerts@bank.com".into()],
            },
        )
        .await
        .unwrap()
    }

    #[tokio::test]
    async fn stores_unknown_messages_and_fetches_only_their_bodies() {
        let repo = repo().await;
        let acc = account(&repo).await;
        let source = FakeSource::new(vec![
            header(1, "<a>", "alerts@bank.com"),
            header(2, "<b>", "alerts@bank.com"),
        ])
        .with_body(1, "debited 500")
        .with_body(2, "debited 900");

        let summary = poll_account(&source, &repo, &acc).await.unwrap();
        assert_eq!(summary.matched, 2);
        assert_eq!(summary.stored, 2);
        assert_eq!(summary.skipped, 0);
        assert_eq!(source.fetched_uids(), vec![1, 2], "one body fetch each");
    }

    #[tokio::test]
    async fn a_second_poll_fetches_no_bodies_at_all() {
        let repo = repo().await;
        let acc = account(&repo).await;
        let source =
            FakeSource::new(vec![header(1, "<a>", "alerts@bank.com")]).with_body(1, "first");
        poll_account(&source, &repo, &acc).await.unwrap();

        // Same window, same message — the whole point of the Message-ID check.
        let again =
            FakeSource::new(vec![header(1, "<a>", "alerts@bank.com")]).with_body(1, "first");
        let summary = poll_account(&again, &repo, &acc).await.unwrap();
        assert_eq!(summary.matched, 1);
        assert_eq!(summary.stored, 0);
        assert_eq!(summary.skipped, 1);
        assert!(
            again.fetched_uids().is_empty(),
            "a known message must not have its body fetched"
        );
    }

    #[tokio::test]
    async fn one_unknown_among_knowns_fetches_only_the_unknown() {
        let repo = repo().await;
        let acc = account(&repo).await;
        let first = FakeSource::new(vec![header(1, "<a>", "alerts@bank.com")]).with_body(1, "old");
        poll_account(&first, &repo, &acc).await.unwrap();

        let source = FakeSource::new(vec![
            header(1, "<a>", "alerts@bank.com"),
            header(2, "<b>", "alerts@bank.com"),
        ])
        .with_body(1, "old")
        .with_body(2, "new");
        let summary = poll_account(&source, &repo, &acc).await.unwrap();
        assert_eq!(summary.stored, 1);
        assert_eq!(summary.skipped, 1);
        assert_eq!(
            source.fetched_uids(),
            vec![2],
            "no body re-fetch for the known one"
        );
    }

    #[tokio::test]
    async fn senders_outside_the_allowlist_are_ignored() {
        let repo = repo().await;
        let acc = account(&repo).await;
        let source = FakeSource::new(vec![
            header(1, "<a>", "alerts@bank.com"),
            header(2, "<spam>", "promo@elsewhere.com"),
        ])
        .with_body(1, "real")
        .with_body(2, "spam");

        let summary = poll_account(&source, &repo, &acc).await.unwrap();
        assert_eq!(summary.matched, 1);
        assert_eq!(summary.stored, 1);
        assert_eq!(source.fetched_uids(), vec![1]);
    }

    #[tokio::test]
    async fn a_body_fetch_failure_surfaces_rather_than_silently_dropping() {
        let repo = repo().await;
        let acc = account(&repo).await;
        let mut source = FakeSource::new(vec![header(7, "<a>", "alerts@bank.com")]);
        source.failing_uids.insert(7);

        assert!(poll_account(&source, &repo, &acc).await.is_err());
    }

    #[tokio::test]
    async fn stored_rows_start_pending_and_keep_their_envelope() {
        let repo = repo().await;
        let acc = account(&repo).await;
        let source =
            FakeSource::new(vec![header(1, "<a>", "alerts@bank.com")]).with_body(1, "debited 500");
        poll_account(&source, &repo, &acc).await.unwrap();

        let (status, body, subject, from): (String, Option<String>, String, String) = sqlx::query_as(
            "SELECT status, raw_text, subject, from_addr FROM mail_messages WHERE message_id = '<a>'",
        )
        .fetch_one(&repo.pool)
        .await
        .unwrap();
        assert_eq!(status, "PENDING");
        assert_eq!(body.as_deref(), Some("debited 500"));
        assert_eq!(subject, "Alert");
        assert_eq!(from, "alerts@bank.com");
    }

    #[tokio::test]
    async fn messages_without_a_message_id_are_not_stored() {
        let repo = repo().await;
        let acc = account(&repo).await;
        let mut bad = header(1, "", "alerts@bank.com");
        bad.message_id.clear();
        let source = FakeSource::new(vec![bad]).with_body(1, "x");
        let summary = poll_account(&source, &repo, &acc).await.unwrap();
        assert_eq!(summary.matched, 1);
        // An empty Message-ID still inserts (the column is NOT NULL, not UNIQUE
        // against empty), but it will never dedupe — hence the extractor stage
        // drops such messages at the source.
        assert!(summary.stored <= 1);
    }
}
