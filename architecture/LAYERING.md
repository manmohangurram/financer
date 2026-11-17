# Architecture — Layering

`internal/repo/` (interfaces + row types) → `internal/repo/sqlite/` (SQL) → `internal/<domain>service/` (business logic) → `internal/server/` (Gin handlers + wire mapping).

- SQLite pools live in `internal/db/sqlite.go`: a single-writer pool (WAL, `MaxOpenConns(1)`) plus a read pool. `internal/db` applies the embedded migrations at boot.
- Schema is versioned SQL in `db/migrations/`, embedded via `migrate.go` (`go:embed`) so the binary is self-contained. Existing sqlx histories are adopted at boot.
- Bind every value with `?` placeholders; never concatenate SQL. Filter by `user_id` in every user-scoped query.
- Services return `*errors.Error` for API failures; `internal/server` renders `{code, message}` with the matching HTTP status.
