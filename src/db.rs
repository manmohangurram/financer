//! `SQLite` database pools + migrations. WAL mode with a single-writer write
//! pool and a read pool. Migrations use sqlx's built-in migrator
//! (`sqlx::migrate!`, tracked in `_sqlx_migrations`).
use std::path::Path;
use std::str::FromStr;
use std::time::Duration;

use sqlx::sqlite::{SqliteConnectOptions, SqliteJournalMode, SqlitePoolOptions, SqliteSynchronous};
use sqlx::SqlitePool;

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
