//! Financer Rust backend — Rust-only server on `SurrealDB` OR `SQLite` (picked
//! at runtime from env), axum API, serves the built Vue frontend.

mod auth;
mod config;
mod error;
mod http;
mod mcp;
mod openapi;
mod pdf;
mod repo;
mod service;
#[cfg(feature = "surreal")]
mod surreal_db;
mod utils;
#[cfg(test)]
mod tests;

use std::sync::Arc;
use tower::ServiceBuilder;
use tower_http::cors::CorsLayer;
use tower_http::trace::TraceLayer;
use tracing_subscriber::EnvFilter;

use crate::auth::Jwt;
use crate::config::Config;
use crate::repo::db::{sqlite_repo_set, RepoSet};
#[cfg(feature = "surreal")]
use crate::repo::db::surreal_repo_set;
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
use crate::service::user_key::UserKeyService;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")))
        .init();

    let cfg = Config::from_env()?;
    // Set the process timezone once, before any date/time work.
    crate::utils::timex::init_tz(&cfg.server.timezone);
    std::fs::create_dir_all(cfg.server.data_dir.join("avatars"))?;

    // Build the repo set for the selected database.
    let repo_set: RepoSet = match cfg.database() {
        crate::config::Database::Sqlite => sqlite_repo_set(&cfg.storage.sqlite).await.map_err(|e| anyhow::anyhow!(e.message))?,
        #[cfg(feature = "surreal")]
        crate::config::Database::Surreal => {
            surreal_repo_set(&cfg.storage.surreal.url, &cfg.storage.surreal.user, &cfg.storage.surreal.pass, &cfg.storage.surreal.ns, &cfg.storage.surreal.db)
                .await
                .map_err(|e| anyhow::anyhow!(e.message))?
        }
        #[cfg(not(feature = "surreal"))]
        crate::config::Database::Surreal => {
            anyhow::bail!("FINANCER_DATABASE=surreal but this build has no SurrealDB support; rebuild with `--features surreal`")
        }
    };

    let jwt = Jwt::new(cfg.server.jwt_secret.clone());
    let repo = repo_set.user.clone();
    let account_repo = repo_set.account.clone();
    let category_repo = repo_set.category.clone();
    let rule_repo = repo_set.rule.clone();
    let investment_repo = Arc::clone(&repo_set.investment);
    let transaction_repo = repo_set.transaction.clone();
    let transaction_repo_clone = transaction_repo.clone();
    let account_repo_clone = account_repo.clone();

    let auth_svc = AuthService::new(repo.clone(), jwt.clone());
    let user_svc = UserService::new(repo, jwt.clone(), cfg.server.data_dir.join("avatars"));
    let user_key_svc = UserKeyService::new(repo_set.user_key.clone());
    let account_svc = AccountService::new(account_repo.clone());
    let category_svc = CategoryService::new(category_repo.clone());
    let investment_svc = InvestmentService::new(investment_repo, YahooClient::new(&cfg.yahoo)?);
    let rule_svc = RuleService::new(rule_repo.clone(), category_repo.clone(), transaction_repo.clone());
    let transfer_rule_svc = TransferRuleService::new(rule_repo, transaction_repo_clone.clone(), account_repo_clone.clone());
    let transaction_svc = TransactionService::new(transaction_repo.clone(), account_repo.clone(), category_repo.clone())
        .with_transfer_rule(transfer_rule_svc.clone())
        .with_rule(Arc::new(rule_svc.clone()));
    let transfer_svc = TransferService::new(transaction_repo, account_repo);

    let state = http::AppState {
        auth: auth_svc,
        user: user_svc,
        user_key: user_key_svc,
        account: account_svc,
        investment: investment_svc,
        category: category_svc,
        rule: rule_svc,
        transfer_rule: transfer_rule_svc,
        transaction: transaction_svc,
        transfer: transfer_svc,
        jwt,
        static_dir: cfg.server.static_dir.to_string_lossy().into_owned(),
        avatar_dir: cfg.server.data_dir.join("avatars").to_string_lossy().into_owned(),
        domain_url: cfg.server.domain_url.clone(),
    };

    let app = http::router(state)
        .layer(ServiceBuilder::new().layer(TraceLayer::new_for_http()).layer(CorsLayer::permissive()));

    let listener = tokio::net::TcpListener::bind(&cfg.server.addr).await?;
    tracing::info!(
        "Financer Rust server listening on {} (local time {})",
        cfg.server.addr,
        crate::utils::timex::now_utc()
            .with_timezone(&crate::utils::timex::tz())
            .format("%Y-%m-%d %H:%M:%S %Z")
    );
    axum::serve(listener, app).await?;
    Ok(())
}
