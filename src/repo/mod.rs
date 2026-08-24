//! Repository layer — `SurrealDB` repos. `repo/mod.rs` is just the module tree;
//! each repo lives in its own file.

pub mod account;
pub mod category;
pub mod investment;
pub mod rule;
pub mod surreal;
pub mod transaction;
pub mod user;

pub use user::UserRepo;
