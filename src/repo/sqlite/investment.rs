//! `SQLite` investments repository — CRUD + lots + price history, implementing
//! `crate::repo::traits::InvestmentRepo` over an `sqlx::SqlitePool`.

use async_trait::async_trait;
use sqlx::SqlitePool;
use std::str::FromStr;
use uuid::Uuid;

use crate::error::Result;
use crate::repo::traits::investment::{InvestmentRow, InvestmentType, LotInput, LotRow};
use crate::timex::go_ts;

pub struct SqliteInvestmentRepo {
    pool: SqlitePool,
}

impl SqliteInvestmentRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    async fn create_investment_inner(&self, user_id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<InvestmentRow> {
        let id = Uuid::new_v4().to_string();
        let now = go_ts(chrono::Utc::now());
        let sym: Option<&str> = if symbol.is_empty() { None } else { Some(symbol) };
        let nav: Option<f64> = if manual_nav == 0.0 { None } else { Some(manual_nav) };
        sqlx::query(
            "INSERT INTO investments (id, user_id, symbol, name, investment_type, current_price, prev_close, last_quote_at, manual_nav, created_at)
             VALUES (?, ?, ?, ?, ?, NULL, NULL, NULL, ?, ?)",
        )
        .bind(&id)
        .bind(user_id)
        .bind(sym)
        .bind(name)
        .bind(it.to_string())
        .bind(nav)
        .bind(&now)
        .execute(&self.pool)
        .await?;
        Ok(self.get_investment_inner(user_id, &id).await?.unwrap())
    }

    async fn get_investment_inner(&self, user_id: &str, id: &str) -> Result<Option<InvestmentRow>> {
        let row = sqlx::query_as::<_, RawInvestment>(
            "SELECT id, user_id, symbol, name, investment_type, current_price, prev_close, last_quote_at, manual_nav, created_at FROM investments WHERE id = ? AND user_id = ?",
        )
        .bind(id)
        .bind(user_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    async fn get_by_symbol_inner(&self, user_id: &str, symbol: &str) -> Result<Option<InvestmentRow>> {
        if symbol.is_empty() {
            return Ok(None);
        }
        let row = sqlx::query_as::<_, RawInvestment>(
            "SELECT id, user_id, symbol, name, investment_type, current_price, prev_close, last_quote_at, manual_nav, created_at FROM investments WHERE user_id = ? AND symbol = ?",
        )
        .bind(user_id)
        .bind(symbol)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    async fn list_investments_inner(&self, user_id: &str) -> Result<Vec<InvestmentRow>> {
        let rows = sqlx::query_as::<_, RawInvestment>(
            "SELECT id, user_id, symbol, name, investment_type, current_price, prev_close, last_quote_at, manual_nav, created_at FROM investments WHERE user_id = ? ORDER BY created_at DESC",
        )
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    async fn update_investment_inner(&self, user_id: &str, id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<InvestmentRow> {
        let old_sym: Option<String> = sqlx::query_scalar("SELECT symbol FROM investments WHERE id = ? AND user_id = ?")
            .bind(id)
            .bind(user_id)
            .fetch_optional(&self.pool)
            .await?;
        let sym: Option<&str> = if symbol.is_empty() { None } else { Some(symbol) };
        let nav: Option<f64> = if manual_nav == 0.0 { None } else { Some(manual_nav) };
        sqlx::query("UPDATE investments SET symbol = COALESCE(?, symbol), name = ?, investment_type = ?, manual_nav = ? WHERE id = ? AND user_id = ?")
            .bind(sym)
            .bind(name)
            .bind(it.to_string())
            .bind(nav)
            .bind(id)
            .bind(user_id)
            .execute(&self.pool)
            .await?;
        if !symbol.is_empty() && old_sym.as_deref() != Some(symbol) {
            sqlx::query("DELETE FROM investment_price_history WHERE investment_id = ?")
                .bind(id)
                .execute(&self.pool)
                .await?;
        }
        Ok(self.get_investment_inner(user_id, id).await?.unwrap())
    }

    async fn delete_investment_inner(&self, user_id: &str, id: &str) -> Result<bool> {
        sqlx::query("DELETE FROM investment_price_history WHERE investment_id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        let result = sqlx::query("DELETE FROM investments WHERE id = ? AND user_id = ?")
            .bind(id)
            .bind(user_id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    async fn list_lots_inner(&self, user_id: &str, investment_id: &str) -> Result<Vec<LotRow>> {
        let rows = sqlx::query_as::<_, RawLot>(
            "SELECT id, investment_id, side, quantity, price, occurred_at, created_at FROM investment_lots WHERE investment_id = ? AND user_id = ? ORDER BY occurred_at ASC, created_at ASC",
        )
        .bind(investment_id)
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    async fn list_lots_by_user_inner(&self, user_id: &str) -> Result<Vec<LotRow>> {
        let rows = sqlx::query_as::<_, RawLot>(
            "SELECT id, investment_id, side, quantity, price, occurred_at, created_at FROM investment_lots WHERE user_id = ? ORDER BY occurred_at ASC, created_at ASC",
        )
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    async fn create_lot_inner(&self, user_id: &str, investment_id: &str, side: i64, quantity: f64, price: f64, occurred_at: &str) -> Result<LotRow> {
        let id = Uuid::new_v4().to_string();
        let now = go_ts(chrono::Utc::now());
        sqlx::query(
            "INSERT INTO investment_lots (id, user_id, investment_id, side, quantity, price, occurred_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
        )
        .bind(&id)
        .bind(user_id)
        .bind(investment_id)
        .bind(side)
        .bind(quantity)
        .bind(price)
        .bind(occurred_at)
        .bind(&now)
        .execute(&self.pool)
        .await?;
        Ok(LotRow { id, investment_id: investment_id.to_string(), side, quantity, price, occurred_at: occurred_at.to_string(), created_at: now })
    }

    async fn delete_lot_inner(&self, user_id: &str, id: &str) -> Result<bool> {
        let result = sqlx::query("DELETE FROM investment_lots WHERE id = ? AND user_id = ?")
            .bind(id)
            .bind(user_id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    async fn insert_lot_inner(&self, user_id: &str, investment_id: &str, input: &LotInput) -> Result<bool> {
        let id = Uuid::new_v4().to_string();
        let now = go_ts(chrono::Utc::now());
        let ext: Option<&str> = if input.external_id.is_empty() { None } else { Some(&input.external_id) };
        let result = sqlx::query(
            "INSERT OR IGNORE INTO investment_lots (id, user_id, investment_id, side, quantity, price, occurred_at, created_at, external_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
        )
        .bind(&id)
        .bind(user_id)
        .bind(investment_id)
        .bind(input.side)
        .bind(input.quantity)
        .bind(input.price)
        .bind(&input.occurred_at)
        .bind(&now)
        .bind(ext)
        .execute(&self.pool)
        .await?;
        Ok(result.rows_affected() > 0)
    }

    async fn update_lot_inner(&self, user_id: &str, id: &str, quantity: f64, price: f64, occurred_at: &str) -> Result<bool> {
        let result = sqlx::query("UPDATE investment_lots SET quantity = ?, price = ?, occurred_at = ? WHERE id = ? AND user_id = ?")
            .bind(quantity)
            .bind(price)
            .bind(occurred_at)
            .bind(id)
            .bind(user_id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    async fn update_quote_inner(&self, id: &str, current_price: f64, prev_close: f64) -> Result<()> {
        sqlx::query("UPDATE investments SET current_price = ?, prev_close = ?, last_quote_at = ? WHERE id = ?")
            .bind(current_price)
            .bind(prev_close)
            .bind(go_ts(chrono::Utc::now()))
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    async fn upsert_price_history_inner(&self, investment_id: &str, range_id: &str, ts: &[i64], closes: &[f64], fetched_at: i64) -> Result<()> {
        let mut tx = self.pool.begin().await?;
        sqlx::query("DELETE FROM investment_price_history WHERE investment_id = ? AND range_id = ?")
            .bind(investment_id)
            .bind(range_id)
            .execute(&mut *tx)
            .await?;
        for (i, t) in ts.iter().enumerate() {
            sqlx::query("INSERT INTO investment_price_history (investment_id, range_id, t, close, fetched_at) VALUES (?, ?, ?, ?, ?)")
                .bind(investment_id)
                .bind(range_id)
                .bind(t)
                .bind(closes.get(i).copied().unwrap_or(0.0))
                .bind(fetched_at)
                .execute(&mut *tx)
                .await?;
        }
        tx.commit().await?;
        Ok(())
    }

    async fn get_price_history_inner(&self, user_id: &str, investment_id: &str, range_id: &str) -> Result<(Vec<i64>, Vec<f64>, i64)> {
        let rows: Vec<(i64, f64, i64)> = sqlx::query_as(
            "SELECT t, close, fetched_at FROM investment_price_history
             WHERE investment_id = ? AND range_id = ? AND investment_id IN (SELECT id FROM investments WHERE user_id = ?)
             ORDER BY t",
        )
        .bind(investment_id)
        .bind(range_id)
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        let mut ts = Vec::new();
        let mut closes = Vec::new();
        let mut fetched = 0i64;
        for (t, c, f) in rows {
            ts.push(t);
            closes.push(c);
            fetched = f;
        }
        Ok((ts, closes, fetched))
    }
}

#[async_trait]
impl crate::repo::traits::InvestmentRepo for SqliteInvestmentRepo {
    async fn create_investment(&self, user_id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<InvestmentRow> {
        self.create_investment_inner(user_id, symbol, name, it, manual_nav).await
    }
    async fn get_investment(&self, user_id: &str, id: &str) -> Result<Option<InvestmentRow>> {
        self.get_investment_inner(user_id, id).await
    }
    async fn get_by_symbol(&self, user_id: &str, symbol: &str) -> Result<Option<InvestmentRow>> {
        self.get_by_symbol_inner(user_id, symbol).await
    }
    async fn list_investments(&self, user_id: &str) -> Result<Vec<InvestmentRow>> {
        self.list_investments_inner(user_id).await
    }
    async fn update_investment(&self, user_id: &str, id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<InvestmentRow> {
        self.update_investment_inner(user_id, id, symbol, name, it, manual_nav).await
    }
    async fn delete_investment(&self, user_id: &str, id: &str) -> Result<bool> {
        self.delete_investment_inner(user_id, id).await
    }
    async fn list_lots(&self, user_id: &str, investment_id: &str) -> Result<Vec<LotRow>> {
        self.list_lots_inner(user_id, investment_id).await
    }
    async fn list_lots_by_user(&self, user_id: &str) -> Result<Vec<LotRow>> {
        self.list_lots_by_user_inner(user_id).await
    }
    async fn create_lot(&self, user_id: &str, investment_id: &str, side: i64, quantity: f64, price: f64, occurred_at: &str) -> Result<LotRow> {
        self.create_lot_inner(user_id, investment_id, side, quantity, price, occurred_at).await
    }
    async fn delete_lot(&self, user_id: &str, id: &str) -> Result<bool> {
        self.delete_lot_inner(user_id, id).await
    }
    async fn insert_lot(&self, user_id: &str, investment_id: &str, input: &LotInput) -> Result<bool> {
        self.insert_lot_inner(user_id, investment_id, input).await
    }
    async fn update_lot(&self, user_id: &str, id: &str, quantity: f64, price: f64, occurred_at: &str) -> Result<bool> {
        self.update_lot_inner(user_id, id, quantity, price, occurred_at).await
    }
    async fn update_quote(&self, id: &str, current_price: f64, prev_close: f64) -> Result<()> {
        self.update_quote_inner(id, current_price, prev_close).await
    }
    async fn upsert_price_history(&self, investment_id: &str, range_id: &str, ts: &[i64], closes: &[f64], fetched_at: i64) -> Result<()> {
        self.upsert_price_history_inner(investment_id, range_id, ts, closes, fetched_at).await
    }
    async fn get_price_history(&self, user_id: &str, investment_id: &str, range_id: &str) -> Result<(Vec<i64>, Vec<f64>, i64)> {
        self.get_price_history_inner(user_id, investment_id, range_id).await
    }
}

#[derive(sqlx::FromRow)]
struct RawInvestment {
    id: String,
    #[allow(dead_code)]
    user_id: String,
    symbol: Option<String>,
    name: String,
    investment_type: String,
    current_price: Option<f64>,
    prev_close: Option<f64>,
    last_quote_at: Option<String>,
    manual_nav: Option<f64>,
    created_at: String,
}

impl From<RawInvestment> for InvestmentRow {
    fn from(r: RawInvestment) -> Self {
        Self {
            id: r.id,
            symbol: r.symbol.unwrap_or_default(),
            name: r.name,
            investment_type: InvestmentType::from_str(&r.investment_type).unwrap_or(InvestmentType::Stock),
            current_price: r.current_price.unwrap_or(0.0),
            prev_close: r.prev_close.unwrap_or(0.0),
            manual_nav: r.manual_nav.unwrap_or(0.0),
            last_quote_at: r.last_quote_at.unwrap_or_default(),
            created_at: r.created_at,
        }
    }
}

#[derive(sqlx::FromRow)]
struct RawLot {
    id: String,
    investment_id: String,
    side: i64,
    quantity: f64,
    price: f64,
    occurred_at: String,
    created_at: String,
}

impl From<RawLot> for LotRow {
    fn from(r: RawLot) -> Self {
        Self {
            id: r.id,
            investment_id: r.investment_id,
            side: r.side,
            quantity: r.quantity,
            price: r.price,
            occurred_at: r.occurred_at,
            created_at: r.created_at,
        }
    }
}
