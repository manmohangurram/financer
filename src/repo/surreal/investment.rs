//! Investments repository — CRUD + lots + price history on `SurrealDB`,
//! mirroring the Go backend's `repository/investment.go`.

use crate::error::Result;
use crate::repo::surreal::{DbClient, RepoConn, rid, take_json};
use surrealdb::Connection;

#[allow(unused_imports)]
pub use crate::repo::traits::investment::{
    Investment, InvestmentRow, InvestmentType, Lot, LotInput, LotRow, effective_price,
    investment_wire, lot_wire,
};

#[derive(Clone)]
pub struct InvestmentRepo<C: Connection = DbClient> {
    db: RepoConn<C>,
}

impl<C: Connection> InvestmentRepo<C> {
    pub fn new(db: RepoConn<C>) -> Self {
        Self { db }
    }

    pub async fn create_investment(
        &self,
        user_id: &str,
        symbol: &str,
        name: &str,
        it: InvestmentType,
        manual_nav: f64,
    ) -> Result<InvestmentRow> {
        let now = crate::utils::timex::now_go_ts();
        let sym = if symbol.is_empty() {
            None
        } else {
            Some(symbol.to_string())
        };
        let nav = if manual_nav == 0.0 {
            None
        } else {
            Some(manual_nav)
        };
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
            .bind(("name", name))
            .bind(("it", it.to_string()))
            .bind(("nav", nav))
            .bind(("created", now.as_str()))
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

    pub async fn get_by_symbol(
        &self,
        user_id: &str,
        symbol: &str,
    ) -> Result<Option<InvestmentRow>> {
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
            .bind(("sym", symbol))
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

    pub async fn update_investment(
        &self,
        user_id: &str,
        id: &str,
        symbol: &str,
        name: &str,
        it: InvestmentType,
        manual_nav: f64,
    ) -> Result<InvestmentRow> {
        let old_sym: Option<String> = {
            let mut res = self
                .db
                .query(
                    "SELECT VALUE symbol FROM investment WHERE id = $rid AND user = $uid LIMIT 1",
                )
                .bind(("rid", rid("investment", id)))
                .bind(("uid", rid("user", user_id)))
                .await?;
            take_json::<String>(&mut res, 0)?.into_iter().next()
        };
        let sym: Option<String> = if symbol.is_empty() {
            None
        } else {
            Some(symbol.to_string())
        };
        let nav: Option<f64> = if manual_nav == 0.0 {
            None
        } else {
            Some(manual_nav)
        };
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
            .bind(("name", name))
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

    pub async fn list_lots_by_user(&self, user_id: &str) -> Result<Vec<LotRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, meta::id(investment) AS investmentId, side,
                    quantity, price, occurredAt, createdAt
                 FROM investment_lot
                 WHERE user = $uid
                 ORDER BY occurredAt ASC, createdAt ASC",
            )
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json(&mut res, 0)?)
    }

