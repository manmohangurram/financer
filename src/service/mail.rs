//! Mailbox configuration: linking an IMAP account, listing it, removing it.

use std::net::IpAddr;
use std::sync::Arc;

use serde::{Deserialize, Serialize};
use utoipa::ToSchema;

use crate::error::{ApiError, Result};
use crate::mail::crypto;
use crate::repo::traits::mail::{
    MailAccountRow, MailMessageRow, MailRepo, MailRuleAction, MailRuleInput, MailRuleRow,
    MailStatus, NewMailAccount,
};
use crate::repo::traits::transaction::TransactionType;
use crate::service::account::AccountService;
use std::str::FromStr;

#[derive(Clone)]
pub struct MailService {
    repo: Arc<dyn MailRepo>,
    secret_key: Option<String>,
}

impl MailService {
    pub fn new(repo: Arc<dyn MailRepo>, secret_key: Option<String>) -> Self {
        Self { repo, secret_key }
    }

    pub async fn create(&self, user_id: &str, req: CreateMailAccountReq) -> Result<MailAccount> {
        let host = validate_host(&req.imap_host)?;
        validate_port(req.imap_port)?;
        if req.username.trim().is_empty() {
            return Err(ApiError::bad_request("username is required"));
        }
        if req.password.is_empty() {
            return Err(ApiError::bad_request("password is required"));
        }
        validate_allowlist(&req.sender_allowlist)?;
        let key = self.secret_key.as_deref().ok_or_else(|| {
            ApiError::internal("FINANCER_SECRET_KEY is not set; cannot store mailbox credentials")
        })?;

        let row = self
            .repo
            .create_account(
                user_id,
                &NewMailAccount {
                    label: req.label.trim().to_string(),
                    imap_host: host,
                    imap_port: req.imap_port,
                    username: req.username.trim().to_string(),
                    secret_enc: crypto::encrypt(key, &req.password)?,
                    sender_allowlist: req.sender_allowlist,
                },
            )
            .await?;
        Ok(row.into())
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<MailAccount>> {
        Ok(self
            .repo
            .list_accounts(user_id)
            .await?
            .into_iter()
            .map(Into::into)
            .collect())
    }

    pub async fn messages(
        &self,
        user_id: &str,
        status: Option<&str>,
        limit: i64,
    ) -> Result<Vec<MailMessageRow>> {
        let status = match status {
            None => None,
            Some(raw) if raw.is_empty() || raw == "all" => None,
            Some(raw) => Some(
                crate::repo::traits::mail::MailStatus::from_str(raw)
                    .map_err(|_| ApiError::bad_request(format!("unknown status {raw:?}")))?,
            ),
        };
        self.repo
            .list_messages(user_id, status, limit.clamp(1, 500))
            .await
    }

    /// Approve or reject a batch. Per row, so a failure reports exactly which
    /// one and why instead of rolling the good ones back.
    pub async fn act(
        &self,
        user_id: &str,
        items: Vec<MailActionItem>,
        transactions: &crate::service::transaction::TransactionService,
        accounts: &AccountService,
    ) -> Result<MailActionOutcome> {
        let mut out = MailActionOutcome::default();
        for item in items {
            let result = self.act_one(user_id, &item, transactions, accounts).await;
            match result {
                Ok(()) => out.succeeded.push(item.id),
                Err(e) => out.failed.push(MailActionFailure {
                    id: item.id,
                    error: e.message,
                }),
            }
        }
        Ok(out)
    }

    async fn act_one(
        &self,
        user_id: &str,
        item: &MailActionItem,
        transactions: &crate::service::transaction::TransactionService,
        accounts: &AccountService,
    ) -> Result<()> {
        let msg = self
            .repo
            .get_message(user_id, &item.id)
            .await?
            .ok_or_else(|| ApiError::not_found(format!("message {} not found", item.id)))?;
        if !matches!(msg.status, MailStatus::Pending | MailStatus::Unparsed) {
            return Err(ApiError::conflict("message already decided"));
        }

        if item.action == MailActionKind::Reject {
            self.repo
                .resolve_message(user_id, &item.id, MailStatus::Rejected, None, None)
                .await?;
            return Ok(());
        }

        let parsed = msg
            .parsed_json
            .as_deref()
            .ok_or_else(|| ApiError::bad_request("nothing was extracted from this mail"))?;
        let extraction: crate::mail::extract::Extraction = serde_json::from_str(parsed)
            .map_err(|_| ApiError::internal("stored extraction is not readable"))?;

        let account_id = if let Some(id) = &item.account_id {
            id.clone()
        } else {
            let owned = accounts.list(user_id).await?;
            let pairs: Vec<(String, Option<String>)> = owned
                .into_iter()
                .map(|a| (a.id, a.ending_numbers))
                .collect();
            crate::mail::extract::resolve_account(extraction.account_hint.as_deref(), &pairs)
                .ok_or_else(|| {
                    ApiError::bad_request("no account matches this mail; pass accountId")
                })?
        };

        let txn_id = uuid::Uuid::new_v4().to_string();
        let req = crate::service::transaction::TransactionReq {
            id: txn_id.clone(),
            name: extraction.merchant.clone(),
            amount: extraction.amount,
            transaction_type: match extraction.direction {
                crate::mail::extract::Direction::Debit => TransactionType::Debit,
                crate::mail::extract::Direction::Credit => TransactionType::Credit,
            },
            occurred_at: extraction.occurred_at.clone().unwrap_or_default(),
            account_id,
            category_ids: item.category_ids.clone(),
            external_id: Some(format!("mail:{}", msg.message_id)),
        };
        let result = transactions.create(user_id, &[req]).await?;
        if !result.success {
            return Err(ApiError::bad_request(result.message));
        }

        self.repo
            .resolve_message(
                user_id,
                &item.id,
                MailStatus::Auto,
                Some(parsed),
                Some(&txn_id),
            )
            .await?;
        Ok(())
    }

    /// Poll one mailbox immediately instead of waiting for the ticker.
    pub async fn poll_now(&self, user_id: &str, id: &str) -> Result<crate::mail::PollSummary> {
        let account = self
            .repo
            .list_accounts(user_id)
            .await?
            .into_iter()
            .find(|a| a.id == id)
            .ok_or_else(|| ApiError::not_found(format!("mail account {id} not found")))?;
        let key = self
            .secret_key
            .as_deref()
            .ok_or_else(|| ApiError::internal("FINANCER_SECRET_KEY is not set"))?;

        let password = crate::mail::crypto::decrypt(key, &account.secret_enc)?;
        let source = crate::mail::imap::ImapSource::connect(&account, &password).await?;
        let summary = crate::mail::poll_account(
            &source,
            self.repo.as_ref(),
            &account,
            &crate::mail::extract::TemplateExtractor,
        )
        .await?;
        self.repo.mark_polled(&account.id, "").await?;
        Ok(summary)
    }

    pub async fn rules(&self, user_id: &str) -> Result<Vec<MailRuleRow>> {
        self.repo.list_rules(user_id).await
    }

    pub async fn create_rule(&self, user_id: &str, req: MailRuleReq) -> Result<MailRuleRow> {
        let input = req.into_input()?;
        self.repo.create_rule(user_id, &input).await
    }

    pub async fn update_rule(&self, user_id: &str, id: &str, req: MailRuleReq) -> Result<()> {
        let input = req.into_input()?;
        if !self.repo.update_rule(user_id, id, &input).await? {
            return Err(ApiError::not_found(format!("mail rule {id} not found")));
        }
        Ok(())
    }

    pub async fn delete_rule(&self, user_id: &str, id: &str) -> Result<()> {
        if !self.repo.delete_rule(user_id, id).await? {
            return Err(ApiError::not_found(format!("mail rule {id} not found")));
        }
        Ok(())
    }

    pub async fn delete(&self, user_id: &str, id: &str) -> Result<()> {
        if !self.repo.delete_account(user_id, id).await? {
            return Err(ApiError::not_found(format!("mail account {id} not found")));
        }
        Ok(())
    }
}

#[derive(Debug, Deserialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct CreateMailAccountReq {
    #[serde(default)]
    pub label: String,
    pub imap_host: String,
    pub imap_port: i64,
    pub username: String,
    pub password: String,
    #[serde(default)]
    pub sender_allowlist: Vec<String>,
}

