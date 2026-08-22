//! Investments repository — CRUD + lots + price history on `SurrealDB`,
//! mirroring the Go backend's `repository/investment.go`.

use surrealdb::Connection;
use utoipa::ToSchema;

use crate::error::Result;
use crate::repo::surreal::{rid, take_json, DbClient, RepoConn};
use crate::timex::{go_ts, round2, ts_rfc3339};

/// Investment type. Wire string (`STOCK`/`MUTUAL_FUND`), stored as-is.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize, strum::Display, strum::EnumString, ToSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum InvestmentType {
    Stock,
    MutualFund,
}

#[derive(Clone)]
pub struct InvestmentRepo<C: Connection = DbClient> {
    db: RepoConn<C>,
}

impl<C: Connection> InvestmentRepo<C> {
    pub fn new(db: RepoConn<C>) -> Self {
        Self { db }
    }

    pub async fn create_investment(&self, user_id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<InvestmentRow> {
        let now = go_ts(chrono::Utc::now());
        let sym = if symbol.is_empty() { None } else { Some(symbol.to_string()) };
        let nav = if manual_nav == 0.0 { None } else { Some(manual_nav) };
        let mut res = self
            .db
            .query(
                "CREATE investment CONTENT {
                    user: $uid, symbol: $sym, name: $name, investmentType: $it,
                    currentPrice: 0.0, prevClose: 0.0, lastQuoteAt: NONE,
                    manualNav: $nav, createdAt: $created
                } RETURN meta::id(id) AS id, symbol, name, investmentType,
                    currentPrice, prevClose, manualNav, lastQuoteAt, createdAt",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("sym", sym))
            .bind(("name", name.to_string()))
            .bind(("it", it.to_string()))
            .bind(("nav", nav))
            .bind(("created", now.clone()))
            .await?
            .check()?;
        let mut row = take_json::<InvestmentRow>(&mut res, 0)?
            .into_iter()
            .next()
            .ok_or_else(|| crate::error::ApiError::internal("investment create returned no row"))?;
        row.created_at = now;
        Ok(row)
    }

    pub async fn get_investment(&self, user_id: &str, id: &str) -> Result<Option<InvestmentRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, symbol, name, investmentType,
                    currentPrice, prevClose, manualNav, lastQuoteAt, createdAt
                 FROM investment WHERE id = $rid AND user = $uid LIMIT 1",
            )
            .bind(("rid", rid("investment", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json(&mut res, 0)?.into_iter().next())
    }

    pub async fn get_by_symbol(&self, user_id: &str, symbol: &str) -> Result<Option<InvestmentRow>> {
        if symbol.is_empty() {
            return Ok(None);
        }
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, symbol, name, investmentType,
                    currentPrice, prevClose, manualNav, lastQuoteAt, createdAt
                 FROM investment WHERE user = $uid AND symbol = $sym LIMIT 1",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("sym", symbol.to_string()))
            .await?;
        Ok(take_json(&mut res, 0)?.into_iter().next())
    }

    pub async fn list_investments(&self, user_id: &str) -> Result<Vec<InvestmentRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, symbol, name, investmentType,
                    currentPrice, prevClose, manualNav, lastQuoteAt, createdAt
                 FROM investment WHERE user = $uid ORDER BY createdAt DESC",
            )
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json(&mut res, 0)?)
    }

    pub async fn update_investment(&self, user_id: &str, id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<InvestmentRow> {
        let old_sym: Option<String> = {
            let mut res = self
                .db
                .query("SELECT VALUE symbol FROM investment WHERE id = $rid AND user = $uid LIMIT 1")
                .bind(("rid", rid("investment", id)))
                .bind(("uid", rid("user", user_id)))
                .await?;
            take_json::<String>(&mut res, 0)?.into_iter().next()
        };
        let sym: Option<String> = if symbol.is_empty() { None } else { Some(symbol.to_string()) };
        let nav: Option<f64> = if manual_nav == 0.0 { None } else { Some(manual_nav) };
        self.db
            .query(
                "UPDATE $rid SET
                    symbol = IF $sym != NONE THEN $sym ELSE symbol END,
                    name = $name, investmentType = $it, manualNav = IF $nav != NONE THEN $nav ELSE manualNav END
                 WHERE user = $uid",
            )
            .bind(("rid", rid("investment", id)))
            .bind(("uid", rid("user", user_id)))
            .bind(("sym", sym))
            .bind(("name", name.to_string()))
            .bind(("it", it.to_string()))
            .bind(("nav", nav))
            .await?
            .check()?;
        // Drop cached price history when the symbol changed so it refetches.
        if !symbol.is_empty() && old_sym.as_deref() != Some(symbol) {
            self.db
                .query("DELETE investment_price_history WHERE investment = $rid")
                .bind(("rid", rid("investment", id)))
                .await?
                .check()?;
        }
        Ok(self.get_investment(user_id, id).await?.unwrap())
    }

    pub async fn delete_investment(&self, user_id: &str, id: &str) -> Result<bool> {
        self.db
            .query("DELETE investment_price_history WHERE investment = $rid")
            .bind(("rid", rid("investment", id)))
            .await?
            .check()?;
        let mut res = self
            .db
            .query("DELETE $rid WHERE user = $uid RETURN BEFORE")
            .bind(("rid", rid("investment", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(!take_json::<serde_json::Value>(&mut res, 0)?.is_empty())
    }

    pub async fn list_lots(&self, user_id: &str, investment_id: &str) -> Result<Vec<LotRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, meta::id(investment) AS investmentId, side,
                    quantity, price, occurredAt, createdAt
                 FROM investment_lot
                 WHERE investment = $rid AND user = $uid
                 ORDER BY occurredAt ASC, createdAt ASC",
            )
            .bind(("rid", rid("investment", investment_id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json(&mut res, 0)?)
    }

    pub async fn create_lot(&self, user_id: &str, investment_id: &str, side: i64, quantity: f64, price: f64, occurred_at: &str) -> Result<LotRow> {
        let now = go_ts(chrono::Utc::now());
        let mut res = self
            .db
            .query(
                "CREATE investment_lot CONTENT {
                    user: $uid, investment: $rid, side: $side, quantity: $quantity,
                    price: $price, occurredAt: $occ, createdAt: $created
                } RETURN meta::id(id) AS id, meta::id(investment) AS investmentId,
                    side, quantity, price, occurredAt, createdAt",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("rid", rid("investment", investment_id)))
            .bind(("side", side))
            .bind(("quantity", quantity))
            .bind(("price", price))
            .bind(("occ", occurred_at.to_string()))
            .bind(("created", now.clone()))
            .await?
            .check()?;
        let mut lot = take_json::<LotRow>(&mut res, 0)?
            .into_iter()
            .next()
            .ok_or_else(|| crate::error::ApiError::internal("lot create returned no row"))?;
        lot.created_at = now;
        Ok(lot)
    }

    pub async fn delete_lot(&self, user_id: &str, id: &str) -> Result<bool> {
        let mut res = self
            .db
            .query("DELETE $rid WHERE user = $uid RETURN BEFORE")
            .bind(("rid", rid("investment_lot", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(!take_json::<serde_json::Value>(&mut res, 0)?.is_empty())
    }

    /// Idempotent insert: a second insert with the same (user, `externalId`) is
    /// skipped. Returns whether the row was inserted.
    pub async fn insert_lot(&self, user_id: &str, investment_id: &str, input: &LotInput) -> Result<bool> {
        let now = go_ts(chrono::Utc::now());
        let ext: Option<String> = if input.external_id.is_empty() { None } else { Some(input.external_id.clone()) };
        // Dedup check (matches the old INSERT OR IGNORE on (user, externalId)).
        if let Some(e) = &ext {
            let mut res = self
                .db
                .query("SELECT VALUE meta::id(id) FROM investment_lot WHERE user = $uid AND externalId = $ext LIMIT 1")
                .bind(("uid", rid("user", user_id)))
                .bind(("ext", e.clone()))
                .await?;
            if !take_json::<String>(&mut res, 0)?.is_empty() {
                return Ok(false);
            }
        }
        self.db
            .query(
                "CREATE investment_lot CONTENT {
                    user: $uid, investment: $rid, side: $side, quantity: $quantity,
                    price: $price, occurredAt: $occ, createdAt: $created, externalId: $ext
                }",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("rid", rid("investment", investment_id)))
            .bind(("side", input.side))
            .bind(("quantity", input.quantity))
            .bind(("price", input.price))
            .bind(("occ", input.occurred_at.clone()))
            .bind(("created", now))
            .bind(("ext", ext))
            .await?
            .check()?;
        Ok(true)
    }

    pub async fn update_lot(&self, user_id: &str, id: &str, quantity: f64, price: f64, occurred_at: &str) -> Result<bool> {
        let mut res = self
            .db
            .query(
                "UPDATE $rid SET quantity = $q, price = $p, occurredAt = $occ
                 WHERE user = $uid RETURN meta::id(id) AS id",
            )
            .bind(("rid", rid("investment_lot", id)))
            .bind(("uid", rid("user", user_id)))
            .bind(("q", quantity))
            .bind(("p", price))
            .bind(("occ", occurred_at.to_string()))
            .await?;
        Ok(!take_json::<serde_json::Value>(&mut res, 0)?.is_empty())
    }

    pub async fn update_quote(&self, id: &str, current_price: f64, prev_close: f64) -> Result<()> {
        self.db
            .query(
                "UPDATE $rid SET currentPrice = $price, prevClose = $prev, lastQuoteAt = $at",
            )
            .bind(("rid", rid("investment", id)))
            .bind(("price", current_price))
            .bind(("prev", prev_close))
            .bind(("at", go_ts(chrono::Utc::now())))
            .await?
            .check()?;
        Ok(())
    }

    /// Replace the cached price history for (investment, range).
    pub async fn upsert_price_history(&self, investment_id: &str, range_id: &str, ts: &[i64], closes: &[f64], fetched_at: i64) -> Result<()> {
        let rid_inv = rid("investment", investment_id);
        self.db
            .query("DELETE investment_price_history WHERE investment = $rid AND rangeId = $range")
            .bind(("rid", rid_inv.clone()))
            .bind(("range", range_id.to_string()))
            .await?
            .check()?;
        for (i, t) in ts.iter().enumerate() {
            self.db
                .query(
                    "CREATE investment_price_history CONTENT {
                        investment: $rid, rangeId: $range, t: $t, close: $close, fetchedAt: $fetched
                    }",
                )
                .bind(("rid", rid_inv.clone()))
                .bind(("range", range_id.to_string()))
                .bind(("t", *t))
                .bind(("close", closes.get(i).copied().unwrap_or(0.0)))
                .bind(("fetched", fetched_at))
                .await?
                .check()?;
        }
        Ok(())
    }

    /// Read cached price history: ts, closes, last fetched timestamp.
    pub async fn get_price_history(&self, user_id: &str, investment_id: &str, range_id: &str) -> Result<(Vec<i64>, Vec<f64>, i64)> {
        let mut res = self
            .db
            .query(
                "SELECT t, close, fetchedAt FROM investment_price_history
                 WHERE investment = $rid AND rangeId = $range
                   AND investment IN (SELECT VALUE id FROM investment WHERE user = $uid)
                 ORDER BY t",
            )
            .bind(("rid", rid("investment", investment_id)))
            .bind(("range", range_id.to_string()))
            .bind(("uid", rid("user", user_id)))
            .await?;
        let rows: Vec<PriceHistoryRow> = take_json(&mut res, 0)?;
        let mut ts = Vec::new();
        let mut closes = Vec::new();
        let mut fetched = 0i64;
        for r in rows {
            ts.push(r.t);
            closes.push(r.close);
            fetched = r.fetched_at;
        }
        Ok((ts, closes, fetched))
    }
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

#[derive(serde::Deserialize)]
struct PriceHistoryRow {
    t: i64,
    close: f64,
    #[serde(rename = "fetchedAt")]
    fetched_at: i64,
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

/// Wire investment with position filled in (Go's `investmentWire` + `withPosition`).
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

#[cfg(test)]
mod tests {
    use std::sync::Arc;
    use surrealdb::Surreal;

    use super::*;
    use crate::surreal_db;

    async fn repo() -> (InvestmentRepo<surrealdb::engine::local::Db>, Arc<surrealdb::Surreal<surrealdb::engine::local::Db>>) {
        let db = Arc::new(surreal_db::connect_mem().await.unwrap());
        db.query("CREATE user CONTENT { id: 'u1', email: 'u1@x.com', passwordHash: 'h', name: 'u1' }")
            .await
            .unwrap()
            .check()
            .unwrap();
        (InvestmentRepo::new(db.clone()), db)
    }



    #[tokio::test]
    async fn investment_crud_and_lots() {
        let (repo, _db) = repo().await;
        let inst = repo.create_investment("u1", "RELIANCE", "Reliance", InvestmentType::Stock, 0.0).await.unwrap();
        assert_eq!(inst.symbol, "RELIANCE");
        assert_eq!(inst.investment_type, InvestmentType::Stock);

        let dup = repo.get_by_symbol("u1", "RELIANCE").await.unwrap();
        assert_eq!(dup.unwrap().id, inst.id);

        let lot = repo.create_lot("u1", &inst.id, 1, 10.0, 100.0, "2024-01-02 10:00:00 +0000 UTC").await.unwrap();
        assert_eq!(lot.side, 1);
        assert_eq!(lot.investment_id, inst.id);
        let lots = repo.list_lots("u1", &inst.id).await.unwrap();
        assert_eq!(lots.len(), 1);

        let updated = repo.update_lot("u1", &lot.id, 20.0, 110.0, "2024-01-02 10:00:00 +0000 UTC").await.unwrap();
        assert!(updated);
        assert_eq!(repo.list_lots("u1", &inst.id).await.unwrap()[0].quantity, 20.0);

        assert!(repo.delete_lot("u1", &lot.id).await.unwrap());
        assert!(repo.delete_investment("u1", &inst.id).await.unwrap());
        assert!(repo.get_investment("u1", &inst.id).await.unwrap().is_none());
    }
}
