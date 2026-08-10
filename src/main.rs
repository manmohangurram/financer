//! Financer Rust backend — Phase 1: auth + strangler gateway.
//! Serves the frontend, owns auth routes, proxies the rest to the Go backend.

mod auth;
mod config;
mod db;
mod error;
mod http;
mod proxy;
mod repo;
mod service;

use tower::ServiceBuilder;
use tower_http::cors::CorsLayer;
use tower_http::trace::TraceLayer;
use tracing_subscriber::EnvFilter;

use crate::auth::Jwt;
use crate::config::Config;
use crate::repo::UserRepo;
use crate::service::auth::AuthService;
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
    let auth_svc = AuthService::new(repo.clone(), jwt.clone());
    let user_svc = UserService::new(repo);

    let state = http::AppState {
        auth: auth_svc,
        user: user_svc,
        jwt,
        go_backend_url: cfg.go_backend_url.clone(),
        static_dir: cfg.static_dir.to_string_lossy().into_owned(),
        domain_url: cfg.domain_url.clone(),
    };

    let app = http::router(state)
        .layer(ServiceBuilder::new().layer(TraceLayer::new_for_http()).layer(CorsLayer::permissive()));

    let listener = tokio::net::TcpListener::bind(&cfg.addr).await?;
    tracing::info!("Financer Rust server listening on {}", cfg.addr);
    axum::serve(listener, app).await?;
    Ok(())
}