#[derive(Debug, serde::Deserialize, ToSchema)]
#[serde(rename_all = "camelCase")]
pub struct MailRuleReq {
    #[serde(default)]
    pub position: i64,
    /// Defaults to on.
    #[serde(default)]
    pub enabled: Option<bool>,
    #[serde(default)]
    pub from_pattern: String,
    #[serde(default)]
    pub subject_contains: String,
    #[serde(default)]
    pub body_contains: String,
    pub action: MailRuleAction,
    #[serde(default)]
    pub account_id: Option<String>,
    #[serde(default)]
    pub category_ids: Vec<String>,
}

impl MailRuleReq {
    fn into_input(self) -> Result<MailRuleInput> {
        let from_pattern = self.from_pattern.trim().to_string();
        let subject_contains = self.subject_contains.trim().to_string();
        let body_contains = self.body_contains.trim().to_string();
        // A rule with no conditions matches every mail, which would make an
        // AUTO rule ingest the whole mailbox.
        if from_pattern.is_empty() && subject_contains.is_empty() && body_contains.is_empty() {
            return Err(ApiError::bad_request("a rule needs at least one condition"));
        }
        Ok(MailRuleInput {
            position: self.position,
            enabled: self.enabled.unwrap_or(true),
            from_pattern,
            subject_contains,
            body_contains,
            action: self.action,
            account_id: self.account_id.filter(|a| !a.is_empty()),
            category_ids: self.category_ids,
        })
    }
}

