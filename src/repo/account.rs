//! Accounts repository — CRUD on `SurrealDB`, mirroring the Go backend semantics.

use surrealdb::Connection;
use utoipa::ToSchema;

use crate::error::Result;
use crate::repo::surreal::{rid, take_json, DbClient, RepoConn};
use crate::timex::go_ts;

/// Account type. Wire value is the uppercase string (`CURRENT`/`CREDIT_CARD`).
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, serde::Serialize, serde::Deserialize, strum::Display, strum::EnumString, ToSchema,
)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[strum(serialize_all = "SCREAMING_SNAKE_CASE")]
#[schema(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum AccountType {
    Current,
    Savings,
    Loan,
    CreditCard,
}

#[derive(Clone)]
pub struct AccountRepo<C: Connection = DbClient> {
    db: RepoConn<C>,
}

impl<C: Connection> AccountRepo<C> {
    pub fn new(db: RepoConn<C>) -> Self {
        Self { db }
    }

    /// Insert a new account; returns the row (id = generated record id).
    #[allow(clippy::cast_precision_loss)]
    pub async fn create(
        &self,
        user_id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: AccountType,
    ) -> Result<AccountRow> {
        let now = go_ts(chrono::Utc::now());

        let mut res = self
            .db
            .query(
                "CREATE account CONTENT {
                    user: $uid, bankName: $bank, nickname: $nick, balance: 0.0,
                    totalCredit: 0.0, totalDebit: 0.0, type: $type, createdAt: $created
                } RETURN meta::id(id) AS id, bankName, nickname, balance, type, createdAt",
            )
            .bind(("uid", rid("user", user_id)))
            .bind(("bank", bank_name.to_string()))
            .bind(("nick", nickname.to_string()))
            .bind(("type", account_type.to_string()))
            .bind(("created", now.clone()))
            .await?
            .check()?;

        let row = take_json::<AccountRow>(&mut res, 0)?
            .into_iter()
            .next()
            .ok_or_else(|| crate::error::ApiError::internal("account create returned no row"))?;
        Ok(AccountRow { created_at: now, ..row })
    }

    pub async fn get_by_id(&self, user_id: &str, id: &str) -> Result<Option<AccountRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, bankName, nickname, balance, type, createdAt
                 FROM account WHERE id = $rid AND user = $uid LIMIT 1",
            )
            .bind(("rid", rid("account", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json(&mut res, 0)?.into_iter().next())
    }

    pub async fn update(
        &self,
        user_id: &str,
        id: &str,
        bank_name: &str,
        nickname: &str,
        account_type: AccountType,
    ) -> Result<Option<AccountRow>> {
        let mut res = self
            .db
            .query(
                "UPDATE $rid SET
                    bankName = IF $bank != '' THEN $bank ELSE bankName END,
                    nickname = IF $nick != '' THEN $nick ELSE nickname END,
                    type = $type
                 WHERE user = $uid
                 RETURN meta::id(id) AS id, bankName, nickname, balance, type, createdAt",
            )
            .bind(("rid", rid("account", id)))
            .bind(("uid", rid("user", user_id)))
            .bind(("bank", bank_name.to_string()))
            .bind(("nick", nickname.to_string()))
            .bind(("type", account_type.to_string()))
            .await?;
        let rows = take_json::<AccountRow>(&mut res, 0)?;
        Ok(rows.into_iter().next())
    }

    pub async fn delete(&self, user_id: &str, id: &str) -> Result<bool> {
        let mut res = self
            .db
            .query("DELETE $rid WHERE user = $uid RETURN BEFORE")
            .bind(("rid", rid("account", id)))
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(!take_json::<serde_json::Value>(&mut res, 0)?.is_empty())
    }

    pub async fn list(&self, user_id: &str) -> Result<Vec<AccountRow>> {
        let mut res = self
            .db
            .query(
                "SELECT meta::id(id) AS id, bankName, nickname, balance, type, createdAt
                 FROM account WHERE user = $uid ORDER BY createdAt DESC",
            )
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json(&mut res, 0)?)
    }

    /// Apply a signed delta to an account's stored balance.
    pub async fn update_balance(&self, account_id: &str, delta: f64) -> Result<()> {
        self.db
            .query("UPDATE $rid SET balance = math::round((balance + $delta) * 100) / 100")
            .bind(("rid", rid("account", account_id)))
            .bind(("delta", delta))
            .await?
            .check()?;
        Ok(())
    }

    /// Apply signed deltas to an account's cached credit/debit totals.
    pub async fn apply_totals(&self, account_id: &str, credit: f64, debit: f64) -> Result<()> {
        self.db
            .query(
                "UPDATE $rid SET
                    totalCredit = math::round((totalCredit + $credit) * 100) / 100,
                    totalDebit = math::round((totalDebit + $debit) * 100) / 100",
            )
            .bind(("rid", rid("account", account_id)))
            .bind(("credit", credit))
            .bind(("debit", debit))
            .await?
            .check()?;
        Ok(())
    }

    /// Total stored balance across a user's accounts.
    pub async fn sum_balance(&self, user_id: &str) -> Result<f64> {
        let mut res = self
            .db
            .query("SELECT math::sum(balance) AS total FROM account WHERE user = $uid GROUP ALL")
            .bind(("uid", rid("user", user_id)))
            .await?;
        Ok(take_json::<SumRow>(&mut res, 0)?.first().map_or(0.0, |r| r.total))
    }

    /// Cached total credits and debits across a user's accounts.
    pub async fn sum_totals(&self, user_id: &str) -> Result<(f64, f64)> {
        let mut res = self
            .db
            .query(
                "SELECT math::sum(totalCredit) AS credit, math::sum(totalDebit) AS debit
                 FROM account WHERE user = $uid GROUP ALL",
            )
            .bind(("uid", rid("user", user_id)))
            .await?;
        let row = take_json::<TotalsRow>(&mut res, 0)?.into_iter().next().unwrap_or_default();
        Ok((row.credit, row.debit))
    }
}

