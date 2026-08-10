//! Accounts HTTP handlers — mirroring Go's `httpserver/account_handlers.go`.

use axum::extract::{Path, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::Serialize;

use crate::error::ApiError;
use crate::http::{require_user, AppState};
use crate::service::account::AccountResponse;

const ACCOUNT_TYPE_NAMES: [(&str, i64); 5] = [
    ("ACCOUNT_TYPE_UNSPECIFIED", 0),
    ("ACCOUNT_TYPE_CHECKING", 1),
    ("ACCOUNT_TYPE_SAVINGS", 2),
    ("ACCOUNT_TYPE_CREDIT_CARD", 3),
    ("ACCOUNT_TYPE_LOAN", 4),
];

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

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct WireAccount {
    id: String,
    bank_name: String,
    account_nickname: String,
    account_type: String,
    balance: f64,
    created_at: String,
}

fn account_wire(a: &AccountResponse) -> WireAccount {
    WireAccount {
        id: a.id.clone(),
        bank_name: a.bank_name.clone(),
        account_nickname: a.account_nickname.clone(),
        account_type: account_type_name(a.account_type).to_string(),
        balance: round2(a.balance),
        created_at: a.created_at.clone(),
    }
}

fn round2(v: f64) -> f64 {
    (v * 100.0).round() / 100.0
}

fn account_type_name(v: i64) -> &'static str {
    ACCOUNT_TYPE_NAMES.iter().find(|(_, n)| *n == v).map_or("ACCOUNT_TYPE_UNSPECIFIED", |(s, _)| *s)
}

/// Parse `accountType`: accepts an enum name string or a numeric value; nil → 0.
fn account_type_value(v: &serde_json::Value) -> Result<i64, ApiError> {
    match v {
        serde_json::Value::Null => Ok(0),
        serde_json::Value::Number(n) => n.as_i64().ok_or_else(|| ApiError::bad_request("invalid account type")),
        serde_json::Value::String(s) => ACCOUNT_TYPE_NAMES
            .iter()
            .find(|(name, _)| *name == s)
            .map(|(_, n)| *n)
            .ok_or_else(|| ApiError::bad_request(format!("unknown account type {s:?}"))),
        _ => Err(ApiError::bad_request("invalid account type")),
    }
}

async fn create(State(st): State<AppState>, headers: HeaderMap, Json(req): Json<ReqAccount>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let t = match account_type_value(&req.account_type) {
        Ok(t) => t,
        Err(e) => return e.into_response(),
    };
    match st.account.create(&uid, &req.bank_name, &req.account_nickname, t).await {
        Ok(a) => (StatusCode::CREATED, Json(account_wire(&a))).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn update(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>, Json(req): Json<ReqAccount>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let t = match account_type_value(&req.account_type) {
        Ok(t) => t,
        Err(e) => return e.into_response(),
    };
    let _ = uid;
    match st.account.update(&id, &req.bank_name, &req.account_nickname, t).await {
        Ok(a) => (StatusCode::CREATED, Json(account_wire(&a))).into_response(),
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
        Ok(items) => {
            let accounts: Vec<WireAccount> = items.iter().map(account_wire).collect();
            Json(serde_json::json!({ "accounts": accounts })).into_response()
        }
        Err(e) => e.into_response(),
    }
}
