//! Repository layer. `repo/mod.rs` declares the module tree and re-exports the
//! shared repo traits. The concrete SQLite repos live in `sqlite/`, each
//! implementing the corresponding `traits::*Repo` trait.

pub mod db;
pub mod sqlite;
pub mod traits;
