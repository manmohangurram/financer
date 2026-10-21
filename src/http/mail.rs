//! Mailbox HTTP handlers: link, list and remove IMAP mailboxes.

use axum::extract::{Path, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};

use crate::error::json_error;
use crate::http::{AppState, JsonResult, require_user};
use crate::service::mail::CreateMailAccountReq;

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/mail/accounts", axum::routing::get(list).post(create))
        .route("/api/mail/accounts/{id}", axum::routing::delete(delete))
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
