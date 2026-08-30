//! API spec generation (`utoipa`) + Swagger UI serving.

use utoipa::OpenApi;

use crate::repo::traits::account::AccountType;
use crate::repo::traits::investment::InvestmentType;
use crate::repo::traits::rule::{ActionOp, ActionType, MatchField, MatchOperator, RuleLogic};
use crate::repo::traits::transaction::TransactionType;

#[derive(OpenApi)]
#[openapi(
    info(
        title = "Financer API",
        description = "Personal finance tracker — REST JSON API. All endpoints except /api/auth/* require `Authorization: Bearer <accessToken>`.",
        version = env!("CARGO_PKG_VERSION")
    ),
    paths(
        crate::http::signup,
        crate::http::login,
        crate::http::refresh,
        crate::http::account::list,
        crate::http::account::create,
        crate::http::account::update,
        crate::http::account::delete,
        crate::http::analytics::dashboard,
        crate::http::analytics::spending,
        crate::http::category::list,
        crate::http::category::create,
        crate::http::category::update,
        crate::http::category::delete,
        crate::http::transaction::list,
        crate::http::transaction::create,
        crate::http::transaction::update,
        crate::http::transaction::delete,
        crate::http::transfer::link,
        crate::http::transfer::unlink,
        crate::http::transfer::counterpart,
        crate::http::rule::list,
        crate::http::rule::create,
        crate::http::rule::update,
        crate::http::rule::delete,
        crate::http::rule::preview,
        crate::http::rule::run,
        crate::http::investment::list,
        crate::http::investment::create,
        crate::http::investment::get,
        crate::http::investment::update,
        crate::http::investment::delete,
        crate::http::investment::list_lots,
        crate::http::investment::add_lot,
        crate::http::investment::update_lot,
        crate::http::investment::delete_lot,
        crate::http::investment::price_history,
        crate::http::investment::search,
        crate::http::investment::refresh_prices,
        crate::http::investment::portfolio_summary,
        crate::http::investment::import,
        crate::http::settings::get,
        crate::http::settings::update,
        crate::http::settings::password,
        crate::http::settings::logout_all,
        crate::http::settings::avatar,
    ),
    components(
        schemas(
            AccountType,
            TransactionType,
            InvestmentType,
            RuleLogic,
            MatchField,
            MatchOperator,
            ActionOp,
            ActionType,
        )
    ),
    tags(
        (name = "auth", description = "Authentication"),
        (name = "accounts", description = "Bank accounts"),
        (name = "transactions", description = "Transactions & transfers"),
        (name = "categories", description = "Categories"),
        (name = "rules", description = "Auto-categorization rules"),
        (name = "investments", description = "Investments, lots, portfolio"),
        (name = "analytics", description = "Dashboard & spending"),
        (name = "settings", description = "Profile, password, avatar"),
    ),
    modifiers(&SecurityAddon)
)]
pub struct ApiDoc;

/// Attach the bearer-auth security scheme to the whole spec.
pub struct SecurityAddon;

impl utoipa::Modify for SecurityAddon {
    fn modify(&self, openapi: &mut utoipa::openapi::OpenApi) {
        if let Some(components) = openapi.components.as_mut() {
            components.add_security_scheme(
                "bearer_auth",
                utoipa::openapi::security::SecurityScheme::Http(
                    utoipa::openapi::security::HttpBuilder::new()
                        .scheme(utoipa::openapi::security::HttpAuthScheme::Bearer)
                        .bearer_format("JWT")
                        .build(),
                ),
            );
        }
    }
}
