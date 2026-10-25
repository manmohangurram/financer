//! Mailbox HTTP handlers: link, list and remove IMAP mailboxes.

use axum::extract::{Path, Query, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};

use crate::error::json_error;
use crate::http::{AppState, JsonResult, require_user};
use crate::service::mail::{CreateMailAccountReq, MailActionReq};

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/mail/accounts", axum::routing::get(list).post(create))
        .route("/api/mail/accounts/{id}", axum::routing::delete(delete))
        .route("/api/mail/messages", axum::routing::get(messages).post(act))
        .route_layer(axum::middleware::from_fn(crate::http::https::require_https))
}

#[utoipa::path(
    post,
    path = "/api/mail/accounts",
    request_body = CreateMailAccountReq,
    responses(
        (status = 201, description = "Mailbox linked", body = crate::service::mail::MailAccount),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
        (status = 503, description = "FINANCER_SECRET_KEY is not set"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn create(
    State(st): State<AppState>,
    headers: HeaderMap,
    req: JsonResult<CreateMailAccountReq>,
) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.mail.create(&uid, req).await {
        Ok(a) => (StatusCode::CREATED, Json(a)).into_response(),
        Err(e) => e.into_response(),
    }
}

#[derive(serde::Deserialize)]
pub struct MessagesQuery {
    #[serde(default)]
    status: Option<String>,
    #[serde(default)]
    limit: Option<i64>,
}

#[utoipa::path(
    get,
    path = "/api/mail/messages",
    params(("status" = Option<String>, Query, description = "pending|unparsed|ignored|auto|rejected|all")),
    responses(
        (status = 200, description = "Stored messages"),
        (status = 400, description = "Unknown status"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn messages(
    State(st): State<AppState>,
    headers: HeaderMap,
    Query(q): Query<MessagesQuery>,
) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st
        .mail
        .messages(&uid, q.status.as_deref(), q.limit.unwrap_or(100))
        .await
    {
        Ok(items) => Json(serde_json::json!({ "messages": items })).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    post,
    path = "/api/mail/messages",
    request_body = MailActionReq,
    responses(
        (status = 200, description = "Per-row outcome", body = crate::service::mail::MailActionOutcome),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn act(
    State(st): State<AppState>,
    headers: HeaderMap,
    req: JsonResult<MailActionReq>,
) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st
        .mail
        .act(&uid, req.messages, &st.transaction, &st.account)
        .await
    {
        Ok(outcome) => Json(outcome).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    get,
    path = "/api/mail/accounts",
    responses(
        (status = 200, description = "Linked mailboxes"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn list(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.mail.list(&uid).await {
        Ok(items) => Json(serde_json::json!({ "accounts": items })).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    delete,
    path = "/api/mail/accounts/{id}",
    params(("id", description = "Mailbox id")),
    responses(
        (status = 204, description = "Mailbox removed"),
        (status = 401, description = "Unauthenticated"),
        (status = 404, description = "Not found"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn delete(
    State(st): State<AppState>,
    headers: HeaderMap,
    Path(id): Path<String>,
) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.mail.delete(&uid, &id).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}
