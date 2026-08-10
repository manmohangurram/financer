//! Transactions + transfer-link repository — SQL mirroring the Go backend's
//! `repository/transaction.go` and `repository/transfer.go`.

use base64::{engine::general_purpose::URL_SAFE as B64URL, Engine as _};
use serde::{Deserialize, Serialize};
use serde_json::json;
use sqlx::{Row, SqlitePool};
use std::fmt::Write as _;
use std::str::FromStr;
use uuid::Uuid;

use crate::error::Result;
use crate::timex::go_ts;

/// Transaction type. Single source of truth: serde emits the uppercase wire
/// value, sqlx stores the same string in the `type` TEXT column. No sentinel —
/// strict 400 at the boundary.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type, strum::Display, strum::EnumString,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[sqlx(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
pub enum TransactionType {
    Debit,
    Credit,
}

#[derive(Clone)]
pub struct TransactionRepo {
    pub pool: SqlitePool,
}

/// A transaction as stored/returned by the repository layer.
#[derive(Debug, Clone)]
pub struct Transaction {
    pub id: String,
    pub name: String,
    pub amount: f64,
    // Field name is `transaction_type` per code-standard, wire/DB key is `type`.
    #[allow(clippy::struct_field_names)]
    pub transaction_type: TransactionType,
    pub occurred_at: String,
    pub account_id: String,
    pub created_at: String,
    pub external_id: Option<String>,
    pub transfer_linked: bool,
}

/// One transaction to create: the txn plus its category links.
pub struct CreateTransactionInput {
    pub txn: Transaction,
    pub category_ids: Vec<String>,
}

/// One transaction to update: the txn plus its full category set.
pub struct UpdateTransactionInput {
    pub txn: Transaction,
    pub category_ids: Vec<String>,
}

/// Filters for listing transactions, mirroring Go's `TransactionListFilter`.
#[derive(Debug, Default, Clone)]
pub struct TransactionListFilter {
    pub account_id: String,
    pub category_ids: Vec<String>,
    pub transaction_type: Option<TransactionType>,
    pub date_from: String,
    pub date_to: String,
    pub min_amount: f64,
    pub max_amount: f64,
    pub names: Vec<String>,
    pub page_size: i64,
    pub page_token: String,
    pub sort_by: String, // "debit" | "credit"
    pub sort_dir: String, // "asc" | "desc"
    pub offset: i64,
}

pub struct ListTransactionResult {
    pub rows: Vec<ListRow>,
    pub next_page_token: String,
    pub total_count: i64,
}

/// A listed row: the transaction plus its transfer link id and category ids.
pub struct ListRow {
    pub txn: Transaction,
    pub link_id: String,
    pub category_ids: Vec<String>,
}

