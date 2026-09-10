//! Investments service — FIFO positions, CRUD, import, refresh, price history,
//! mirroring Go's `services/investment.go`.

use std::collections::HashMap;
use std::sync::Arc;

use crate::error::{ApiError, Result};
use crate::repo::traits::investment::{effective_price, investment_wire, lot_wire, Investment, InvestmentRow, InvestmentType, Lot, LotInput, LotRow};
use crate::repo::traits::InvestmentRepo;
use crate::service::fifo::{compute_fifo, FifoLot, Position};
use crate::service::yahoo::{PricePoint, YahooClient};
use crate::utils::math::round2;
use crate::utils::timex::go_ts;
use chrono::Datelike;
use utoipa::ToSchema;

#[derive(Clone)]
pub struct InvestmentService {
    repo: Arc<dyn InvestmentRepo>,
    yahoo: YahooClient,
}

impl InvestmentService {
    pub fn new(repo: Arc<dyn InvestmentRepo>, yahoo: YahooClient) -> Self {
        Self { repo, yahoo }
    }

    /// Compute FIFO position, current value, and unrealized P&L.
    async fn position_for(&self, user_id: &str, inst: &InvestmentRow) -> Result<(Position, f64, f64)> {
        let lots = self.repo.list_lots(user_id, &inst.id).await?;
        Ok(Self::position_from_lots(inst, &lots))
    }

    fn position_from_lots(inst: &InvestmentRow, lots: &[LotRow]) -> (Position, f64, f64) {
        let fifo: Vec<FifoLot> = lots.iter().map(|l| FifoLot { side: l.side, quantity: l.quantity, price: l.price }).collect();
        let pos = compute_fifo(&fifo);
        let price = effective_price(inst);
        let current_value = pos.quantity * price;
        let unrealized = (price - pos.avg_cost) * pos.quantity;
        (pos, current_value, unrealized)
    }

    async fn with_position(&self, user_id: &str, inst: &InvestmentRow) -> Result<Investment> {
        let (pos, current_value, unrealized) = self.position_for(user_id, inst).await?;
        Ok(investment_wire(inst, pos.quantity, pos.avg_cost, current_value, unrealized, pos.realized_pnl))
    }

