//! User settings HTTP handlers — mirroring Go's `httpserver/user_handlers.go`.

use axum::extract::{Multipart, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::Deserialize;

use crate::error::json_error;
use crate::http::transaction::bulk_wire;
use crate::http::{require_user, AppState, JsonResult};

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/me/profile", axum::routing::get(get).put(update))
        .route("/api/me/password", axum::routing::post(password))
        .route("/api/me/logout-all", axum::routing::post(logout_all))
        .route("/api/me/avatar", axum::routing::post(avatar))
}


#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReqProfile {
    #[serde(default)]
    name: String,
    #[serde(default)]
    email: String,
    #[serde(default)]
    avatar_url: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReqPassword {
    #[serde(default)]
    current_password: String,
    #[serde(default)]
    new_password: String,
}

async fn get(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.user.profile(&uid).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn update(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqProfile>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.user.update_profile(&uid, &req.name, &req.email, &req.avatar_url).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn password(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqPassword>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.user.change_password(&uid, &req.current_password, &req.new_password).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn logout_all(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.user.logout_all(&uid).await {
        Ok(()) => Json(bulk_wire(&crate::service::transaction::BulkResult {
            success: true,
            message: "all sessions logged out".to_string(),
            failed_ids: Vec::new(),
            skipped: 0,
        }))
        .into_response(),
        Err(e) => e.into_response(),
    }
}

async fn avatar(State(st): State<AppState>, headers: HeaderMap, mut mp: Multipart) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let mut data: Vec<u8> = Vec::new();
    while let Ok(Some(field)) = mp.next_field().await {
        if field.name() == Some("file") {
            match field.bytes().await {
                Ok(b) => data = b.to_vec(),
                Err(_) => return crate::error::ApiError::bad_request("failed to read avatar").into_response(),
            }
            break;
        }
    }
    if data.is_empty() {
        return crate::error::ApiError::bad_request("missing 'file' field").into_response();
    }
    match st.user.save_avatar(&uid, &data).await {
        Ok(r) => (StatusCode::CREATED, Json(r)).into_response(),
        Err(e) => e.into_response(),
    }
}
