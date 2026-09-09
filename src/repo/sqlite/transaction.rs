//! `SQLite` transactions + transfer-link repository, implementing
//! `crate::repo::traits::TransactionRepo` over an `sqlx::SqlitePool`.

use async_trait::async_trait;
use sqlx::SqlitePool;
use std::fmt::Write;
use std::str::FromStr;
use uuid::Uuid;

use crate::error::Result;
use crate::repo::traits::transaction::{
    decode_transaction_cursor, encode_transaction_cursor, CreateOutcome, CreateTransactionInput, ListRow,
    ListTransactionResult, SpendingBucketRow, SpendingCategoryRow, SpendingFilter, Transaction,
    TransactionListFilter, TransactionType, UpdateTransactionInput,
};
use crate::timex::go_ts;

pub struct SqliteTransactionRepo {
    pool: SqlitePool,
}

impl SqliteTransactionRepo {
    pub fn new(pool: SqlitePool) -> Self {
        Self { pool }
    }

    async fn get_by_id_for_transfer_inner(&self, user_id: &str, id: &str) -> Result<Option<(String, f64, String)>> {
        let row = sqlx::query_as::<_, (String, f64, String)>(
            "SELECT type, amount, account_id FROM transactions WHERE id = ? AND user_id = ?",
        )
        .bind(id)
        .bind(user_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row)
    }

    async fn get_full_inner(&self, user_id: &str, id: &str) -> Result<Option<Transaction>> {
        let row = sqlx::query_as::<_, RawTransaction>(
            "SELECT t.id, t.name, t.clean_name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at, t.external_id
             FROM transactions t WHERE t.id = ? AND t.user_id = ?",
        )
        .bind(id)
        .bind(user_id)
        .fetch_optional(&self.pool)
        .await?;
        let mut txn: Option<Transaction> = row.map(Into::into);
        if let Some(t) = &mut txn {
            t.transfer_linked = self.is_transfer_linked_inner(id).await?;
        }
        Ok(txn)
    }

