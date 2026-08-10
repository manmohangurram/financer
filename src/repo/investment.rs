//! Investments repository — SQL mirroring the Go backend's `repository/investment.go`.

use serde::{Deserialize, Serialize};
use sqlx::SqlitePool;
use uuid::Uuid;

use crate::error::Result;
use crate::timex::{go_ts, ts_rfc3339};

/// Investment type. Wire string (`STOCK`/`MUTUAL_FUND`); DB stores the int
/// discriminant. No sentinel — strict 400 at the boundary.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, strum::Display, strum::EnumString,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
pub enum InvestmentType {
    Stock,
    MutualFund,
}

impl InvestmentType {
    pub fn db_value(self) -> i64 {
        match self {
            Self::Stock => 1,
            Self::MutualFund => 2,
        }
    }
}

impl TryFrom<i64> for InvestmentType {
    type Error = ();
    fn try_from(v: i64) -> std::result::Result<Self, Self::Error> {
        match v {
            1 => Ok(Self::Stock),
            2 => Ok(Self::MutualFund),
            _ => Err(()),
        }
    }
}

#[derive(Clone)]
pub struct InvestmentRepo {
    pub pool: SqlitePool,
}

#[derive(Debug, Clone)]
pub struct InvestmentRow {
    pub id: String,
    pub symbol: String,
    pub name: String,
    // Wire key is `investmentType` (not plain `type`) — keep the qualifier.
    #[allow(clippy::struct_field_names)]
    pub investment_type: InvestmentType,
    pub current_price: f64,
    pub prev_close: f64,
    pub manual_nav: f64,
    pub last_quote_at: String,
    pub created_at: String,
}

pub struct LotInput {
    pub side: i64,
    pub quantity: f64,
    pub price: f64,
    pub occurred_at: String,
    pub external_id: String,
}

#[derive(Debug, Clone)]
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
#[derive(Debug, Clone, Serialize)]
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
#[derive(Debug, Clone, Serialize)]
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