impl TransactionRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    /// Fetch a single transaction (used by transfer counterpart logic).
    pub async fn get_by_id_for_transfer(&self, user_id: &str, id: &str) -> Result<Option<(String, f64, String)>> {
        let row = sqlx::query_as::<_, (String, f64, String)>(
            "SELECT type, amount, account_id FROM transactions WHERE id = ? AND user_id = ?",
        )
        .bind(id)
        .bind(user_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row)
    }

    /// Fetch a full transaction row including its transfer-link state.
    pub async fn get_full(&self, user_id: &str, id: &str) -> Result<Option<Transaction>> {
        let row = sqlx::query_as::<_, RawTransaction>(
            "SELECT t.id, t.name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at, t.external_id
             FROM transactions t WHERE t.id = ? AND t.user_id = ?",
        )
        .bind(id)
        .bind(user_id)
        .fetch_optional(&self.pool)
        .await?;
        let mut txn: Option<Transaction> = row.map(Into::into);
        if let Some(t) = &mut txn {
            t.transfer_linked = self.is_transfer_linked(id).await?;
        }
        Ok(txn)
    }

    /// Find the unlinked transaction in `account_id` with the given type/amount
    /// closest in date to `around` (within ±3 days). Mirrors Go's query.
    pub async fn find_transfer_counterpart(
        &self,
        user_id: &str,
        account_id: &str,
        transaction_type: TransactionType,
        amount: f64,
        around: &str,
    ) -> Result<Option<Transaction>> {
        // Go passes `around.UTC().Format("2006-01-02 15:04:05")` (no zone
        // suffix) into datetime(); the stored column keeps the "+0000 UTC".
        let around_str = &around[..around.len().min(19)];
        let row = sqlx::query_as::<_, RawTransaction>(
            "SELECT t.id, t.name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at, NULL AS external_id
            FROM transactions t
            WHERE t.user_id = ? AND t.account_id = ? AND t.type = ? AND t.amount = ?
              AND t.occurred_at >= datetime(?, '-3 days') AND t.occurred_at <= datetime(?, '+3 days')
              AND NOT EXISTS (SELECT 1 FROM transfer_links l WHERE l.debit_transaction_id = t.id OR l.credit_transaction_id = t.id)
            ORDER BY ABS(CAST(strftime('%s', t.occurred_at) AS INTEGER) - CAST(strftime('%s', ?) AS INTEGER))
            LIMIT 1",
        )
        .bind(user_id)
        .bind(account_id)
        .bind(transaction_type)
        .bind(amount)
        .bind(around_str)
        .bind(around_str)
        .bind(around_str)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    /// Report whether txnID is already part of a transfer link.
    pub async fn is_transfer_linked(&self, txn_id: &str) -> Result<bool> {
        let one: Option<i64> = sqlx::query_scalar(
            "SELECT 1 FROM transfer_links l WHERE l.debit_transaction_id = ? OR l.credit_transaction_id = ? LIMIT 1",
        )
        .bind(txn_id)
        .bind(txn_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(one.is_some())
    }

    /// Insert transactions with `INSERT OR IGNORE`, reporting which ids were
    /// actually inserted (duplicate `external_id` rows are skipped). One tx.
    pub async fn create(&self, user_id: &str, inputs: &[CreateTransactionInput]) -> Result<CreateOutcome> {
        let mut inserted = std::collections::HashSet::new();
        let mut errors: Vec<String> = Vec::new();
        if inputs.is_empty() {
            return Ok(CreateOutcome { inserted, errors });
        }
        let mut tx = self.pool.begin().await?;
        for input in inputs {
            let t = &input.txn;
            let res = sqlx::query(
                "INSERT OR IGNORE INTO transactions (id, user_id, name, amount, type, occurred_at, account_id, created_at, external_id)
                 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
            )
            .bind(&t.id)
            .bind(user_id)
            .bind(&t.name)
            .bind(t.amount)
            .bind(t.transaction_type)
            .bind(&t.occurred_at)
            .bind(&t.account_id)
            .bind(&t.created_at)
            .bind(&t.external_id)
            .execute(&mut *tx)
            .await;
            match res {
                Ok(r) if r.rows_affected() > 0 => {
                    inserted.insert(t.id.clone());
                    for cat in &input.category_ids {
                        if let Err(e) = sqlx::query(
                            "INSERT OR IGNORE INTO transaction_categories (transaction_id, category_id) VALUES (?, ?)",
                        )
                        .bind(&t.id)
                        .bind(cat)
                        .execute(&mut *tx)
                        .await
                        {
                            errors.push(format!("failed to link category {cat} to {}", t.id));
                            tracing::error!("failed to link category {cat} to {}: {e}", t.id);
                        }
                    }
                }
                Ok(_) => {} // duplicate external_id — skip silently
                Err(e) => {
                    errors.push(format!("failed to create {}", t.id));
                    tracing::error!("failed to create transaction {}: {e}", t.id);
                }
            }
        }
        tx.commit().await?;
        Ok(CreateOutcome { inserted, errors })
    }

    /// Update transactions, replacing each one's category set. One tx.
    pub async fn update(&self, user_id: &str, inputs: &[UpdateTransactionInput]) -> Result<Vec<String>> {
        let mut errors: Vec<String> = Vec::new();
        if inputs.is_empty() {
            return Ok(errors);
        }
        let mut tx = self.pool.begin().await?;
        for input in inputs {
            let t = &input.txn;
            let res = sqlx::query(
                "UPDATE transactions SET
                    name = COALESCE(NULLIF(?, ''), name),
                    amount = CASE WHEN ? != 0 THEN ? ELSE amount END,
                    type = CASE WHEN ? != 0 THEN ? ELSE type END,
                    occurred_at = COALESCE(NULLIF(?, ''), occurred_at),
                    account_id = COALESCE(NULLIF(?, ''), account_id)
                 WHERE id = ? AND user_id = ?",
            )
            .bind(&t.name)
            .bind(t.amount)
            .bind(t.amount)
            .bind(t.transaction_type)
            .bind(t.transaction_type)
            .bind(&t.occurred_at)
            .bind(&t.account_id)
            .bind(&t.id)
            .bind(user_id)
            .execute(&mut *tx)
            .await;
            match res {
                Ok(r) if r.rows_affected() > 0 => {
                    if let Err(e) = sqlx::query("DELETE FROM transaction_categories WHERE transaction_id = ?")
                        .bind(&t.id)
                        .execute(&mut *tx)
                        .await
                    {
                        errors.push(format!("failed to clear categories for {}", t.id));
                        tracing::error!("failed to clear categories for {}: {e}", t.id);
                        continue;
                    }
                    for cat in &input.category_ids {
                        if let Err(e) = sqlx::query(
                            "INSERT OR IGNORE INTO transaction_categories (transaction_id, category_id) VALUES (?, ?)",
                        )
                        .bind(&t.id)
                        .bind(cat)
                        .execute(&mut *tx)
                        .await
                        {
                            errors.push(format!("failed to link category {cat} to {}", t.id));
                            tracing::error!("failed to link category {cat} to {}: {e}", t.id);
                        }
                    }
                }
                Ok(_) => errors.push(format!("transaction {} not found", t.id)),
                Err(e) => {
                    errors.push(format!("failed to update {}", t.id));
                    tracing::error!("failed to update transaction {}: {e}", t.id);
                }
            }
        }
        tx.commit().await?;
        Ok(errors)
    }

    /// Delete transactions by id. One tx.
    pub async fn delete(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        let mut errors: Vec<String> = Vec::new();
        if ids.is_empty() {
            return Ok(errors);
        }
        let mut tx = self.pool.begin().await?;
        for id in ids {
            let res = sqlx::query("DELETE FROM transactions WHERE id = ? AND user_id = ?")
                .bind(id)
                .bind(user_id)
                .execute(&mut *tx)
                .await;
            match res {
                Ok(r) if r.rows_affected() > 0 => {}
                Ok(_) => errors.push(format!("transaction {id} not found")),
                Err(e) => {
                    errors.push(format!("failed to delete {id}"));
                    tracing::error!("failed to delete transaction {id}: {e}");
                }
            }
        }
        tx.commit().await?;
        Ok(errors)
    }

    /// Fetch a batch of transactions by id (used to preload before update/delete).
    pub async fn get_by_id_batch(&self, user_id: &str, ids: &[String]) -> Result<Vec<Transaction>> {
        if ids.is_empty() {
            return Ok(Vec::new());
        }
        let mut query = String::from(
            "SELECT id, name, amount, type, occurred_at, account_id, created_at, external_id FROM transactions WHERE user_id = ? AND id IN (",
        );
        query.push_str("?, ".repeat(ids.len()).trim_end_matches(", "));
        query.push(')');
        let mut q = sqlx::query_as::<_, RawTransaction>(&query).bind(user_id);
        for id in ids {
            q = q.bind(id);
        }
        let rows = q.fetch_all(&self.pool).await?;
        Ok(rows.into_iter().map(Into::into).collect())
    }

    /// List transactions with filters, cursor/offset pagination, and sort.
    pub async fn list(&self, user_id: &str, f: &TransactionListFilter) -> Result<ListTransactionResult> {
        let (where_sql, args) = build_txn_where(f);

        let mut query = String::from(
            "SELECT t.id, t.name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at, t.external_id,
                    COALESCE(l.id, ''),
                    (SELECT GROUP_CONCAT(tc.category_id) FROM transaction_categories tc WHERE tc.transaction_id = t.id),
                    COUNT(*) OVER () AS total
                FROM transactions t
                LEFT JOIN transfer_links l ON l.debit_transaction_id = t.id OR l.credit_transaction_id = t.id ",
        );
        query.push_str(&where_sql);

        let mut bind_args: Vec<String> = Vec::with_capacity(args.len() + 1);
        bind_args.push(user_id.to_string());
        bind_args.extend(args);

        if f.sort_by.is_empty() {
            if !f.page_token.is_empty() {
                if let Some(cur) = decode_transaction_cursor(&f.page_token) {
                    query.push_str(" AND (t.occurred_at < ? OR (t.occurred_at = ? AND t.id < ?))");
                    bind_args.push(cur.occurred_at.clone());
                    bind_args.push(cur.occurred_at.clone());
                    bind_args.push(cur.id.clone());
                }
            }
            query.push_str(" ORDER BY t.occurred_at DESC, t.id DESC");
            if f.page_size > 0 {
                let _ = write!(query, " LIMIT {}", f.page_size + 1);
            }
        } else {
            // `type` column is TEXT; the sort picks a primary type ("debit" by
            // default) whose amounts sort first in the chosen direction.
            let primary = if f.sort_by == "credit" { "CREDIT" } else { "DEBIT" };
            let dir = if f.sort_dir == "asc" { "ASC" } else { "DESC" };
            let _ = write!(
                query,
                " ORDER BY CASE WHEN t.type = '{primary}' THEN 0 ELSE 1 END,
                    CASE WHEN t.type = '{primary}' THEN t.amount ELSE -t.amount END {dir}, t.id ASC"
            );
            if f.page_size > 0 {
                let _ = write!(query, " LIMIT {} OFFSET {}", f.page_size, f.offset);
            }
        }

        let mut q = sqlx::query_as::<_, RawListTxn>(&query);
        for a in &bind_args {
            q = q.bind(a);
        }
        let rows = q.fetch_all(&self.pool).await?;

        let mut items: Vec<ListRow> = Vec::new();
        let mut total = 0i64;
        for r in rows {
            total = r.3;
            let mut category_ids = Vec::new();
            if let Some(g) = &r.2 {
                category_ids = g.split(',').map(str::to_string).collect();
            }
            items.push(ListRow { txn: r.0.into(), link_id: r.1.unwrap_or_default(), category_ids });
        }

        let mut next_token = String::new();
        let page_size = usize::try_from(f.page_size).unwrap_or(0);
        if f.sort_by.is_empty() && page_size > 0 && items.len() > page_size {
            items.truncate(page_size);
            let last = items.last().unwrap();
            next_token = encode_transaction_cursor(&last.txn.occurred_at, &last.txn.id);
        }

        Ok(ListTransactionResult { rows: items, next_page_token: next_token, total_count: total })
    }
}

pub struct CreateOutcome {
    pub inserted: std::collections::HashSet<String>,
    pub errors: Vec<String>,
}

/// Build the `WHERE` clause (without the leading `user_id` arg) for a list query.
fn build_txn_where(f: &TransactionListFilter) -> (String, Vec<String>) {
    let mut query = String::from("WHERE t.user_id = ?");
    let mut args: Vec<String> = Vec::new();
    if !f.account_id.is_empty() {
        query.push_str(" AND t.account_id = ?");
        args.push(f.account_id.clone());
    }
    if !f.category_ids.is_empty() {
        let ph: Vec<String> = f.category_ids.iter().map(|_| "?".to_string()).collect();
        let _ = write!(
            query,
            " AND EXISTS (SELECT 1 FROM transaction_categories tc WHERE tc.transaction_id = t.id AND tc.category_id IN ({}))",
            ph.join(",")
        );
        for id in &f.category_ids {
            args.push(id.clone());
        }
    }
    if let Some(t) = f.transaction_type {
        query.push_str(" AND t.type = ?");
        args.push(t.to_string());
    }
    if !f.date_from.is_empty() {
        query.push_str(" AND t.occurred_at >= ?");
        args.push(f.date_from.clone());
    }
    if !f.date_to.is_empty() {
        query.push_str(" AND t.occurred_at < ?");
        // dateTo is inclusive end-of-day: add 24h
        if let Ok(d) = chrono::NaiveDate::parse_from_str(&f.date_to, "%Y-%m-%d") {
            let end = d.and_hms_opt(0, 0, 0).unwrap().and_utc() + chrono::Duration::days(1);
            args.push(crate::timex::go_ts(end));
        } else {
            args.push(f.date_to.clone());
        }
    }
    if f.min_amount > 0.0 {
        query.push_str(" AND t.amount >= ?");
        args.push(f.min_amount.to_string());
    }
    if f.max_amount > 0.0 {
        query.push_str(" AND t.amount <= ?");
        args.push(f.max_amount.to_string());
    }
    if !f.names.is_empty() {
        let clauses: Vec<String> = f
            .names
            .iter()
            .map(|_| "LOWER(t.name) LIKE '%' || LOWER(?) || '%' ESCAPE '\\'".to_string())
            .collect();
        let _ = write!(query, " AND ({})", clauses.join(" OR "));
        for n in &f.names {
            args.push(like_escape(n));
        }
    }
    (query, args)
}

/// The full row shape returned by the list query: txn, link id, category group.
struct RawListTxn(pub RawTransaction, pub Option<String>, pub Option<String>, pub i64);

impl<'r> sqlx::FromRow<'r, sqlx::sqlite::SqliteRow> for RawListTxn {
    fn from_row(row: &'r sqlx::sqlite::SqliteRow) -> std::result::Result<Self, sqlx::Error> {
        let txn = RawTransaction {
            id: row.try_get(0)?,
            name: row.try_get(1)?,
            amount: row.try_get(2)?,
            transaction_type: row.try_get(3)?,
            occurred_at: row.try_get(4)?,
            account_id: row.try_get(5)?,
            created_at: row.try_get(6)?,
            external_id: row.try_get(7)?,
        };
        let link: Option<String> = row.try_get(8)?;
        // GROUP_CONCAT yields NULL when no rows match; empty string otherwise.
        let cat_group: Option<String> = row.try_get(9)?;
        let total: i64 = row.try_get(10)?;
        Ok(RawListTxn(txn, link, cat_group, total))
    }
}

#[derive(sqlx::FromRow)]
struct RawTransaction {
    id: String,
    name: String,
    amount: f64,
    #[sqlx(rename = "type")]
    transaction_type: String,
    occurred_at: String,
    account_id: String,
    created_at: String,
    external_id: Option<String>,
}

impl From<RawTransaction> for Transaction {
    fn from(r: RawTransaction) -> Self {
        Self {
            id: r.id,
            name: r.name,
            amount: r.amount,
            transaction_type: TransactionType::from_str(&r.transaction_type).unwrap_or(TransactionType::Debit),
            occurred_at: r.occurred_at,
            account_id: r.account_id,
            created_at: r.created_at,
            external_id: r.external_id,
            transfer_linked: false,
        }
    }
}

// --- transfer links ---

/// Insert transfer links. Mirrors Go's `TransferRepository.Create`.
pub async fn create_links(
    pool: &SqlitePool,
    user_id: &str,
    links: &[(String, String)],
) -> Result<Vec<String>> {
    let mut errors: Vec<String> = Vec::new();
    if links.is_empty() {
        return Ok(errors);
    }
    let mut tx = pool.begin().await?;
    for (debit, credit) in links {
        if let Err(e) = sqlx::query(
            "INSERT INTO transfer_links (id, user_id, debit_transaction_id, credit_transaction_id, created_at)
             VALUES (?, ?, ?, ?, ?)",
        )
        .bind(Uuid::new_v4().to_string())
        .bind(user_id)
        .bind(debit)
        .bind(credit)
        .bind(go_ts(chrono::Utc::now()))
        .execute(&mut *tx)
        .await
        {
            errors.push(format!("failed to link transfer {debit}/{credit}"));
            tracing::error!("failed to create transfer link {debit}/{credit}: {e}");
        }
    }
    tx.commit().await?;
    Ok(errors)
}

/// Delete transfer links by id. Mirrors Go's `TransferRepository.Delete`.
pub async fn delete_links(pool: &SqlitePool, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
    let mut errors: Vec<String> = Vec::new();
    if ids.is_empty() {
        return Ok(errors);
    }
    let mut tx = pool.begin().await?;
    for id in ids {
        let res = sqlx::query("DELETE FROM transfer_links WHERE id = ? AND user_id = ?")
            .bind(id)
            .bind(user_id)
            .execute(&mut *tx)
            .await;
        match res {
            Ok(r) if r.rows_affected() > 0 => {}
            Ok(_) => errors.push(format!("transfer link {id} not found")),
            Err(e) => {
                errors.push(format!("failed to delete transfer link {id}"));
                tracing::error!("failed to delete transfer link {id}: {e}");
            }
        }
    }
    tx.commit().await?;
    Ok(errors)
}

/// Report whether txnID is already part of a transfer link (link-level check).
pub async fn is_transaction_linked(pool: &SqlitePool, txn_id: &str) -> Result<bool> {
    let one: Option<String> = sqlx::query_scalar(
        "SELECT id FROM transfer_links WHERE debit_transaction_id = ? OR credit_transaction_id = ? LIMIT 1",
    )
    .bind(txn_id)
    .bind(txn_id)
    .fetch_optional(pool)
    .await?;
    Ok(one.is_some())
}

// --- cursor helpers (mirror Go's base64 URL-encoded JSON cursor) ---

/// A decoded page cursor. `occurred_at` is kept in the stored Go-driver
/// format so it can be bound directly in the SQL comparison.
pub struct TransactionCursor {
    pub occurred_at: String,
    pub id: String,
}

/// Go encodes `json.Marshal(TransactionCursor{OccurredAt time.Time, ID})` (RFC3339
/// `OccurredAt`) then base64 URL. We round-trip through RFC3339 too.
pub fn encode_transaction_cursor(occurred_at: &str, id: &str) -> String {
    let payload = json!({ "OccurredAt": crate::timex::ts_rfc3339(occurred_at), "ID": id }).to_string();
    B64URL.encode(payload.as_bytes())
}

pub fn decode_transaction_cursor(token: &str) -> Option<TransactionCursor> {
    let raw = B64URL.decode(token.as_bytes()).ok()?;
    let v: serde_json::Value = serde_json::from_slice(&raw).ok()?;
    let rfc = v["OccurredAt"].as_str()?;
    // Convert RFC3339 back to the Go-driver stored format for SQL binding.
    let stored = chrono::DateTime::parse_from_rfc3339(rfc)
        .map_or_else(|_| rfc.to_string(), |dt| go_ts(dt.with_timezone(&chrono::Utc)));
    Some(TransactionCursor { occurred_at: stored, id: v["ID"].as_str()?.to_string() })
}

fn like_escape(s: &str) -> String {
    s.replace('\\', "\\\\").replace('%', "\\%").replace('_', "\\_")
}

#[cfg(test)]
mod tests {
    use std::str::FromStr;

    use super::*;

    #[test]
    fn rejects_unknown_type() {
        assert!(TransactionType::from_str("bogus").is_err());
        assert_eq!(TransactionType::from_str("DEBIT"), Ok(TransactionType::Debit));
        assert_eq!(TransactionType::from_str("CREDIT"), Ok(TransactionType::Credit));
    }

    async fn test_pool() -> SqlitePool {
        let dir = tempfile::tempdir().unwrap();
        // Keep the temp dir alive for the pool's lifetime (auto-delete on drop
        // would unlink the DB file before the lazy pool opens it).
        let path = dir.into_path().join("test.db");
        let opt = sqlx::sqlite::SqliteConnectOptions::from_str(path.to_str().unwrap())
            .unwrap()
            .create_if_missing(true)
            .journal_mode(sqlx::sqlite::SqliteJournalMode::Wal)
            .foreign_keys(true);
        let pool = SqlitePool::connect_with(opt).await.unwrap();
        crate::db::run_migrations(&pool, std::path::Path::new("db/migrations"))
            .await
            .unwrap();
        pool
    }

    async fn seed_user_and_account(pool: &SqlitePool, uid: &str, aid: &str) {
        sqlx::query("INSERT INTO users (id, email, password_hash, name) VALUES (?, ?, ?, ?)")
            .bind(uid)
            .bind(format!("{uid}@x.com"))
            .bind("hash")
            .bind(uid)
            .execute(pool)
            .await
            .unwrap();
        sqlx::query("INSERT INTO accounts (id, user_id, bank_name, type) VALUES (?, ?, ?, ?)")
            .bind(aid)
            .bind(uid)
            .bind("Bank")
            .bind(1i64)
            .execute(pool)
            .await
            .unwrap();
        for cid in ["c1", "c2"] {
            sqlx::query("INSERT INTO categories (id, user_id, name) VALUES (?, ?, ?)")
                .bind(cid)
                .bind(uid)
                .bind(cid)
                .execute(pool)
                .await
                .unwrap();
        }
    }

    fn txn(id: &str, name: &str, amount: f64, transaction_type: TransactionType, account: &str, ext: Option<&str>) -> Transaction {
        Transaction {
            id: id.to_string(),
            name: name.to_string(),
            amount,
            transaction_type,
            occurred_at: "2024-01-02 03:04:05 +0000 UTC".to_string(),
            account_id: account.to_string(),
            created_at: "2024-01-02 03:04:06 +0000 UTC".to_string(),
            external_id: ext.map(str::to_string),
            transfer_linked: false,
        }
    }

    #[tokio::test]
    async fn create_inserts_with_categories_and_dedups_external() {
        let pool = test_pool().await;
        seed_user_and_account(&pool, "u1", "a1").await;
        let repo = TransactionRepo::new(pool.clone());
        let input = CreateTransactionInput {
            txn: txn("t1", "Coffee", 5.5, TransactionType::Debit, "a1", Some("ext-1")),
            category_ids: vec!["c1".to_string(), "c2".to_string()],
        };
        let out = repo.create("u1", &[input]).await.unwrap();
        assert!(out.inserted.contains("t1"));
        assert!(out.errors.is_empty());

        // duplicate external_id is skipped silently
        let dup = CreateTransactionInput { txn: txn("t2", "Coffee again", 5.5, TransactionType::Debit, "a1", Some("ext-1")), category_ids: vec![] };
        let out2 = repo.create("u1", &[dup]).await.unwrap();
        assert!(!out2.inserted.contains("t2"));

        let cats: Vec<String> =
            sqlx::query_scalar("SELECT category_id FROM transaction_categories WHERE transaction_id = 't1' ORDER BY category_id")
                .fetch_all(&pool)
                .await
                .unwrap();
        assert_eq!(cats, vec!["c1", "c2"]);
    }

    #[tokio::test]
    async fn list_filters_and_paginates() {
        let pool = test_pool().await;
        seed_user_and_account(&pool, "u1", "a1").await;
        let repo = TransactionRepo::new(pool.clone());
        let mut inputs = Vec::new();
        for i in 0..5 {
            inputs.push(CreateTransactionInput {
                txn: txn(&format!("t{i}"), &format!("Item {i}"), 10.0 * f64::from(i + 1), if i % 2 == 0 { TransactionType::Debit } else { TransactionType::Credit }, "a1", None),
                category_ids: vec![],
            });
        }
        repo.create("u1", &inputs).await.unwrap();

        // default sort: occurred_at desc, id desc → t4..t0
        let res = repo.list("u1", &TransactionListFilter { page_size: 2, ..Default::default() }).await.unwrap();
        assert_eq!(res.rows.len(), 2);
        assert_eq!(res.rows[0].txn.id, "t4");
        assert_eq!(res.rows[1].txn.id, "t3");
        assert_eq!(res.total_count, 5);
        assert!(!res.next_page_token.is_empty());

        // page 2 via cursor
        let res2 = repo
            .list("u1", &TransactionListFilter { page_size: 2, page_token: res.next_page_token, ..Default::default() })
            .await
            .unwrap();
        assert_eq!(res2.rows.len(), 2);
        assert_eq!(res2.rows[0].txn.id, "t2");

        // filter by type (credit)
        let res3 = repo
            .list("u1", &TransactionListFilter { transaction_type: Some(TransactionType::Credit), ..Default::default() })
            .await
            .unwrap();
        assert_eq!(res3.rows.len(), 2);

        // filter by name
        let res4 = repo.list("u1", &TransactionListFilter { names: vec!["Item 3".to_string()], ..Default::default() }).await.unwrap();
        assert_eq!(res4.rows.len(), 1);
        assert_eq!(res4.rows[0].txn.id, "t3");

        // sort by debit amount asc
        let res5 = repo.list("u1", &TransactionListFilter { sort_by: "debit".to_string(), sort_dir: "asc".to_string(), ..Default::default() }).await.unwrap();
        // debits: t0=10, t2=30, t4=50 → t0 first
        assert_eq!(res5.rows[0].txn.id, "t0");
    }

    #[tokio::test]
    async fn update_replaces_categories() {
        let pool = test_pool().await;
        seed_user_and_account(&pool, "u1", "a1").await;
        let repo = TransactionRepo::new(pool.clone());
        repo.create(
            "u1",
            &[CreateTransactionInput { txn: txn("t1", "Old", 5.0, TransactionType::Debit, "a1", None), category_ids: vec!["c1".to_string()] }],
        )
        .await
        .unwrap();

        let mut updated = txn("t1", "New Name", 9.0, TransactionType::Credit, "a1", None);
        updated.occurred_at = "2024-02-02 03:04:05 +0000 UTC".to_string();
        let errs = repo.update("u1", &[UpdateTransactionInput { txn: updated, category_ids: vec!["c2".to_string()] }]).await.unwrap();
        assert!(errs.is_empty());

        let row: (String, f64, String, String) = sqlx::query_as("SELECT name, amount, type, occurred_at FROM transactions WHERE id = 't1'")
            .fetch_one(&pool)
            .await
            .unwrap();
        assert_eq!(row.0, "New Name");
        assert_eq!(row.1, 9.0);
        assert_eq!(row.2, "CREDIT");
        assert!(row.3.starts_with("2024-02-02"));

        let cats: Vec<String> =
            sqlx::query_scalar("SELECT category_id FROM transaction_categories WHERE transaction_id = 't1'").fetch_all(&pool).await.unwrap();
        assert_eq!(cats, vec!["c2"]);
    }

    #[tokio::test]
    async fn delete_removes_rows() {
        let pool = test_pool().await;
        seed_user_and_account(&pool, "u1", "a1").await;
        let repo = TransactionRepo::new(pool.clone());
        repo.create(
            "u1",
            &[
                CreateTransactionInput { txn: txn("t1", "A", 1.0, TransactionType::Debit, "a1", None), category_ids: vec![] },
                CreateTransactionInput { txn: txn("t2", "B", 2.0, TransactionType::Debit, "a1", None), category_ids: vec![] },
            ],
        )
        .await
        .unwrap();
        let errs = repo.delete("u1", &["t1".to_string()]).await.unwrap();
        assert!(errs.is_empty());
        let n: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM transactions").fetch_one(&pool).await.unwrap();
        assert_eq!(n, 1);

        // deleting another user's transaction is scoped out
        let scoped = repo.delete("u2", &["t2".to_string()]).await.unwrap();
        assert_eq!(scoped, vec!["transaction t2 not found"]);
        let n: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM transactions").fetch_one(&pool).await.unwrap();
        assert_eq!(n, 1);
    }



    #[tokio::test]
    async fn transfer_counterpart_and_links() {
        let pool = test_pool().await;
        seed_user_and_account(&pool, "u1", "a1").await;
        sqlx::query("INSERT INTO accounts (id, user_id, bank_name, type) VALUES (?, ?, ?, ?)")
            .bind("a2")
            .bind("u1")
            .bind("Bank2")
            .bind(1i64)
            .execute(&pool)
            .await
            .unwrap();
        let repo = TransactionRepo::new(pool.clone());
        repo.create(
            "u1",
            &[CreateTransactionInput {
                txn: txn("d1", "Transfer out", 50.0, TransactionType::Debit, "a1", None),
                category_ids: vec![],
            }],
        )
        .await
        .unwrap();
        repo.create(
            "u1",
            &[CreateTransactionInput {
                txn: txn("c1", "Transfer in", 50.0, TransactionType::Credit, "a2", None),
                category_ids: vec![],
            }],
        )
        .await
        .unwrap();

        let cand = repo
            .find_transfer_counterpart("u1", "a2", TransactionType::Credit, 50.0, "2024-01-02 03:04:05 +0000 UTC")
            .await
            .unwrap()
            .unwrap();
        assert_eq!(cand.id, "c1");

        let errs = create_links(&pool, "u1", &[("d1".to_string(), "c1".to_string())]).await.unwrap();
        assert!(errs.is_empty());
        assert!(is_transaction_linked(&pool, "d1").await.unwrap());

        // counterpart now excludes linked transactions
        let after = repo
            .find_transfer_counterpart("u1", "a2", TransactionType::Credit, 50.0, "2024-01-02 03:04:05 +0000 UTC")
            .await
            .unwrap();
        assert!(after.is_none());
    }
}


// --- spending aggregation ---

pub struct SpendingFilter {
    pub granularity: String, // "day" | "month"
    pub from: String,        // Go-driver format
    pub to: String,
    pub account_id: String,
}

pub struct SpendingBucketRow {
    pub key: String,
    pub amount: f64,
}

pub struct SpendingCategoryRow {
    pub id: String,
    pub name: String,
    pub debit: f64,
    pub credit: f64,
}

/// The LEFT JOIN chain exposing the transfer counterpart account, so a
/// transfer is excluded unless one side is a debt account.
fn spending_where() -> &'static str {
    "LEFT JOIN transfer_links tl ON tl.debit_transaction_id = t.id OR tl.credit_transaction_id = t.id
     LEFT JOIN accounts a_self ON a_self.id = t.account_id
     LEFT JOIN accounts a_other ON a_other.id = CASE WHEN tl.debit_transaction_id = t.id THEN tl.credit_transaction_id ELSE tl.debit_transaction_id END"
}

