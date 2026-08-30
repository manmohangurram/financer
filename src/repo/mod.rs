//! Repository layer — backend-agnostic. `repo/mod.rs` declares the module tree
//! and re-exports the shared repo traits. Concrete repos live in `surreal/` and
//! `sqlite/`, each implementing the corresponding `traits::*Repo` trait.

pub mod db;
pub mod sqlite;
pub mod surreal;
pub mod traits;

