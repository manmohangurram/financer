//! Accounts HTTP handlers — mirroring Go's `httpserver/account_handlers.go`.

use axum::extract::{Path, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};

use crate::http::{require_user, AppState};
use crate::service::account::type_value;

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/accounts", axum::routing::get(list).post(create))
        .route("/api/accounts/{id}", axum::routing::put(update).delete(delete))
}

#[derive(serde::Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReqAccount {
    // Accepted for wire parity (Go carries it, though handlers use the path id).
    #[serde(default)]
    #[allow(dead_code)]
    id: String,
    #[serde(default)]
    bank_name: String,
    #[serde(default)]
    account_nickname: String,
    #[serde(default)]
    account_type: serde_json::Value,
}

async fn create(State(st): State<AppState>, headers: HeaderMap, Json(req): Json<ReqAccount>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let t = match type_value(&req.account_type) {
        Ok(t) => t,
        Err(e) => return e.into_response(),
    };
    match st.account.create(&uid, &req.bank_name, &req.account_nickname, t).await {
        Ok(a) => (StatusCode::CREATED, Json(a)).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn update(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>, Json(req): Json<ReqAccount>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let t = match type_value(&req.account_type) {
        Ok(t) => t,
        Err(e) => return e.into_response(),
    };
    let _ = uid;
    match st.account.update(&id, &req.bank_name, &req.account_nickname, t).await {
        Ok(a) => (StatusCode::CREATED, Json(a)).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn delete(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let _ = uid;
    match st.account.delete(&id).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}

async fn list(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.account.list(&uid).await {
        Ok(items) => Json(serde_json::json!({ "accounts": items })).into_response(),
        Err(e) => e.into_response(),
    }
}

