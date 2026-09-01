//! MCP server (read-only) — exposes financer data to external AI agents over
//! the Model Context Protocol. Mounted as a streamable-HTTP server at `/mcp`.
//! Auth uses the per-user API key from the `Authorization` header; the
//! authenticated `user_id` is injected into the request extensions and read by
//! each tool method (per-request, no shared mutable state).


use std::sync::Arc;

use axum::http::{header, HeaderMap, Request};
use axum::middleware::{self, Next};
use axum::response::{IntoResponse, Response};
use axum::Router;
use rmcp::handler::server::wrapper::Parameters;
use rmcp::service::RequestContext;
use rmcp::transport::streamable_http_server::session::local::LocalSessionManager;
use rmcp::transport::{StreamableHttpServerConfig, StreamableHttpService};
use rmcp::{RoleServer, ServerHandler, tool_handler, tool_router, tool};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

use crate::repo::traits::transaction::TransactionListFilter;
use crate::service::account::AccountService;
use crate::service::category::CategoryService;
use crate::service::investment::InvestmentService;
use crate::service::transaction::TransactionService;
use crate::service::user_key::UserKeyService;

/// The authenticated user id, injected into request extensions by the auth
/// middleware and read by each tool method.
#[derive(Clone)]
pub struct UserId(pub String);

/// Read-only MCP handler. Holds the services + auth service.
#[derive(Clone)]
pub struct FinancerHandler {
    pub account: Arc<AccountService>,
    pub transaction: Arc<TransactionService>,
    pub category: Arc<CategoryService>,
    pub investment: Arc<InvestmentService>,
}

impl FinancerHandler {
    pub fn new(
        account: Arc<AccountService>,
        transaction: Arc<TransactionService>,
        category: Arc<CategoryService>,
        investment: Arc<InvestmentService>,
    ) -> Self {
        Self {
            account,
            transaction,
            category,
            investment,
        }
    }

    fn user_id(ctx: &RequestContext<RoleServer>) -> crate::error::Result<String> {
        let parts = ctx
            .extensions
            .get::<http::request::Parts>()
            .ok_or_else(|| crate::error::ApiError::unauthorized("missing request context"))?;
        let uid = parts
            .extensions
            .get::<UserId>()
            .map(|u| u.0.clone())
            .ok_or_else(|| crate::error::ApiError::unauthorized("no authenticated user"))?;
        Ok(uid)
    }
}

#[derive(Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "camelCase")]
pub struct ListTransactionsReq {
    #[serde(default)]
    pub category_ids: Vec<String>,
    pub ty: Option<String>,
}

#[tool_router]
impl FinancerHandler {
    #[tool(description = "List the user's accounts with balances.")]
    async fn list_accounts(&self, ctx: RequestContext<RoleServer>) -> String {
        let uid = match Self::user_id(&ctx) {
            Ok(u) => u,
            Err(e) => return err_json(e.message),
        };
        let rows = match self.account.list(&uid).await {
            Ok(r) => r,
            Err(e) => return err_json(e.message),
        };
        serde_json::to_string(&rows).unwrap_or_else(|e| err_json(format!("serialize error: {e}")))
    }

    #[tool(description = "List the user's transactions (optionally filtered).")]
    async fn list_transactions(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<ListTransactionsReq>,
    ) -> String {
        let uid = match Self::user_id(&ctx) {
            Ok(u) => u,
            Err(e) => return err_json(e.message),
        };
        let f = TransactionListFilter {
            category_ids: req.category_ids,
            transaction_type: req.ty.as_deref().and_then(parse_type),
            ..Default::default()
        };
        let r = match self.transaction.list(&uid, f).await {
            Ok(r) => r,
            Err(e) => return err_json(e.message),
        };
        serde_json::to_string(&r.rows).unwrap_or_else(|e| err_json(format!("serialize error: {e}")))
    }

    #[tool(description = "Total balance and credit/debit totals across accounts.")]
    async fn get_balance_summary(&self, ctx: RequestContext<RoleServer>) -> String {
        let uid = match Self::user_id(&ctx) {
            Ok(u) => u,
            Err(e) => return err_json(e.message),
        };
        let d = match self.transaction.dashboard(&uid).await {
            Ok(d) => d,
            Err(e) => return err_json(e.message),
        };
        serde_json::to_string(&d).unwrap_or_else(|e| err_json(format!("serialize error: {e}")))
    }

    #[tool(description = "Investments with lots and FIFO positions.")]
    async fn get_portfolio(&self, ctx: RequestContext<RoleServer>) -> String {
        let uid = match Self::user_id(&ctx) {
            Ok(u) => u,
            Err(e) => return err_json(e.message),
        };
        let p = match self.investment.portfolio_summary(&uid).await {
            Ok(p) => p,
            Err(e) => return err_json(e.message),
        };
        serde_json::to_string(&p).unwrap_or_else(|e| err_json(format!("serialize error: {e}")))
    }

    #[tool(description = "List the user's categories.")]
    async fn list_categories(&self, ctx: RequestContext<RoleServer>) -> String {
        let uid = match Self::user_id(&ctx) {
            Ok(u) => u,
            Err(e) => return err_json(e.message),
        };
        let rows = match self.category.list(&uid, 100, "").await {
            Ok((rows, _)) => rows,
            Err(e) => return err_json(e.message),
        };
        serde_json::to_string(&rows).unwrap_or_else(|e| err_json(format!("serialize error: {e}")))
    }
}

#[allow(clippy::unused_async_trait_impl)]
#[tool_handler]
impl ServerHandler for FinancerHandler {}

/// Build and mount the MCP server at `/mcp`, authenticated by API key.
pub fn mcp_router(st: &crate::http::AppState) -> Router<()> {
    let handler = FinancerHandler::new(
        Arc::new(st.account.clone()),
        Arc::new(st.transaction.clone()),
        Arc::new(st.category.clone()),
        Arc::new(st.investment.clone()),
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
/// into the request extensions, then continue.
async fn auth_middleware(
    mut req: Request<axum::body::Body>,
    next: Next,
    user_key_service: UserKeyService,
) -> Response {
    let key = extract_bearer(req.headers());
    match key {
        Some(k) => match user_key_service.authenticate(&k).await {
            Ok(uid) => {
                req.extensions_mut().insert(UserId(uid));
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

fn err_json(msg: impl Into<String>) -> String {
    serde_json::json!({ "error": msg.into() }).to_string()
}

fn parse_type(s: &str) -> Option<crate::repo::traits::transaction::TransactionType> {
    match s.to_uppercase().as_str() {
        "DEBIT" => Some(crate::repo::traits::transaction::TransactionType::Debit),
        "CREDIT" => Some(crate::repo::traits::transaction::TransactionType::Credit),
        _ => None,
    }
}

fn extract_bearer(headers: &HeaderMap) -> Option<String> {
    headers
        .get(header::AUTHORIZATION)
        .and_then(|v| v.to_str().ok())
        .and_then(|s| s.strip_prefix("Bearer "))
        .map(str::to_string)
}
