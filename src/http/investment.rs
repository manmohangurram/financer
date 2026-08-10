//! Investments HTTP handlers — mirroring Go's `httpserver/investment_handlers.go`.

use axum::extract::{Path, Query, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::Deserialize;

use crate::error::{json_error, ApiError};
use crate::http::{require_user, AppState, JsonResult};
use crate::repo::investment::InvestmentType;
use crate::service::investment::{occurred_at, ImportRow};
use std::str::FromStr;

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/investments", axum::routing::get(list).post(create))
        .route("/api/investments/search", axum::routing::get(search))
        .route("/api/investments/refresh-prices", axum::routing::post(refresh_prices))
        .route("/api/investments/import", axum::routing::post(import))
        .route("/api/investments/{id}", axum::routing::get(get).put(update).delete(delete))
        .route("/api/investments/{id}/lots", axum::routing::get(list_lots).post(add_lot))
        .route("/api/investments/{id}/lots/{lot_id}", axum::routing::put(update_lot).delete(delete_lot))
        .route("/api/investments/{id}/price-history", axum::routing::get(price_history))
        .route("/api/portfolio/summary", axum::routing::get(portfolio_summary))
}


/// Investment create/update body: `investmentType` is required (strict).
#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReqInvestment {
    #[serde(default)]
    #[allow(dead_code)]
    id: String,
    #[serde(default)]
    symbol: String,
    #[serde(default)]
    name: String,
    #[serde(rename = "investmentType")]
    investment_type: InvestmentType,
    #[serde(default)]
    manual_nav: f64,
}

/// Lot body: no type field (Go's addLot/updateLot don't carry one).
#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReqLot {
    #[serde(default)]
    side: Option<i64>,
    #[serde(default)]
    quantity: f64,
    #[serde(default)]
    price: f64,
    #[serde(default)]
    occurred_at: serde_json::Value,
}

fn parse_type(v: &serde_json::Value) -> Result<InvestmentType, ApiError> {
    match v {
        serde_json::Value::String(s) => InvestmentType::from_str(s).map_err(|_| ApiError::bad_request(format!("unknown investment type {s:?}"))),
        serde_json::Value::Number(n) => match n.as_i64() {
            Some(1) => Ok(InvestmentType::Stock),
            Some(2) => Ok(InvestmentType::MutualFund),
            _ => Err(ApiError::bad_request("invalid investment type")),
        },
        serde_json::Value::Null => Ok(InvestmentType::Stock),
        _ => Err(ApiError::bad_request("invalid investment type")),
    }
}

