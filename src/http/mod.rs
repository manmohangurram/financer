//! HTTP handlers + router. Phase 1: Rust owns auth + me/profile; everything
//! else under `/api/*` is proxied to the Go backend; everything else is served
//! as the SPA.

pub mod account;
pub mod analytics;
pub mod category;
pub mod investment;
pub mod mcp;
pub mod rule;
pub mod settings;
pub mod transaction;
pub mod transfer;
pub mod user_key;
use axum::extract::State;
use axum::http::{header, HeaderMap, StatusCode, Uri};
use axum::response::{IntoResponse, Response};
use utoipa::OpenApi;
use axum::routing::get;
use axum::{Json, Router};

use crate::auth::Jwt;
use crate::error::{json_error, ApiError, Result};
use crate::http::account::routes as account_routes;
use crate::http::analytics::routes as analytics_routes;
use crate::http::category::routes as category_routes;
use crate::http::investment::routes as investment_routes;
use crate::http::rule::routes as rule_routes;
use crate::http::settings::routes as settings_routes;
use crate::http::transaction::routes as transaction_routes;
use crate::http::transfer::routes as transfer_routes;
use crate::http::user_key::routes as user_key_routes;
use crate::service::account::AccountService;
use crate::service::investment::InvestmentService;
use crate::service::category::CategoryService;
use crate::service::rule::RuleService;
use crate::service::transfer_rule::TransferRuleService;
use crate::service::auth::{AuthService, LoginRequest, RefreshTokenRequest, SignupRequest};
use crate::service::transaction::TransactionService;
use crate::service::transfer::TransferService;
use crate::service::user::UserService;
use crate::service::user_key::UserKeyService;

#[derive(Clone)]
pub struct AppState {
    pub auth: AuthService,
    pub user: UserService,
    pub user_key: UserKeyService,
    pub account: AccountService,
    pub investment: InvestmentService,
    pub category: CategoryService,
    pub rule: RuleService,
    pub transfer_rule: TransferRuleService,
    pub transaction: TransactionService,
    pub transfer: TransferService,
    pub jwt: Jwt,
    pub static_dir: String,
    pub avatar_dir: String,
    pub domain_url: String,
}

pub fn router(state: AppState) -> Router {
    let mcp = crate::http::mcp::mcp_router(&state).with_state(());
    Router::new()
        .route("/api/auth/signup", axum::routing::post(signup))
        .route("/api/auth/login", axum::routing::post(login))
        .route("/api/auth/refresh", axum::routing::post(refresh))
        .merge(utoipa_swagger_ui::SwaggerUi::new("/docs").url("/openapi.json", crate::openapi::ApiDoc::openapi()))
        .merge(account_routes())
        .merge(analytics_routes())
        .merge(category_routes())
        .merge(investment_routes())
        .merge(rule_routes())
        .merge(transaction_routes())
        .merge(transfer_routes())
        .merge(settings_routes())
        .merge(user_key_routes())
        .merge(mcp)
        .route("/avatars/{name}", get(avatar_file))
        .fallback(spa)
        .with_state(state)
}

/// Serve an uploaded avatar from the avatar dir.
async fn avatar_file(State(s): State<AppState>, uri: Uri) -> Response {
    let name = uri.path().rsplit('/').next().unwrap_or("");
    if name.is_empty() || name.contains("..") {
        return StatusCode::NOT_FOUND.into_response();
    }
    let candidate = std::path::Path::new(&s.avatar_dir).join(name);
    match std::fs::read(&candidate) {
        Ok(content) => {
            let mime = match name.rsplit('.').next().unwrap_or("") {
                "png" => "image/png",
                "jpg" | "jpeg" => "image/jpeg",
                "webp" => "image/webp",
                _ => "application/octet-stream",
            };
            (StatusCode::OK, [(header::CONTENT_TYPE, mime)], axum::body::Body::from(content)).into_response()
        }
        Err(_) => StatusCode::NOT_FOUND.into_response(),
    }
}

/// SPA fallback: serve a real file if it exists, else index.html (with
/// `FINANCER_DOMAIN_URL` injection), mirroring Go's `spaHandler`.
async fn spa(State(s): State<AppState>, uri: Uri) -> Response {
    let path = uri.path();
    // Unknown /api/* routes are API 404s, not SPA assets (mirror Go's apiHandler).
    if path.starts_with("/api/") {
        return ApiError::not_found(format!("unknown endpoint {path}")).into_response();
    }
    let index = read_index(&s);
    if let Some(p) = path.strip_prefix('/') {
        if !p.is_empty() && !p.starts_with('/') && !p.split('/').any(|seg| seg == "..") {
            let candidate = std::path::Path::new(&s.static_dir).join(p);
            if candidate.is_file() {
                if let Ok(content) = std::fs::read(&candidate) {
                    return (
                        StatusCode::OK,
                        [(header::CONTENT_TYPE, mime_for(p))],
                        axum::body::Body::from(content),
                    )
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

pub type JsonResult<T> = std::result::Result<Json<T>, axum::extract::rejection::JsonRejection>;

#[utoipa::path(
    post,
    path = "/api/auth/signup",
    request_body = SignupRequest,
    responses(
        (status = 201, description = "Account created", body = crate::service::auth::AuthResponse),
        (status = 400, description = "Invalid input"),
        (status = 409, description = "Email already registered"),
    ),
)]
pub async fn signup(State(st): State<AppState>, req: JsonResult<SignupRequest>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    match st.auth.signup(req).await {
        Ok(r) => (StatusCode::CREATED, Json(r)).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    post,
    path = "/api/auth/login",
    request_body = LoginRequest,
    responses(
        (status = 200, description = "Logged in", body = crate::service::auth::AuthResponse),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Invalid credentials"),
    ),
)]
pub async fn login(State(st): State<AppState>, req: JsonResult<LoginRequest>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    match st.auth.login(req).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    post,
    path = "/api/auth/refresh",
    request_body = RefreshTokenRequest,
    responses(
        (status = 200, description = "Tokens refreshed", body = crate::service::auth::AuthResponse),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Invalid or revoked refresh token"),
    ),
)]
pub async fn refresh(State(st): State<AppState>, req: JsonResult<RefreshTokenRequest>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    match st.auth.refresh_token(req).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}
