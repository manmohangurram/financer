//! Transactions HTTP handlers — mirroring Go's `httpserver/transaction_handlers.go`.

use axum::extract::{Multipart, Query, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::{Deserialize, Serialize};
use utoipa::{IntoParams, ToSchema};

use crate::error::{json_error, ApiError};
use crate::http::{require_user, AppState, JsonResult};
use crate::repo::traits::transaction::{TransactionListFilter, TransactionType};
use crate::service::transaction::TransactionReq;
use crate::utils::math::round2;
use crate::utils::timex::ts_rfc3339;

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/transactions", axum::routing::get(list).post(create).put(update).delete(delete))
        .route("/api/transactions/import/pdf", axum::routing::post(import_pdf))
}

/// Parse a PDF statement and return its first table as `{headers, rows}`, ready
/// for the same column mapping the CSV/XLSX path uses.
#[utoipa::path(
    post,
    path = "/api/transactions/import/pdf",
    request_body(content = String, content_type = "multipart/form-data"),
    responses(
        (status = 200, description = "Extracted table (headers + rows)"),
        (status = 400, description = "Missing file, encrypted, unreadable, or no table"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn import_pdf(State(st): State<AppState>, headers: HeaderMap, mut mp: Multipart) -> Response {
    if let Err(e) = require_user(&headers, &st.jwt) {
        return e.into_response();
    }
    let mut data: Vec<u8> = Vec::new();
    while let Ok(Some(field)) = mp.next_field().await {
        if field.name() == Some("file") {
            match field.bytes().await {
                Ok(b) => data = b.to_vec(),
                Err(_) => return ApiError::bad_request("failed to read the uploaded PDF").into_response(),
            }
            break;
        }
    }
    if data.is_empty() {
        return ApiError::bad_request("missing 'file' field").into_response();
    }
    match crate::pdf::extract_table(&data) {
        Ok(t) => Json(serde_json::json!({ "headers": t.headers, "rows": t.rows })).into_response(),
        Err(e) => e.into_response(),
    }
}

#[derive(Deserialize, ToSchema)]
pub struct ReqTxn {
    #[serde(default)]
    transactions: Vec<JsonListTxn>,
    #[serde(default)]
    ids: Vec<String>,
}

#[derive(Deserialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
struct JsonListTxn {
    #[serde(default)]
    id: String,
    #[serde(default)]
    name: String,
    #[serde(default)]
    amount: f64,
    /// Strict: missing/unknown value → serde rejection → 400.
    #[serde(rename = "type")]
    transaction_type: TransactionType,
    #[serde(default)]
    occurred_at: serde_json::Value,
    #[serde(default)]
    account_id: String,
    #[serde(default)]
    category_ids: Vec<String>,
    #[serde(default)]
    external_id: String,
}

#[derive(Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
struct WireTxn {
    id: String,
    name: String,
    amount: f64,
    #[serde(rename = "type")]
    transaction_type: String,
    occurred_at: String,
    account_id: String,
    created_at: String,
    linked_transfer_id: String,
    category_ids: Option<Vec<String>>,
}

/// Parse `occurredAt`: {seconds,nanos}, RFC3339, or date string → Go-driver format.
fn txn_occurred_at(v: &serde_json::Value) -> Result<String, ApiError> {
    match v {
        serde_json::Value::Null => Ok(String::new()),
        serde_json::Value::Object(m) => {
            let secs = m.get("seconds").and_then(serde_json::Value::as_i64).unwrap_or(0);
            let nanos = m.get("nanos").and_then(serde_json::Value::as_i64).unwrap_or(0);
            let nanos = u32::try_from(nanos).unwrap_or(0);
            let dt = chrono::DateTime::from_timestamp(secs, nanos)
                .map(|d| d.with_timezone(&chrono::Utc))
                .ok_or_else(|| ApiError::bad_request("invalid date"))?;
            Ok(crate::utils::timex::go_ts(dt))
        }
        serde_json::Value::String(s) => {
            if let Ok(dt) = chrono::DateTime::parse_from_rfc3339(s) {
                return Ok(crate::utils::timex::go_ts(dt.with_timezone(&chrono::Utc)));
            }
            if let Ok(d) = chrono::NaiveDate::parse_from_str(s, "%Y-%m-%d") {
                return Ok(crate::utils::timex::go_ts(d.and_hms_opt(0, 0, 0).unwrap().and_utc()));
            }
            Err(ApiError::bad_request(format!("invalid date {s:?}")))
        }
        _ => Err(ApiError::bad_request("invalid date")),
    }
}

#[derive(Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct WireBulk {
    success: bool,
    message: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    failed_ids: Vec<String>,
    #[serde(skip_serializing_if = "is_zero")]
    skipped: i64,
}

// serde's skip_serializing_if requires fn(&T) -> bool.
#[allow(clippy::trivially_copy_pass_by_ref)]
fn is_zero(v: &i64) -> bool {
    *v == 0
}

pub fn bulk_wire(b: &crate::service::transaction::BulkResult) -> WireBulk {
    WireBulk { success: b.success, message: b.message.clone(), failed_ids: b.failed_ids.clone(), skipped: b.skipped }
}

#[derive(Deserialize, ToSchema, IntoParams)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct ListQuery {
    #[serde(default)]
    page_size: Option<i32>,
    #[serde(default)]
    page_token: Option<String>,
    #[serde(default)]
    account_id: Option<String>,
    #[serde(default, rename = "type")]
    transaction_type: Option<String>,
    #[serde(default)]
    category_id: Option<String>,
    #[serde(default)]
    names: Option<String>,
    #[serde(default)]
    date_from: Option<String>,
    #[serde(default)]
    date_to: Option<String>,
    #[serde(default)]
    min_amount: Option<f64>,
    #[serde(default)]
    max_amount: Option<f64>,
    #[serde(default)]
    sort_by: Option<String>,
    #[serde(default)]
    sort_dir: Option<String>,
    #[serde(default)]
    offset: Option<i32>,
}

