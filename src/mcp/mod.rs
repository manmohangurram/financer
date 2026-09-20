//! MCP server — exposes financer data to external AI agents over the Model
//! Context Protocol. Mounted as a streamable-HTTP server at `/mcp`. Read tools
//! cover accounts/transactions/categories/investments; rule + category tools
//! allow a client (e.g. an AI agent) to manage rules and categories. Auth uses
//! the per-user API key from the `Authorization` header; the authenticated
//! `user_id` is injected into the request extensions and read by each tool
//! method (per-request, no shared mutable state).

pub mod models;

/// Auth preamble for a tool: resolve the user id or return an error string.
/// `return` targets the calling tool fn. Usage: `let uid = uid!(&ctx);`
macro_rules! uid {
    ($ctx:expr) => {
        match FinancerHandler::user_id($ctx) {
            Ok(u) => u,
            Err(e) => return err_json(e.message),
        }
    };
}

use std::sync::Arc;

use axum::Router;
use axum::http::{HeaderMap, Request, header};
use axum::middleware::{self, Next};
use axum::response::{IntoResponse, Response};
use rmcp::handler::server::wrapper::Parameters;
use rmcp::service::RequestContext;
use rmcp::transport::streamable_http_server::session::local::LocalSessionManager;
use rmcp::transport::{StreamableHttpServerConfig, StreamableHttpService};
use rmcp::{RoleServer, ServerHandler, tool, tool_handler, tool_router};

use crate::http::AppState;
use crate::mcp::models::{
    CategoriesReq, CreateAccountReq, CreateCategoryReq, CreateTransactionsReq,
    CreateTransferCounterpartReq, DeleteReq, DeleteTransactionsReq, InvestmentReq,
    ListTransactionsReq, LotInputReq, PreviewRuleReq, RuleReq, RunRuleReq, TransfersReq,
    TxnPayload, UpdateAccountReq, UpdateRuleReq, UpdateTransactionsReq,
};
use crate::repo::traits::transaction::TransactionListFilter;
use crate::service::account::AccountService;
use crate::service::category::CategoryService;
use crate::service::investment::InvestmentService;
use crate::service::rule::RuleService;
use crate::service::transaction::{TransactionReq, TransactionService};
use crate::service::transfer::TransferService;
use crate::service::transfer_rule::TransferRuleService;
use crate::service::user_key::UserKeyService;

/// The authenticated user id, injected into request extensions by the auth
/// middleware and read by each tool method.
#[derive(Clone)]
pub struct UserId(pub String);

/// API key scope: `read` or `read_write`. Injected by the auth middleware.
/// Consumed by write tools (later branches); dead-code until then.
#[derive(Clone)]
#[allow(dead_code)]
pub struct KeyScope(pub String);

impl KeyScope {
    /// Whether this key may perform write (mutating) tools.
    #[allow(dead_code)]
    pub fn can_write(&self) -> bool {
        self.0.eq_ignore_ascii_case("read_write")
    }
}

/// MCP handler. Holds the services (read + rule/category manage) + auth service.
#[derive(Clone)]
pub struct FinancerHandler {
    pub account: Arc<AccountService>,
    pub transaction: Arc<TransactionService>,
    pub category: Arc<CategoryService>,
    pub investment: Arc<InvestmentService>,
    pub rule: Arc<RuleService>,
    pub transfer: Arc<TransferService>,
    pub transfer_rule: Arc<TransferRuleService>,
}

impl FinancerHandler {
    pub fn new(
        account: Arc<AccountService>,
        transaction: Arc<TransactionService>,
        category: Arc<CategoryService>,
        investment: Arc<InvestmentService>,
        rule: Arc<RuleService>,
        transfer: Arc<TransferService>,
        transfer_rule: Arc<TransferRuleService>,
    ) -> Self {
        Self {
            account,
            transaction,
            category,
            investment,
            rule,
            transfer,
            transfer_rule,
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

#[tool_router]
impl FinancerHandler {
    #[tool(description = "List the user's accounts with balances. Example: no arguments.")]
    async fn list_accounts(&self, ctx: RequestContext<RoleServer>) -> String {
        let uid = uid!(&ctx);
        let rows = match self.account.list(&uid).await {
            Ok(r) => r,
            Err(e) => return err_json(e.message),
        };
        serde_json::to_string(&rows).unwrap_or_else(|e| err_json(format!("serialize error: {e}")))
    }

    #[tool(
        description = "List the user's transactions. Example args: {\"categoryIds\":[\"<id>\"], \"ty\":\"DEBIT\"}."
    )]
    async fn list_transactions(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<ListTransactionsReq>,
    ) -> String {
        let uid = uid!(&ctx);
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

    #[tool(
        description = "Total balance and credit/debit totals across accounts. Example: no arguments."
    )]
    async fn get_balance_summary(&self, ctx: RequestContext<RoleServer>) -> String {
        let uid = uid!(&ctx);
        let d = match self.transaction.dashboard(&uid).await {
            Ok(d) => d,
            Err(e) => return err_json(e.message),
        };
        serde_json::to_string(&d).unwrap_or_else(|e| err_json(format!("serialize error: {e}")))
    }

