//! Dashboard + spending HTTP handlers — mirroring Go's `httpserver/`
//! `dashboard_handlers.go` and `spending_handlers.go`.

use axum::extract::{Query, State};
use axum::http::HeaderMap;
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::Deserialize;

use crate::http::{require_user, AppState};
use crate::timex::round2;

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/dashboard", axum::routing::get(dashboard))
        .route("/api/spending", axum::routing::get(spending))
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct SpendingQuery {
    #[serde(default)]
    range: String,
    #[serde(default)]
    from: String,
    #[serde(default)]
    to: String,
    #[serde(default)]
    account_id: String,
}

async fn dashboard(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let dashboard = match st.transaction.dashboard(&uid).await {
        Ok(v) => v,
        Err(e) => return e.into_response(),
    };
    let summary = match st.investment.portfolio_summary(&uid).await {
        Ok(s) => s,
        Err(e) => return e.into_response(),
    };
    let accounts = match st.account.list(&uid).await {
        Ok(v) => v,
        Err(e) => return e.into_response(),
    };
    let investments = match st.investment.list(&uid).await {
        Ok(v) => v,
        Err(e) => return e.into_response(),
    };

    Json(serde_json::json!({
        "totalBalance": round2(dashboard.total_balance),
        "totalIncome": round2(dashboard.total_income),
        "totalExpenses": round2(dashboard.total_expenses),
        "portfolioValue": round2(summary.total_current_value),
        "accounts": accounts,
        "investments": investments,
    }))
    .into_response()
}

async fn spending(State(st): State<AppState>, headers: HeaderMap, Query(q): Query<SpendingQuery>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.transaction.spending(&uid, &q.range, &q.from, &q.to, &q.account_id).await {
        Ok(res) => {
            let buckets: Vec<_> = res.buckets.iter().map(|b| serde_json::json!({
                "key": b.key, "label": b.label, "amount": round2(b.amount),
            })).collect();
            let cats: Vec<_> = res.categories.iter().map(|c| serde_json::json!({
                "id": c.id, "name": c.name, "debit": round2(c.debit), "credit": round2(c.credit), "net": round2(c.net),
            })).collect();
            Json(serde_json::json!({ "buckets": buckets, "categories": cats })).into_response()
        }
        Err(e) => e.into_response(),
    }
}
