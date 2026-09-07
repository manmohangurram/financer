//! Shares investment types + helpers + the backend-agnostic `InvestmentRepo` trait.

use async_trait::async_trait;
use utoipa::ToSchema;

use crate::error::Result;
use crate::timex::{round2, ts_rfc3339};

#[derive(
    Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize, strum::Display, strum::EnumString, ToSchema, schemars::JsonSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum InvestmentType {
    Stock,
    MutualFund,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct InvestmentRow {
    pub id: String,
    #[serde(default)]
    pub symbol: String,
    pub name: String,
    #[allow(clippy::struct_field_names)]
    pub investment_type: InvestmentType,
    #[serde(default, deserialize_with = "de_null_f64")]
    pub current_price: f64,
    #[serde(default, deserialize_with = "de_null_f64")]
    pub prev_close: f64,
    #[serde(default, deserialize_with = "de_null_f64")]
    pub manual_nav: f64,
    #[serde(default, deserialize_with = "de_null_string")]
    pub last_quote_at: String,
    pub created_at: String,
}

fn de_null_string<'de, D: serde::Deserializer<'de>>(d: D) -> std::result::Result<String, D::Error> {
    let v: Option<String> = serde::Deserialize::deserialize(d)?;
    Ok(v.unwrap_or_default())
}

fn de_null_f64<'de, D: serde::Deserializer<'de>>(d: D) -> std::result::Result<f64, D::Error> {
    let v: Option<f64> = serde::Deserialize::deserialize(d)?;
    Ok(v.unwrap_or(0.0))
}

pub struct LotInput {
    pub side: i64,
    pub quantity: f64,
    pub price: f64,
    pub occurred_at: String,
    pub external_id: String,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LotRow {
    pub id: String,
    pub investment_id: String,
    pub side: i64, // 1 buy, -1 sell
    pub quantity: f64,
    pub price: f64,
    pub occurred_at: String,
    pub created_at: String,
}

/// Wire investment (serde covers Go's `investmentWire`).
#[derive(Debug, Clone, serde::Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
pub struct Investment {
    pub id: String,
    pub symbol: String,
    pub name: String,
    #[allow(clippy::struct_field_names)]
    pub investment_type: InvestmentType,
    pub current_price: f64,
    pub prev_close: f64,
    pub manual_nav: f64,
    #[serde(rename = "lastQuoteAt")]
    pub last_quote_at: String,
    #[serde(rename = "createdAt")]
    pub created_at: String,
    pub quantity: f64,
    pub avg_cost: f64,
    pub current_value: f64,
    pub unrealized_pnl: f64,
    pub realized_pnl: f64,
}

/// Wire lot.
#[derive(Debug, Clone, serde::Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
pub struct Lot {
    pub id: String,
    #[serde(rename = "investmentId")]
    pub investment_id: String,
    pub side: i64,
    pub quantity: f64,
    pub price: f64,
    #[serde(rename = "occurredAt")]
    pub occurred_at: String,
    #[serde(rename = "createdAt")]
    pub created_at: String,
}

/// Wire investment with position filled in.
pub fn investment_wire(r: &InvestmentRow, qty: f64, avg_cost: f64, current_value: f64, unrealized: f64, realized: f64) -> Investment {
    let price = effective_price(r);
    Investment {
        id: r.id.clone(),
        symbol: r.symbol.clone(),
        name: r.name.clone(),
        investment_type: r.investment_type,
        current_price: round2(price),
        prev_close: round2(r.prev_close),
        manual_nav: round2(r.manual_nav),
        last_quote_at: if r.last_quote_at.is_empty() { String::new() } else { ts_rfc3339(&r.last_quote_at) },
        created_at: ts_rfc3339(&r.created_at),
        quantity: round2(qty),
        avg_cost: round2(avg_cost),
        current_value: round2(current_value),
        unrealized_pnl: round2(unrealized),
        realized_pnl: round2(realized),
    }
}

pub fn lot_wire(l: &LotRow) -> Lot {
    Lot {
        id: l.id.clone(),
        investment_id: l.investment_id.clone(),
        side: l.side,
        quantity: round2(l.quantity),
        price: round2(l.price),
        occurred_at: ts_rfc3339(&l.occurred_at),
        created_at: ts_rfc3339(&l.created_at),
    }
}

/// effective price: `manual_nav` if set, else cached Yahoo price.
pub fn effective_price(r: &InvestmentRow) -> f64 {
    if r.manual_nav > 0.0 {
        r.manual_nav
    } else {
        r.current_price
    }
}

#[async_trait]
pub trait InvestmentRepo: Send + Sync {
    async fn create_investment(&self, user_id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<InvestmentRow>;
    async fn get_investment(&self, user_id: &str, id: &str) -> Result<Option<InvestmentRow>>;
    async fn get_by_symbol(&self, user_id: &str, symbol: &str) -> Result<Option<InvestmentRow>>;
    async fn list_investments(&self, user_id: &str) -> Result<Vec<InvestmentRow>>;
    async fn update_investment(&self, user_id: &str, id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<InvestmentRow>;
    async fn delete_investment(&self, user_id: &str, id: &str) -> Result<bool>;
    async fn list_lots(&self, user_id: &str, investment_id: &str) -> Result<Vec<LotRow>>;
    /// All lots for a user (batch — avoids N+1 in portfolio/refresh loops).
    async fn list_lots_by_user(&self, user_id: &str) -> Result<Vec<LotRow>>;
    async fn create_lot(&self, user_id: &str, investment_id: &str, side: i64, quantity: f64, price: f64, occurred_at: &str) -> Result<LotRow>;
    async fn delete_lot(&self, user_id: &str, id: &str) -> Result<bool>;
    async fn insert_lot(&self, user_id: &str, investment_id: &str, input: &LotInput) -> Result<bool>;
    async fn update_lot(&self, user_id: &str, id: &str, quantity: f64, price: f64, occurred_at: &str) -> Result<bool>;
    async fn update_quote(&self, id: &str, current_price: f64, prev_close: f64) -> Result<()>;
    async fn upsert_price_history(&self, investment_id: &str, range_id: &str, ts: &[i64], closes: &[f64], fetched_at: i64) -> Result<()>;
    async fn get_price_history(&self, user_id: &str, investment_id: &str, range_id: &str) -> Result<(Vec<i64>, Vec<f64>, i64)>;
}
