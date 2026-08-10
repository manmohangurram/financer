//! Transfers HTTP handlers — mirroring Go's `httpserver/transfer_handlers.go`.

use axum::extract::State;
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::Deserialize;

use crate::error::json_error;
use crate::http::transaction::bulk_wire;
use crate::http::{require_user, AppState, JsonResult};


#[derive(Deserialize)]
struct ReqTransfer {
    #[serde(default)]
    ids: Vec<String>,
    #[serde(default)]
    links: Vec<ReqLink>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReqLink {
    #[serde(default)]
    debit_transaction_id: String,
    #[serde(default)]
    credit_transaction_id: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReqCreateCounterpart {
    #[serde(default)]
    transaction_id: String,
    #[serde(default)]
    to_account_id: String,
}

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/transfer-links", axum::routing::post(link).delete(unlink))
        .route("/api/transfer-links/counterpart", axum::routing::post(counterpart))
}

async fn link(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqTransfer>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let links: Vec<(String, String)> = req
        .links
        .iter()
        .map(|l| (l.debit_transaction_id.clone(), l.credit_transaction_id.clone()))
        .collect();
    match st.transfer.link_transfers(&uid, &links).await {
        Ok(b) => (StatusCode::CREATED, Json(bulk_wire(&b))).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn counterpart(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqCreateCounterpart>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.transfer.create_counterpart(&uid, &req.transaction_id, &req.to_account_id).await {
        Ok(r) => (
            StatusCode::CREATED,
            Json(serde_json::json!({
                "debitTransactionId": r.debit_transaction_id,
                "creditTransactionId": r.credit_transaction_id,
            })),
        )
            .into_response(),
        Err(e) => e.into_response(),
    }
}

async fn unlink(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqTransfer>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.transfer.unlink_transfers(&uid, &req.ids).await {
        Ok(_) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}