fn spending_range_clause(f: &SpendingFilter) -> (String, Vec<String>) {
    let mut s = String::new();
    let mut args = Vec::new();
    if !f.from.is_empty() {
        s.push_str(" AND t.occurred_at >= ?");
        args.push(f.from.clone());
    }
    if !f.to.is_empty() {
        s.push_str(" AND t.occurred_at < ?");
        args.push(f.to.clone());
    }
    if !f.account_id.is_empty() {
        s.push_str(" AND t.account_id = ?");
        args.push(f.account_id.clone());
    }
    (s, args)
}

impl TransactionRepo {
    /// Sum debit spending per day/month, excluding non-debt transfers.
    pub async fn spending_buckets(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingBucketRow>> {
        let key_expr = if f.granularity == "month" { "substr(t.occurred_at, 1, 7)" } else { "substr(t.occurred_at, 1, 10)" };
        let (range, range_args) = spending_range_clause(f);
        let query = format!(
            "SELECT {key_expr} AS key, COALESCE(SUM(t.amount), 0) AS amount
             FROM transactions t
             {}
             WHERE t.user_id = ? AND t.type = 'DEBIT'
               AND (tl.id IS NULL OR a_self.type IN ('CREDIT_CARD','LOAN') OR a_other.type IN ('CREDIT_CARD','LOAN'))
             {range}
             GROUP BY key ORDER BY key",
            spending_where()
        );
        let mut bind_args: Vec<String> = vec![user_id.to_string()];
        bind_args.extend(range_args);
        let mut q = sqlx::query_as::<_, (String, f64)>(&query);
        for a in &bind_args {
            q = q.bind(a);
        }
        let rows = q.fetch_all(&self.pool).await?;
        Ok(rows.into_iter().map(|(key, amount)| SpendingBucketRow { key, amount }).collect())
    }

    /// Sum debit/credit per category, with the same transfer exclusion.
    pub async fn spending_categories(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingCategoryRow>> {
        let (range, range_args) = spending_range_clause(f);
        let query = format!(
            "SELECT COALESCE(c.id, '__uncategorized__'), COALESCE(c.name, 'Uncategorized'),
                COALESCE(SUM(CASE WHEN t.type = 'DEBIT' THEN t.amount ELSE 0.0 END), 0.0) AS debit,
                COALESCE(SUM(CASE WHEN t.type = 'CREDIT' THEN t.amount ELSE 0.0 END), 0.0) AS credit
             FROM transactions t
             LEFT JOIN transaction_categories tc ON tc.transaction_id = t.id
             LEFT JOIN categories c ON c.id = tc.category_id
             {}
             WHERE t.user_id = ? AND (tl.id IS NULL OR a_self.type IN ('CREDIT_CARD','LOAN') OR a_other.type IN ('CREDIT_CARD','LOAN'))
             {range}
             GROUP BY COALESCE(c.id, '__uncategorized__') ORDER BY debit DESC",
            spending_where()
        );
        let mut bind_args: Vec<String> = vec![user_id.to_string()];
        bind_args.extend(range_args);
        let mut q = sqlx::query_as::<_, (String, String, f64, f64)>(&query);
        for a in &bind_args {
            q = q.bind(a);
        }
        let rows = q.fetch_all(&self.pool).await?;
        Ok(rows.into_iter().map(|(id, name, debit, credit)| SpendingCategoryRow { id, name, debit, credit }).collect())
    }
}

#[cfg(test)]
mod spending_tests {
    use super::*;

    async fn test_pool() -> SqlitePool {
        use std::str::FromStr;
        let dir = tempfile::tempdir().unwrap();
        // Keep the temp dir alive for the pool's lifetime (auto-delete on drop
        // would unlink the DB file before the lazy pool opens it).
        let path = dir.into_path().join("test.db");
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
    async fn spending_buckets_and_categories() {
        let pool = test_pool().await;
        // user + accounts: one current (non-debt), one credit card (debt)
        sqlx::query("INSERT INTO users (id, email, password_hash, name) VALUES ('u1','u1@x.com','h','u1')").execute(&pool).await.unwrap();
        sqlx::query("INSERT INTO accounts (id, user_id, bank_name, type) VALUES ('a1','u1','Bank','CURRENT')").execute(&pool).await.unwrap();
        sqlx::query("INSERT INTO accounts (id, user_id, bank_name, type) VALUES ('a2','u1','CC','CREDIT_CARD')").execute(&pool).await.unwrap();
        sqlx::query("INSERT INTO categories (id, user_id, name) VALUES ('c1','u1','Food')").execute(&pool).await.unwrap();
        let repo = TransactionRepo::new(pool.clone());
        let txn = |id: &str, name: &str, amt: f64, ty: TransactionType, acc: &str| Transaction {
            id: id.to_string(), name: name.to_string(), amount: amt, transaction_type: ty,
            occurred_at: "2024-01-02 10:00:00 +0000 UTC".to_string(),
            account_id: acc.to_string(), created_at: "2024-01-02 10:00:01 +0000 UTC".to_string(),
            external_id: None, transfer_linked: false,
        };
        repo.create("u1", &[
            CreateTransactionInput { txn: txn("t1", "Groceries", 50.0, TransactionType::Debit, "a1"), category_ids: vec!["c1".to_string()] },
            CreateTransactionInput { txn: txn("t2", "Salary", 1000.0, TransactionType::Credit, "a1"), category_ids: vec![] },
        ]).await.unwrap();

        let filter = SpendingFilter { granularity: "day".into(), from: String::new(), to: String::new(), account_id: String::new() };
        let buckets = repo.spending_buckets("u1", &filter).await.unwrap();
        assert_eq!(buckets.len(), 1, "one day bucket (only debit counts)");
        assert_eq!(buckets[0].key, "2024-01-02");
        assert_eq!(buckets[0].amount, 50.0);

        let cats = repo.spending_categories("u1", &filter).await.unwrap();
        assert_eq!(cats.len(), 2, "Food + Uncategorized");
        let food = cats.iter().find(|c| c.name == "Food").unwrap();
        assert_eq!(food.debit, 50.0);
        assert_eq!(food.credit, 0.0);
        let uncat = cats.iter().find(|c| c.name == "Uncategorized").unwrap();
        assert_eq!(uncat.credit, 1000.0, "salary credit lands in uncategorized");
    }
}
