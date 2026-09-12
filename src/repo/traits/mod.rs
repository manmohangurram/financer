//! Backend-agnostic shared types + repository traits.
//!
//! All row/input/enum types that services consume are defined here ONCE, then
//! imported by both the `SurrealDB` and `SQLite` backends (no duplication).
//! Each file holds one domain: its trait plus its shared data types.

pub mod account;
pub mod category;
pub mod investment;
pub mod rule;
pub mod transaction;
pub mod user;
pub mod user_key;

// Public domain-type surface shared by both backends and the http layer. Not
// every symbol is consumed inside this (binary) crate, so silence the warnings.
#[allow(unused_imports)]
pub use account::{AccountRepo, AccountRow, AccountType};
#[allow(unused_imports)]
pub use category::{CategoryCreateInput, CategoryRepo, CategoryRow, CategoryUpdateInput, ListCategoriesResult};
#[allow(unused_imports)]
pub use investment::{InvestmentRepo, InvestmentRow, InvestmentType, LotInput, LotRow};
#[allow(unused_imports)]
pub use rule::{OverlayRule, Rule, RuleAction, RuleCondition, RuleLogic, RuleRepo};
#[allow(unused_imports)]
pub use transaction::{CreateOutcome, CreateTransactionInput, ListTransactionResult, SpendingFilter, SpendingTxn, Transaction, TransactionListFilter, TransactionRepo, TransactionType, UpdateTransactionInput};
#[allow(unused_imports)]
pub use user::{UserRepo, UserRow};
#[allow(unused_imports)]
pub use user_key::{UserKeyRepo, UserKeyRow};