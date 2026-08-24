//! `SurrealDB` connection + schema (migration target). Mirrors flowsmith's
//! `db.rs` exactly: server-mode HTTP-RPC client in PROD, embedded `Mem` in
//! tests, idempotent `define_tables()` at boot.
// TODO(surrealdb): consumed by the repo rewrite (migrate/surrealdb-repos).
#![allow(dead_code)]

use anyhow::Result;
use surrealdb::engine::local::{Db, Mem};
use surrealdb::engine::remote::http::{Client as HttpClient, Http};
use surrealdb::{Connection, Surreal};

/// Connect to a `SurrealDB` server over `HTTP-RPC`, sign in, select ns/db, and
/// ensure the schema (tables + indexes) exists.
pub async fn connect(
    url: &str,
    user: &str,
    pass: &str,
    ns: &str,
    db: &str,
) -> Result<Surreal<HttpClient>> {
    let client = Surreal::new::<Http>(url).await?;
    client
        .signin(surrealdb::opt::auth::Root {
            username: user.to_string(),
            password: pass.to_string(),
        })
        .await?;
    client.use_ns(ns).use_db(db).await?;
    define_tables(&client).await?;
    Ok(client)
}

/// Embedded in-memory `SurrealDB` for tests.
pub async fn connect_mem() -> Result<Surreal<Db>> {
    let client = Surreal::new::<Mem>(()).await?;
    client.use_ns("financer").use_db("financer").await?;
    define_tables(&client).await?;
    Ok(client)
}

/// Idempotent schema setup: tables + indexes. Runs at boot and in tests.
pub async fn define_tables<C: Connection>(client: &Surreal<C>) -> Result<()> {
    client
        .query(
            r"
            DEFINE TABLE IF NOT EXISTS user;
            DEFINE INDEX IF NOT EXISTS user_email ON TABLE user COLUMNS email UNIQUE;

            DEFINE TABLE IF NOT EXISTS account;
            DEFINE INDEX IF NOT EXISTS account_user ON TABLE account COLUMNS user;

            DEFINE TABLE IF NOT EXISTS transaction;
            DEFINE INDEX IF NOT EXISTS txn_account ON TABLE transaction COLUMNS account;
            DEFINE INDEX IF NOT EXISTS txn_user ON TABLE transaction COLUMNS user;
            DEFINE INDEX IF NOT EXISTS txn_external ON TABLE transaction COLUMNS user, externalId;

            DEFINE TABLE IF NOT EXISTS category;
            DEFINE INDEX IF NOT EXISTS cat_user ON TABLE category COLUMNS user;

            DEFINE TABLE IF NOT EXISTS rule;
            DEFINE INDEX IF NOT EXISTS rule_user ON TABLE rule COLUMNS user;

            DEFINE TABLE IF NOT EXISTS investment;
            DEFINE INDEX IF NOT EXISTS inv_symbol ON TABLE investment COLUMNS user, symbol;

            DEFINE TABLE IF NOT EXISTS investment_lot;
            DEFINE INDEX IF NOT EXISTS lot_external ON TABLE investment_lot COLUMNS user, externalId;

            DEFINE TABLE IF NOT EXISTS investment_price_history;
            DEFINE TABLE IF NOT EXISTS transfer_link;
            ",
        )
        .await?
        .check()?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn connect_mem_defines_tables() {
        let db = connect_mem().await.unwrap();
        db.query("CREATE user CONTENT { email: 'a@b.c', password_hash: 'x' }")
            .await
            .unwrap()
            .check()
            .unwrap();
        let _ = db;
    }
}
