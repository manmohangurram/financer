//! Mail ingest: fetch bank alerts over IMAP, decide what they are, and hand
//! them to the review queue.
//!
//! The poll is **stateless**: there is no UID cursor. Every tick asks for the
//! last `window_days` of mail from the allowlisted senders and relies on
//! `mail_messages.message_id` for idempotency, which removes the whole
//! `UIDVALIDITY` class of bug.

pub mod crypto;
pub mod extract;
pub mod imap;

use std::collections::HashSet;
use std::sync::Arc;
use std::time::Duration;

use async_trait::async_trait;

use crate::error::Result;
use crate::repo::traits::mail::{
    MailAccountRow, MailRepo, MailRuleAction, MailRuleRow, MailStatus, NewMailMessage,
};

pub const WINDOW_DAYS: u32 = 2;

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
    async fn headers(&self, senders: &[String], since_days: u32) -> Result<Vec<MailHeader>>;
    async fn body(&self, uid: u32) -> Result<String>;
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct PollSummary {
    pub matched: usize,
    pub stored: usize,
    pub skipped: usize,
}

/// Every non-empty condition must hold; empty means "don't care".
fn rule_matches(rule: &MailRuleRow, header: &MailHeader, body: &str) -> bool {
    let hits = |haystack: &str, needle: &str| {
        needle.is_empty() || haystack.to_lowercase().contains(&needle.to_lowercase())
    };
    hits(&header.from_addr, &rule.from_pattern)
        && hits(&header.subject, &rule.subject_contains)
        && hits(body, &rule.body_contains)
}

pub async fn poll_account(
    source: &dyn MailSource,
    repo: &dyn MailRepo,
    account: &MailAccountRow,
    extractor: &dyn extract::Extractor,
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

    let rules = repo.list_rules(&account.user_id).await?;
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
        let extraction = extractor.extract(&header.subject, &raw_text);
        let mut status = if extraction.is_some() {
            MailStatus::Pending
        } else {
            MailStatus::Unparsed
        };
        // The first enabled rule whose conditions all hold decides the disposition.
        if let Some(rule) = rules
            .iter()
            .find(|r| r.enabled && rule_matches(r, header, &raw_text))
        {
            status = match rule.action {
                MailRuleAction::Ignore => MailStatus::Ignored,
                // shortcut: AUTO reviews instead of creating; the poll has no
                // TransactionService yet. Becomes real when poll_once gets it.
                MailRuleAction::Auto => {
                    // Eligible for unattended creation only on a full parse;
                    // anything less goes to review like a REVIEW rule.
                    if extraction.as_ref().is_some_and(|e| e.confidence >= 1.0) {
                        MailStatus::Auto
                    } else {
                        status
                    }
                }
                MailRuleAction::Review => status,
            };
        }
        let parsed_json = extraction
            .as_ref()
            .and_then(|e| serde_json::to_string(e).ok());
        let confidence = extraction.as_ref().map_or(0.0, |e| f64::from(e.confidence));
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
                    status,
                    parsed_json,
                    confidence,
                    extractor_source: extractor.source().to_string(),
                },
            )
            .await?;
        if stored {
            summary.stored += 1;
        } else {
            summary.skipped += 1;
        }
    }
    Ok(summary)
}

