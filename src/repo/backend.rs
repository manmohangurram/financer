//! Runtime backend selection: builds a `RepoSet` (all six repos boxed as
//! `Arc<dyn Trait>`) from either the `SurrealDB` or `SQLite` backend, decided
//! by which env vars are present.

use std::sync::Arc;

use crate::error::Result;
use crate::repo::surreal;
use crate::repo::traits::{AccountRepo, CategoryRepo, InvestmentRepo, RuleRepo, TransactionRepo, UserRepo};

/// All repositories, type-erased so the service layer is backend-agnostic.
pub struct RepoSet {
    pub user: Arc<dyn UserRepo>,
    pub account: Arc<dyn AccountRepo>,
    pub category: Arc<dyn CategoryRepo>,
    pub rule: Arc<dyn RuleRepo>,
    pub investment: Arc<dyn InvestmentRepo>,
    pub transaction: Arc<dyn TransactionRepo>,
}

/// Build the repo set for the `SurrealDB` backend.
pub async fn surreal_repo_set(
    url: &str,
    user: &str,
    pass: &str,
    ns: &str,
    db: &str,
) -> Result<RepoSet> {
    let client = crate::surreal_db::connect(url, user, pass, ns, db).await?;
    let c = Arc::new(client);
    tracing::info!("connected to SurrealDB at {url}");
    Ok(RepoSet {
        user: Arc::new(surreal::user::UserRepo::new(c.clone())),
        account: Arc::new(surreal::account::AccountRepo::new(c.clone())),
        category: Arc::new(surreal::category::CategoryRepo::new(c.clone())),
        rule: Arc::new(surreal::rule::RuleRepo::new(c.clone())),
        investment: Arc::new(surreal::investment::InvestmentRepo::new(c.clone())),
        transaction: Arc::new(surreal::transaction::TransactionRepo::new(c.clone())),
    })
}

/// Build the repo set for the `SQLite` backend (WAL, write + read pools).
pub async fn sqlite_repo_set(db_path: &str) -> Result<RepoSet> {
    let db = crate::db::open_pools(std::path::Path::new(db_path)).await?;
    crate::db::run_migrations(&db.write).await?;
    tracing::info!("connected to SQLite at {db_path}");
    let pool = db.write.clone();
    Ok(RepoSet {
        user: Arc::new(crate::repo::sqlite::user::SqliteUserRepo::new(pool.clone())),
        account: Arc::new(crate::repo::sqlite::account::SqliteAccountRepo::new(pool.clone())),
        category: Arc::new(crate::repo::sqlite::category::SqliteCategoryRepo::new(pool.clone())),
        rule: Arc::new(crate::repo::sqlite::rule::SqliteRuleRepo::new(pool.clone())),
        investment: Arc::new(crate::repo::sqlite::investment::SqliteInvestmentRepo::new(pool.clone())),
        transaction: Arc::new(crate::repo::sqlite::transaction::SqliteTransactionRepo::new(pool)),
    })
}