    async fn find_transfer_counterpart_inner(
        &self,
        user_id: &str,
        account_id: &str,
        transaction_type: TransactionType,
        amount: f64,
        around: &str,
    ) -> Result<Option<Transaction>> {
        let around_str = &around[..around.len().min(19)];
        let row = sqlx::query_as::<_, RawTransaction>(
            "SELECT t.id, t.name, t.clean_name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at, NULL AS external_id
            FROM transactions t
            WHERE t.user_id = ? AND t.account_id = ? AND t.type = ? AND t.amount = ?
              AND t.occurred_at >= datetime(?, '-3 days') AND t.occurred_at <= datetime(?, '+3 days')
              AND NOT EXISTS (SELECT 1 FROM transfer_links l WHERE l.debit_transaction_id = t.id OR l.credit_transaction_id = t.id)
            ORDER BY ABS(CAST(strftime('%s', t.occurred_at) AS INTEGER) - CAST(strftime('%s', ?) AS INTEGER))
            LIMIT 1",
        )
        .bind(user_id)
        .bind(account_id)
        .bind(transaction_type.to_string())
        .bind(amount)
        .bind(around_str)
        .bind(around_str)
        .bind(around_str)
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(Into::into))
    }

    async fn is_transfer_linked_inner(&self, txn_id: &str) -> Result<bool> {
        let one: Option<i64> = sqlx::query_scalar(
            "SELECT 1 FROM transfer_links l WHERE l.debit_transaction_id = ? OR l.credit_transaction_id = ? LIMIT 1",
        )
        .bind(txn_id)
        .bind(txn_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(one.is_some())
    }

    async fn create_inner(&self, user_id: &str, inputs: &[CreateTransactionInput]) -> Result<CreateOutcome> {
        let mut inserted = std::collections::HashSet::new();
        let mut errors: Vec<String> = Vec::new();
        if inputs.is_empty() {
            return Ok(CreateOutcome { inserted, errors });
        }
        let mut tx = self.pool.begin().await?;
        for input in inputs {
            let t = &input.txn;
            let res = sqlx::query(
                "INSERT OR IGNORE INTO transactions (id, user_id, name, clean_name, amount, type, occurred_at, account_id, created_at, external_id)
                 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
            )
            .bind(&t.id)
            .bind(user_id)
            .bind(&t.name)
            .bind(&t.clean_name)
            .bind(t.amount)
            .bind(t.transaction_type.to_string())
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
                Ok(_) => {}
                Err(e) => {
                    errors.push(format!("failed to create {}", t.id));
                    tracing::error!("failed to create transaction {}: {e}", t.id);
                }
            }
        }
        tx.commit().await?;
        Ok(CreateOutcome { inserted, errors })
    }

    async fn update_inner(&self, user_id: &str, inputs: &[UpdateTransactionInput]) -> Result<Vec<String>> {
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
            .bind(t.transaction_type.to_string())
            .bind(t.transaction_type.to_string())
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

    async fn delete_inner(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
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

    async fn get_by_id_batch_inner(&self, user_id: &str, ids: &[String]) -> Result<Vec<Transaction>> {
        if ids.is_empty() {
            return Ok(Vec::new());
        }
        let mut query = String::from(
            "SELECT id, name, clean_name, amount, type, occurred_at, account_id, created_at, external_id FROM transactions WHERE user_id = ? AND id IN (",
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

    async fn list_inner(&self, user_id: &str, f: &TransactionListFilter) -> Result<ListTransactionResult> {
        let (where_sql, filter_args) = build_txn_where(f);

        let mut query = String::from(
            "SELECT t.id, t.name, t.clean_name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at, t.external_id,
                    COALESCE(l.id, ''),
                    (SELECT GROUP_CONCAT(tc.category_id) FROM transaction_categories tc WHERE tc.transaction_id = t.id),
                    COUNT(*) OVER () AS total
                FROM transactions t
                LEFT JOIN transfer_links l ON l.debit_transaction_id = t.id OR l.credit_transaction_id = t.id ",
        );
        query.push_str(&where_sql);

        let mut bind_args: Vec<String> = Vec::with_capacity(filter_args.len() + 1);
        bind_args.push(user_id.to_string());
        bind_args.extend(filter_args);

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

    async fn spending_buckets_inner(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingBucketRow>> {
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

    async fn spending_categories_inner(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingCategoryRow>> {
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

    async fn create_links_inner(&self, user_id: &str, links: &[(String, String)]) -> Result<Vec<String>> {
        let mut errors: Vec<String> = Vec::new();
        if links.is_empty() {
            return Ok(errors);
        }
        let mut tx = self.pool.begin().await?;
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

    async fn delete_links_inner(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        let mut errors: Vec<String> = Vec::new();
        if ids.is_empty() {
            return Ok(errors);
        }
        let mut tx = self.pool.begin().await?;
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

    async fn is_transaction_linked_inner(&self, txn_id: &str) -> Result<bool> {
        let one: Option<String> = sqlx::query_scalar(
            "SELECT id FROM transfer_links WHERE debit_transaction_id = ? OR credit_transaction_id = ? LIMIT 1",
        )
        .bind(txn_id)
        .bind(txn_id)
        .fetch_optional(&self.pool)
        .await?;
        Ok(one.is_some())
    }
}

#[async_trait]
impl crate::repo::traits::TransactionRepo for SqliteTransactionRepo {
    async fn get_by_id_for_transfer(&self, user_id: &str, id: &str) -> Result<Option<(String, f64, String)>> {
        self.get_by_id_for_transfer_inner(user_id, id).await
    }
    async fn get_full(&self, user_id: &str, id: &str) -> Result<Option<Transaction>> {
        self.get_full_inner(user_id, id).await
    }
    async fn find_transfer_counterpart(&self, user_id: &str, account_id: &str, transaction_type: TransactionType, amount: f64, around: &str) -> Result<Option<Transaction>> {
        self.find_transfer_counterpart_inner(user_id, account_id, transaction_type, amount, around).await
    }
    async fn is_transfer_linked(&self, txn_id: &str) -> Result<bool> {
        self.is_transfer_linked_inner(txn_id).await
    }
    async fn create(&self, user_id: &str, inputs: &[CreateTransactionInput]) -> Result<CreateOutcome> {
        self.create_inner(user_id, inputs).await
    }
    async fn update(&self, user_id: &str, inputs: &[UpdateTransactionInput]) -> Result<Vec<String>> {
        self.update_inner(user_id, inputs).await
    }
    async fn delete(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        self.delete_inner(user_id, ids).await
    }
    async fn get_by_id_batch(&self, user_id: &str, ids: &[String]) -> Result<Vec<Transaction>> {
        self.get_by_id_batch_inner(user_id, ids).await
    }
    async fn list(&self, user_id: &str, f: &TransactionListFilter) -> Result<ListTransactionResult> {
        self.list_inner(user_id, f).await
    }
    async fn spending_buckets(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingBucketRow>> {
        self.spending_buckets_inner(user_id, f).await
    }
    async fn spending_categories(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingCategoryRow>> {
        self.spending_categories_inner(user_id, f).await
    }
    async fn create_links(&self, user_id: &str, links: &[(String, String)]) -> Result<Vec<String>> {
        self.create_links_inner(user_id, links).await
    }
    async fn delete_links(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        self.delete_links_inner(user_id, ids).await
    }
    async fn is_transaction_linked(&self, txn_id: &str) -> Result<bool> {
        self.is_transaction_linked_inner(txn_id).await
    }
}

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

fn like_escape(s: &str) -> String {
    s.replace('\\', "\\\\").replace('%', "\\%").replace('_', "\\_")
}

/// The full row shape returned by the list query: txn, link id, category group.
struct RawListTxn(pub RawTransaction, pub Option<String>, pub Option<String>, pub i64);

impl<'r> sqlx::FromRow<'r, sqlx::sqlite::SqliteRow> for RawListTxn {
    fn from_row(row: &'r sqlx::sqlite::SqliteRow) -> std::result::Result<Self, sqlx::Error> {
        use sqlx::Row;
        let txn = RawTransaction {
            id: row.try_get(0)?,
            name: row.try_get(1)?,
            clean_name: row.try_get(2)?,
            amount: row.try_get(3)?,
            transaction_type: row.try_get(4)?,
            occurred_at: row.try_get(5)?,
            account_id: row.try_get(6)?,
            created_at: row.try_get(7)?,
            external_id: row.try_get(8)?,
        };
        let link: Option<String> = row.try_get(9)?;
        let cat_group: Option<String> = row.try_get(10)?;
        let total: i64 = row.try_get(11)?;
        Ok(RawListTxn(txn, link, cat_group, total))
    }
}

#[derive(sqlx::FromRow)]
struct RawTransaction {
    id: String,
    name: String,
    clean_name: Option<String>,
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
            clean_name: r.clean_name,
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
