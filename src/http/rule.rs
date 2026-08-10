//! Rules HTTP handlers — mirroring Go's `httpserver/rule_handlers.go`.

use axum::extract::{Path, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::Deserialize;

use crate::error::json_error;
use crate::http::{require_user, AppState};
use crate::repo::rule::{ActionOp, MatchField, MatchOperator, RuleAction, RuleCondition, RuleLogic};
use crate::service::rule::RuleService;

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/rules", axum::routing::get(list).post(create))
        .route("/api/rules/{id}", axum::routing::put(update).delete(delete))
        .route("/api/rules/preview", axum::routing::post(preview))
        .route("/api/rules/{id}/run", axum::routing::post(run))
}

type JsonResult<T> = std::result::Result<Json<T>, axum::extract::rejection::JsonRejection>;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReqRule {
    #[serde(default)]
    #[allow(dead_code)]
    id: String,
    #[serde(default)]
    name: String,
    #[serde(default)]
    priority: i64,
    logic: RuleLogic,
    #[serde(default)]
    conditions: Vec<ReqCondition>,
    #[serde(default)]
    actions: Vec<ReqAction>,
}

#[derive(Deserialize)]
struct ReqCondition {
    #[serde(rename = "matchField")]
    match_field: MatchField,
    operator: MatchOperator,
    #[serde(default)]
    pattern: String,
}

#[allow(clippy::struct_field_names)]
#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct ReqAction {
    #[serde(default)]
    set_name: String,
    #[serde(default)]
    set_name_op: Option<ActionOp>,
    #[serde(default)]
    set_category_id: String,
    #[serde(default)]
    set_transfer_account_id: String,
}

impl ReqRule {
    fn conditions(&self) -> Vec<RuleCondition> {
        self.conditions.iter().map(|c| RuleCondition {
            match_field: c.match_field,
            operator: c.operator,
            pattern: c.pattern.clone(),
        }).collect()
    }

    fn actions(&self) -> Vec<RuleAction> {
        self.actions.iter().map(|a| RuleAction {
            set_name: a.set_name.clone(),
            set_name_op: a.set_name_op,
            set_category_id: a.set_category_id.clone(),
            set_transfer_account_id: a.set_transfer_account_id.clone(),
        }).collect()
    }
}

#[derive(Deserialize)]
struct ReqPreview {
    logic: RuleLogic,
    #[serde(default)]
    conditions: Vec<ReqCondition>,
    #[serde(default)]
    limit: i64,
}

async fn list(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.rule.list(&uid).await {
        Ok(rules) => Json(serde_json::json!({ "rules": rules })).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn create(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqRule>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.rule.create(&uid, &req.name, req.priority, req.logic, &req.conditions(), &req.actions()).await {
        Ok(r) => (StatusCode::CREATED, Json(r)).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn update(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>, req: JsonResult<ReqRule>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let _uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.rule.update(&id, &req.name, req.priority, req.logic, &req.conditions(), &req.actions()).await {
        Ok(r) => Json(r).into_response(),
        Err(e) => e.into_response(),
    }
}

async fn delete(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>) -> Response {
    let _uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.rule.delete(&id).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}

async fn preview(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqPreview>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let conds: Vec<RuleCondition> = req.conditions.iter().map(|c| RuleCondition {
        match_field: c.match_field,
        operator: c.operator,
        pattern: c.pattern.clone(),
    }).collect();
    match st.rule.preview(&uid, req.logic, &conds, req.limit).await {
        Ok(views) => {
            let items: Vec<_> = views.iter().map(|v| serde_json::json!({
                "name": v.name,
                "amount": v.amount,
                "type": v.transaction_type.to_string(),
                "accountId": v.account_id,
                "categoryIds": v.category_ids,
            })).collect();
            Json(serde_json::json!({ "transactions": items })).into_response()
        }
        Err(e) => e.into_response(),
    }
}

async fn run(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.transfer_rule.run_rule(&uid, &id).await {
        Ok(r) => Json(serde_json::json!({ "matched": r.matched, "linked": r.linked, "created": r.created })).into_response(),
        Err(e) => e.into_response(),
    }
}

#[allow(dead_code)]
fn _keep_svc(_: &RuleService) {}
