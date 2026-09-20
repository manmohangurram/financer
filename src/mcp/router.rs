//! Mounts the MCP server at `/mcp`, authenticated by a per-user API key.

use std::sync::Arc;

use axum::Router;
use axum::http::{HeaderMap, Request, header};
use axum::middleware::{self, Next};
use axum::response::{IntoResponse, Response};
use rmcp::transport::streamable_http_server::session::local::LocalSessionManager;
use rmcp::transport::{StreamableHttpServerConfig, StreamableHttpService};

use crate::http::AppState;
use crate::mcp::handler::{KeyScope, McpHandler, UserId};
use crate::service::user_key::UserKeyService;

/// Build and mount the MCP server at `/mcp`, authenticated by API key.
pub fn mcp_router(st: &AppState) -> Router<()> {
    let handler = McpHandler::new(
        Arc::new(st.account.clone()),
        Arc::new(st.transaction.clone()),
        Arc::new(st.category.clone()),
        Arc::new(st.investment.clone()),
        Arc::new(st.rule.clone()),
        Arc::new(st.transfer.clone()),
        Arc::new(st.transfer_rule.clone()),
    );
    let service = StreamableHttpService::new(
        move || Ok(handler.clone()),
        Arc::new(LocalSessionManager::default()),
        StreamableHttpServerConfig::default()
            .with_json_response(true)
            .disable_allowed_hosts(),
    );
    let user_key_service = st.user_key.clone();

    Router::new()
        .nest_service("/mcp", service)
        .route_layer(middleware::from_fn(move |req, next| {
            auth_middleware(req, next, user_key_service.clone())
        }))
}

/// axum middleware: validate `Authorization: Bearer <api_key>`, inject `UserId`
/// and `KeyScope` into the request extensions, then continue.
async fn auth_middleware(
    mut req: Request<axum::body::Body>,
    next: Next,
    user_key_service: UserKeyService,
) -> Response {
    let key = extract_bearer(req.headers());
    match key {
        Some(k) => match user_key_service.authenticate(&k).await {
            Ok((uid, scope)) => {
                req.extensions_mut().insert(UserId(uid));
                req.extensions_mut().insert(KeyScope(scope));
            }
            Err(_) => return unauthorized(),
        },
        None => return unauthorized(),
    }
    next.run(req).await
}

fn unauthorized() -> Response {
    (
        axum::http::StatusCode::UNAUTHORIZED,
        axum::Json(serde_json::json!({ "error": "invalid or missing API key" })),
    )
        .into_response()
}
fn extract_bearer(headers: &HeaderMap) -> Option<String> {
    headers
        .get(header::AUTHORIZATION)
        .and_then(|v| v.to_str().ok())
        .and_then(|s| s.strip_prefix("Bearer "))
        .map(str::to_string)
}
