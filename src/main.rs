//! Financer Rust backend — Rust-only server on `SurrealDB` OR `SQLite` (picked
//! at runtime from env), axum API, serves the built Vue frontend.

mod auth;
mod config;
mod db;
mod error;
mod http;
mod openapi;
mod repo;
mod service;
mod surreal_db;
mod timex;

use std::sync::Arc;
use tower::ServiceBuilder;
use tower_http::cors::CorsLayer;
use tower_http::trace::TraceLayer;
use tracing_subscriber::EnvFilter;

use crate::auth::Jwt;
use crate::config::{Backend, Config};
use crate::repo::backend::{sqlite_repo_set, surreal_repo_set, RepoSet};
use crate::service::account::AccountService;
use crate::service::category::CategoryService;
use crate::service::investment::InvestmentService;
use crate::service::yahoo::YahooClient;
use crate::service::rule::RuleService;
use crate::service::transfer_rule::TransferRuleService;
use crate::service::auth::AuthService;
use crate::service::transaction::TransactionService;
use crate::service::transfer::TransferService;
use crate::service::user::UserService;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")))
        .init();

    let cfg = Config::from_env();
    std::fs::create_dir_all(cfg.data_dir.join("avatars"))?;

    // Build the repo set for the selected backend.
    let repo_set: RepoSet = match cfg.backend {
        Backend::Surreal => {
            surreal_repo_set(&cfg.surreal_url, &cfg.surreal_user, &cfg.surreal_pass, &cfg.surreal_ns, &cfg.surreal_db)
                .await
                .map_err(|e| anyhow::anyhow!(e.message))?
        }
        Backend::Sqlite => sqlite_repo_set(&cfg.sqlite_path).await.map_err(|e| anyhow::anyhow!(e.message))?,
    };

    let jwt = Jwt::new(cfg.jwt_secret.clone());
    let repo = repo_set.user.clone();
    let account_repo = repo_set.account.clone();
    let category_repo = repo_set.category.clone();
    let rule_repo = repo_set.rule.clone();
    let investment_repo = Arc::clone(&repo_set.investment);
    let transaction_repo = repo_set.transaction.clone();
    let transaction_repo_clone = transaction_repo.clone();
    let account_repo_clone = account_repo.clone();

    let auth_svc = AuthService::new(repo.clone(), jwt.clone());
    let user_svc = UserService::new(repo, jwt.clone(), cfg.data_dir.join("avatars"));
    let account_svc = AccountService::new(account_repo.clone());
    let category_svc = CategoryService::new(category_repo.clone());
    let investment_svc = InvestmentService::new(investment_repo, YahooClient::new()?);
    let rule_svc = RuleService::new(rule_repo.clone(), category_repo.clone(), transaction_repo.clone());
    let transfer_rule_svc = TransferRuleService::new(rule_repo, transaction_repo_clone.clone(), account_repo_clone.clone());
    let transaction_svc = TransactionService::new(transaction_repo.clone(), account_repo.clone(), category_repo.clone())
        .with_transfer_rule(transfer_rule_svc.clone());
    let transfer_svc = TransferService::new(transaction_repo, account_repo);

    let state = http::AppState {
        auth: auth_svc,
        user: user_svc,
        account: account_svc,
        investment: investment_svc,
        category: category_svc,
        rule: rule_svc,
        transfer_rule: transfer_rule_svc,
        transaction: transaction_svc,
        transfer: transfer_svc,
        jwt,
        static_dir: cfg.static_dir.to_string_lossy().into_owned(),
        avatar_dir: cfg.data_dir.join("avatars").to_string_lossy().into_owned(),
        domain_url: cfg.domain_url.clone(),
    };

    let app = http::router(state)
        .layer(ServiceBuilder::new().layer(TraceLayer::new_for_http()).layer(CorsLayer::permissive()));

    let listener = tokio::net::TcpListener::bind(&cfg.addr).await?;
    tracing::info!("Financer Rust server listening on {}", cfg.addr);
    axum::serve(listener, app).await?;
    Ok(())
}
