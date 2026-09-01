//! Backend bootstrapping: `SQLite` pool + migrations, and runtime repo-set
//! selection (`SurrealDB` or `SQLite`).

use std::path::Path;
use std::str::FromStr;
use std::sync::Arc;
use std::time::Duration;

use sqlx::sqlite::{SqliteConnectOptions, SqliteJournalMode, SqlitePoolOptions, SqliteSynchronous};
use sqlx::SqlitePool;

use crate::error::Result;
use crate::repo::surreal;
use crate::repo::traits::{AccountRepo, CategoryRepo, InvestmentRepo, RuleRepo, TransactionRepo, UserKeyRepo, UserRepo};

/// All repositories, type-erased so the service layer is backend-agnostic.
pub struct RepoSet {
    pub user: Arc<dyn UserRepo>,
    pub account: Arc<dyn AccountRepo>,
    pub category: Arc<dyn CategoryRepo>,
    pub rule: Arc<dyn RuleRepo>,
    pub investment: Arc<dyn InvestmentRepo>,
    pub transaction: Arc<dyn TransactionRepo>,
    pub user_key: Arc<dyn UserKeyRepo>,
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
        user_key: Arc::new(surreal::user_key::UserKeyRepo::new(c.clone())),
    })
}

/// Build the repo set for the `SQLite` backend (WAL, write + read pools).
pub async fn sqlite_repo_set(db_path: &str) -> Result<RepoSet> {
    let db = open_pools(Path::new(db_path)).await?;
    run_migrations(&db.write).await?;
    tracing::info!("connected to SQLite at {db_path}");
    let pool = db.write.clone();
    Ok(RepoSet {
        user: Arc::new(crate::repo::sqlite::user::SqliteUserRepo::new(pool.clone())),
        account: Arc::new(crate::repo::sqlite::account::SqliteAccountRepo::new(pool.clone())),
        category: Arc::new(crate::repo::sqlite::category::SqliteCategoryRepo::new(pool.clone())),
        rule: Arc::new(crate::repo::sqlite::rule::SqliteRuleRepo::new(pool.clone())),
        investment: Arc::new(crate::repo::sqlite::investment::SqliteInvestmentRepo::new(pool.clone())),
        transaction: Arc::new(crate::repo::sqlite::transaction::SqliteTransactionRepo::new(pool.clone())),
        user_key: Arc::new(crate::repo::sqlite::user_key::SqliteUserKeyRepo::new(pool)),
    })
}

/// SQLite pools: WAL mode, single-writer write pool + a read pool.
pub struct Db {
    #[allow(dead_code)]
    pub read: SqlitePool,
    pub write: SqlitePool,
}

/// Open the write (single-conn) and read pools against the same `SQLite` file,
/// mirroring Go's `OpenDBs`.
pub async fn open_pools(db_path: &Path) -> anyhow::Result<Db> {
    let base = SqliteConnectOptions::from_str(db_path.to_str().unwrap())
        .unwrap()
        .journal_mode(SqliteJournalMode::Wal)
        .busy_timeout(Duration::from_secs(5))
        .foreign_keys(true)
        .synchronous(SqliteSynchronous::Normal);

    let write_opts = base.clone().create_if_missing(true);
    let write = SqlitePoolOptions::new()
        .max_connections(1)
        .connect_with(write_opts)
        .await?;

    let read_opts = base.clone().read_only(true);
    let read = SqlitePoolOptions::new()
        .max_connections(5)
        .connect_with(read_opts)
        .await?;

    Ok(Db { read, write })
}

/// Apply pending migrations via sqlx's built-in migrator, tracking applied
/// versions in `_sqlx_migrations`. Migrations live in `db/migrations/` as
/// `NNNNNN_description.up.sql` / `.down.sql` pairs (sqlx's convention).
pub static MIGRATOR: sqlx::migrate::Migrator = sqlx::migrate!("./db/migrations");

pub async fn run_migrations(pool: &SqlitePool) -> anyhow::Result<()> {
    MIGRATOR.run(pool).await?;
    Ok(())
}
