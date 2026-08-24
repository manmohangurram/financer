# Architecture — Layering

`src/repo/` (SQL) → `src/service/` (business logic) → `src/http/` (JSON mapping + handlers). Repos use `surrealdb` v3 against a server-mode HTTP-RPC client (`src/surreal_db.rs` `connect`), generic over `C: Connection` so tests use the embedded `Mem` engine (`connect_mem`). Schema is idempotent `define_tables()` SurrealQL run at boot — no migration files. Use `db.query(...)` with bound params (`$param`), never string-concatenate values.
