# Architecture — Layering

`src/repo/` (SQL) → `src/service/` (business logic) → `src/http/` (JSON mapping + handlers). Repos are `sqlx` implementations in `src/repo/sqlite/` over a single-writer pool plus a read pool (`src/repo/db.rs`). Schema is versioned SQL migrations in `db/migrations/`, applied at boot by `sqlx::migrate!`. Bind every value with `?` placeholders, never string-concatenate SQL.
