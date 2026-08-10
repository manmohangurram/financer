//! Transfers HTTP handlers — mirroring Go's `httpserver/transfer_handlers.go`.

use axum::extract::State;
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::Deserialize;

use crate::http::transaction::bulk_wire;
use crate::http::{require_user, AppState};

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

async fn link(State(st): State<AppState>, headers: HeaderMap, Json(req): Json<ReqTransfer>) -> Response {
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

async fn counterpart(State(st): State<AppState>, headers: HeaderMap, Json(req): Json<ReqCreateCounterpart>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.transfer.create_counterpart(&uid, &req.transaction_id, &req.to_account_id).await {
        Ok(r) => (
            StatusCode::CREATED,
            // Go's api.CreateTransferResponse has no json tags → PascalCase wire.
            Json(serde_json::json!({
                "DebitTransactionId": r.debit_transaction_id,
                "CreditTransactionId": r.credit_transaction_id,
            })),
        )
            .into_response(),
        Err(e) => e.into_response(),
    }
}

async fn unlink(State(st): State<AppState>, headers: HeaderMap, Json(req): Json<ReqTransfer>) -> Response {
    let _uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.transfer.unlink_transfers(&req.ids).await {
        Ok(_) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}
