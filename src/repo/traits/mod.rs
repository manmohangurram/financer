//! Shared types + repository traits.
//!
//! All row/input/enum types that services consume are defined here ONCE, then
//! imported by the `SQLite` repos (no duplication).
//! Each file holds one domain: its trait plus its shared data types.

pub mod account;
pub mod category;
pub mod investment;
pub mod rule;
pub mod transaction;
pub mod user;
pub mod user_key;

// Repository traits, re-exported for `repo::db`, which builds the `RepoSet`
// from them. Domain row/input types are imported from their own module.
pub use account::AccountRepo;
pub use category::CategoryRepo;
pub use investment::InvestmentRepo;
pub use rule::RuleRepo;
pub use transaction::TransactionRepo;
pub use user::UserRepo;
pub use user_key::{UserKeyRepo, UserKeyRow};