    #[tool(description = "Investments with lots and FIFO positions. Example: no arguments.")]
    async fn get_portfolio(&self, ctx: RequestContext<RoleServer>) -> String {
        let uid = uid!(&ctx);
        let p = match self.investment.portfolio_summary(&uid).await {
            Ok(p) => p,
            Err(e) => return err_json(e.message),
        };
        serde_json::to_string(&p).unwrap_or_else(|e| err_json(format!("serialize error: {e}")))
    }

    #[tool(description = "List the user's categories. Example: no arguments.")]
    async fn list_categories(&self, ctx: RequestContext<RoleServer>) -> String {
        let uid = uid!(&ctx);
        let rows = match self.category.list(&uid, 100, "").await {
            Ok((rows, _)) => rows,
            Err(e) => return err_json(e.message),
        };
        serde_json::to_string(&rows).unwrap_or_else(|e| err_json(format!("serialize error: {e}")))
    }

    #[tool(
        description = "Create categories by name. Example args: {\"names\":[\"Food Delivery\"]}."
    )]
    async fn create_category(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<CreateCategoryReq>,
    ) -> String {
        let uid = uid!(&ctx);
        match self.category.create(&uid, &req.names).await {
            Ok(r) => serde_json::json!({ "success": r.success, "message": r.message, "failedIds": r.failed_ids }).to_string(),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(description = "List the user's rules. Example: no arguments.")]
    async fn list_rules(&self, ctx: RequestContext<RoleServer>) -> String {
        let uid = uid!(&ctx);
        match self.rule.list(&uid).await {
            Ok(r) => serde_json::to_string(&r)
                .unwrap_or_else(|e| err_json(format!("serialize error: {e}"))),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Create a rule. Example args: {\"name\":\"Swiggy\",\"conditions\":[{\"matchField\":\"NAME\",\"operator\":\"CONTAINS\",\"pattern\":\"swiggy\"}],\"actions\":[{\"setName\":\"Swiggy\"}]}."
    )]
    async fn create_rule(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<RuleReq>,
    ) -> String {
        let uid = uid!(&ctx);
        match self
            .rule
            .create(
                &uid,
                &req.name,
                req.priority,
                req.logic,
                &req.conditions,
                &req.actions,
            )
            .await
        {
            Ok(r) => serde_json::to_string(&r)
                .unwrap_or_else(|e| err_json(format!("serialize error: {e}"))),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Update a rule by id. Example args: {\"id\":\"<rule-id>\",\"name\":\"Swiggy\"}."
    )]
    async fn update_rule(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<UpdateRuleReq>,
    ) -> String {
        let uid = uid!(&ctx);
        match self
            .rule
            .update(
                &uid,
                &req.id,
                &req.name,
                req.priority,
                req.logic,
                &req.conditions,
                &req.actions,
            )
            .await
        {
            Ok(r) => serde_json::to_string(&r)
                .unwrap_or_else(|e| err_json(format!("serialize error: {e}"))),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(description = "Delete a rule by id. Example args: {\"id\":\"<rule-id>\"}.")]
    async fn delete_rule(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<DeleteReq>,
    ) -> String {
        let uid = uid!(&ctx);
        match self.rule.delete(&uid, &req.id).await {
            Ok(()) => serde_json::json!({"ok": true}).to_string(),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Create transactions. Example args: {\"transactions\":[{\"name\":\"Coffee\",\"amount\":4.5,\"type\":\"DEBIT\",\"accountId\":\"<id>\"}]}."
    )]
    async fn create_transaction(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<CreateTransactionsReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot create transactions");
        }
        let inputs: Vec<TransactionReq> = req
            .transactions
            .into_iter()
            .map(TransactionReq::from)
            .collect();
        match self.transaction.create(&uid, &inputs).await {
            Ok(r) => bulk_json(&r),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Update transactions. Example args: {\"transactions\":[{\"id\":\"<id>\",\"name\":\"Updated\"}]}."
    )]
    async fn update_transaction(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<UpdateTransactionsReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot update transactions");
        }
        let inputs: Vec<TransactionReq> = req
            .transactions
            .into_iter()
            .map(TransactionReq::from)
            .collect();
        match self.transaction.update(&uid, &inputs).await {
            Ok(r) => bulk_json(&r),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(description = "Delete transactions by id. Example args: {\"ids\":[\"<id>\"]}.")]
    async fn delete_transaction(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<DeleteTransactionsReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot delete transactions");
        }
        match self.transaction.delete(&uid, &req.ids).await {
            Ok(r) => bulk_json(&r),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Create an account. Example args: {\"bankName\":\"Chase\",\"type\":\"CURRENT\"}."
    )]
    async fn create_account(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<CreateAccountReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot create accounts");
        }
        match self
            .account
            .create(&uid, &req.bank_name, &req.nickname, req.account_type)
            .await
        {
            Ok(r) => serde_json::to_string(&r)
                .unwrap_or_else(|e| err_json(format!("serialize error: {e}"))),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Update an account by id. Example args: {\"id\":\"<id>\",\"bankName\":\"Chase Blue\"}."
    )]
    async fn update_account(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<UpdateAccountReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot update accounts");
        }
        match self
            .account
            .update(
                &uid,
                &req.id,
                &req.bank_name,
                &req.nickname,
                req.account_type,
            )
            .await
        {
            Ok(r) => serde_json::to_string(&r)
                .unwrap_or_else(|e| err_json(format!("serialize error: {e}"))),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(description = "Delete an account by id. Example args: {\"id\":\"<id>\"}.")]
    async fn delete_account(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<DeleteReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot delete accounts");
        }
        match self.account.delete(&uid, &req.id).await {
            Ok(()) => serde_json::json!({"ok": true}).to_string(),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Link transfer debit/credit pairs. Example args: {\"links\":[{\"debitTransactionId\":\"<id>\",\"creditTransactionId\":\"<id>\"}]}."
    )]
    async fn link_transfers(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<TransfersReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot link transfers");
        }
        let links: Vec<(String, String)> = req
            .links
            .into_iter()
            .map(|l| (l.debit_transaction_id, l.credit_transaction_id))
            .collect();
        match self.transfer.link_transfers(&uid, &links).await {
            Ok(r) => bulk_json(&r),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(description = "Unlink transfers by link id. Example args: {\"ids\":[\"<link-id>\"]}.")]
    async fn unlink_transfers(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<TransfersReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot unlink transfers");
        }
        match self.transfer.unlink_transfers(&uid, &req.ids).await {
            Ok(r) => bulk_json(&r),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Create the missing side of a transfer (counterpart). Example args: {\"transactionId\":\"<id>\",\"toAccountId\":\"<id>\"}."
    )]
    async fn create_transfer_counterpart(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<CreateTransferCounterpartReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot create transfer counterpart");
        }
        match self.transfer.create_counterpart(&uid, &req.transaction_id, &req.to_account_id).await {
            Ok(r) => serde_json::json!({ "debitTransactionId": r.debit_transaction_id, "creditTransactionId": r.credit_transaction_id }).to_string(),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Preview which transactions match a rule (no mutation). Example args: {\"logic\":\"AND\",\"conditions\":[{\"matchField\":\"NAME\",\"operator\":\"CONTAINS\",\"pattern\":\"swiggy\"}]}."
    )]
    async fn preview_rule(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<PreviewRuleReq>,
    ) -> String {
        let uid = uid!(&ctx);
        match self
            .rule
            .preview(&uid, req.logic, &req.conditions, req.limit)
            .await
        {
            Ok(views) => {
                let items: Vec<_> = views.iter().map(|v| serde_json::json!({ "name": v.name, "amount": v.amount, "type": v.transaction_type.to_string(), "accountId": v.account_id, "categoryIds": v.category_ids })).collect();
                serde_json::json!({ "transactions": items }).to_string()
            }
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Run a rule against existing transactions (applies it). Example args: {\"id\":\"<rule-id>\"}."
    )]
    async fn run_rule(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<RunRuleReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot run rules");
        }
        match self.transfer_rule.run_rule(&uid, &req.id).await {
            Ok(r) => serde_json::json!({ "matched": r.matched, "linked": r.linked, "created": r.created }).to_string(),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Update categories by id+name. Example args: {\"categories\":[{\"id\":\"<id>\",\"name\":\"New\"}]}."
    )]
    async fn update_category(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<CategoriesReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot update categories");
        }
        let inputs: Vec<(String, String)> =
            req.categories.into_iter().map(|c| (c.id, c.name)).collect();
        match self.category.update(&uid, &inputs).await {
            Ok(r) => bulk_json(&r),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(description = "Delete categories by id. Example args: {\"ids\":[\"<id>\"]}.")]
    async fn delete_category(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<CategoriesReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot delete categories");
        }
        match self.category.delete(&uid, &req.ids).await {
            Ok(r) => bulk_json(&r),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Create an investment. Example args: {\"symbol\":\"RELIANCE\",\"name\":\"Reliance\",\"investmentType\":\"STOCK\"}."
    )]
    async fn create_investment(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<InvestmentReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot create investments");
        }
        match self
            .investment
            .create(
                &uid,
                &req.symbol,
                &req.name,
                req.investment_type,
                req.manual_nav,
            )
            .await
        {
            Ok(r) => serde_json::to_string(&r)
                .unwrap_or_else(|e| err_json(format!("serialize error: {e}"))),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Update an investment by id. Example args: {\"id\":\"<id>\",\"symbol\":\"RELIANCE\"}."
    )]
    async fn update_investment(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<InvestmentReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot update investments");
        }
        match self
            .investment
            .update(
                &uid,
                &req.id,
                &req.symbol,
                &req.name,
                req.investment_type,
                req.manual_nav,
            )
            .await
        {
            Ok(r) => serde_json::to_string(&r)
                .unwrap_or_else(|e| err_json(format!("serialize error: {e}"))),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(description = "Delete an investment by id. Example args: {\"id\":\"<id>\"}.")]
    async fn delete_investment(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<DeleteReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot delete investments");
        }
        match self.investment.delete(&uid, &req.id).await {
            Ok(()) => serde_json::json!({"ok": true}).to_string(),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Add a lot to an investment. Example args: {\"investmentId\":\"<id>\",\"side\":1,\"quantity\":10,\"price\":100,\"occurredAt\":\"2024-01-02 10:00:00 +0000 UTC\"}."
    )]
    async fn add_lot(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<LotInputReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot add lots");
        }
        match self
            .investment
            .add_lot(
                &uid,
                &req.investment_id,
                req.side.unwrap_or(1),
                req.quantity,
                req.price,
                &req.occurred_at,
            )
            .await
        {
            Ok(r) => serde_json::to_string(&r)
                .unwrap_or_else(|e| err_json(format!("serialize error: {e}"))),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(
        description = "Update a lot by investment id + lot id. Example args: {\"investmentId\":\"<id>\",\"lotId\":\"<lot>\",\"quantity\":20}."
    )]
    async fn update_lot(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<LotInputReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot update lots");
        }
        match self
            .investment
            .update_lot(
                &uid,
                &req.investment_id,
                &req.lot_id,
                req.quantity,
                req.price,
                &req.occurred_at,
            )
            .await
        {
            Ok(r) => serde_json::to_string(&r)
                .unwrap_or_else(|e| err_json(format!("serialize error: {e}"))),
            Err(e) => err_json(e.message),
        }
    }

    #[tool(description = "Delete a lot by lot id. Example args: {\"id\":\"<lot-id>\"}.")]
    async fn delete_lot(
        &self,
        ctx: RequestContext<RoleServer>,
        Parameters(req): Parameters<DeleteReq>,
    ) -> String {
        let uid = uid!(&ctx);
        if !Self::can_write(&ctx) {
            return err_json("read-only API key; cannot delete lots");
        }
        match self.investment.delete_lot(&uid, &req.id).await {
            Ok(()) => serde_json::json!({"ok": true}).to_string(),
            Err(e) => err_json(e.message),
        }
    }
}

/// Read the injected `KeyScope` and whether writes are allowed.
impl FinancerHandler {
    fn can_write(ctx: &RequestContext<RoleServer>) -> bool {
        ctx.extensions
            .get::<http::request::Parts>()
            .and_then(|p| p.extensions.get::<KeyScope>())
            .is_some_and(KeyScope::can_write)
    }
}

fn bulk_json(r: &crate::service::transaction::BulkResult) -> String {
    serde_json::json!({ "success": r.success, "message": r.message, "failedIds": r.failed_ids })
        .to_string()
}

impl From<TxnPayload> for TransactionReq {
    fn from(p: TxnPayload) -> Self {
        TransactionReq {
            id: p.id,
            name: p.name,
            amount: p.amount,
            transaction_type: p.transaction_type,
            occurred_at: p.occurred_at,
            account_id: p.account_id,
            category_ids: p.category_ids,
            external_id: p.external_id,
        }
    }
}

#[allow(clippy::unused_async_trait_impl)]
#[tool_handler]
impl ServerHandler for FinancerHandler {}

/// Build and mount the MCP server at `/mcp`, authenticated by API key.
pub fn mcp_router(st: &AppState) -> Router<()> {
    let handler = FinancerHandler::new(
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
