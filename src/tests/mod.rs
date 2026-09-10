//! In-crate integration tests: build a SQLite-backed `AppState` and drive the
//! axum router in-process. Separate from unit tests in `src/**`.
#![cfg(test)]
#![allow(clippy::float_cmp)] // exact round-number amounts are the point of these assertions

pub mod harness;

pub mod accounts;
pub mod analytics;
pub mod auth;
pub mod categories;
pub mod rules;
pub mod transactions;
