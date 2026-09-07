//! Per-user API-key HTTP handlers (settings-style, JWT-authed).

use axum::extract::{Path, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::Deserialize;
use utoipa::ToSchema;

use crate::error::json_error;
use crate::http::{require_user, AppState, JsonResult};

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/me/keys", axum::routing::get(list).post(create))
        .route("/api/me/keys/{id}", axum::routing::delete(delete))
}

#[derive(Deserialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct ReqCreateKey {
    #[serde(default)]
    name: String,
    #[serde(default)]
    #[serde(rename = "expiresInDays")]
    expires_in_days: i64,
    #[serde(default)]
    scope: String,
}

#[utoipa::path(
    get,
    path = "/api/me/keys",
    responses(
        (status = 200, description = "Listed API keys", body = [crate::repo::traits::UserKeyRow]),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn list(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.user_key.list(&uid).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    post,
    path = "/api/me/keys",
    request_body = ReqCreateKey,
    responses(
        (status = 201, description = "Key created (plaintext shown once)", body = crate::service::user_key::UserKeyView),
        (status = 400, description = "Invalid input, or max 50 keys reached"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn create(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqCreateKey>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.user_key.create(&uid, &req.name, req.expires_in_days, &req.scope).await {
        Ok(r) => (StatusCode::CREATED, Json(r)).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    delete,
    path = "/api/me/keys/{id}",
    responses(
        (status = 204, description = "Key revoked"),
        (status = 404, description = "Key not found"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn delete(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.user_key.delete(&uid, &id).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}
