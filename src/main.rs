//! Financer Rust backend — Phase 1: auth + strangler gateway.
//! Serves the frontend, owns auth routes, proxies the rest to the Go backend.

mod auth;
mod config;
mod db;
mod error;
mod http;
mod repo;
mod service;
mod timex;

use tower::ServiceBuilder;
use tower_http::cors::CorsLayer;
use tower_http::trace::TraceLayer;
use tracing_subscriber::EnvFilter;

use crate::auth::Jwt;
use crate::config::Config;
use crate::repo::account::AccountRepo;
use crate::repo::category::CategoryRepo;
use crate::repo::investment::InvestmentRepo;
use crate::repo::rule::RuleRepo;
use crate::repo::transaction::TransactionRepo;
use crate::repo::UserRepo;
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

    std::fs::create_dir_all(cfg.data_dir.join("db"))?;
    let db = db::open_pools(&cfg.db_path).await?;
    db::run_migrations(&db.write, std::path::Path::new("db/migrations")).await?;
    tracing::info!("database migrations applied successfully");

    let jwt = Jwt::new(cfg.jwt_secret.clone());
    let repo = UserRepo::new(db.write.clone());
    let account_repo = AccountRepo::new(db.write.clone());
    let category_repo = CategoryRepo::new(db.write.clone());
    let rule_repo = RuleRepo::new(db.write.clone());
    let investment_repo = InvestmentRepo::new(db.write.clone());
    let transaction_repo = TransactionRepo::new(db.write.clone());
    let transaction_repo_clone = transaction_repo.clone();
    let account_repo_clone = account_repo.clone();
    let auth_svc = AuthService::new(repo.clone(), jwt.clone());
    let user_svc = UserService::new(repo, jwt.clone(), cfg.data_dir.join("avatars"));
    let account_svc = AccountService::new(account_repo.clone());
    let category_svc = CategoryService::new(category_repo.clone());
    let investment_svc = InvestmentService::new(investment_repo, YahooClient::new()?);
    let rule_svc = RuleService::new(rule_repo.clone(), category_repo.clone(), transaction_repo.clone());
    let transfer_rule_svc = TransferRuleService::new(rule_repo, transaction_repo_clone, account_repo_clone);
    let transaction_svc = TransactionService::new(transaction_repo.clone(), account_repo.clone())
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
