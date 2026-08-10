# Roadmap — Financer (Go → Rust Backend Migration)

How the Go backend is replaced by a Rust backend, following the **strangler pattern**: the Rust server becomes the entry point early and takes over routes feature by feature, until the Go backend is idle and removed. The **Vue frontend is untouched** except where the contract deliberately changes — it only talks JSON over HTTP, so the migration is backend-only.

**Wire-compat is relaxed, not strict.** The migration does **not** need to be byte-for-byte identical to the Go wire. As long as the same query/filter semantics and functionality are maintained, field and enum values may be **renamed to match code standards** (e.g. `accountType`/`type` become lowercase strings `checking`/`debit` instead of proto-style `ACCOUNT_TYPE_CHECKING` and ints `0`/`1`). The error envelope (`{code, message}`) and status codes stay. The frontend is updated alongside any renamed values; UI behavior is preserved.

**Enum values are strings, not ints.** Enums travel and are stored as stable lowercase strings (`"debit"`, `"credit"`, `"checking"`, …) — never positional ints (`0`/`1`). Storing the string decouples stored data from enum ordering: reordering/inserting a variant later cannot corrupt existing rows. Request bodies deserialize directly into the Rust enum so invalid values are rejected with `400`. Old int data is converted by a DB migration.

**Data migration is free:** both backends use the **same SQLite file and schema** (WAL). The Rust side reuses the existing `migrations/*.sql` and the same `data/` layout, so there is no data copy — the file is the contract. Not production: schema edits (column type changes, renames) may update a migration in place and reset the dev DB; where real data exists, a conversion migration maps old values to new.

## Migration strategy

1. **Build the replacement behind a gateway.** From Phase 1 the Rust server serves the static frontend and owns the routes it has implemented; everything else under `/api/*` is **proxied to the Go backend** running alongside. Both processes share the SQLite file (WAL, `busy_timeout`; low-traffic personal scale).
2. **Feature-flag route ownership.** Each phase flips its endpoint group to Rust-owned (a hardcoded owned-route set, overridable via `FINANCER_RUST_ROUTES` for canary). Go still serves the routes Rust doesn't own yet.
3. **Parity is the gate — relaxed.** Every migrated route must preserve **query/filter semantics and functionality** (same fields, same filtering behavior, same status codes, same error envelope). Wire values may be renamed to standards (e.g. lowercase string enums) as long as the frontend is updated in the same merge. Fixture diffing is a tool, not a hard byte-for-byte gate.
4. **Remove Go last.** Only when Rust owns 100% of routes and the Go process has been idle for the parity suite is Go removed (code, tests, docs, container). Until then Go may degrade silently on columns the Rust migration renames (its int queries match nothing); that is accepted and unowned routes are rebuilt in Rust phases.

## Conventions (apply to every phase)