async fn create(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqInvestment>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.investment.create(&uid, &req.symbol, &req.name, req.investment_type, req.manual_nav).await {
        Ok(r) => (StatusCode::CREATED, Json(r)).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn list(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.investment.list(&uid).await {
        Ok(items) => Json(serde_json::json!({ "investments": items })).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn get(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.investment.get(&uid, &id).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn update(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>, req: JsonResult<ReqInvestment>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.investment.update(&uid, &id, &req.symbol, &req.name, req.investment_type, req.manual_nav).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn delete(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.investment.delete(&uid, &id).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}

async fn add_lot(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>, req: JsonResult<ReqLot>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let side = match parse_side(req.side) {
        Ok(s) => s,
        Err(e) => return e.into_response(),
    };
    let occ = match occurred_at(&req.occurred_at) {
        Ok(v) => v,
        Err(e) => return e.into_response(),
    };
    match st.investment.add_lot(&uid, &id, side, req.quantity, req.price, &occ).await {
        Ok(r) => (StatusCode::CREATED, Json(r)).into_response(),
        Err(e) => e.into_response(),
    }
}

fn parse_side(v: Option<i64>) -> Result<i64, ApiError> {
    match v {
        Some(s @ (1 | -1)) => Ok(s),
        _ => Err(ApiError::bad_request("side must be 1 (buy) or -1 (sell)")),
    }
}

async fn delete_lot(State(st): State<AppState>, headers: HeaderMap, Path((_, lot_id)): Path<(String, String)>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.investment.delete_lot(&uid, &lot_id).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}

async fn update_lot(State(st): State<AppState>, headers: HeaderMap, Path((id, lot_id)): Path<(String, String)>, req: JsonResult<ReqLot>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let occ = match occurred_at(&req.occurred_at) {
        Ok(v) => v,
        Err(e) => return e.into_response(),
    };
    match st.investment.update_lot(&uid, &id, &lot_id, req.quantity, req.price, &occ).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn list_lots(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.investment.list_lots(&uid, &id).await {
        Ok(items) => Json(serde_json::json!({ "lots": items })).into_response(),
        Err(e) => e.into_response(),
    }
}

#[derive(Deserialize)]
struct ImportReq {
    #[serde(default)]
    rows: Vec<ReqImportRow>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReqImportRow {
    #[serde(default)]
    symbol: String,
    #[serde(default)]
    name: String,
    #[serde(default)]
    investment_type: serde_json::Value,
    #[serde(default)]
    side: serde_json::Value,
    #[serde(default)]
    quantity: f64,
    #[serde(default)]
    price: f64,
    #[serde(default)]
    occurred_at: serde_json::Value,
    #[serde(default)]
    external_id: String,
}

async fn import(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ImportReq>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let mut rows = Vec::new();
    for row in &req.rows {
        let typ = match parse_type(&row.investment_type) {
            Ok(t) => t,
            Err(_) => InvestmentType::Stock,
        };
        let side = parse_import_side(&row.side).unwrap_or(1);
        let Ok(occ) = occurred_at(&row.occurred_at) else { continue };
        rows.push(ImportRow {
            symbol: row.symbol.clone(),
            name: row.name.clone(),
            investment_type: typ,
            side,
            quantity: row.quantity,
            price: row.price,
            occurred_at: occ,
            external_id: row.external_id.clone(),
        });
    }
    match st.investment.import(&uid, &rows).await {
        Ok((created, skipped)) => Json(serde_json::json!({ "created": created, "skipped": skipped })).into_response(),
        Err(e) => e.into_response(),
    }
}

fn parse_import_side(v: &serde_json::Value) -> Option<i64> {
    match v {
        serde_json::Value::Number(n) => match n.as_i64() {
            Some(-1) => Some(-1),
            _ => Some(1),
        },
        serde_json::Value::String(s) => match s.as_str() {
            "sell" => Some(-1),
            _ => Some(1),
        },
        _ => None,
    }
}

#[derive(Deserialize)]
struct HistoryQuery {
    #[serde(default)]
    range: String,
    #[serde(default)]
    from: String,
    #[serde(default)]
    to: String,
    #[serde(default)]
    refresh: String,
}

async fn price_history(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>, Query(q): Query<HistoryQuery>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let force = q.refresh == "1";
    match st.investment.get_price_history(&uid, &id, &q.range, &q.from, &q.to, force).await {
        Ok(points) => Json(serde_json::json!({ "points": points })).into_response(),
        Err(e) => e.into_response(),
    }
}

#[derive(Deserialize)]
struct SearchQuery {
    #[serde(default)]
    query: String,
}

async fn search(State(st): State<AppState>, headers: HeaderMap, Query(q): Query<SearchQuery>) -> Response {
    let _uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.investment.search_symbols(&q.query).await {
        Ok(results) => {
            let items: Vec<_> = results.iter().map(|r| serde_json::json!({
                "symbol": r.symbol,
                "name": r.name,
                "investmentType": r.investment_type.to_string(),
            })).collect();
            Json(serde_json::json!({ "results": items })).into_response()
        }
        Err(e) => e.into_response(),
    }
}

async fn refresh_prices(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.investment.refresh_prices(&uid).await {
        Ok(updated) => Json(serde_json::json!({ "updated": updated })).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn portfolio_summary(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.investment.portfolio_summary(&uid).await {
        Ok(s) => Json(s).into_response(),
        Err(e) => e.into_response(),
    }
}
