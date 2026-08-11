//! Accounts HTTP handlers — mirroring Go's `httpserver/account_handlers.go`.

use axum::extract::{Path, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use utoipa::ToSchema;

use crate::error::json_error;
use crate::http::{require_user, AppState, JsonResult};
use crate::repo::account::AccountType;

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/accounts", axum::routing::get(list).post(create))
        .route("/api/accounts/{id}", axum::routing::put(update).delete(delete))
}

#[derive(serde::Deserialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct ReqAccount {
    // Accepted for wire parity (Go carries it, though handlers use the path id).
    #[serde(default)]
    #[allow(dead_code)]
    id: String,
    #[serde(default)]
    bank_name: String,
    #[serde(default)]
    nickname: String,
    /// Strict: missing/unknown value → serde rejection → 400.
    #[serde(rename = "type")]
    account_type: AccountType,
}


#[utoipa::path(
    post,
    path = "/api/accounts",
    request_body = ReqAccount,
    responses(
        (status = 201, description = "Account created", body = crate::service::account::AccountResponse),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn create(
    State(st): State<AppState>,
    headers: HeaderMap,
    req: JsonResult<ReqAccount>,
) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.account.create(&uid, &req.bank_name, &req.nickname, req.account_type).await {
        Ok(a) => (StatusCode::CREATED, Json(a)).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    put,
    path = "/api/accounts/{id}",
    params(("id", description = "Account id")),
    request_body = ReqAccount,
    responses(
        (status = 200, description = "Account updated", body = crate::service::account::AccountResponse),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
        (status = 404, description = "Account not found"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn update(
    State(st): State<AppState>,
    headers: HeaderMap,
    Path(id): Path<String>,
    req: JsonResult<ReqAccount>,
) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.account.update(&uid, &id, &req.bank_name, &req.nickname, req.account_type).await {
        Ok(a) => Json(a).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    delete,
    path = "/api/accounts/{id}",
    params(("id", description = "Account id")),
    responses(
        (status = 204, description = "Account deleted"),
        (status = 401, description = "Unauthenticated"),
        (status = 404, description = "Account not found"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn delete(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.account.delete(&uid, &id).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    get,
    path = "/api/accounts",
    responses(
        (status = 200, description = "Accounts list"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn list(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.account.list(&uid).await {
        Ok(items) => Json(serde_json::json!({ "accounts": items })).into_response(),
        Err(e) => e.into_response(),
    }
}