    pub async fn create(&self, user_id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<Investment> {
        if name.is_empty() {
            return Err(ApiError::bad_request("name is required"));
        }
        if it != InvestmentType::Stock && it != InvestmentType::MutualFund {
            return Err(ApiError::bad_request("INVESTMENT_TYPE must be STOCK or MUTUAL_FUND"));
        }
        if it == InvestmentType::Stock && symbol.is_empty() {
            return Err(ApiError::bad_request("symbol is required for STOCK"));
        }
        let inst = self.repo.create_investment(user_id, symbol, name, it, manual_nav).await?;
        // Fetch current quote once so a new stock shows a price right away.
        if it == InvestmentType::Stock && !symbol.is_empty() {
            let quotes = self.yahoo.get_quotes(&[symbol.to_string()]).await;
            if let Some(q) = quotes.get(symbol) {
                let _ = self.repo.update_quote(&inst.id, q.price, q.prev_close).await;
            }
        }
        let mut inst = inst;
        inst.current_price = effective_price(&inst);
        self.with_position(user_id, &inst).await
    }

    pub async fn get(&self, user_id: &str, id: &str) -> Result<Investment> {
        let inst = self.repo.get_investment(user_id, id).await?.ok_or_else(|| ApiError::not_found(format!("investment {id} not found")))?;
        self.with_position(user_id, &inst).await
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<Investment>> {
        let instruments = self.repo.list_investments(user_id).await?;
        let mut out = Vec::with_capacity(instruments.len());
        for inst in &instruments {
            out.push(self.with_position(user_id, inst).await?);
        }
        Ok(out)
    }

    pub async fn update(&self, user_id: &str, id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<Investment> {
        if name.is_empty() {
            return Err(ApiError::bad_request("name is required"));
        }
        if it != InvestmentType::Stock && it != InvestmentType::MutualFund {
            return Err(ApiError::bad_request("INVESTMENT_TYPE must be STOCK or MUTUAL_FUND"));
        }
        if it == InvestmentType::Stock && symbol.is_empty() {
            return Err(ApiError::bad_request("symbol is required for STOCK"));
        }
        let inst = self.repo.update_investment(user_id, id, symbol, name, it, manual_nav).await?;
        self.with_position(user_id, &inst).await
    }

    pub async fn delete(&self, user_id: &str, id: &str) -> Result<()> {
        if !self.repo.delete_investment(user_id, id).await? {
            return Err(ApiError::not_found(format!("investment {id} not found")));
        }
        Ok(())
    }

    pub async fn add_lot(&self, user_id: &str, investment_id: &str, side: i64, quantity: f64, price: f64, occurred_at: &str) -> Result<Lot> {
        let inst = self.repo.get_investment(user_id, investment_id).await?.ok_or_else(|| ApiError::not_found(format!("investment {investment_id} not found")))?;
        if side != 1 && side != -1 {
            return Err(ApiError::bad_request("side must be 1 (buy) or -1 (sell)"));
        }
        if quantity <= 0.0 {
            return Err(ApiError::bad_request("quantity must be greater than zero"));
        }
        if price < 0.0 {
            return Err(ApiError::bad_request("price must not be negative"));
        }
        if side == -1 {
            let (pos, _, _) = self.position_for(user_id, &inst).await?;
            if quantity > pos.quantity {
                return Err(ApiError::bad_request("cannot sell more than held"));
            }
        }
        let lot = self.repo.create_lot(user_id, investment_id, side, round2(quantity), round2(price), occurred_at).await?;
        Ok(lot_wire(&lot))
    }

    pub async fn delete_lot(&self, user_id: &str, id: &str) -> Result<()> {
        if !self.repo.delete_lot(user_id, id).await? {
            return Err(ApiError::not_found(format!("lot {id} not found")));
        }
        Ok(())
    }

    pub async fn update_lot(&self, user_id: &str, investment_id: &str, id: &str, quantity: f64, price: f64, occurred_at: &str) -> Result<Lot> {
        if quantity <= 0.0 {
            return Err(ApiError::bad_request("quantity must be greater than zero"));
        }
        if price < 0.0 {
            return Err(ApiError::bad_request("price must not be negative"));
        }
        let _ = self.repo.get_investment(user_id, investment_id).await?.ok_or_else(|| ApiError::not_found(format!("investment {investment_id} not found")))?;
        if !self.repo.update_lot(user_id, id, round2(quantity), round2(price), occurred_at).await? {
            return Err(ApiError::not_found(format!("lot {id} not found")));
        }
        let lots = self.repo.list_lots(user_id, investment_id).await?;
        lots.into_iter().find(|l| l.id == id).map(|l| lot_wire(&l)).ok_or_else(|| ApiError::not_found(format!("lot {id} not found")))
    }

    pub async fn list_lots(&self, user_id: &str, id: &str) -> Result<Vec<Lot>> {
        let _ = self.repo.get_investment(user_id, id).await?.ok_or_else(|| ApiError::not_found(format!("investment {id} not found")))?;
        Ok(self.repo.list_lots(user_id, id).await?.iter().map(lot_wire).collect())
    }

    pub async fn import(&self, user_id: &str, rows: &[ImportRow]) -> Result<(i64, i64)> {
        let mut created = 0i64;
        let mut skipped = 0i64;
        for row in rows {
            if row.symbol.is_empty() || row.quantity <= 0.0 || row.price < 0.0 {
                skipped += 1;
                continue;
            }
            let mut typ = row.investment_type;
            if typ != InvestmentType::Stock && typ != InvestmentType::MutualFund {
                typ = InvestmentType::Stock;
            }
            let inst = match self.repo.get_by_symbol(user_id, &row.symbol).await? {
                Some(i) => i,
                None => self.repo.create_investment(user_id, &row.symbol, &row.name, typ, 0.0).await?,
            };
            if row.side == -1 {
                let (pos, _, _) = self.position_for(user_id, &inst).await?;
                if row.quantity > pos.quantity {
                    skipped += 1;
                    continue;
                }
            }
            let inserted = self.repo.insert_lot(user_id, &inst.id, &LotInput {
                side: row.side,
                quantity: round2(row.quantity),
                price: round2(row.price),
                occurred_at: row.occurred_at.clone(),
                external_id: row.external_id.clone(),
            }).await?;
            if inserted {
                created += 1;
            } else {
                skipped += 1;
            }
        }
        Ok((created, skipped))
    }

    pub async fn search_symbols(&self, query: &str) -> Result<Vec<crate::service::yahoo::SymbolResult>> {
        if query.is_empty() {
            return Ok(Vec::new());
        }
        self.yahoo.search(query).await.map_err(|e| ApiError::bad_gateway(format!("symbol search temporarily unavailable: {e}")))
    }

    pub async fn refresh_prices(&self, user_id: &str) -> Result<i64> {
        let instruments = self.repo.list_investments(user_id).await?;
        let mut symbols: Vec<String> = Vec::new();
        let mut sym_idx: HashMap<String, String> = HashMap::new();
        for inst in &instruments {
            if !inst.symbol.is_empty() {
                symbols.push(inst.symbol.clone());
                sym_idx.insert(inst.symbol.clone(), inst.id.clone());
            }
        }
        let quotes = self.yahoo.get_quotes(&symbols).await;
        let mut updated = 0i64;
        for (sym, quote) in quotes {
            if let Some(id) = sym_idx.get(&sym) {
                self.repo.update_quote(id, quote.price, quote.prev_close).await?;
                updated += 1;
            }
        }
        Ok(updated)
    }

    pub async fn portfolio_summary(&self, user_id: &str) -> Result<PortfolioSummary> {
        let instruments = self.repo.list_investments(user_id).await?;
        // Batch-fetch all lots once instead of one query per investment.
        let all_lots = self.repo.list_lots_by_user(user_id).await?;
        let mut lots_by_inv: std::collections::HashMap<String, Vec<LotRow>> = std::collections::HashMap::new();
        for l in all_lots {
            lots_by_inv.entry(l.investment_id.clone()).or_default().push(l);
        }
        let mut summary = PortfolioSummary::default();
        for inst in &instruments {
            let lots = lots_by_inv.get(&inst.id).map_or(&[][..], |v| v.as_slice());
            let (pos, current_value, _) = Self::position_from_lots(inst, lots);
            let price = effective_price(inst);
            summary.total_invested += pos.cost_basis;
            summary.total_current_value += current_value;
            summary.total_unrealized_pnl += (price - pos.avg_cost) * pos.quantity;
            summary.total_realized_pnl += pos.realized_pnl;
        }
        Ok(summary)
    }

    pub async fn get_price_history(&self, user_id: &str, investment_id: &str, range_id: &str, from: &str, to: &str, force: bool) -> Result<Vec<PricePoint>> {
        let (cfg, cache_key, period1, period2) = resolve_range(range_id, from, to)?;
        let inst = self.repo.get_investment(user_id, investment_id).await?.ok_or_else(|| ApiError::not_found(format!("investment {investment_id} not found")))?;
        if inst.symbol.is_empty() {
            return Ok(Vec::new());
        }
        if !force {
            let (ts, closes, last_fetched) = self.repo.get_price_history(user_id, investment_id, &cache_key).await?;
            let fresh_secs = price_history_freshness(range_id);
            if !ts.is_empty() && crate::utils::timex::now_utc().timestamp() - last_fetched < fresh_secs {
                let points: Vec<PricePoint> = ts.iter().zip(closes.iter()).map(|(t, c)| PricePoint { t: *t, close: *c }).collect();
                return Ok(points);
            }
        }
        self.fetch_and_cache(&inst, &cache_key, &cfg, period1, period2).await
    }

    async fn fetch_and_cache(&self, inst: &InvestmentRow, cache_key: &str, cfg: &RangeConfig, period1: i64, period2: i64) -> Result<Vec<PricePoint>> {
        let raw = self.yahoo.get_history(&inst.symbol, &cfg.interval, cfg.limit, period1, period2).await.map_err(|e| ApiError::bad_gateway(format!("price data temporarily unavailable: {e}")))?;
        let points = aggregate_price_points(&raw, &cfg.agg);
        let ts: Vec<i64> = points.iter().map(|p| p.t).collect();
        let closes: Vec<f64> = points.iter().map(|p| p.close).collect();
        let _ = self.repo.upsert_price_history(&inst.id, cache_key, &ts, &closes, crate::utils::timex::now_utc().timestamp()).await;
        Ok(points)
    }
}

/// One import row (wire).
#[derive(Debug, Clone)]
pub struct ImportRow {
    pub symbol: String,
    pub name: String,
    pub investment_type: InvestmentType,
    pub side: i64,
    pub quantity: f64,
    pub price: f64,
    pub occurred_at: String,
    pub external_id: String,
}

#[derive(Debug, Default, serde::Serialize, ToSchema)]
#[serde(rename_all = "camelCase")]
#[schema(rename_all = "camelCase")]
// `total_` prefix matches the wire keys (totalInvested, ...).
#[allow(clippy::struct_field_names)]
pub struct PortfolioSummary {
    pub total_invested: f64,
    pub total_current_value: f64,
    pub total_unrealized_pnl: f64,
    pub total_realized_pnl: f64,
}

#[derive(Debug, Clone)]
struct RangeConfig {
    interval: String,
    limit: usize,
    agg: String,
}

fn price_history_ranges() -> HashMap<&'static str, RangeConfig> {
    let mut m = HashMap::new();
    m.insert("1d", RangeConfig { interval: "15m".into(), limit: 0, agg: String::new() });
    m.insert("7d", RangeConfig { interval: "1d".into(), limit: 7, agg: String::new() });
    m.insert("1m", RangeConfig { interval: "1d".into(), limit: 0, agg: String::new() });
    m.insert("6m", RangeConfig { interval: "1d".into(), limit: 0, agg: String::new() });
    m.insert("1y", RangeConfig { interval: "1d".into(), limit: 0, agg: "week".into() });
    m.insert("3y", RangeConfig { interval: "1d".into(), limit: 0, agg: "month".into() });
    m
}

fn price_history_freshness(range: &str) -> i64 {
    match range {
        "1d" => 5 * 60,
        "7d" | "1m" => 3600,
        "3y" => 24 * 3600,
        _ => 6 * 3600,
    }
}

/// Resolve a range/custom period to `RangeConfig` + cache key + Unix timestamps.
fn resolve_range(range_id: &str, from: &str, to: &str) -> Result<(RangeConfig, String, i64, i64)> {
    if !from.is_empty() || !to.is_empty() {
        let f = chrono::NaiveDate::parse_from_str(from, "%Y-%m-%d").map_err(|_| ApiError::bad_request("from and to must be YYYY-MM-DD"))?;
        let t = chrono::NaiveDate::parse_from_str(to, "%Y-%m-%d").map_err(|_| ApiError::bad_request("from and to must be YYYY-MM-DD"))?;
        if t < f || (t - f).num_days() > 5 * 365 {
            return Err(ApiError::bad_request("custom range must be at most 5 years"));
        }
        let period1 = f.and_hms_opt(0, 0, 0).unwrap().and_utc().timestamp();
        let period2 = (t + chrono::Duration::days(1)).and_hms_opt(0, 0, 0).unwrap().and_utc().timestamp();
        return Ok((RangeConfig { interval: "1d".into(), limit: 0, agg: String::new() }, format!("custom:{from}:{to}"), period1, period2));
    }

    let ranges = price_history_ranges();
    let cfg = ranges.get(range_id).cloned().ok_or_else(|| ApiError::bad_request("range must be one of 1d,7d,1m,6m,1y,3y or provide from/to"))?;
    let now = crate::utils::timex::now_utc();
    let (p1, p2) = if range_id == "1d" {
        preset_1d(now)
    } else {
        let days = match range_id {
            "7d" => 7,
            "1m" => 30,
            "6m" => 180,
            "1y" => 365,
            "3y" => 1095,
            _ => 1,
        };
        ((now - chrono::Duration::days(days)).timestamp(), now.timestamp())
    };
    Ok((cfg, range_id.to_string(), p1, p2))
}

/// The 1d (intraday) range uses Friday's session on weekends.
fn preset_1d(now: chrono::DateTime<chrono::Utc>) -> (i64, i64) {
    let wd = now.weekday();
    if wd == chrono::Weekday::Sat || wd == chrono::Weekday::Sun {
        let days_back = if wd == chrono::Weekday::Sun { 2 } else { 1 };
        let friday = (now - chrono::Duration::days(days_back)).date_naive();
        let start = friday.and_hms_opt(0, 0, 0).unwrap().and_utc();
        return (start.timestamp(), (start + chrono::Duration::days(1)).timestamp());
    }
    ((now - chrono::Duration::days(1)).timestamp(), now.timestamp())
}

/// Aggregate points to weekly/monthly averages (Go's `aggregatePricePoints`).
#[allow(clippy::cast_precision_loss)]
fn aggregate_price_points(points: &[PricePoint], agg: &str) -> Vec<PricePoint> {
    if points.is_empty() || agg.is_empty() {
        return points.to_vec();
    }
    let mut buckets: Vec<(String, f64, usize, i64)> = Vec::new(); // key, sum, n, lastT
    for p in points {
        let t = chrono::DateTime::from_timestamp(p.t, 0).unwrap_or_default().naive_utc();
        let key = if agg == "week" {
            format!("{}-W{:02}", t.iso_week().year(), t.iso_week().week())
        } else {
            t.format("%Y-%m").to_string()
        };
        match buckets.iter_mut().find(|(k, _, _, _)| *k == key) {
            Some(b) => {
                b.1 += p.close;
                b.2 += 1;
                b.3 = p.t;
            }
            None => buckets.push((key, p.close, 1, p.t)),
        }
    }
    buckets.into_iter().map(|(_, sum, n, last_t)| PricePoint { t: last_t, close: sum / n as f64 }).collect()
}



/// Parse an `occurredAt` value: RFC3339, date, or `{seconds,nanos}` → Go format.
pub fn occurred_at(v: &serde_json::Value) -> Result<String> {
    match v {
        serde_json::Value::Null => Err(ApiError::bad_request("occurredAt is required")),
        serde_json::Value::Object(m) => {
            let secs = m.get("seconds").and_then(serde_json::Value::as_i64).unwrap_or(0);
            let nanos = m.get("nanos").and_then(serde_json::Value::as_i64).unwrap_or(0);
            let nanos = u32::try_from(nanos).unwrap_or(0);
            let dt = chrono::DateTime::from_timestamp(secs, nanos).map(|d| d.with_timezone(&chrono::Utc)).ok_or_else(|| ApiError::bad_request("invalid date"))?;
            Ok(go_ts(dt))
        }
        serde_json::Value::String(s) => {
            if let Ok(dt) = chrono::DateTime::parse_from_rfc3339(s) {
                return Ok(go_ts(dt.with_timezone(&chrono::Utc)));
            }
            if let Ok(d) = chrono::NaiveDate::parse_from_str(s, "%Y-%m-%d") {
                return Ok(go_ts(d.and_hms_opt(0, 0, 0).unwrap().and_utc()));
            }
            Err(ApiError::bad_request(format!("invalid date {s:?}")))
        }
        _ => Err(ApiError::bad_request("invalid date")),
    }
}
