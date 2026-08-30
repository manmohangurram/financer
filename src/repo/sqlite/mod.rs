//! `SQLite` repository backend. Each file implements the corresponding
//! backend-agnostic trait (`crate::repo::traits::*Repo`) over an `sqlx::SqlitePool`.

pub mod account;
pub mod category;
pub mod investment;
pub mod rule;
pub mod transaction;
pub mod user;
