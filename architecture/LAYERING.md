# Architecture — Layering

`src/repo/` (SQL) → `src/service/` (business logic) → `src/http/` (JSON mapping + handlers). Repos use `sqlx` against a `Db` holding **separate write (1 conn) and read pools** (`src/db.rs`, `open_pools` — SQLite WAL, single-writer). Migrations are raw SQL in `db/migrations/`, applied by sqlx's built-in migrator (`src/db.rs` `MIGRATOR`, tracked in `_sqlx_migrations`). Use `sqlx::query`/`query_as` with bound params — never string-concatenate values.
