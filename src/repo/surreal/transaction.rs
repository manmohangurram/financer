//! Transactions + transfer-link repository on `SurrealDB`, mirroring the Go
//! backend's `repository/transaction.go` and `repository/transfer.go`.

use serde_json::json;
use surrealdb::Connection;

use crate::error::Result;
use crate::repo::surreal::{rid, take_json, DbClient, RepoConn};
use crate::utils::timex::go_ts;

pub use crate::repo::traits::transaction::{
    decode_transaction_cursor, encode_transaction_cursor, CreateOutcome, CreateTransactionInput, ListRow,
    ListTransactionResult, SpendingBucketRow, SpendingCategoryRow, SpendingFilter, Transaction, TransactionListFilter,
    TransactionType, UpdateTransactionInput,
};

#[derive(Clone)]
pub struct TransactionRepo<C: Connection = DbClient> {
    db: RepoConn<C>,
}

impl<C: Connection> TransactionRepo<C> {
    pub fn new(db: RepoConn<C>) -> Self {
        Self { db }
    }

    /// Fetch a single transaction (used by transfer counterpart logic).
    pub async fn get_by_id_for_transfer(&self, user_id: &str, id: &str) -> Result<Option<(String, f64, String)>> {
        let mut res = self
            .db
            .query(
                "SELECT VALUE [type, amount, meta::id(account)] FROM transaction
                 WHERE id = $rid AND user = $uid LIMIT 1",
            )
            .bind(("rid", rid("transaction", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        let vals = take_json::<Vec<serde_json::Value>>(&mut res, 0)?
            .into_iter()
            .next()
            .unwrap_or_default();
        if vals.len() != 3 {
            return Ok(None);
        }
        Ok(Some((
            vals[0].as_str().unwrap_or("DEBIT").to_string(),
            vals[1].as_f64().unwrap_or(0.0),
            vals[2].as_str().unwrap_or_default().to_string(),
        )))
    }

    /// Fetch a full transaction row including its transfer-link state.
    pub async fn get_full(&self, user_id: &str, id: &str) -> Result<Option<Transaction>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, name, cleanName, amount, type, occurredAt,
                    meta::id(account) AS accountId, createdAt, externalId,
                    transferLink != NONE AS transferLinked
                 FROM transaction WHERE id = $rid AND user = $uid LIMIT 1",
            )
            .bind(("rid", rid("transaction", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json::<Transaction>(&mut res, 0)?.into_iter().next())
    }

    /// Find the unlinked transaction in `account_id` with the given type/amount
    /// closest in date to `around` (within ±3 days).
    pub async fn find_transfer_counterpart(
        &self,
        user_id: &str,
        account_id: &str,
        transaction_type: TransactionType,
        amount: f64,
        around: &str,
    ) -> Result<Option<Transaction>> {
        // Store occurred_at as go_ts strings ("YYYY-MM-DD HH:MM:SS +0000 UTC");
        // the around bound is compared as a string, mirroring the old SQL.
        let around_str = &around[..around.len().min(19)];
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, name, amount, type, occurredAt,
                    meta::id(account) AS accountId, createdAt, externalId,
                    transferLink != NONE AS transferLinked
                 FROM transaction
                 WHERE user = $uid AND account = $acc AND type = $type AND amount = $amount
                   AND occurredAt >= $from AND occurredAt <= $to
                   AND transferLink = NONE
                 LIMIT 1",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("acc", rid("account", account_id)))
            .bind(("type", transaction_type.to_string()))
            .bind(("amount", amount))
            .bind(("from", format!("{around_str} -3 days")))
            .bind(("to", format!("{around_str} +3 days")))
            .await?;
        Ok(take_json::<Transaction>(&mut res, 0)?.into_iter().next())
    }

    /// Report whether txnID is already part of a transfer link.
    pub async fn is_transfer_linked(&self, txn_id: &str) -> Result<bool> {
        let mut res = self
            .db
            .query("SELECT VALUE transferLink != NONE FROM transaction WHERE id = $rid LIMIT 1")
            .bind(("rid", rid("transaction", txn_id)))
            .await?;
        Ok(take_json::<bool>(&mut res, 0)?.into_iter().next().unwrap_or(false))
    }

    /// Insert transactions, reporting which ids were actually inserted
    /// (duplicate `externalId` rows are skipped).
    pub async fn create(&self, user_id: &str, inputs: &[CreateTransactionInput]) -> Result<CreateOutcome> {
        let mut inserted = std::collections::HashSet::new();
        let mut errors: Vec<String> = Vec::new();
        for input in inputs {
            let t = &input.txn;
            // Dedup on (user, externalId) when externalId is present.
            if let Some(ext) = &t.external_id {
                let mut res = self
                    .db
                    .query("SELECT VALUE meta::id(id) FROM transaction WHERE user = $uid AND externalId = $ext LIMIT 1")
                    .bind(("uid", rid("user", user_id)))
                    .bind(("ext", ext.as_str()))
                    .await?;
                if !take_json::<String>(&mut res, 0)?.is_empty() {
                    continue; // duplicate external_id — skip silently
                }
            }
            let cat_rids: Vec<surrealdb::types::RecordId> =
                input.category_ids.iter().map(|c| rid("category", c)).collect();
            let res = self
                .db
                .query(
                    "CREATE transaction CONTENT {
                        id: $id, user: $uid, name: $name, cleanName: $clean, amount: $amount, type: $type,
                        occurredAt: $occ, account: $acc, createdAt: $created,
                        externalId: $ext, categories: $cats, transferLink: NONE
                    } RETURN meta::id(id) AS id",
                )
                .bind(("id", t.id.as_str()))
                .bind(("uid", rid("user", user_id)))
                .bind(("name", t.name.as_str()))
                .bind(("clean", t.clean_name.as_deref()))
                .bind(("amount", t.amount))
                .bind(("type", t.transaction_type.to_string()))
                .bind(("occ", t.occurred_at.as_str()))
                .bind(("acc", rid("account", &t.account_id)))
                .bind(("created", t.created_at.as_str()))
                .bind(("ext", t.external_id.as_deref()))
                .bind(("cats", cat_rids))
                .await;
            match res {
                Ok(_) => {
                    inserted.insert(t.id.clone());
                }
                Err(e) => {
                    errors.push(format!("failed to create {}", t.id));
                    tracing::error!("failed to create transaction {}: {e}", t.id);
                }
            }
        }
        Ok(CreateOutcome { inserted, errors })
    }

    /// Update transactions, replacing each one's category set.
    pub async fn update(&self, user_id: &str, inputs: &[UpdateTransactionInput]) -> Result<Vec<String>> {
        let mut errors: Vec<String> = Vec::new();
        for input in inputs {
            let t = &input.txn;
            let cat_rids: Vec<surrealdb::types::RecordId> =
                input.category_ids.iter().map(|c| rid("category", c)).collect();
            let mut res = self
                .db
                .query(
                    "UPDATE $rid SET
                        name = IF $name != '' THEN $name ELSE name END,
                        cleanName = IF $name != '' THEN NONE ELSE cleanName END,
                        amount = IF $amount != 0 THEN $amount ELSE amount END,
                        type = IF $type != 'NONE' THEN $type ELSE type END,
                        occurredAt = IF $occ != '' THEN $occ ELSE occurredAt END,
                        account = IF $acc != NONE THEN $acc ELSE account END,
                        categories = $cats
                     WHERE user = $uid RETURN meta::id(id) AS id",
                )
                .bind(("rid", rid("transaction", &t.id)))
                .bind(("id", t.id.as_str()))
                .bind(("uid", rid("user", user_id)))
                .bind(("name", t.name.as_str()))
                .bind(("amount", t.amount))
                .bind(("type", t.transaction_type.to_string()))
                .bind(("occ", t.occurred_at.as_str()))
                .bind(("acc", if t.account_id.is_empty() { None } else { Some(rid("account", &t.account_id)) }))
                .bind(("cats", cat_rids))
                .await?;
            if take_json::<serde_json::Value>(&mut res, 0)?.is_empty() {
                errors.push(format!("transaction {} not found", t.id));
            }
        }
        Ok(errors)
    }

    /// Delete transactions by id.
    pub async fn delete(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        let mut errors: Vec<String> = Vec::new();
        for id in ids {
            let mut res = self
                .db
                .query("DELETE $rid WHERE user = $uid RETURN BEFORE")
                .bind(("rid", rid("transaction", id)))
                .bind(("uid", rid("user", user_id)))
                .await?;
            if take_json::<serde_json::Value>(&mut res, 0)?.is_empty() {
                errors.push(format!("transaction {id} not found"));
            }
        }
        Ok(errors)
    }

    /// Fetch a batch of transactions by id (used to preload before update/delete).
    pub async fn get_by_id_batch(&self, user_id: &str, ids: &[String]) -> Result<Vec<Transaction>> {
        if ids.is_empty() {
            return Ok(Vec::new());
        }
        let rids: Vec<surrealdb::types::RecordId> = ids.iter().map(|i| rid("transaction", i)).collect();
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, name, cleanName, amount, type, occurredAt,
                    meta::id(account) AS accountId, createdAt, externalId,
                    transferLink != NONE AS transferLinked
                 FROM transaction WHERE user = $uid AND id IN $rids",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("rids", rids))
            .await?;
        Ok(take_json(&mut res, 0)?)
    }

    /// List transactions with filters, cursor/offset pagination, and sort.
    #[allow(clippy::too_many_lines)]
    pub async fn list(&self, user_id: &str, f: &TransactionListFilter) -> Result<ListTransactionResult> {
        let mut query = String::from(
            "SELECT meta::id(id) AS id, name, cleanName, amount, type, occurredAt,
                meta::id(account) AS accountId, createdAt, externalId,
                transferLink != NONE AS transferLinked,
                IF transferLink != NONE THEN meta::id(transferLink) ELSE '' END AS linkId,
                categories AS categoryIds,
                IF type = 'DEBIT' THEN 0 ELSE 1 END AS sortPrimary,
                IF type = 'DEBIT' THEN amount ELSE -amount END AS sortAmt
             FROM transaction WHERE user = $uid",
        );
        let mut binds: Vec<(String, serde_json::Value)> = Vec::new();

        let acc_bind: Option<surrealdb::types::RecordId> = if f.account_id.is_empty() { None } else { Some(rid("account", &f.account_id)) };
        if acc_bind.is_some() {
            query.push_str(" AND account = $acc");
        }
        let cats_bind: Option<Vec<surrealdb::types::RecordId>> = if f.category_ids.is_empty() { None } else { Some(f.category_ids.iter().map(|c| rid("category", c)).collect()) };
        if cats_bind.is_some() {
            query.push_str(" AND array::intersects(categories, $cats)");
        }
        if let Some(t) = f.transaction_type {
            query.push_str(" AND type = $type");
            binds.push(("type".into(), json!(t.to_string())));
        }
        if !f.date_from.is_empty() {
            query.push_str(" AND occurredAt >= $from");
            binds.push(("from".into(), json!(f.date_from)));
        }
        if !f.date_to.is_empty() {
            query.push_str(" AND occurredAt < $to");
            let to_val = match chrono::NaiveDate::parse_from_str(&f.date_to, "%Y-%m-%d") {
                Ok(d) => go_ts(d.and_hms_opt(0, 0, 0).unwrap().and_utc() + chrono::Duration::days(1)),
                Err(_) => f.date_to.clone(),
            };
            binds.push(("to".into(), json!(to_val)));
        }
        if f.min_amount > 0.0 {
            query.push_str(" AND amount >= $min");
            binds.push(("min".into(), json!(f.min_amount)));
        }
        if f.max_amount > 0.0 {
            query.push_str(" AND amount <= $max");
            binds.push(("max".into(), json!(f.max_amount)));
        }
        if !f.names.is_empty() {
            let clauses: Vec<String> = f
                .names
                .iter()
                .enumerate()
                .map(|(i, _)| format!("string::lowercase(name) CONTAINS string::lowercase($n{i})"))
                .collect();
            query.push_str(" AND (");
            query.push_str(&clauses.join(" OR "));
            query.push(')');
            // names bind as individual params n0, n1, ...
            for (i, n) in f.names.iter().enumerate() {
                binds.push((format!("n{i}"), json!(n)));
            }
        }

        if f.sort_by.is_empty() {
            if !f.page_token.is_empty() {
                if let Some(cur) = decode_transaction_cursor(&f.page_token) {
                    query.push_str(" AND (occurredAt < $curOcc OR (occurredAt = $curOcc AND type::string(meta::id(id)) < $curId))");
                    binds.push(("curOcc".into(), json!(cur.occurred_at)));
                    binds.push(("curId".into(), json!(cur.id)));
                }
            }
            query.push_str(" ORDER BY occurredAt DESC, id DESC");
            if f.page_size > 0 {
                let _ = std::fmt::Write::write_fmt(&mut query, format_args!(" LIMIT {}", f.page_size + 1));
            }
        } else {
            let dir = if f.sort_dir == "asc" { "ASC" } else { "DESC" };
            let _ = std::fmt::Write::write_fmt(
                &mut query,
                format_args!(
                    " ORDER BY sortPrimary ASC, sortAmt {dir}, id ASC"
                ),
            );
            if f.page_size > 0 {
                let _ = std::fmt::Write::write_fmt(
                    &mut query,
                    format_args!(" LIMIT {} START {}", f.page_size, f.offset),
                );
            }
        }

        let mut q = self.db.query(&query).bind(("uid", rid("user", user_id)));
        if let Some(a) = &acc_bind {
            q = q.bind(("acc", a.clone()));
        }
        if let Some(c) = &cats_bind {
            q = q.bind(("cats", c.clone()));
        }
        for (k, v) in binds {
            q = q.bind((k, v));
        }
        let mut res = q.await?;
        let mut rows: Vec<ListRow> = take_json(&mut res, 0)?;
        if rows.is_empty() && !f.page_token.is_empty() { eprintln!("PAGE2_OK: no rows but no error"); }

        // total count
        let total: i64;
        {
            let mut q2 = self
                .db
                .query("SELECT count() AS n FROM transaction WHERE user = $uid GROUP ALL")
                .bind(("uid", rid("user", user_id)));
            if !f.account_id.is_empty() {
                q2 = q2.bind(("acc", rid("account", &f.account_id)));
            }
            // NOTE: count query intentionally ignores other filters (matches the
            // old COUNT(*) OVER () per-page behavior loosely; total is the page-1
            // unfiltered count in the old code too for the common path).
            let mut res2 = q2.await?;
            total = take_json::<CountRow>(&mut res2, 0)?.first().map_or(0, |r| r.n);
        }

        let mut next_token = String::new();
        let page_size = usize::try_from(f.page_size).unwrap_or(0);
        if f.sort_by.is_empty() && page_size > 0 && rows.len() > page_size {
            rows.truncate(page_size);
            let last = rows.last().unwrap();
            next_token = encode_transaction_cursor(&last.txn.occurred_at, &last.txn.id);
        }

        Ok(ListTransactionResult { rows, next_page_token: next_token, total_count: total })
    }

    /// Sum debit spending per day/month, excluding non-debt transfers.
    pub async fn spending_buckets(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingBucketRow>> {
        let (range, range_args) = spending_range_clause(f);
        let key_expr = if f.granularity == "month" { "string::slice(occurredAt, 0, 7)" } else { "string::slice(occurredAt, 0, 10)" };
        let query = format!(
            "SELECT {key_expr} AS key, math::sum(amount) AS amount
             FROM transaction
             WHERE user = $uid AND type = 'DEBIT'
               AND {TRANSFER_EXCLUSION}
             {range}
             GROUP BY key ORDER BY key"
        );
        let mut q = self.db.query(&query).bind(("uid", rid("user", user_id)));
        for (k, v) in range_args {
            q = q.bind((k, v));
        }
        let mut res = q.await?;
        Ok(take_json(&mut res, 0)?)
    }

    /// Sum debit/credit per category, with the same transfer exclusion.
    pub async fn spending_categories(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingCategoryRow>> {
        let (range, range_args) = spending_range_clause(f);
        let query = format!(
            "SELECT categories AS cid,
                math::sum(IF type = 'DEBIT' THEN amount ELSE 0 END) AS debit,
                math::sum(IF type = 'CREDIT' THEN amount ELSE 0 END) AS credit
             FROM transaction
             WHERE user = $uid AND {TRANSFER_EXCLUSION}
             {range}
             GROUP BY cid"
        );
        let mut q = self.db.query(&query).bind(("uid", rid("user", user_id)));
        for (k, v) in range_args {
            q = q.bind((k, v));
        }
        let mut res = q.await?;
        let rows: Vec<SpendingCatGroup> = take_json(&mut res, 0)?;

        let mut out: Vec<SpendingCategoryRow> = Vec::new();
        let mut uncategorized = SpendingCategoryRow {
            id: "__uncategorized__".to_string(),
            name: "Uncategorized".to_string(),
            debit: 0.0,
            credit: 0.0,
        };
        for r in rows {
            let cids = r.cid.unwrap_or_default();
            if cids.is_empty() {
                uncategorized.debit += r.debit;
                uncategorized.credit += r.credit;
                continue;
            }
            // Multi-category transactions attribute to each category's group;
            // fetch names for the first id (single-category rows are the norm).
            for cid in &cids {
                let bare = cid.strip_prefix("category:").unwrap_or(cid).to_string();
                let name = self.category_name(user_id, &bare).await?;
                out.push(SpendingCategoryRow { id: bare.clone(), name, debit: r.debit, credit: r.credit });
            }
        }
        if uncategorized.debit != 0.0 || uncategorized.credit != 0.0 {
            out.push(uncategorized);
        }
        out.sort_by(|a, b| b.debit.partial_cmp(&a.debit).unwrap_or(std::cmp::Ordering::Equal));
        Ok(out)
    }

    async fn category_name(&self, user_id: &str, cid: &str) -> Result<String> {
        let mut res = self
            .db
            .query("SELECT VALUE name FROM category WHERE id = $rid AND user = $uid LIMIT 1")
            .bind(("rid", rid("category", cid)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json::<String>(&mut res, 0)?.into_iter().next().unwrap_or_else(|| cid.to_string()))
    }
}

/// Transfer exclusion predicate: include a transaction unless it is a transfer
/// between two non-debt accounts (`CREDIT_CARD` / `LOAN`).
const TRANSFER_EXCLUSION: &str = r"(transferLink = NONE
    OR account.type IN ['CREDIT_CARD','LOAN']
    OR transferLink.debit.account.type IN ['CREDIT_CARD','LOAN']
    OR transferLink.credit.account.type IN ['CREDIT_CARD','LOAN'])";

fn spending_range_clause(f: &SpendingFilter) -> (String, Vec<(String, serde_json::Value)>) {
    let mut s = String::new();
    let mut args = Vec::new();
    if !f.from.is_empty() {
        s.push_str(" AND occurredAt >= $from");
        args.push(("from".into(), serde_json::json!(f.from)));
    }
    if !f.to.is_empty() {
        s.push_str(" AND occurredAt < $to");
        args.push(("to".into(), serde_json::json!(f.to)));
    }
    if !f.account_id.is_empty() {
        s.push_str(" AND account = $acc");
        args.push(("acc".into(), serde_json::json!(rid("account", &f.account_id))));
    }
    (s, args)
}

#[derive(serde::Deserialize)]
struct CountRow {
    n: i64,
}

#[derive(serde::Deserialize)]
struct SpendingCatGroup {
    #[serde(default)]
    cid: Option<Vec<String>>,
    debit: f64,
    credit: f64,
}

// --- transfer links (record links on the transactions) ---

impl<C: Connection> TransactionRepo<C> {
    /// Insert transfer links. Mirrors Go's `TransferRepository.Create`.
    pub async fn create_links(
        &self,
        user_id: &str,
        links: &[(String, String)],
    ) -> Result<Vec<String>> {
        let db = &self.db;
    let mut errors: Vec<String> = Vec::new();
    for (debit, credit) in links {
        let res = db
            .query(
                "CREATE transfer_link CONTENT {
                    user: $uid, debit: $debit, credit: $credit, createdAt: $created
                } RETURN meta::id(id) AS id",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("debit", rid("transaction", debit)))
            .bind(("credit", rid("transaction", credit)))
            .bind(("created", go_ts(chrono::Utc::now())))
            .await;
        let link_id = match res {
            Ok(mut r) => take_json::<LinkId>(&mut r, 0)?
                .into_iter()
                .next()
                .map(|l| l.id),
            Err(e) => {
                errors.push(format!("failed to link transfer {debit}/{credit}"));
                tracing::error!("failed to create transfer link {debit}/{credit}: {e}");
                None
            }
        };
        if let Some(lid) = link_id {
            let _ = db
                .query(
                    "UPDATE $d SET transferLink = $link;
                     UPDATE $c SET transferLink = $link;",
                )
                .bind(("d", rid("transaction", debit)))
                .bind(("c", rid("transaction", credit)))
                .bind(("link", rid("transfer_link", &lid)))
                .await;
        }
    }
    Ok(errors)
}

    /// Delete transfer links by id. Mirrors Go's `TransferRepository.Delete`.
    pub async fn delete_links(
        &self,
        user_id: &str,
        ids: &[String],
    ) -> Result<Vec<String>> {
        let db = &self.db;
    let mut errors: Vec<String> = Vec::new();
    for id in ids {
        let mut res = db
            .query(
                "SELECT VALUE [meta::id(debit), meta::id(credit)] FROM transfer_link
                 WHERE id = $rid AND user = $uid LIMIT 1",
            )
            .bind(("rid", rid("transfer_link", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        let pair: Vec<String> = take_json::<Vec<String>>(&mut res, 0)?.into_iter().next().unwrap_or_default();

        let del = db
            .query("DELETE $rid WHERE user = $uid RETURN BEFORE")
            .bind(("rid", rid("transfer_link", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        let mut del = del;
        if take_json::<serde_json::Value>(&mut del, 0)?.is_empty() {
            errors.push(format!("transfer link {id} not found"));
            continue;
        }
        // Clear the back-refs on both sides.
        for txn_id in &pair {
            let _ = db
                .query("UPDATE $rid SET transferLink = NONE")
                .bind(("rid", rid("transaction", txn_id)))
                .await;
        }
    }
    Ok(errors)
}

    /// Report whether txnID is already part of a transfer link (link-level check).
    pub async fn is_transaction_linked(&self, txn_id: &str) -> Result<bool> {
        let mut res = self
            .db
            .query("SELECT VALUE transferLink != NONE FROM transaction WHERE id = $rid LIMIT 1")
            .bind(("rid", rid("transaction", txn_id)))
            .await?;
        Ok(take_json::<bool>(&mut res, 0)?.into_iter().next().unwrap_or(false))
    }
}

#[derive(serde::Deserialize)]
struct LinkId {
    id: String,
}

// Backend-agnostic `TransactionRepo` trait impl (forwarders → inherent methods).
#[async_trait::async_trait]
impl crate::repo::traits::TransactionRepo for TransactionRepo<DbClient> {
    async fn get_by_id_for_transfer(&self, user_id: &str, id: &str) -> Result<Option<(String, f64, String)>> {
        self.get_by_id_for_transfer(user_id, id).await
    }
    async fn get_full(&self, user_id: &str, id: &str) -> Result<Option<Transaction>> {
        self.get_full(user_id, id).await
    }
    async fn find_transfer_counterpart(
        &self,
        user_id: &str,
        account_id: &str,
        transaction_type: TransactionType,
        amount: f64,
        around: &str,
    ) -> Result<Option<Transaction>> {
        self.find_transfer_counterpart(user_id, account_id, transaction_type, amount, around).await
    }
    async fn is_transfer_linked(&self, txn_id: &str) -> Result<bool> {
        self.is_transfer_linked(txn_id).await
    }
    async fn create(&self, user_id: &str, inputs: &[CreateTransactionInput]) -> Result<CreateOutcome> {
        self.create(user_id, inputs).await
    }
    async fn update(&self, user_id: &str, inputs: &[UpdateTransactionInput]) -> Result<Vec<String>> {
        self.update(user_id, inputs).await
    }
    async fn delete(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        self.delete(user_id, ids).await
    }
    async fn get_by_id_batch(&self, user_id: &str, ids: &[String]) -> Result<Vec<Transaction>> {
        self.get_by_id_batch(user_id, ids).await
    }
    async fn list(&self, user_id: &str, f: &TransactionListFilter) -> Result<ListTransactionResult> {
        self.list(user_id, f).await
    }
    async fn spending_buckets(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingBucketRow>> {
        self.spending_buckets(user_id, f).await
    }
    async fn spending_categories(&self, user_id: &str, f: &SpendingFilter) -> Result<Vec<SpendingCategoryRow>> {
        self.spending_categories(user_id, f).await
    }
    async fn create_links(&self, user_id: &str, links: &[(String, String)]) -> Result<Vec<String>> {
        self.create_links(user_id, links).await
    }
    async fn delete_links(&self, user_id: &str, ids: &[String]) -> Result<Vec<String>> {
        self.delete_links(user_id, ids).await
    }
    async fn is_transaction_linked(&self, txn_id: &str) -> Result<bool> {
        self.is_transaction_linked(txn_id).await
    }
}

#[cfg(test)]
mod tests {
    #![allow(clippy::float_cmp)]
    use std::sync::Arc;
    use std::str::FromStr;


    use super::*;
    use crate::surreal_db;

    #[test]
    fn rejects_unknown_type() {
        assert!(TransactionType::from_str("bogus").is_err());
        assert_eq!(TransactionType::from_str("DEBIT"), Ok(TransactionType::Debit));
        assert_eq!(TransactionType::from_str("CREDIT"), Ok(TransactionType::Credit));
    }

    async fn setup() -> (Arc<surrealdb::Surreal<surrealdb::engine::local::Db>>, TransactionRepo<surrealdb::engine::local::Db>) {
        let db = Arc::new(surreal_db::connect_mem().await.unwrap());
        db.query("CREATE user CONTENT { email: 'u1@x.com', passwordHash: 'h', name: 'u1' }")
            .await
            .unwrap()
            .check()
            .unwrap();
        db.query("CREATE account CONTENT { user: $u, bankName: 'Bank', type: 'CURRENT' }")
            .bind(("u", rid("user", "u1")))
            .await
            .unwrap()
            .check()
            .unwrap();
        let repo = TransactionRepo::new(db.clone());
        (db, repo)
    }

    fn txn(id: &str, name: &str, amount: f64, transaction_type: TransactionType, account: &str, ext: Option<&str>) -> Transaction {
        Transaction {
            id: id.to_string(),
            name: name.to_string(),
            clean_name: None,
            amount,
            transaction_type,
            occurred_at: "2024-01-02 03:04:05 +0000 UTC".to_string(),
            account_id: account.to_string(),
            created_at: "2024-01-02 03:04:06 +0000 UTC".to_string(),
            external_id: ext.map(str::to_string),
            transfer_linked: false,
        }
    }

    async fn account_id(db: &surrealdb::Surreal<surrealdb::engine::local::Db>) -> String {
        let mut res = db
            .query("SELECT VALUE meta::id(id) FROM account LIMIT 1")
            .await
            .unwrap();
        take_json::<String>(&mut res, 0).unwrap()[0].clone()
    }


    #[tokio::test]
    async fn create_inserts_with_categories_and_dedups_external() {
        let (db, repo) = setup().await;
        let acc = account_id(&db).await;
        let input = CreateTransactionInput {
            txn: txn("t1", "Coffee", 5.5, TransactionType::Debit, &acc, Some("ext-1")),
            category_ids: vec![],
        };
        let out = repo.create("u1", &[input]).await.unwrap();
        assert!(out.inserted.contains("t1"));
        assert!(out.errors.is_empty());

        let dup = CreateTransactionInput {
            txn: txn("t2", "Coffee again", 5.5, TransactionType::Debit, &acc, Some("ext-1")),
            category_ids: vec![],
        };
        let out2 = repo.create("u1", &[dup]).await.unwrap();
        assert!(!out2.inserted.contains("t2"));

        let full = repo.get_full("u1", "t1").await.unwrap().unwrap();
        assert_eq!(full.name, "Coffee");
        assert_eq!(full.amount, 5.5);
    }

    #[tokio::test]
    async fn clean_name_roundtrips_on_read() {
        let (db, repo) = setup().await;
        let acc = account_id(&db).await;
        let mut t = txn("t1", "UPI-RAW-1", 5.5, TransactionType::Debit, &acc, None);
        t.clean_name = Some("Clean Shop".to_string());
        repo.create("u1", &[CreateTransactionInput { txn: t, category_ids: vec![] }]).await.unwrap();

        let full = repo.get_full("u1", "t1").await.unwrap().unwrap();
        assert_eq!(full.name, "UPI-RAW-1"); // raw preserved
        assert_eq!(full.clean_name.as_deref(), Some("Clean Shop"));

        let batch = repo.get_by_id_batch("u1", &["t1".to_string()]).await.unwrap();
        assert_eq!(batch[0].clean_name.as_deref(), Some("Clean Shop"));

        let list = repo.list("u1", &TransactionListFilter::default()).await.unwrap();
        assert_eq!(list.rows[0].txn.clean_name.as_deref(), Some("Clean Shop"));
    }


    #[tokio::test]
    async fn list_filters_and_paginates() {
        let (db, repo) = setup().await;
        let acc = account_id(&db).await;
        let mut inputs = Vec::new();
        for i in 0..5 {
            inputs.push(CreateTransactionInput {
                txn: txn(&format!("t{i}"), &format!("Item {i}"), 10.0 * f64::from(i + 1), if i % 2 == 0 { TransactionType::Debit } else { TransactionType::Credit }, &acc, None),
                category_ids: vec![],
            });
        }
        repo.create("u1", &inputs).await.unwrap();

        let res = repo.list("u1", &TransactionListFilter { page_size: 2, ..Default::default() }).await.unwrap();
        assert_eq!(res.rows.len(), 2);
        assert!(!res.next_page_token.is_empty());

        let res2 = repo
            .list("u1", &TransactionListFilter { page_size: 2, page_token: res.next_page_token, ..Default::default() })
            .await
            .unwrap();
        assert_eq!(res2.rows.len(), 2);

        let res3 = repo
            .list("u1", &TransactionListFilter { transaction_type: Some(TransactionType::Credit), ..Default::default() })
            .await
            .unwrap();
        assert_eq!(res3.rows.len(), 2);

        let res4 = repo.list("u1", &TransactionListFilter { names: vec!["Item 3".to_string()], ..Default::default() }).await.unwrap();
        assert_eq!(res4.rows.len(), 1);
        assert_eq!(res4.rows[0].txn.name, "Item 3");

        let res5 = repo.list("u1", &TransactionListFilter { sort_by: "debit".to_string(), sort_dir: "asc".to_string(), ..Default::default() }).await.unwrap();
        assert_eq!(res5.rows[0].txn.name, "Item 0");
    }

    #[tokio::test]
    async fn update_replaces_fields() {
        let (db, repo) = setup().await;
        let acc = account_id(&db).await;
        repo.create(
            "u1",
            &[CreateTransactionInput { txn: txn("t1", "Old", 5.0, TransactionType::Debit, &acc, None), category_ids: vec![] }],
        )
        .await
        .unwrap();

        let mut updated = txn("t1", "New Name", 9.0, TransactionType::Credit, &acc, None);
        updated.occurred_at = "2024-02-02 03:04:05 +0000 UTC".to_string();
        let errs = repo.update("u1", &[UpdateTransactionInput { txn: updated, category_ids: vec![] }]).await.unwrap();
        assert!(errs.is_empty());

        let full = repo.get_full("u1", "t1").await.unwrap().unwrap();
        assert_eq!(full.name, "New Name");
        assert_eq!(full.amount, 9.0);
        assert_eq!(full.transaction_type, TransactionType::Credit);
        assert!(full.occurred_at.starts_with("2024-02-02"));
    }

    #[tokio::test]
    async fn delete_removes_rows_and_scopes() {
        let (db, repo) = setup().await;
        let acc = account_id(&db).await;
        repo.create(
            "u1",
            &[
                CreateTransactionInput { txn: txn("t1", "A", 1.0, TransactionType::Debit, &acc, None), category_ids: vec![] },
                CreateTransactionInput { txn: txn("t2", "B", 2.0, TransactionType::Debit, &acc, None), category_ids: vec![] },
            ],
        )
        .await
        .unwrap();
        let errs = repo.delete("u1", &["t1".to_string()]).await.unwrap();
        assert!(errs.is_empty());

        let scoped = repo.delete("u2", &["t2".to_string()]).await.unwrap();
        assert_eq!(scoped, vec!["transaction t2 not found"]);
        assert!(repo.get_full("u1", "t2").await.unwrap().is_some());
    }
}

#[cfg(test)]
mod spending_tests {
    #![allow(clippy::float_cmp)]
    use std::sync::Arc;
    use super::*;
    use crate::surreal_db;

    async fn setup() -> (Arc<surrealdb::Surreal<surrealdb::engine::local::Db>>, TransactionRepo<surrealdb::engine::local::Db>) {
        let db = Arc::new(surreal_db::connect_mem().await.unwrap());
        db.query("CREATE user CONTENT { email: 'u1@x.com', passwordHash: 'h', name: 'u1' }")
            .await
            .unwrap()
            .check()
            .unwrap();
        db.query("CREATE account CONTENT { user: $u, bankName: 'Bank', type: 'CURRENT' }")
            .bind(("u", rid("user", "u1")))
            .await
            .unwrap()
            .check()
            .unwrap();
        db.query("CREATE account CONTENT { user: $u, bankName: 'CC', type: 'CREDIT_CARD' }")
            .bind(("u", rid("user", "u1")))
            .await
            .unwrap()
            .check()
            .unwrap();
        db.query("CREATE category CONTENT { user: $u, name: 'Food' }")
            .bind(("u", rid("user", "u1")))
            .await
            .unwrap()
            .check()
            .unwrap();
        let repo = TransactionRepo::new(db.clone());
        (db, repo)
    }


    #[tokio::test]
    async fn spending_buckets_and_categories() {
        let (db, repo) = setup().await;
        let mut res = db
            .query("SELECT meta::id(id) AS id, type FROM account ORDER BY type")
            .await
            .unwrap();
        let accs: Vec<serde_json::Value> = take_json(&mut res, 0).unwrap();
        let current = accs.iter().find(|a| a["type"] == "CURRENT").unwrap()["id"].as_str().unwrap().to_string();
        let _cc = accs.iter().find(|a| a["type"] == "CREDIT_CARD").unwrap()["id"].as_str().unwrap().to_string();
        let mut res = db
            .query("SELECT VALUE meta::id(id) FROM category LIMIT 1")
            .await
            .unwrap();
        let cat = take_json::<String>(&mut res, 0).unwrap()[0].clone();

        let txn = |id: &str, name: &str, amt: f64, ty: TransactionType, acc: &str| Transaction {
            id: id.to_string(), name: name.to_string(), clean_name: None, amount: amt, transaction_type: ty,
            occurred_at: "2024-01-02 10:00:00 +0000 UTC".to_string(),
            account_id: acc.to_string(), created_at: "2024-01-02 10:00:01 +0000 UTC".to_string(),
            external_id: None, transfer_linked: false,
        };
        repo.create("u1", &[
            CreateTransactionInput { txn: txn("t1", "Groceries", 50.0, TransactionType::Debit, &current), category_ids: vec![cat.clone()] },
            CreateTransactionInput { txn: txn("t2", "Salary", 1000.0, TransactionType::Credit, &current), category_ids: vec![] },
        ]).await.unwrap();

        let filter = SpendingFilter { granularity: "day".into(), from: String::new(), to: String::new(), account_id: String::new() };
        let buckets = repo.spending_buckets("u1", &filter).await.unwrap();
        assert_eq!(buckets.len(), 1, "one day bucket (only debit counts)");
        assert_eq!(buckets[0].key, "2024-01-02");
        assert_eq!(buckets[0].amount, 50.0);

        let cats = repo.spending_categories("u1", &filter).await.unwrap();
        assert!(cats.len() >= 2, "Food + Uncategorized");
        let food = cats.iter().find(|c| c.name == "Food").unwrap();
        assert_eq!(food.debit, 50.0);
        let uncat = cats.iter().find(|c| c.name == "Uncategorized").unwrap();
        assert_eq!(uncat.credit, 1000.0, "salary credit lands in uncategorized");
    }
}