#[derive(Debug, serde::Deserialize, ToSchema)]
#[serde(rename_all = "camelCase")]
pub struct MailActionReq {
    pub messages: Vec<MailActionItem>,
}

#[derive(Debug, serde::Deserialize, ToSchema)]
#[serde(rename_all = "camelCase")]
pub struct MailActionItem {
    pub id: String,
    pub action: MailActionKind,
    /// Overrides the account resolved from the mail's trailing digits.
    #[serde(default)]
    pub account_id: Option<String>,
    #[serde(default)]
    pub category_ids: Vec<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, serde::Deserialize, ToSchema)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum MailActionKind {
    Approve,
    Reject,
}

#[derive(Debug, Default, serde::Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
pub struct MailActionOutcome {
    pub succeeded: Vec<String>,
    pub failed: Vec<MailActionFailure>,
}

#[derive(Debug, serde::Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
pub struct MailActionFailure {
    pub id: String,
    pub error: String,
}

#[derive(Debug, Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct MailAccount {
    pub id: String,
    pub label: String,
    pub imap_host: String,
    pub imap_port: i64,
    pub username: String,
    pub sender_allowlist: Vec<String>,
    pub enabled: bool,
    pub last_polled_at: Option<String>,
    pub last_error: String,
}

impl From<MailAccountRow> for MailAccount {
    fn from(r: MailAccountRow) -> Self {
        Self {
            id: r.id,
            label: r.label,
            imap_host: r.imap_host,
            imap_port: r.imap_port,
            username: r.username,
            sender_allowlist: r.sender_allowlist,
            enabled: r.enabled,
            last_polled_at: r.last_polled_at,
            last_error: r.last_error,
        }
    }
}

/// Link-local and metadata addresses are refused; private ranges are allowed
fn validate_host(host: &str) -> Result<String> {
    let host = host.trim().to_ascii_lowercase();
    if host.is_empty() {
        return Err(ApiError::bad_request("imapHost is required"));
    }
    if host
        .chars()
        .any(|c| c.is_whitespace() || c == '/' || c == '@')
    {
        return Err(ApiError::bad_request("imapHost must be a bare hostname"));
    }
    if let Ok(ip) = host.parse::<IpAddr>()
        && is_blocked(&ip)
    {
        return Err(ApiError::bad_request(
            "imapHost resolves to a link-local or metadata address",
        ));
    }
    Ok(host)
}

fn is_blocked(ip: &IpAddr) -> bool {
    match ip {
        IpAddr::V4(v4) => v4.octets()[..2] == [169, 254],
        IpAddr::V6(v6) => {
            let seg = v6.segments()[0];
            (seg & 0xffc0) == 0xfe80 // fe80::/10 link-local
                || (seg & 0xfe00) == 0xfc00 // fc00::/7 unique-local
        }
    }
}

/// Only implicit TLS is implemented, so only the implicit-TLS port is allowed.
fn validate_port(port: i64) -> Result<()> {
    match port {
        993 => Ok(()),
        143 => Err(ApiError::bad_request(
            "port 143 needs STARTTLS, which is not supported yet",
        )),
        _ => Err(ApiError::bad_request("imapPort must be 993 (implicit TLS)")),
    }
}

