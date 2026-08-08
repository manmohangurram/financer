# Roadmap — Financer (Personal Finance Tracker)

How this project is built from the ground up, in development order, with how each piece lands and merges. **All backend/DB work ships first (Phases 1–6); the frontend shell ships together in Phase 7** (shell + UI for everything built in Phases 1–6), and later phases add their own UI. If a missing feature is discovered during development, it is documented here (added to the relevant phase or a new phase) before or alongside the work.

## Conventions (apply to every phase)

- **Commit style:** [Conventional Commits](https://www.conventionalcommits.org) — `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, `perf:`. One logical change per commit; imperative subject line.
- **Branching:** every phase ships on a feature branch (`feat/<phase>`) merged to `main` via a pull request after tests pass. No direct commits to `main`.
- **Merge gate:** `go build ./...`, `go vet ./...`, `go test ./...`, `npm run build`, `npm test` all green, plus a manual browser smoke test.
- **Review before merge:** before opening a PR, run `ponytail-review`, `code-simplification`, `code-review-and-quality`, `performance-optimization`, and `finishing-a-development-branch`; fold any fixes into the branch.
- **PR template:** every pull request uses `.github/PULL_REQUEST_TEMPLATE.md`.
- **Schema per phase:** each phase that touches the database ships its own migration (`NNNNNN_description.up.sql` / `.down.sql`), applied in filename order. The schema builds incrementally in the same order as the features below.
- **Per-feature files:** each feature owns its handler, service, and repository files (e.g. `httpserver/account_handlers.go`, `services/account.go`, `repository/account.go`) rather than one shared `handlers.go` — this keeps features separable so a phase ships only its own files.

---

## Phase 1 — Foundation
`feat/foundation` → `main`
- Go module + `net/http` server skeleton on `FINANCER_ADDR`, graceful shutdown.
- SQLite integration: WAL mode, separate write pool (1 conn, `_txlock=immediate`) and read pool, `FINANCER_DB_PATH`, `FINANCER_JWT_SECRET` env config.
- Embedded migrations (`db/migrations/*.sql`) auto-applied at boot, tracked in `schema_migrations`.
- JWT auth: signup/login/refresh, `GET /api/me`, bearer-token middleware injecting `user_id` into request context.
- **Done when:** server boots, migrations run, auth round-trip works, all future endpoints are user-scoped.

## Phase 2 — Accounts
`feat/accounts` → `main`
- Migration `000002_accounts`: `accounts`.
- Accounts CRUD with account types (Checking, Savings, Credit Card, Loan, Crypto Wallet) and stored balance.
- **Done when:** create/edit/delete/list accounts end-to-end; balances stored and scoped per user. Backend only — UI lands in Phase 5.

## Phase 3 — Transactions & Transfers
`feat/transactions-transfers` → `main`
- Migration `000003_transactions_transfers`: `transactions`, `transfer_links`.
- Transactions CRUD: create (single + bulk), list with server-side filters (date range, amount, name, category) and pagination, update, delete; account balance recalc.
- Transfers: link (debit ↔ credit), unlink, create missing counterpart (candidate matching ±5 days / ±10% amount, fee-tolerant).
- **Done when:** transaction lifecycle works, balances stay correct, transfers link/unlink/create-counterpart work. Backend only — UI lands in Phase 5.

## Phase 4 — Categories & Rules
`feat/categories-rules` → `main`
- Migration `000004_categories_rules`: `categories`, `transaction_categories`, `rules`, `rule_conditions`, `rule_actions`.
- Categories CRUD (bulk); transaction → category mapping.
- Rules: conditions (name/amount/type/category/account; contains/starts-with/ends-with/equals/gt/lt/regex; AND/OR), outputs (rename / set category / transfer-to-account), read-time non-destructive overlay, SQL preview, run-now.
- **Done when:** a rule auto-categorizes on next load and deletes cleanly (no persisted mutation). Backend only — UI lands in Phase 5.

## Phase 5 — User Settings (Backend)
`feat/user-settings` → `main`
- Migration `000005_user_settings`: adds `avatar_url` to `users`.
- Profile page: view/edit display name, email, and profile picture (avatar upload); change password (current + new, re-auth on change); logout-all-sessions.
- Backend: `GET`/`PUT /api/me/profile`, `POST /api/me/password`, avatar upload endpoint.
- **Done when:** name, avatar, and password updates persist and the updated profile shows across the app.

## Phase 6 — Investments (Backend)
`feat/investments-backend` → `main`
- Migration `000006_investments`: `instruments`, `instrument_lots`, `instrument_price_history`.
- Instruments (stock/mutual fund), buy/sell lots, FIFO cost basis and realized P&L, symbol search, Yahoo quote fetch + cache, price-history chart, portfolio summary.
- **Done when:** add lots, refresh prices, and see portfolio P&L match a manual FIFO calculation.

## Phase 7 — Frontend Shell & Design System
`feat/shell` → `main`
- App layout (sidebar nav, responsive), routing, and the `financer` theme (Tailwind v4 + daisyUI v5 tokens).
- **Light + dark theme support:** token-driven per-theme mapping (`color-scheme` toggles), contrast verified independently in both themes, persisted theme toggle.
- Reusable components: `AppModal`, `AppInput`, `AppSelect`, `Pagination`, `ConfirmDialog`, `Popover`, `StatCard`, `AccountCard`.
- **UI for phases 1–6 ships here:** login/signup pages, token store, accounts list/cards + filter bar, transactions table + add/edit modals, unified transfer modal, rules builder/preview/run-now, categories management, profile/settings page.
- `UI_STANDARDS.md` — tokens, spacing, typography, button variants, accessibility baseline.
- **Done when:** every phase 1–6 feature is usable in the shared shell with consistent components, contrast passes in both themes.

## Phase 8 — Dashboard
`feat/dashboard` → `main`
- `GET /api/dashboard` summary: balance/income/expense sums (SQL), accounts, instruments (populated from Phase 6).
- Frontend dashboard: stat cards, account snapshot, portfolio-by-P&L.
- **Done when:** the page loads from a single request, zero client-side math.

## Phase 9 — CSV Import
`feat/csv-import` → `main`
- Client-side CSV parsing + column mapping + preview; bulk create through the transactions API.
- **Done when:** a bank CSV imports with correct category mapping and balances update.

## Phase 10 — Spending Analysis
`feat/spending` → `main`
- No schema change — pure SQL aggregation over `transactions`, `transfer_links`, `accounts`, `categories`.
- `GET /api/spending`: day/month buckets + per-category debit/credit/net, transfer exclusion (debt accounts count as real spending).
- Frontend spending tracker: bar chart (7D/1M/6M/1Y/custom), donut pies with hover, Transactions/Categories tabs, server-sorted drill-down.
- **Done when:** buckets/categories match a hand-checked SQL query for the same data.

## Phase 11 — Transaction Search & Filters
`feat/transaction-filters` → `main`
- No schema change — reuses the existing server-side filter pipeline already shipped in Phase 3 (`GET /api/transactions`: `name`, `nameMatch`, `dateFrom`/`dateTo`, `minAmount`/`maxAmount`, `categoryId`, `type`).
- Restore the missing filter UI in the transactions panel: a **Filter** button (beside Add / Transfer / Select) opening a `Popover` with:
  - Name search (`contains` / `exact`)
  - Date range (`dateFrom` / `dateTo`)
  - Min / max amount
  - Category select
  - Debit / credit type
- Active filters render as removable chips above the table; each chip removes its filter, **Clear all** resets, page resets on change.
- **Done when:** every backend filter param is settable from the UI, chips reflect and clear them, and filtered results round-trip correctly.

## Phase 12 — Cross-Cutting Quality
`feat/quality` → `main`
- Test coverage pass: repository integration tests (in-memory DB harness), service unit tests, frontend vitest for pure helpers.
- Accessibility pass (WCAG 2.1 AA): keyboard nav, labels, focus, reduced motion, contrast — verified in both themes.
- Performance pass: server-side logic audit, bundle review, avoid N+1 in new code.
- **Done when:** full test suite green; ponytail-review and performance-optimization audits have no open high-severity findings.

## Phase 13 — Deployment
`feat/deploy` → `main`
- Single container: the Go server serves both the API and the built Vue frontend (embedded/`frontend/dist`, SPA fallback) — no nginx.
- Multi-stage Docker build (node → frontend dist; Go `CGO_ENABLED=0` static binary for `arm64`/`amd64` via buildx `TARGETARCH`).
- GitHub Action (`docker-publish.yml`) builds and pushes to GHCR (`ghcr.io/manmohangurram/financer`) on push to `main` / tags; Pi runs `docker compose up -d`.
- Runtime data lives under one root (`FINANCER_DATA_DIR`, default `data`, `/data` in container) with `db/`, `certs/`, `config/`, `avatars/` subfolders — a single volume mount persists everything; the bundled Yahoo config is copied to `config/` on first run.
- **Done when:** `docker compose up -d` serves the app and API on a clean Raspberry Pi.

---

## Future / Post-1.0 (not blocking merge)

- Rule "run now" result feedback UI (linked/created counts surfaced).
- Budget targets, month-over-month deltas.
- Investments: dividend/interest tracking.
- Reports: export (CSV/PDF), net-worth history chart.