pub async fn poll_once(
    repo: &Arc<dyn MailRepo>,
    key: Option<&str>,
    transactions: &crate::service::transaction::TransactionService,
    account_svc: &crate::service::account::AccountService,
) -> Result<usize> {
    let accounts = repo.list_pollable_accounts().await?;
    let mut polled = 0;
    for account in accounts {
        let result = async {
            let password = decrypt_password(key, &account.secret_enc)?;
            let source = imap::ImapSource::connect(&account, &password).await?;
            poll_account(
                &source,
                repo.as_ref(),
                &account,
                &extract::TemplateExtractor,
            )
            .await
        }
        .await;
        match result {
            Ok(_) => {
                repo.mark_polled(&account.id, "").await?;
                auto_create(repo, key, &account, transactions, account_svc).await;
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

/// Approve the rows an AUTO rule marked, leaving anything that cannot be
/// resolved in the review queue.
async fn auto_create(
    repo: &Arc<dyn MailRepo>,
    key: Option<&str>,
    account: &MailAccountRow,
    transactions: &crate::service::transaction::TransactionService,
    accounts: &crate::service::account::AccountService,
) {
    let rows = match repo
        .list_messages(&account.user_id, Some(MailStatus::Auto), 200)
        .await
    {
        Ok(rows) => rows,
        Err(e) => {
            tracing::warn!("auto-create: could not list candidates: {}", e.message);
            return;
        }
    };
    let ids: Vec<String> = rows
        .into_iter()
        .filter(|m| m.transaction_id.is_none())
        .map(|m| m.id)
        .collect();
    if ids.is_empty() {
        return;
    }

    let service = crate::service::mail::MailService::new(repo.clone(), key.map(str::to_string));
    let actions = ids
        .iter()
        .map(|id| crate::service::mail::MailActionItem {
            id: id.clone(),
            action: crate::service::mail::MailActionKind::Approve,
            account_id: None,
            category_ids: vec![],
        })
        .collect();
    match service
        .act(&account.user_id, actions, transactions, accounts)
        .await
    {
        Ok(out) => {
            for failure in out.failed {
                // Back to the queue: an ambiguous account must be reviewed,
                // not retried every tick.
                let _ = repo
                    .resolve_message(
                        &account.user_id,
                        &failure.id,
                        MailStatus::Pending,
                        None,
                        None,
                    )
                    .await;
                tracing::warn!("auto-create skipped {}: {}", failure.id, failure.error);
            }
        }
        Err(e) => tracing::warn!("auto-create failed: {}", e.message),
    }
}

pub fn spawn(
    repo: Arc<dyn MailRepo>,
    key: Option<String>,
    interval: Duration,
    transactions: crate::service::transaction::TransactionService,
    accounts: crate::service::account::AccountService,
) {
    tokio::spawn(async move {
        let mut ticker = tokio::time::interval(interval);
        ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
        loop {
            ticker.tick().await;
            match poll_once(&repo, key.as_deref(), &transactions, &accounts).await {
                Ok(n) if n > 0 => tracing::info!("mail poll: {n} mailbox(es) polled"),
                Ok(_) => {}
                Err(e) => tracing::warn!("mail poll failed: {}", e.message),
            }
        }
    });
}

#[cfg(test)]
#[derive(Default)]
pub struct FakeSource {
    envelopes: Vec<MailHeader>,
    bodies: std::collections::HashMap<u32, String>,
    pub failing_uids: HashSet<u32>,
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
        .with_body(1, "INR 500.00 debited at SHOP on 12/09/2026")
        .with_body(2, "INR 900.00 debited at SHOP on 12/09/2026");

        let summary = poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();
        assert_eq!(summary.matched, 2);
        assert_eq!(summary.stored, 2);
        assert_eq!(summary.skipped, 0);
        assert_eq!(source.fetched_uids(), vec![1, 2], "one body fetch each");
    }

    #[tokio::test]
    async fn a_second_poll_fetches_no_bodies_at_all() {
        let repo = repo().await;
        let acc = account(&repo).await;
        let source = FakeSource::new(vec![header(1, "<a>", "alerts@bank.com")])
            .with_body(1, "INR 100.00 debited at SHOP on 01/09/2026");
        poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();

        let again = FakeSource::new(vec![header(1, "<a>", "alerts@bank.com")])
            .with_body(1, "INR 100.00 debited at SHOP on 01/09/2026");
        let summary = poll_account(&again, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();
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
        let first = FakeSource::new(vec![header(1, "<a>", "alerts@bank.com")])
            .with_body(1, "INR 100.00 debited at SHOP on 01/09/2026");
        poll_account(&first, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();

        let source = FakeSource::new(vec![
            header(1, "<a>", "alerts@bank.com"),
            header(2, "<b>", "alerts@bank.com"),
        ])
        .with_body(1, "INR 100.00 debited at SHOP on 01/09/2026")
        .with_body(2, "INR 200.00 debited at SHOP on 02/09/2026");
        let summary = poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();
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
        .with_body(1, "INR 300.00 debited at SHOP on 03/09/2026")
        .with_body(2, "INR 400.00 debited at SHOP on 04/09/2026");

        let summary = poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();
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

        assert!(
            poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
                .await
                .is_err()
        );
    }

    #[tokio::test]
    async fn stored_rows_start_pending_and_keep_their_envelope() {
        let repo = repo().await;
        let acc = account(&repo).await;
        let source = FakeSource::new(vec![header(1, "<a>", "alerts@bank.com")])
            .with_body(1, "INR 500.00 debited at SHOP on 12/09/2026");
        poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();

        let (status, body, subject, from): (String, Option<String>, String, String) = sqlx::query_as(
            "SELECT status, raw_text, subject, from_addr FROM mail_messages WHERE message_id = '<a>'",
        )
        .fetch_one(&repo.pool)
        .await
        .unwrap();
        assert_eq!(status, "PENDING");
        assert!(body.as_deref().unwrap_or_default().contains("INR 500.00"));
        assert_eq!(subject, "Alert");
        assert_eq!(from, "alerts@bank.com");
    }

    #[tokio::test]
    async fn a_matching_rule_decides_the_disposition() {
        let repo = repo().await;
        let acc = account(&repo).await;
        repo.create_rule(
            "u1",
            &crate::repo::traits::mail::MailRuleInput {
                position: 0,
                enabled: true,
                from_pattern: "alerts@bank.com".into(),
                subject_contains: String::new(),
                body_contains: String::new(),
                action: MailRuleAction::Ignore,
                account_id: None,
                category_ids: vec![],
            },
        )
        .await
        .unwrap();

        let source = FakeSource::new(vec![header(1, "<a>", "alerts@bank.com")])
            .with_body(1, "INR 500.00 debited at SHOP on 12/09/2026");
        poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();

        let status: String =
            sqlx::query_scalar("SELECT status FROM mail_messages WHERE message_id = '<a>'")
                .fetch_one(&repo.pool)
                .await
                .unwrap();
        assert_eq!(status, "IGNORED", "the rule wins over the extractor");
    }

    #[tokio::test]
    async fn a_non_matching_rule_leaves_the_message_pending() {
        let repo = repo().await;
        let acc = account(&repo).await;
        repo.create_rule(
            "u1",
            &crate::repo::traits::mail::MailRuleInput {
                position: 0,
                enabled: true,
                from_pattern: "someone-else@bank.com".into(),
                subject_contains: String::new(),
                body_contains: String::new(),
                action: MailRuleAction::Ignore,
                account_id: None,
                category_ids: vec![],
            },
        )
        .await
        .unwrap();

        let source = FakeSource::new(vec![header(1, "<b>", "alerts@bank.com")])
            .with_body(1, "INR 500.00 debited at SHOP on 12/09/2026");
        poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();

        let status: String =
            sqlx::query_scalar("SELECT status FROM mail_messages WHERE message_id = '<b>'")
                .fetch_one(&repo.pool)
                .await
                .unwrap();
        assert_eq!(status, "PENDING");
    }

    async fn auto_rule(repo: &dyn MailRepo) {
        repo.create_rule(
            "u1",
            &crate::repo::traits::mail::MailRuleInput {
                position: 0,
                enabled: true,
                from_pattern: "alerts@bank.com".into(),
                subject_contains: String::new(),
                body_contains: String::new(),
                action: MailRuleAction::Auto,
                account_id: None,
                category_ids: vec![],
            },
        )
        .await
        .unwrap();
    }

    #[tokio::test]
    async fn an_auto_rule_with_a_full_parse_is_marked_for_creation() {
        let repo = repo().await;
        let acc = account(&repo).await;
        auto_rule(&repo).await;

        let source = FakeSource::new(vec![header(1, "<auto>", "alerts@bank.com")]).with_body(
            1,
            "Your A/c XX1234 has been debited with INR 1,249.50 at SWIGGY on 12/09/2026.",
        );
        poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();

        let status: String =
            sqlx::query_scalar("SELECT status FROM mail_messages WHERE message_id = '<auto>'")
                .fetch_one(&repo.pool)
                .await
                .unwrap();
        assert_eq!(status, "AUTO");
    }

    #[tokio::test]
    async fn an_auto_rule_with_a_partial_parse_stays_in_review() {
        let repo = repo().await;
        let acc = account(&repo).await;
        auto_rule(&repo).await;

        // No account digits and no date, so confidence is below 1.0.
        let source = FakeSource::new(vec![header(2, "<partial>", "alerts@bank.com")])
            .with_body(2, "INR 500.00 debited at SHOP");
        poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();

        let status: String =
            sqlx::query_scalar("SELECT status FROM mail_messages WHERE message_id = '<partial>'")
                .fetch_one(&repo.pool)
                .await
                .unwrap();
        assert_eq!(status, "PENDING", "a partial parse must never auto-create");
    }

    #[tokio::test]
    async fn messages_without_a_message_id_are_not_stored() {
        let repo = repo().await;
        let acc = account(&repo).await;
        let mut bad = header(1, "", "alerts@bank.com");
        bad.message_id.clear();
        let source = FakeSource::new(vec![bad]).with_body(1, "x");
        let summary = poll_account(&source, &repo, &acc, &extract::TemplateExtractor)
            .await
            .unwrap();
        assert_eq!(summary.matched, 1);
        assert!(summary.stored <= 1);
    }
}
