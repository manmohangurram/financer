//! `SurrealDB` repo framework — mirrors flowsmith's `repo/mod.rs`: server client
//! type, record-id helper, JSON round-trip deserialization, update-or-create,
//! and the shared repo error.
// TODO(surrealdb): consumed by the repo rewrite (migrate/surrealdb-repos).
#![allow(dead_code)]

use std::sync::Arc;
use surrealdb::types::RecordId;
use surrealdb::{Connection, Surreal};

/// Server-mode `SurrealDB` client (PROD).
pub type DbClient = surrealdb::engine::remote::http::Client;

/// Native record-id (`table:<id>`) for matching `WHERE id = $rid`.
pub fn rid(table: &str, id: &str) -> RecordId {
    RecordId::new(table, id)
}

/// Take query results as JSON and deserialize into `T` with serde.
pub(crate) fn take_json<T: serde::de::DeserializeOwned>(
    res: &mut surrealdb::IndexedResults,
    index: usize,
) -> Result<Vec<T>, RepoError> {
    let vals = res.take::<Vec<serde_json::Value>>(index)?;
    vals.into_iter()
        .map(serde_json::from_value)
        .collect::<Result<_, _>>()
        .map_err(RepoError::Json)
}

/// Update-or-create: run `update`; if it matches no rows, run `create`.
pub(crate) async fn update_or_create<C>(
    db: &Surreal<C>,
    update: &str,
    create: &str,
    bind: impl Fn(surrealdb::method::Query<'_, C>) -> surrealdb::method::Query<'_, C>,
) -> Result<(), RepoError>
where
    C: Connection,
{
    let updated = bind(db.query(update))
        .await?
        .check()?
        .take::<Vec<String>>(0)?
        .len();
    if updated == 0 {
        bind(db.query(create)).await?.check()?;
    }
    Ok(())
}

/// Error raised by repositories. Rows missing or unique collisions map here.
#[derive(Debug, thiserror::Error)]
pub enum RepoError {
    #[error("not found")]
    NotFound,
    #[error("conflict: {0}")]
    Conflict(String),
    #[error("database error: {0}")]
    Database(#[from] anyhow::Error),
    #[error("json error: {0}")]
    Json(#[from] serde_json::Error),
}

impl From<surrealdb::Error> for RepoError {
    fn from(e: surrealdb::Error) -> Self {
        RepoError::Database(e.into())
    }
}

/// Shorthand for repos holding a shared connection (`Arc<Surreal<C>>`).
pub type RepoConn<C> = Arc<Surreal<C>>;