#[utoipa::path(
    get,
    path = "/api/transactions",
    params(ListQuery),
    responses(
        (status = 200, description = "Transactions list"),
        (status = 400, description = "Invalid query"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn list(State(st): State<AppState>, headers: HeaderMap, Query(q): Query<ListQuery>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let transaction_type = match q.transaction_type.as_deref() {
        None | Some("") => None,
        Some(s) => match s.parse::<TransactionType>() {
            Ok(t) => Some(t),
            Err(_) => return ApiError::bad_request(format!("unknown transaction type {s:?}")).into_response(),
        },
    };
    let f = TransactionListFilter {
        account_id: q.account_id.unwrap_or_default(),
        category_ids: q.category_id.as_deref().map(|s| s.split(',').map(str::to_string).collect()).unwrap_or_default(),
        transaction_type,
        date_from: q.date_from.unwrap_or_default(),
        date_to: q.date_to.unwrap_or_default(),
        min_amount: q.min_amount.unwrap_or(0.0),
        max_amount: q.max_amount.unwrap_or(0.0),
        names: q.names.as_deref().map(|s| s.split(',').map(str::to_string).collect()).unwrap_or_default(),
        page_size: i64::from(q.page_size.unwrap_or(0)),
        page_token: q.page_token.unwrap_or_default(),
        sort_by: q.sort_by.unwrap_or_default(),
        sort_dir: q.sort_dir.unwrap_or_default(),
        offset: i64::from(q.offset.unwrap_or(0)),
    };
    match st.transaction.list(&uid, f).await {
        Ok(res) => {
            // Actual-Budget model: rules applied at write time are persisted in
            // clean_name. Reads return the resolved display (clean_name when a
            // rule matched, else the raw name) — no read-time rule overlay.
            let items: Vec<WireTxn> = res
                .rows
                .iter()
                .map(|r| WireTxn {
                    id: r.txn.id.clone(),
                    name: r.txn.clean_name.clone().unwrap_or_else(|| r.txn.name.clone()),
                    amount: round2(r.txn.amount),
                    transaction_type: r.txn.transaction_type.to_string(),
                    occurred_at: ts_rfc3339(&r.txn.occurred_at),
                    account_id: r.txn.account_id.clone(),
                    created_at: ts_rfc3339(&r.txn.created_at),
                    linked_transfer_id: r.link_id.clone(),
                    // Wire contract: null (not []) when no categories.
                    category_ids: if r.category_ids.is_empty() { None } else { Some(r.category_ids.clone()) },
                })
                .collect();
            Json(serde_json::json!({
                "transactions": items,
                "nextPageToken": res.next_page_token,
                "totalCount": res.total_count,
            }))
            .into_response()
        }
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    post,
    path = "/api/transactions",
    request_body = ReqTxn,
    responses(
        (status = 201, description = "Transactions created", body = WireBulk),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn create(
    State(st): State<AppState>,
    headers: HeaderMap,
    req: JsonResult<ReqTxn>,
) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let mut txns: Vec<TransactionReq> = Vec::new();
    for t in &req.transactions {
        let occ = match txn_occurred_at(&t.occurred_at) {
            Ok(v) => v,
            Err(e) => return e.into_response(),
        };
        txns.push(TransactionReq {
            id: if t.id.is_empty() { uuid::Uuid::new_v4().to_string() } else { t.id.clone() },
            name: t.name.clone(),
            amount: t.amount,
            transaction_type: t.transaction_type,
            occurred_at: occ,
            account_id: t.account_id.clone(),
            category_ids: t.category_ids.clone(),
            external_id: if t.external_id.is_empty() { None } else { Some(t.external_id.clone()) },
        });
    }
    match st.transaction.create(&uid, &txns).await {
        Ok(b) => (StatusCode::CREATED, Json(bulk_wire(&b))).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    put,
    path = "/api/transactions",
    request_body = ReqTxn,
    responses(
        (status = 200, description = "Transactions updated", body = WireBulk),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn update(
    State(st): State<AppState>,
    headers: HeaderMap,
    req: JsonResult<ReqTxn>,
) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let mut txns: Vec<TransactionReq> = Vec::new();
    for t in &req.transactions {
        let occ = match txn_occurred_at(&t.occurred_at) {
            Ok(v) => v,
            Err(e) => return e.into_response(),
        };
        txns.push(TransactionReq {
            id: t.id.clone(),
            name: t.name.clone(),
            amount: t.amount,
            transaction_type: t.transaction_type,
            occurred_at: occ,
            account_id: t.account_id.clone(),
            category_ids: t.category_ids.clone(),
            external_id: None,
        });
    }
    match st.transaction.update(&uid, &txns).await {
        Ok(b) => Json(bulk_wire(&b)).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    delete,
    path = "/api/transactions",
    request_body = ReqTxn,
    responses(
        (status = 204, description = "Transactions deleted"),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn delete(
    State(st): State<AppState>,
    headers: HeaderMap,
    req: JsonResult<ReqTxn>,
) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.transaction.delete(&uid, &req.ids).await {
        Ok(_) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}
