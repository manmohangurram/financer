//! Test harness: build a SQLite-backed `AppState` and drive the axum router
//! in-process (no network) via `tower`'s `oneshot`.

use std::sync::Arc;

use axum::body::Body;
use axum::http::{Request, StatusCode};
use http_body_util::BodyExt;
use serde_json::Value;
use tower::ServiceExt;

use crate::auth::Jwt;
use crate::config::Sqlite;
use crate::http::AppState;
use crate::service::account::AccountService;
use crate::service::auth::AuthService;
use crate::service::category::CategoryService;
use crate::service::investment::InvestmentService;
use crate::service::rule::RuleService;
use crate::service::transaction::TransactionService;
use crate::service::transfer::TransferService;
use crate::service::transfer_rule::TransferRuleService;
use crate::service::user::UserService;
use crate::service::user_key::UserKeyService;
use crate::service::yahoo::YahooClient;

/// A running test app on a temp SQLite DB. Keeps the tempdir alive.
pub struct TestApp {
    pub router: axum::Router,
    pub token: String,
    pub account_id: String,
    _dir: tempfile::TempDir,
}

impl TestApp {
    /// Spin up a fresh app, sign up a user, and create one current account.
    pub async fn new() -> Self {
        let dir = tempfile::tempdir().unwrap();
        let cfg = Sqlite {
            path: dir
                .path()
                .join("financer.db")
                .to_string_lossy()
                .into_owned(),
            journal_mode: "memory".into(),
            synchronous: "off".into(),
            busy_timeout_ms: 5000,
            foreign_keys: true,
            page_size: 0,
            write_pool_size: 1,
            read_pool_size: 1,
        };
        let repos = crate::repo::db::sqlite_repo_set(&cfg).await.unwrap();
        let jwt = Jwt::new("test-secret-test-secret-test-secret-test-secret".into());

        let account = AccountService::new(repos.account.clone());
        let category = CategoryService::new(repos.category.clone());
        let rule = RuleService::new(
            repos.rule.clone(),
            repos.category.clone(),
            repos.transaction.clone(),
        );
        let transfer_rule = TransferRuleService::new(
            repos.rule.clone(),
            repos.transaction.clone(),
            repos.account.clone(),
        );
        let transaction = TransactionService::new(
            repos.transaction.clone(),
            repos.account.clone(),
            repos.category.clone(),
        )
        .with_transfer_rule(transfer_rule.clone())
        .with_rule(Arc::new(rule.clone()));
        let transfer = TransferService::new(repos.transaction.clone(), repos.account.clone());

        let state = AppState {
            auth: AuthService::new(repos.user.clone(), jwt.clone()),
            user: UserService::new(repos.user.clone(), jwt.clone(), dir.path().join("avatars")),
            user_key: UserKeyService::new(repos.user_key.clone()),
            account,
            investment: InvestmentService::new(
                repos.investment.clone(),
                YahooClient::new(&crate::config::YahooConfigSection::default()).unwrap(),
            ),
            category,
            rule,
            transfer_rule,
            transaction,
            transfer,
            jwt,
            static_dir: String::new(),
            avatar_dir: dir.path().join("avatars").to_string_lossy().into_owned(),
            domain_url: String::new(),
        };
        let router = crate::http::router(state);

        let mut app = Self {
            router,
            token: String::new(),
            account_id: String::new(),
            _dir: dir,
        };
        let signup = app
            .post_json(
                "/api/auth/signup",
                &serde_json::json!({
                    "email": "test@x.com", "password": "secret1", "name": "Test"
                }),
                None,
            )
            .await;
        assert_eq!(signup.0, StatusCode::CREATED, "signup failed: {}", signup.1);
        app.token = signup.1["accessToken"].as_str().unwrap().to_string();

        let acc = app
            .post_json(
                "/api/accounts",
                &serde_json::json!({
                    "bankName": "Chase", "nickname": "Main", "type": "CURRENT"
                }),
                Some(&app.token.clone()),
            )
            .await;
        assert_eq!(
            acc.0,
            StatusCode::CREATED,
            "account create failed: {}",
            acc.1
        );
        app.account_id = acc.1["id"].as_str().unwrap().to_string();
        app
    }

    /// Send a request; returns (status, parsed JSON body). `token` = optional bearer.
    pub async fn request(
        &self,
        method: &str,
        path: &str,
        body: Option<Value>,
        token: Option<&str>,
    ) -> (StatusCode, Value) {
        let mut b = Request::builder().method(method).uri(path);
        if body.is_some() {
            b = b.header("content-type", "application/json");
        }
        if let Some(t) = token {
            b = b.header("authorization", format!("Bearer {t}"));
        }
        let req = b
            .body(body.map_or_else(Body::empty, |v| Body::from(v.to_string())))
            .unwrap();
        let res = self.router.clone().oneshot(req).await.unwrap();
        let status = res.status();
        let bytes = res.into_body().collect().await.unwrap().to_bytes();
        let json = if bytes.is_empty() {
            Value::Null
        } else {
            serde_json::from_slice(&bytes).unwrap_or(Value::Null)
        };
        (status, json)
    }

    pub async fn get_json(&self, path: &str, token: Option<&str>) -> (StatusCode, Value) {
        self.request("GET", path, None, token).await
    }

    pub async fn post_json(
        &self,
        path: &str,
        body: &Value,
        token: Option<&str>,
    ) -> (StatusCode, Value) {
        self.request("POST", path, Some(body.clone()), token).await
    }

    /// Create a transaction with the test account.
    pub async fn create_txn(&self, name: &str, amount: f64, ty: &str) -> (StatusCode, Value) {
        self.post_json("/api/transactions", &serde_json::json!({
            "transactions": [{ "name": name, "amount": amount, "type": ty, "accountId": self.account_id }]
        }), Some(&self.token)).await
    }
}
