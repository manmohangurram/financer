//! HTTP handlers + router. Phase 1: Rust owns auth + me/profile; everything
//! else under `/api/*` is proxied to the Go backend; everything else is served
//! as the SPA.

mod account;
mod category;
mod rule;
mod transaction;
mod transfer;

use axum::body::Body;
use axum::extract::State;
use axum::http::{header, HeaderMap, Request, StatusCode, Uri};
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Json, Router};

use crate::auth::Jwt;
use crate::error::{ApiError, Result};
use crate::http::account::routes as account_routes;
use crate::http::category::routes as category_routes;
use crate::http::rule::routes as rule_routes;
use crate::http::transaction::routes as transaction_routes;
use crate::http::transfer::routes as transfer_routes;
use crate::proxy;
use crate::service::account::AccountService;
use crate::service::category::CategoryService;
use crate::service::rule::RuleService;
use crate::service::transfer_rule::TransferRuleService;
use crate::service::auth::{AuthService, LoginRequest, RefreshTokenRequest, SignupRequest};
use crate::service::transaction::TransactionService;
use crate::service::transfer::TransferService;
use crate::service::user::UserService;

#[derive(Clone)]
pub struct AppState {
    pub auth: AuthService,
    pub user: UserService,
    pub account: AccountService,
    pub category: CategoryService,
    pub rule: RuleService,
    pub transfer_rule: TransferRuleService,
    pub transaction: TransactionService,
    pub transfer: TransferService,
    pub jwt: Jwt,
    pub go_backend_url: String,
    pub static_dir: String,
    pub domain_url: String,
}

pub fn router(state: AppState) -> Router {
    Router::new()
        .route("/api/auth/signup", post(signup))
        .route("/api/auth/login", post(login))
        .route("/api/auth/refresh", post(refresh))
        .route("/api/me/profile", get(me))
        .merge(account_routes())
        .merge(category_routes())
        .merge(rule_routes())
        .merge(transaction_routes())
        .merge(transfer_routes())
        .route("/api/{*rest}", axum::routing::any(proxy_route))
        .fallback(spa)
        .with_state(state)
}

async fn proxy_route(
    State(s): State<AppState>,
    req: Request<Body>,
) -> Response<Body> {
    proxy::proxy(req, &s.go_backend_url).await
}

/// SPA fallback: serve a real file if it exists, else index.html (with
/// `FINANCER_DOMAIN_URL` injection), mirroring Go's `spaHandler`.
async fn spa(State(s): State<AppState>, uri: Uri) -> Response {
    let index = read_index(&s);
    if let Some(p) = uri.path().strip_prefix('/') {
        if !p.is_empty() {
            let candidate = std::path::Path::new(&s.static_dir).join(p);
            if candidate.is_file() {
                if let Ok(content) = std::fs::read(&candidate) {
                    return (
                        StatusCode::OK,
                        [(header::CONTENT_TYPE, mime_for(p))],
                        axum::body::Body::from(content),                    )
                        .into_response();
                }
            }
        }
    }
    (
        StatusCode::OK,
        [(header::CONTENT_TYPE, "text/html; charset=utf-8")],
        index,
    )
        .into_response()
}

fn read_index(s: &AppState) -> String {
    let index = std::fs::read_to_string(format!("{}/index.html", s.static_dir))
        .unwrap_or_else(|_| "<html><body>Financer</body></html>".to_string());
    if s.domain_url.is_empty() {
        return index;
    }
    let script = format!("<script>window.__API_BASE__={:?};</script>", s.domain_url);
    index.replacen("</head>", &format!("{script}</head>"), 1)
}

fn mime_for(path: &str) -> &'static str {
    match path.rsplit('.').next().unwrap_or("") {
        "js" => "text/javascript",
        "css" => "text/css",
        "png" => "image/png",
        "svg" => "image/svg+xml",
        "woff2" => "font/woff2",
        "ico" => "image/x-icon",
        "map" => "application/json",
        _ => "application/octet-stream",
    }
}

/// Extract and validate the bearer token, returning the user id.
pub fn require_user(headers: &HeaderMap, jwt: &Jwt) -> Result<String> {
    let hdr = headers
        .get("authorization")
        .and_then(|v| v.to_str().ok())
        .ok_or_else(|| ApiError::unauthorized("missing or invalid Authorization header"))?;
    let token = hdr
        .strip_prefix("Bearer ")
        .or_else(|| hdr.strip_prefix("bearer "))
        .ok_or_else(|| ApiError::unauthorized("missing or invalid Authorization header"))?;
    let claims = jwt.validate(token)?;
    Ok(claims.user_id)
}

async fn signup(State(st): State<AppState>, Json(req): Json<SignupRequest>) -> Response {
    match st.auth.signup(req).await {
        Ok(r) => (StatusCode::CREATED, Json(r)).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn login(State(st): State<AppState>, Json(req): Json<LoginRequest>) -> Response {
    match st.auth.login(req).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn refresh(State(st): State<AppState>, Json(req): Json<RefreshTokenRequest>) -> Response {
    match st.auth.refresh_token(req).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn me(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.user.profile(&uid).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}
