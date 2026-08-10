//! Categories HTTP handlers — mirroring Go's `httpserver/category_handlers.go`.

use axum::extract::{Query, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::Deserialize;

use crate::error::json_error;
use crate::http::transaction::bulk_wire;
use crate::http::{require_user, AppState, JsonResult};

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/categories", axum::routing::get(list).post(create).put(update).delete(delete))
}


#[derive(Deserialize)]
struct ReqCategory {
    #[serde(default)]
    ids: Vec<String>,
    #[serde(default)]
    categories: Vec<ReqCat>,
}

#[derive(Deserialize)]
struct ReqCat {
    #[serde(default)]
    id: String,
    #[serde(default)]
    name: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ListQuery {
    #[serde(default)]
    page_size: Option<i32>,
    #[serde(default)]
    page_token: Option<String>,
}

async fn list(State(st): State<AppState>, headers: HeaderMap, Query(q): Query<ListQuery>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.category.list(&uid, i64::from(q.page_size.unwrap_or(0)), q.page_token.as_deref().unwrap_or("")).await {
        Ok((categories, next_page_token)) => {
            Json(serde_json::json!({ "categories": categories, "nextPageToken": next_page_token })).into_response()
        }
        Err(e) => e.into_response(),
    }
}

async fn create(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqCategory>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let names: Vec<String> = req.categories.iter().map(|c| c.name.clone()).collect();
    match st.category.create(&uid, &names).await {
        Ok(b) => (StatusCode::CREATED, Json(bulk_wire(&b))).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn update(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqCategory>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let pairs: Vec<(String, String)> = req.categories.iter().map(|c| (c.id.clone(), c.name.clone())).collect();
    match st.category.update(&uid, &pairs).await {
        Ok(b) => Json(bulk_wire(&b)).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn delete(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqCategory>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.category.delete(&uid, &req.ids).await {
        Ok(_) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}
