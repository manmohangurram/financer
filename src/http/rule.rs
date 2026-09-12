//! Rules HTTP handlers — mirroring Go's `httpserver/rule_handlers.go`.

use axum::extract::{Path, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::{Json, Router};
use serde::Deserialize;
use utoipa::ToSchema;

use crate::error::json_error;
use crate::http::{require_user, AppState, JsonResult};
use crate::repo::traits::rule::{ActionOp, MatchField, MatchOperator, RuleAction, RuleCondition, RuleLogic};

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/api/rules", axum::routing::get(list).post(create))
        .route("/api/rules/{id}", axum::routing::put(update).delete(delete))
        .route("/api/rules/preview", axum::routing::post(preview))
        .route("/api/rules/{id}/run", axum::routing::post(run))
}


#[derive(Deserialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
pub struct ReqRule {
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

#[derive(Deserialize, ToSchema)]
struct ReqCondition {
    #[serde(rename = "matchField")]
    match_field: MatchField,
    operator: MatchOperator,
    #[serde(default)]
    pattern: String,
}

#[allow(clippy::struct_field_names)]
#[derive(Deserialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
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

#[derive(Deserialize, ToSchema)]
pub struct ReqPreview {
    logic: RuleLogic,
    #[serde(default)]
    conditions: Vec<ReqCondition>,
    #[serde(default)]
    limit: i64,
}

#[utoipa::path(
    get,
    path = "/api/rules",
    responses(
        (status = 200, description = "Rules list"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn list(State(st): State<AppState>, headers: HeaderMap) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.rule.list(&uid).await {
        Ok(rules) => Json(serde_json::json!({ "rules": rules })).into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    post,
    path = "/api/rules",
    request_body = ReqRule,
    responses(
        (status = 201, description = "Rule created", body = crate::repo::traits::rule::Rule),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn create(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqRule>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.rule.create(&uid, &req.name, req.priority, req.logic, &req.conditions(), &req.actions()).await {
        Ok(r) => {
            if let Err(e) = st.rule.apply_to_existing(&uid).await {
                tracing::warn!("rule backfill failed: {}", e.message);
            }
            (StatusCode::CREATED, Json(r)).into_response()
        }
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    put,
    path = "/api/rules/{id}",
    params(("id", description = "Rule id")),
    request_body = ReqRule,
    responses(
        (status = 200, description = "Rule updated", body = crate::repo::traits::rule::Rule),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
        (status = 404, description = "Rule not found"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn update(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>, req: JsonResult<ReqRule>) -> Response {
    let Json(req) = match req {
        Ok(r) => r,
        Err(e) => return json_error(&e).into_response(),
    };
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.rule.update(&uid, &id, &req.name, req.priority, req.logic, &req.conditions(), &req.actions()).await {
        Ok(r) => {
            if let Err(e) = st.rule.apply_to_existing(&uid).await {
                tracing::warn!("rule backfill failed: {}", e.message);
            }
            Json(r).into_response()
        }
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    delete,
    path = "/api/rules/{id}",
    params(("id", description = "Rule id")),
    responses(
        (status = 204, description = "Rule deleted"),
        (status = 401, description = "Unauthenticated"),
        (status = 404, description = "Rule not found"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn delete(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    match st.rule.delete(&uid, &id).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    post,
    path = "/api/rules/preview",
    request_body = ReqPreview,
    responses(
        (status = 200, description = "Preview matching transactions"),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn preview(State(st): State<AppState>, headers: HeaderMap, req: JsonResult<ReqPreview>) -> Response {
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
                "occurredAt": crate::utils::timex::ts_rfc3339(&v.occurred_at),
            })).collect();
            Json(serde_json::json!({ "transactions": items })).into_response()
        }
        Err(e) => e.into_response(),
    }
}

#[utoipa::path(
    post,
    path = "/api/rules/{id}/run",
    params(("id", description = "Rule id")),
    responses(
        (status = 200, description = "Rule run against existing transactions"),
        (status = 400, description = "Invalid input"),
        (status = 401, description = "Unauthenticated"),
        (status = 404, description = "Rule not found"),
    ),
    security(("bearer_auth" = []))
)]
pub async fn run(State(st): State<AppState>, headers: HeaderMap, Path(id): Path<String>) -> Response {
    let uid = match require_user(&headers, &st.jwt) {
        Ok(u) => u,
        Err(e) => return e.into_response(),
    };
    let updated = match st.rule.run(&uid, &id).await {
        Ok(n) => n,
        Err(e) => return e.into_response(),
    };
    match st.transfer_rule.run_rule(&uid, &id).await {
        Ok(r) => Json(serde_json::json!({ "updated": updated, "matched": r.matched, "linked": r.linked, "created": r.created })).into_response(),
        Err(e) => e.into_response(),
    }
}
