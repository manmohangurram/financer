//! `SQLite` database pools + migration runner. Replicates the Go backend's
//! `db/db.go` so the two processes share the same `SQLite` file, `WAL`, and
//! `schema_migrations` tracking byte-for-byte.
//!
//! The migration runner mirrors Go exactly (not `sqlx::migrate!`) so both
//! backends record the same `schema_migrations` versions in the shared file.
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

/// Apply pending `.up.sql` migrations in filename order, tracking applied
/// versions in `schema_migrations` — the same algorithm as Go's `RunMigrations`.
pub async fn run_migrations(pool: &SqlitePool, dir: &Path) -> anyhow::Result<()> {
    let mut up: Vec<String> = std::fs::read_dir(dir)?
        .filter_map(Result::ok)
        .map(|e| e.file_name().to_string_lossy().into_owned())
        .filter(|n| n.ends_with(".up.sql"))
        .collect();
    up.sort();

    // Mirror Go: only check the tracking table if it exists (a fresh DB has
    // none; migration 000001 creates it). Otherwise every migration applies.
    let tracking_exists: bool =
        sqlx::query_scalar("SELECT name = 'schema_migrations' FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'")
            .fetch_optional(pool)
            .await?
            .unwrap_or(false);

    for name in up {
        if tracking_exists {
            let applied: bool =
                sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)")
                    .bind(&name)
                    .fetch_one(pool)
                    .await?;
            if applied {
                continue;
            }
        }
        let content = std::fs::read_to_string(dir.join(&name))?;
        let mut tx = pool.begin().await?;
        for stmt in split_sql(&content) {
            sqlx::query(&stmt).execute(&mut *tx).await?;
        }
        sqlx::query("INSERT INTO schema_migrations (version) VALUES (?)")
            .bind(&name)
            .execute(&mut *tx)
            .await?;
        tx.commit().await?;
    }
    Ok(())
}

/// Split SQL into statements on trailing semicolons, skipping `--` comments —
/// mirrors Go's `splitSQL`.
fn split_sql(content: &str) -> Vec<String> {
    let mut out = Vec::new();
    let mut cur = String::new();
    for line in content.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with("--") {
            continue;
        }
        cur.push_str(line);
        cur.push('\n');
        if trimmed.ends_with(';') {
            out.push(std::mem::take(&mut cur));
        }
    }
    if !cur.trim().is_empty() {
        out.push(cur);
    }
    out
}