#[derive(serde::Deserialize)]
struct SumRow {
    total: f64,
}

#[derive(serde::Deserialize, Default)]
struct TotalsRow {
    credit: f64,
    debit: f64,
}

/// A stored account row (serde wire shape: camelCase).
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AccountRow {
    pub id: String,
    pub bank_name: String,
    pub nickname: String,
    pub balance: f64,
    #[serde(rename = "type")]
    pub account_type: AccountType,
    pub created_at: String,
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;
    use std::str::FromStr;
    use surrealdb::Surreal;

    use super::*;
    use crate::surreal_db;


    #[test]
    fn rejects_unknown_type() {
        assert!(AccountType::from_str("bogus").is_err());
        assert_eq!(AccountType::from_str("CURRENT"), Ok(AccountType::Current));
        assert_eq!(AccountType::from_str("CREDIT_CARD"), Ok(AccountType::CreditCard));
    }

    #[tokio::test]
    async fn crud_scoped_to_user() {
        let db = Arc::new(surreal_db::connect_mem().await.unwrap());
        for uid in ["u1", "u2"] {
            db.query("CREATE user CONTENT { id: $id, email: $email, passwordHash: 'h', name: $id }")
                .bind(("id", uid.to_string()))
                .bind(("email", format!("{uid}@x.com")))
                .await
                .unwrap()
                .check()
                .unwrap();
        }
        let repo = AccountRepo::new(db);
        let a = repo.create("u1", "Chase", "Main", AccountType::Current).await.unwrap();
        repo.create("u1", "Amex", "", AccountType::CreditCard).await.unwrap();
        repo.create("u2", "Other", "", AccountType::Savings).await.unwrap();

        let list = repo.list("u1").await.unwrap();
        assert_eq!(list.len(), 2, "only u1's accounts");
        let mut types: Vec<String> = list.iter().map(|a| a.account_type.to_string()).collect();
        types.sort();
        assert_eq!(types, vec!["CREDIT_CARD", "CURRENT"], "both of u1's accounts");

        let upd = repo.update("u1", &a.id, "Chase Blue", "New", AccountType::Savings).await.unwrap().unwrap();
        assert_eq!(upd.bank_name, "Chase Blue");
        assert_eq!(upd.nickname, "New");
        assert_eq!(upd.account_type, AccountType::Savings);

        assert!(repo.delete("u1", &a.id).await.unwrap());
        assert!(!repo.delete("u1", &a.id).await.unwrap());
        assert_eq!(repo.list("u1").await.unwrap().len(), 1);
    }



    #[tokio::test]
    async fn balance_and_totals_deltas() {
        let db = Arc::new(surreal_db::connect_mem().await.unwrap());
        db.query("CREATE user CONTENT { id: 'u1', email: 'u1@x.com', passwordHash: 'h', name: 'u1' }")
            .await
            .unwrap()
            .check()
            .unwrap();
        let repo = AccountRepo::new(db);
        let a = repo.create("u1", "Chase", "", AccountType::Current).await.unwrap();

        repo.update_balance(&a.id, -5.5).await.unwrap();
        repo.update_balance(&a.id, 10.0).await.unwrap();
        repo.apply_totals(&a.id, 10.0, 5.5).await.unwrap();

        let row = repo.get_by_id("u1", &a.id).await.unwrap().unwrap();
        assert_eq!(row.balance, 4.5);
        let (c, d) = repo.sum_totals("u1").await.unwrap();
        assert_eq!(c, 10.0);
        assert_eq!(d, 5.5);
    }
}