    pub async fn create_lot(
        &self,
        user_id: &str,
        investment_id: &str,
        side: i64,
        quantity: f64,
        price: f64,
        occurred_at: &str,
    ) -> Result<LotRow> {
        let now = crate::utils::timex::now_go_ts();
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
            .bind(("occ", occurred_at))
            .bind(("created", now.as_str()))
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
    pub async fn insert_lot(
        &self,
        user_id: &str,
        investment_id: &str,
        input: &LotInput,
    ) -> Result<bool> {
        let now = crate::utils::timex::now_go_ts();
        let ext: Option<String> = if input.external_id.is_empty() {
            None
        } else {
            Some(input.external_id.clone())
        };
        // Dedup check (matches the old INSERT OR IGNORE on (user, externalId)).
        if let Some(e) = &ext {
            let mut res = self
                .db
                .query("SELECT VALUE meta::id(id) FROM investment_lot WHERE user = $uid AND externalId = $ext LIMIT 1")
                .bind(("uid", rid("user", user_id)))
                .bind(("ext", e.as_str()))
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
            .bind(("occ", input.occurred_at.as_str()))
            .bind(("created", now))
            .bind(("ext", ext))
            .await?
            .check()?;
        Ok(true)
    }

    pub async fn update_lot(
        &self,
        user_id: &str,
        id: &str,
        quantity: f64,
        price: f64,
        occurred_at: &str,
    ) -> Result<bool> {
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
            .bind(("occ", occurred_at))
            .await?;
        Ok(!take_json::<serde_json::Value>(&mut res, 0)?.is_empty())
    }

    pub async fn update_quote(&self, id: &str, current_price: f64, prev_close: f64) -> Result<()> {
        self.db
            .query("UPDATE $rid SET currentPrice = $price, prevClose = $prev, lastQuoteAt = $at")
            .bind(("rid", rid("investment", id)))
            .bind(("price", current_price))
            .bind(("prev", prev_close))
            .bind(("at", crate::utils::timex::now_go_ts()))
            .await?
            .check()?;
        Ok(())
    }

    /// Replace the cached price history for (investment, range).
    pub async fn upsert_price_history(
        &self,
        investment_id: &str,
        range_id: &str,
        ts: &[i64],
        closes: &[f64],
        fetched_at: i64,
    ) -> Result<()> {
        let rid_inv = rid("investment", investment_id);
        self.db
            .query("DELETE investment_price_history WHERE investment = $rid AND rangeId = $range")
            .bind(("rid", rid_inv.clone()))
            .bind(("range", range_id))
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
                .bind(("range", range_id))
                .bind(("t", *t))
                .bind(("close", closes.get(i).copied().unwrap_or(0.0)))
                .bind(("fetched", fetched_at))
                .await?
                .check()?;
        }
        Ok(())
    }

    /// Read cached price history: ts, closes, last fetched timestamp.
    pub async fn get_price_history(
        &self,
        user_id: &str,
        investment_id: &str,
        range_id: &str,
    ) -> Result<(Vec<i64>, Vec<f64>, i64)> {
        let mut res = self
            .db
            .query(
                "SELECT t, close, fetchedAt FROM investment_price_history
                 WHERE investment = $rid AND rangeId = $range
                   AND investment IN (SELECT VALUE id FROM investment WHERE user = $uid)
                 ORDER BY t",
            )
            .bind(("rid", rid("investment", investment_id)))
            .bind(("range", range_id))
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

#[derive(serde::Deserialize)]
struct PriceHistoryRow {
    t: i64,
    close: f64,
    #[serde(rename = "fetchedAt")]
    fetched_at: i64,
}

// Backend-agnostic `InvestmentRepo` trait impl (forwarders → inherent methods).
#[async_trait::async_trait]
impl crate::repo::traits::InvestmentRepo for InvestmentRepo<DbClient> {
    async fn create_investment(
        &self,
        user_id: &str,
        symbol: &str,
        name: &str,
        it: InvestmentType,
        manual_nav: f64,
    ) -> Result<InvestmentRow> {
        self.create_investment(user_id, symbol, name, it, manual_nav)
            .await
    }
    async fn get_investment(&self, user_id: &str, id: &str) -> Result<Option<InvestmentRow>> {
        self.get_investment(user_id, id).await
    }
    async fn get_by_symbol(&self, user_id: &str, symbol: &str) -> Result<Option<InvestmentRow>> {
        self.get_by_symbol(user_id, symbol).await
    }
    async fn list_investments(&self, user_id: &str) -> Result<Vec<InvestmentRow>> {
        self.list_investments(user_id).await
    }
    async fn update_investment(
        &self,
        user_id: &str,
        id: &str,
        symbol: &str,
        name: &str,
        it: InvestmentType,
        manual_nav: f64,
    ) -> Result<InvestmentRow> {
        self.update_investment(user_id, id, symbol, name, it, manual_nav)
            .await
    }
    async fn delete_investment(&self, user_id: &str, id: &str) -> Result<bool> {
        self.delete_investment(user_id, id).await
    }
    async fn list_lots(&self, user_id: &str, investment_id: &str) -> Result<Vec<LotRow>> {
        self.list_lots(user_id, investment_id).await
    }
    async fn list_lots_by_user(&self, user_id: &str) -> Result<Vec<LotRow>> {
        self.list_lots_by_user(user_id).await
    }
    async fn create_lot(
        &self,
        user_id: &str,
        investment_id: &str,
        side: i64,
        quantity: f64,
        price: f64,
        occurred_at: &str,
    ) -> Result<LotRow> {
        self.create_lot(user_id, investment_id, side, quantity, price, occurred_at)
            .await
    }
    async fn delete_lot(&self, user_id: &str, id: &str) -> Result<bool> {
        self.delete_lot(user_id, id).await
    }
    async fn insert_lot(
        &self,
        user_id: &str,
        investment_id: &str,
        input: &LotInput,
    ) -> Result<bool> {
        self.insert_lot(user_id, investment_id, input).await
    }
    async fn update_lot(
        &self,
        user_id: &str,
        id: &str,
        quantity: f64,
        price: f64,
        occurred_at: &str,
    ) -> Result<bool> {
        self.update_lot(user_id, id, quantity, price, occurred_at)
            .await
    }
    async fn update_quote(&self, id: &str, current_price: f64, prev_close: f64) -> Result<()> {
        self.update_quote(id, current_price, prev_close).await
    }
    async fn upsert_price_history(
        &self,
        investment_id: &str,
        range_id: &str,
        ts: &[i64],
        closes: &[f64],
        fetched_at: i64,
    ) -> Result<()> {
        self.upsert_price_history(investment_id, range_id, ts, closes, fetched_at)
            .await
    }
    async fn get_price_history(
        &self,
        user_id: &str,
        investment_id: &str,
        range_id: &str,
    ) -> Result<(Vec<i64>, Vec<f64>, i64)> {
        self.get_price_history(user_id, investment_id, range_id)
            .await
    }
}

#[cfg(test)]
mod tests {
    #![allow(clippy::float_cmp)]
    use std::sync::Arc;

    use super::*;
    use crate::surreal_db;

    async fn repo() -> (
        InvestmentRepo<surrealdb::engine::local::Db>,
        Arc<surrealdb::Surreal<surrealdb::engine::local::Db>>,
    ) {
        let db = Arc::new(surreal_db::connect_mem().await.unwrap());
        db.query(
            "CREATE user CONTENT { id: 'u1', email: 'u1@x.com', passwordHash: 'h', name: 'u1' }",
        )
        .await
        .unwrap()
        .check()
        .unwrap();
        (InvestmentRepo::new(db.clone()), db)
    }

    #[tokio::test]
    async fn investment_crud_and_lots() {
        let (repo, _db) = repo().await;
        let inst = repo
            .create_investment("u1", "RELIANCE", "Reliance", InvestmentType::Stock, 0.0)
            .await
            .unwrap();
        assert_eq!(inst.symbol, "RELIANCE");
        assert_eq!(inst.investment_type, InvestmentType::Stock);

        let dup = repo.get_by_symbol("u1", "RELIANCE").await.unwrap();
        assert_eq!(dup.unwrap().id, inst.id);

        let lot = repo
            .create_lot(
                "u1",
                &inst.id,
                1,
                10.0,
                100.0,
                "2024-01-02 10:00:00 +0000 UTC",
            )
            .await
            .unwrap();
        assert_eq!(lot.side, 1);
        assert_eq!(lot.investment_id, inst.id);
        let lots = repo.list_lots("u1", &inst.id).await.unwrap();
        assert_eq!(lots.len(), 1);

        let updated = repo
            .update_lot("u1", &lot.id, 20.0, 110.0, "2024-01-02 10:00:00 +0000 UTC")
            .await
            .unwrap();
        assert!(updated);
        assert_eq!(
            repo.list_lots("u1", &inst.id).await.unwrap()[0].quantity,
            20.0
        );

        assert!(repo.delete_lot("u1", &lot.id).await.unwrap());
        assert!(repo.delete_investment("u1", &inst.id).await.unwrap());
        assert!(repo.get_investment("u1", &inst.id).await.unwrap().is_none());
    }
}