fn validate_allowlist(senders: &[String]) -> Result<()> {
    if senders.is_empty() {
        return Err(ApiError::bad_request(
            "senderAllowlist must name at least one sender",
        ));
    }
    for sender in senders {
        let s = sender.trim();
        if s.is_empty() || !s.contains('@') || s.chars().any(|c| c.is_whitespace() || c == '"') {
            return Err(ApiError::bad_request(format!(
                "senderAllowlist entry {sender:?} is not an email address"
            )));
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use super::*;
    use crate::repo::sqlite::account::SqliteAccountRepo;
    use crate::repo::sqlite::category::SqliteCategoryRepo;
    use crate::repo::sqlite::mail::SqliteMailRepo;
    use crate::repo::sqlite::transaction::SqliteTransactionRepo;
    use crate::repo::traits::account::AccountType;
    use crate::repo::traits::mail::MailRepo;
    use crate::repo::traits::{AccountRepo, CategoryRepo, TransactionRepo};
    use crate::service::transaction::TransactionService;

    const KEY: &str = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f";

    struct Fixture {
        pool: sqlx::SqlitePool,
        mail: Arc<SqliteMailRepo>,
        account_repo: Arc<dyn AccountRepo>,
        transactions: TransactionService,
        accounts: AccountService,
    }

    async fn fixture() -> Fixture {
        let pool: sqlx::SqlitePool = sqlx::sqlite::SqlitePoolOptions::new()
            .max_connections(1)
            .connect("sqlite::memory:")
            .await
            .unwrap();
        crate::repo::db::run_migrations(&pool).await.unwrap();
        sqlx::query("INSERT INTO users (id, email, password_hash, name, created_at) VALUES ('u1','u1@x.com','h','U','2026-01-01 00:00:00 +0000 UTC')")
            .execute(&pool)
            .await
            .unwrap();

        let account_repo: Arc<dyn AccountRepo> = Arc::new(SqliteAccountRepo::new(pool.clone()));
        let txn_repo: Arc<dyn TransactionRepo> = Arc::new(SqliteTransactionRepo::new(pool.clone()));
        let cat_repo: Arc<dyn CategoryRepo> = Arc::new(SqliteCategoryRepo::new(pool.clone()));
        Fixture {
            mail: Arc::new(SqliteMailRepo::new(pool.clone())),
            transactions: TransactionService::new(txn_repo, account_repo.clone(), cat_repo),
            accounts: AccountService::new(account_repo.clone()),
            account_repo,
            pool,
        }
    }

    /// A pending message whose extraction points at `hint`.
    async fn seed(f: &Fixture, ending: &str, hint: &str, message_id: &str) -> String {
        f.account_repo
            .create("u1", "Bank", "Main", AccountType::Current, ending)
            .await
            .unwrap();
        let acc = f
            .mail
            .create_account(
                "u1",
                &NewMailAccount {
                    label: "P".into(),
                    imap_host: "imap.x.com".into(),
                    imap_port: 993,
                    username: "a@x.com".into(),
                    secret_enc: "x".into(),
                    sender_allowlist: vec!["alerts@bank.com".into()],
                },
            )
            .await
            .unwrap();
        let extraction = crate::mail::extract::Extraction {
            amount: 500.0,
            direction: crate::mail::extract::Direction::Debit,
            occurred_at: Some("2026-09-12".into()),
            merchant: "SHOP".into(),
            account_hint: Some(hint.into()),
            confidence: 1.0,
        };
        let id = uuid::Uuid::new_v4().to_string();
        sqlx::query(
            "INSERT INTO mail_messages (id, user_id, mail_account_id, message_id, status, parsed_json, confidence, created_at)
             VALUES (?, 'u1', ?, ?, 'PENDING', ?, 1.0, '2026-09-12 00:00:00 +0000 UTC')",
        )
        .bind(&id)
        .bind(&acc.id)
        .bind(message_id)
        .bind(serde_json::to_string(&extraction).unwrap())
        .execute(&f.pool)
        .await
        .unwrap();
        id
    }

    fn approve(id: &str) -> MailActionItem {
        MailActionItem {
            id: id.into(),
            action: MailActionKind::Approve,
            account_id: None,
            category_ids: vec![],
        }
    }

    #[tokio::test]
    async fn approve_creates_the_transaction_and_marks_the_row() {
        let f = fixture().await;
        let msg = seed(&f, "1234", "1234", "<m1>").await;
        let svc = MailService::new(f.mail.clone(), Some(KEY.to_string()));

        let out = svc
            .act("u1", vec![approve(&msg)], &f.transactions, &f.accounts)
            .await
            .unwrap();
        assert_eq!(out.succeeded, vec![msg.clone()]);
        assert!(out.failed.is_empty(), "{:?}", out.failed);

        let (status, txn): (String, Option<String>) =
            sqlx::query_as("SELECT status, transaction_id FROM mail_messages WHERE id = ?")
                .bind(&msg)
                .fetch_one(&f.pool)
                .await
                .unwrap();
        assert_eq!(status, "AUTO");
        assert!(txn.is_some(), "transaction id recorded on the row");

        let ext: Option<String> =
            sqlx::query_scalar("SELECT external_id FROM transactions LIMIT 1")
                .fetch_one(&f.pool)
                .await
                .unwrap();
        assert_eq!(
            ext.as_deref(),
            Some("mail:<m1>"),
            "dedupe rides on Message-ID"
        );
    }

    #[tokio::test]
    async fn reject_marks_the_row_and_creates_nothing() {
        let f = fixture().await;
        let msg = seed(&f, "1234", "1234", "<m2>").await;
        let svc = MailService::new(f.mail.clone(), Some(KEY.to_string()));

        svc.act(
            "u1",
            vec![MailActionItem {
                id: msg.clone(),
                action: MailActionKind::Reject,
                account_id: None,
                category_ids: vec![],
            }],
            &f.transactions,
            &f.accounts,
        )
        .await
        .unwrap();

        let status: String = sqlx::query_scalar("SELECT status FROM mail_messages WHERE id = ?")
            .bind(&msg)
            .fetch_one(&f.pool)
            .await
            .unwrap();
        assert_eq!(status, "REJECTED");
        let n: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM transactions")
            .fetch_one(&f.pool)
            .await
            .unwrap();
        assert_eq!(n, 0);
    }

    #[tokio::test]
    async fn an_ambiguous_hint_fails_the_row_instead_of_guessing() {
        let f = fixture().await;
        let first = seed(&f, "1234", "1234", "<m3>").await;
        let second = seed(&f, "1234", "1234", "<m4>").await;
        let svc = MailService::new(f.mail.clone(), Some(KEY.to_string()));

        let out = svc
            .act(
                "u1",
                vec![approve(&first), approve(&second)],
                &f.transactions,
                &f.accounts,
            )
            .await
            .unwrap();
        assert!(
            out.succeeded.is_empty(),
            "two accounts end in 1234, so neither may be guessed"
        );
        assert_eq!(out.failed.len(), 2);
        assert!(out.failed[0].error.contains("no account matches"));
    }

    #[tokio::test]
    async fn an_already_decided_row_is_refused() {
        let f = fixture().await;
        let msg = seed(&f, "1234", "1234", "<m5>").await;
        let svc = MailService::new(f.mail.clone(), Some(KEY.to_string()));

        svc.act("u1", vec![approve(&msg)], &f.transactions, &f.accounts)
            .await
            .unwrap();
        let again = svc
            .act("u1", vec![approve(&msg)], &f.transactions, &f.accounts)
            .await
            .unwrap();
        assert_eq!(again.failed.len(), 1);
        assert!(again.failed[0].error.contains("already decided"));
    }

    #[test]
    fn refuses_metadata_and_link_local_hosts() {
        for host in ["169.254.169.254", "169.254.1.1", "fe80::1", "fd00::1"] {
            assert!(validate_host(host).is_err(), "{host} must be refused");
        }
    }

    #[test]
    fn allows_public_and_lan_hosts() {
        for host in ["imap.gmail.com", "192.168.1.10", "127.0.0.1", "mail.local"] {
            assert!(validate_host(host).is_ok(), "{host} should be allowed");
        }
    }

    #[test]
    fn refuses_hosts_that_are_not_bare_hostnames() {
        for host in ["", "  ", "imap.gmail.com/evil", "user@host", "a b"] {
            assert!(validate_host(host).is_err(), "{host:?} must be refused");
        }
    }

    #[test]
    fn only_implicit_tls_ports_are_accepted() {
        assert!(validate_port(993).is_ok());
        assert!(validate_port(143).is_err(), "STARTTLS is not implemented");
        assert!(validate_port(8080).is_err());
    }

    #[test]
    fn allowlist_must_hold_addresses() {
        assert!(validate_allowlist(&[]).is_err());
        assert!(validate_allowlist(&["not-an-address".into()]).is_err());
        assert!(validate_allowlist(&["a@b\"c".into()]).is_err());
        assert!(validate_allowlist(&["alerts@hdfcbank.net".into()]).is_ok());
    }
}