- **Commit style:** [Conventional Commits](https://www.conventionalcommits.org) — `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `perf:`, `chore:`. One logical change per commit.
- **Branching:** every phase ships on a feature branch (`feat/rust-<phase>`) merged to `main` via PR after tests pass.
- **Merge gate:** `cargo build`, `cargo clippy -- -D warnings`, `cargo test`, `go test ./...` (still green until Go is removed), `npm run build`/`npm test` (frontend unchanged), plus a browser smoke test.
- **Review before merge:** `ponytail-review`, `code-simplification`, `code-review-and-quality`, `performance-optimization`, `finishing-a-development-branch`; fold fixes into the branch.
- **PR template:** `.github/PULL_REQUEST_TEMPLATE.md`.
- **Per-feature files:** each feature owns its handler, service, and repository files (`src/http/<feature>.rs`, `src/service/<feature>.rs`, `src/repo/<feature>.rs`).

## Target stack

| Concern | Choice | Notes |
|---|---|---|
| HTTP | `axum` on `tokio` | HTTP/1.1 + HTTP/2 (h2c); optional HTTP/3 behind TLS |
| Router / extractors | axum (`Router`, `Path`, `Query`, `Json`, `State`) | mirrors Go `http.ServeMux` + `PathValue` |
| Reverse proxy (strangler) | `axum`/`hyper` forward to the Go backend | routes Rust doesn't own yet |
| DB | `sqlx` (async, `SQLite`) | same file as Go; WAL, write pool (1 conn) + read pool, `busy_timeout` |
| Migrations | `sqlx::migrate!` reusing the existing `migrations/*.sql` | same filename-ordered, tracked schema as Go |
| JWT | `jsonwebtoken` | HS256, same claims + bearer middleware |
| HTTP client (Yahoo) | `reqwest` | base retry + `.NS` fallback, JSON config |
| Logging | `tracing` + `tracing-subscriber` | replaces `logx` |
| Config | env vars, same names | `FINANCER_*` unchanged |
| Static frontend | `axum` serving `frontend/dist` (embedded via `rust-embed`) | SPA fallback + `FINANCER_DOMAIN_URL` injection |
| Testing | `#[tokio::test]` + in-process SQLite | unit + integration mirrors of Go's `repository`/`services` tests |

---

## Phase 1 — Foundation + strangler gateway
`feat/rust-foundation` → `main`
- Cargo workspace + `axum`/`tokio` server on `FINANCER_ADDR`, graceful shutdown, `tracing`.
- SQLite via `sqlx`: same file as Go, WAL, write/read pools, `FINANCER_DATA_DIR` path resolution, embedded `migrations/*.sql` auto-applied.
- **Strangler gateway:** Rust serves the built `frontend/dist` (SPA fallback, `FINANCER_DOMAIN_URL` injection) and proxies every `/api/*` route it doesn't own to the Go backend (which still runs). Owned-route set + `FINANCER_RUST_ROUTES` env override.
- JWT auth: `POST /api/auth/{signup,login,refresh}`, `GET /api/me`, bearer middleware — served from Rust from the start.
- **Done when:** both processes run in the container, auth round-trips are **byte-identical** to Go (fixture diff), and all unowned routes still work through the proxy.

## Phase 2 — Accounts
`feat/rust-accounts` → `main`
- Accounts CRUD (types: Checking, Savings, Credit Card, Loan, Crypto Wallet) with stored balance, user-scoped. Owned routes flip off the proxy.
- **Done when:** create/edit/delete/list accounts match Go's responses exactly (fixture diff passes).

## Phase 3 — Transactions & Transfers
`feat/rust-transactions` → `main`
- Transactions CRUD: create (single + bulk), list with server-side filters (`names` OR-match, date range, amount, category, type) and pagination, update, delete; account balance recalc.
- Transfers: link (debit ↔ credit), unlink, create missing counterpart (±5 days / ±10% amount).
- **Done when:** transaction lifecycle + transfers match Go's query/filter behavior; balances stay correct.

## Phase 3.5 — String-based enums
`feat/rust-type-strings` → `main`
- Transaction `type` and account `accountType` become **lowercase string enums** on the wire and in the DB: `"debit"`/`"credit"`, `"checking"`/`"savings"`/`"credit_card"`/`"loan"`. DB columns `type TEXT`; a conversion migration maps any old ints to strings (dev DB reset acceptable).
- Request bodies deserialize directly into the Rust enum; unknown values → `400 invalid_argument`. Frontend sends/receives the lowercase values (drop unsupported Crypto Wallet).
- Go not updated for renamed columns (being replaced); its unowned-route int queries degrade silently.
- **Done when:** wire + DB carry strings only, enums validate strictly, balances/query behavior unchanged, frontend functional.

## Phase 4 — Categories & Rules
`feat/rust-rules` → `main`
- Categories CRUD (bulk); transaction → category mapping.
- Rules: conditions (name/amount/type/category/account; contains/starts-with/ends-with/equals/gt/lt/regex; AND/OR), outputs (rename / set category / transfer-to-account), read-time non-destructive overlay, preview, run-now.
- **Done when:** rule overlay output matches Go byte-for-byte for the same data.

## Phase 5 — User Settings
`feat/rust-settings` → `main`
- Profile: view/edit name/email, avatar upload (`multipart`), change password, logout-all-sessions.
- **Done when:** profile + avatar + password flows match Go.

## Phase 6 — Investments
`feat/rust-investments` → `main`
- Instruments (stock/mutual fund), buy/sell lots, FIFO cost basis + P&L, symbol search, Yahoo quote/history fetch (`reqwest`, base retry, `.NS` fallback) + cache, price-history chart, portfolio summary, quote-on-add, bulk import (`POST /api/investments/import`, merge by symbol, idempotent `external_id`).
- **Done when:** FIFO math + Yahoo flows match Go; import/update/lot-edit endpoints identical.

## Phase 7 — Spending & Dashboard
`feat/rust-analytics` → `main`
- SQL aggregation: `GET /api/spending` (day/month buckets, per-category debit/credit/net, transfer exclusion) and `GET /api/dashboard` (balance/income/expense, portfolio).
- **Done when:** buckets/categories match a hand-checked SQL query (same as Go) and dashboard matches Go.

## Phase 8 — Cutover: Go idle, then removed
`feat/rust-cutover` → `main`
- Full parity sweep: every route owned by Rust; the Go backend is idle (proxy forwards nothing). Run the complete fixture diff — zero mismatches.
- Replace the Go binary in the Docker image / GHCR workflow with the Rust build (multi-arch `arm64`/`amd64`, no CGO), self-host docs updated.
- **Remove the Go backend:** `main.go`, `services/`, `repository/`, `httpserver/`, `api/`, `auth/`, `db/`, `cmd/seed`, and the proxy path — code, tests, and docs. Drop the `go test ./...` gate.
- **Done when:** the container runs only the Rust binary end-to-end, the browser smoke passes, and no Go code or references remain in `main`.

## Phase 9 — Cross-cutting quality
`feat/rust-quality` → `main`
- Full Rust test pass (unit + integration), `cargo clippy -D warnings` clean, `cargo audit`.
- Performance pass: `axum`/`sqlx` hot paths, batch queries, no N+1.
- **Done when:** all tests green and the audit has no open high-severity findings.