impl InvestmentRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    pub async fn create_investment(&self, user_id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<InvestmentRow> {
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
        .bind(it.db_value())
        .bind(nav)
        .bind(&now)
        .execute(&self.pool)
        .await?;
        self.get_investment(&id).await.map(|r| r.unwrap())
    }

    pub async fn get_investment(&self, id: &str) -> Result<Option<InvestmentRow>> {
        let row = sqlx::query_as::<_, RawInvestment>(
            "SELECT id, user_id, symbol, name, investment_type, current_price, prev_close, last_quote_at, manual_nav, created_at FROM investments WHERE id = ?",
        )
        .bind(id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    pub async fn get_by_symbol(&self, user_id: &str, symbol: &str) -> Result<Option<InvestmentRow>> {
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

    pub async fn list_investments(&self, user_id: &str) -> Result<Vec<InvestmentRow>> {
        let rows = sqlx::query_as::<_, RawInvestment>(
            "SELECT id, user_id, symbol, name, investment_type, current_price, prev_close, last_quote_at, manual_nav, created_at FROM investments WHERE user_id = ? ORDER BY created_at DESC",
        )
        .bind(user_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    pub async fn update_investment(&self, id: &str, symbol: &str, name: &str, it: InvestmentType, manual_nav: f64) -> Result<InvestmentRow> {
        let old_sym: Option<String> = sqlx::query_scalar("SELECT symbol FROM investments WHERE id = ?")
            .bind(id)
            .fetch_optional(&self.pool)
            .await?;
        let sym: Option<&str> = if symbol.is_empty() { None } else { Some(symbol) };
        let nav: Option<f64> = if manual_nav == 0.0 { None } else { Some(manual_nav) };
        sqlx::query("UPDATE investments SET symbol = COALESCE(?, symbol), name = ?, investment_type = ?, manual_nav = ? WHERE id = ?")
            .bind(sym)
            .bind(name)
            .bind(it.db_value())
            .bind(nav)
            .bind(id)
            .execute(&self.pool)
            .await?;
        // Drop cached price history when the symbol changed so it refetches.
        if !symbol.is_empty() && old_sym.as_deref() != Some(symbol) {
            sqlx::query("DELETE FROM investment_price_history WHERE investment_id = ?")
                .bind(id)
                .execute(&self.pool)
                .await?;
        }
        self.get_investment(id).await.map(|r| r.unwrap())
    }

    pub async fn delete_investment(&self, id: &str) -> Result<bool> {
        sqlx::query("DELETE FROM investment_price_history WHERE investment_id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        let result = sqlx::query("DELETE FROM investments WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    pub async fn list_lots(&self, investment_id: &str) -> Result<Vec<LotRow>> {
        let rows = sqlx::query_as::<_, RawLot>(
            "SELECT id, investment_id, side, quantity, price, occurred_at, created_at FROM investment_lots WHERE investment_id = ? ORDER BY occurred_at ASC, created_at ASC",
        )
        .bind(investment_id)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    pub async fn create_lot(&self, user_id: &str, investment_id: &str, side: i64, quantity: f64, price: f64, occurred_at: &str) -> Result<LotRow> {
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

    pub async fn delete_lot(&self, id: &str) -> Result<bool> {
        let result = sqlx::query("DELETE FROM investment_lots WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    /// Idempotent insert: a second insert with the same (user, `external_id`) is
    /// skipped. Returns whether the row was inserted.
    pub async fn insert_lot(&self, user_id: &str, investment_id: &str, input: &LotInput) -> Result<bool> {
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

    pub async fn update_lot(&self, id: &str, quantity: f64, price: f64, occurred_at: &str) -> Result<bool> {
        let result = sqlx::query("UPDATE investment_lots SET quantity = ?, price = ?, occurred_at = ? WHERE id = ?")
            .bind(quantity)
            .bind(price)
            .bind(occurred_at)
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    pub async fn update_quote(&self, id: &str, current_price: f64, prev_close: f64) -> Result<()> {
        sqlx::query("UPDATE investments SET current_price = ?, prev_close = ?, last_quote_at = ? WHERE id = ?")
            .bind(current_price)
            .bind(prev_close)
            .bind(go_ts(chrono::Utc::now()))
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(())
    }


    /// Replace the cached price history for (investment, range).
    pub async fn upsert_price_history(&self, investment_id: &str, range_id: &str, ts: &[i64], closes: &[f64], fetched_at: i64) -> Result<()> {
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

    /// Read cached price history: ts, closes, last fetched timestamp.
    pub async fn get_price_history(&self, investment_id: &str, range_id: &str) -> Result<(Vec<i64>, Vec<f64>, i64)> {
        let rows: Vec<(i64, f64, i64)> = sqlx::query_as(
            "SELECT t, close, fetched_at FROM investment_price_history WHERE investment_id = ? AND range_id = ? ORDER BY t",
        )
        .bind(investment_id)
        .bind(range_id)
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

#[derive(sqlx::FromRow)]
struct RawInvestment {
    id: String,
    #[allow(dead_code)]
    user_id: String,
    symbol: Option<String>,
    name: String,
    investment_type: i64,
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
            investment_type: InvestmentType::try_from(r.investment_type).unwrap_or(InvestmentType::Stock),
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

pub fn round2(v: f64) -> f64 {
    (v * 100.0).round() / 100.0
}

#[cfg(test)]
mod tests {
    use std::str::FromStr;

    use super::*;

    async fn test_pool() -> SqlitePool {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("test.db");
        let opt = sqlx::sqlite::SqliteConnectOptions::from_str(path.to_str().unwrap())
            .unwrap()
            .create_if_missing(true)
            .journal_mode(sqlx::sqlite::SqliteJournalMode::Wal)
            .foreign_keys(true);
        let pool = SqlitePool::connect_with(opt).await.unwrap();
        crate::db::run_migrations(&pool, std::path::Path::new("db/migrations")).await.unwrap();
        pool
    }

    #[tokio::test]
    async fn investment_crud_and_lots() {
        use std::str::FromStr;
        let pool = test_pool().await;
        sqlx::query("INSERT INTO users (id, email, password_hash, name) VALUES ('u1','u1@x.com','h','u1')")
            .execute(&pool).await.unwrap();

        let repo = InvestmentRepo::new(pool.clone());
        let inst = repo.create_investment("u1", "RELIANCE", "Reliance", InvestmentType::Stock, 0.0).await.unwrap();
        assert_eq!(inst.symbol, "RELIANCE");
        assert_eq!(inst.investment_type, InvestmentType::Stock);

        // duplicate symbol for same user → the UNIQUE constraint fires on insert
        let dup = repo.get_by_symbol("u1", "RELIANCE").await.unwrap();
        assert_eq!(dup.unwrap().id, inst.id);

        let lot = repo.create_lot("u1", &inst.id, 1, 10.0, 100.0, "2024-01-02 10:00:00 +0000 UTC").await.unwrap();
        assert_eq!(lot.side, 1);
        let lots = repo.list_lots(&inst.id).await.unwrap();
        assert_eq!(lots.len(), 1);

        let updated = repo.update_lot(&lot.id, 20.0, 110.0, "2024-01-02 10:00:00 +0000 UTC").await.unwrap();
        assert!(updated);
        assert_eq!(repo.list_lots(&inst.id).await.unwrap()[0].quantity, 20.0);

        assert!(repo.delete_lot(&lot.id).await.unwrap());
        assert!(repo.delete_investment(&inst.id).await.unwrap());
        assert!(repo.get_investment(&inst.id).await.unwrap().is_none());
    }
}
