# Roadmap — Financer (Go → Rust Backend Migration)

How the Go backend is replaced by a Rust backend, following the **strangler pattern**: the Rust server becomes the entry point early and takes over routes feature by feature, until the Go backend is idle and removed. **The Vue frontend is untouched** — it only talks JSON over HTTP, so the migration is backend-only, keeping the **exact wire contract** (`/api/...` request/response shapes, error envelope `{code, message}`, status codes). The frontend must keep working at every merge.

**Data migration is free:** both backends use the **same SQLite file and schema** (WAL). The Rust side reuses the existing `migrations/*.sql` and the same `data/` layout, so there is no data copy — the file is the contract. Any future schema change follows expand/contract (additive first, drop/rename in their own deploy), never in place.

## Migration strategy

1. **Build the replacement behind a gateway.** From Phase 1 the Rust server serves the static frontend and owns the routes it has implemented; everything else under `/api/*` is **proxied to the Go backend** running alongside. Both processes share the SQLite file (WAL, `busy_timeout`; low-traffic personal scale).
2. **Feature-flag route ownership.** Each phase flips its endpoint group to Rust-owned (a hardcoded owned-route set, overridable via `FINANCER_RUST_ROUTES` for canary). Go still serves the routes Rust doesn't own yet.
3. **Parity is the gate.** Every migrated route is verified **wire-identical** to the Go reference: recorded fixtures from the current Go backend diffed byte-for-byte (JSON + status), re-run in CI. No frontend change unless the contract forces it (it shouldn't — we own the consumer, so the Churn Rule means *we* absorb migration cost, not the frontend).
4. **Remove Go last.** Only when Rust owns 100% of routes and the Go process has been idle for the parity suite is Go removed (code, tests, docs, container).

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
- **Done when:** transaction lifecycle + transfers are wire-identical; balances stay correct.

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
