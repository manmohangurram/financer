# Roadmap — Financer (Personal Finance Tracker)

How this project is built from the ground up, in development order, with how each piece lands and merges. **Account-related features ship first; investments and spending analysis come last.** If a missing feature is discovered during development, it is documented here (added to the relevant phase or a new phase) before or alongside the work.

## Conventions (apply to every phase)

- **Commit style:** [Conventional Commits](https://www.conventionalcommits.org) — `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, `perf:`. One logical change per commit; imperative subject line.
- **Branching:** every phase ships on a feature branch (`feat/<phase>`) merged to `main` via a pull request after tests pass. No direct commits to `main`.
- **Merge gate:** `go build ./...`, `go vet ./...`, `go test ./...`, `npm run build`, `npm test` all green, plus a manual browser smoke test.
- **Review before merge:** run a code-quality pass (ponytail-review for over-engineering, performance-optimization for measurable bottlenecks) before each PR.
- **PR template:** every pull request uses `.github/PULL_REQUEST_TEMPLATE.md`.
- **Schema per phase:** each phase that touches the database ships its own migration (`NNNNNN_description.up.sql` / `.down.sql`), applied in filename order. The schema builds incrementally in the same order as the features below.

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
- Migration `000001_users_accounts`: `users`, `accounts`.
- Accounts CRUD with account types (Checking, Savings, Credit Card, Loan, Crypto Wallet) and stored balance.
- Frontend: login/signup pages, token store, accounts list/cards, account filter bar.
- **Done when:** create/edit/delete accounts end-to-end; balances stored and scoped per user.

## Phase 3 — Transactions & Transfers
`feat/transactions-transfers` → `main`
- Migration `000002_transactions_transfers`: `transactions`, `transfer_links`.
- Transactions CRUD: create (single + bulk), list with server-side filters (date range, amount, name, category) and pagination, update, delete; account balance recalc.
- Transfers: link (debit ↔ credit), unlink, create missing counterpart; unified transfer modal with candidate matching (±5 days, ±10% amount, fee-tolerant).
- Frontend: transactions table with sort/filters/pagination, add/edit modals, transfer modal.
- **Done when:** transaction lifecycle works, balances stay correct, transfers link/unlink/create-counterpart work.

## Phase 4 — Categories & Rules
`feat/categories-rules` → `main`
- Migration `000003_categories_rules`: `categories`, `transaction_categories`, `aliases`, `alias_conditions`, `alias_actions`.
- Categories CRUD (bulk); transaction → category mapping.
- Rules: conditions (name/amount/type/category/account; contains/starts-with/ends-with/equals/gt/lt/regex; AND/OR), outputs (rename / set category / transfer-to-account), read-time non-destructive overlay, SQL preview, run-now.
- Frontend: rule builder, preview table, categories management.
- **Done when:** a rule auto-categorizes on next load and deletes cleanly (no persisted mutation).

## Phase 5 — Frontend Shell & Design System
`feat/shell` → `main`
- App layout (sidebar nav, responsive), routing, and the `financer` theme (Tailwind v4 + daisyUI v5 tokens).
- **Light + dark theme support:** token-driven per-theme mapping (`color-scheme` toggles), contrast verified independently in both themes, persisted theme toggle.
- Reusable components: `AppModal`, `AppInput`, `AppSelect`, `Pagination`, `ConfirmDialog`, `Popover`, `StatCard`, `AccountCard`.
- `UI_STANDARDS.md` — tokens, spacing, typography, button variants, accessibility baseline.
- **Done when:** every page renders in the shared shell with consistent components, contrast passes in both themes.

## Phase 6 — User Settings
`feat/settings` → `main`
- Migration `000005_user_settings`: adds `avatar_url` to `users`.
- Profile page: view/edit display name, email, and profile picture (avatar upload); change password (current + new, re-auth on change); logout-all-sessions.
- Backend: `GET`/`PUT /api/me/profile`, `POST /api/me/password`, avatar upload endpoint.
- **Done when:** name, avatar, and password updates persist and the updated profile shows across the app.

## Phase 7 — Dashboard
`feat/dashboard` → `main`
- `GET /api/dashboard` summary: balance/income/expense sums (SQL), accounts, instruments (empty until Phase 9).
- Frontend dashboard: stat cards, account snapshot, portfolio-by-P&L (empty state until investments land).
- **Done when:** the page loads from a single request, zero client-side math.

## Phase 8 — CSV Import
`feat/csv-import` → `main`
- Client-side CSV parsing + column mapping + preview; bulk create through the transactions API.
- **Done when:** a bank CSV imports with correct category mapping and balances update.

## Phase 9 — Spending Analysis
`feat/spending` → `main`
- No schema change — pure SQL aggregation over `transactions`, `transfer_links`, `accounts`, `categories`.
- `GET /api/spending`: day/month buckets + per-category debit/credit/net, transfer exclusion (debt accounts count as real spending).
- Frontend spending tracker: bar chart (7D/1M/6M/1Y/custom), donut pies with hover, Transactions/Categories tabs, server-sorted drill-down.
- **Done when:** buckets/categories match a hand-checked SQL query for the same data.

## Phase 10 — Investments
`feat/investments` → `main`
- Migration `000004_investments`: `instruments`, `instrument_lots`, `instrument_price_history`.
- Instruments (stock/mutual fund), buy/sell lots, FIFO cost basis and realized P&L, symbol search, Yahoo quote fetch + cache, price-history chart, portfolio summary.
- **Done when:** add lots, refresh prices, and see portfolio P&L match a manual FIFO calculation.

## Phase 11 — Cross-Cutting Quality
`feat/quality` → `main`
- Test coverage pass: repository integration tests (in-memory DB harness), service unit tests, frontend vitest for pure helpers.
- Accessibility pass (WCAG 2.1 AA): keyboard nav, labels, focus, reduced motion, contrast — verified in both themes.
- Performance pass: server-side logic audit, bundle review, avoid N+1 in new code.
- **Done when:** full test suite green; ponytail-review and performance-optimization audits have no open high-severity findings.

## Phase 12 — Deployment
`feat/deploy` → `main`
- Multi-stage Docker build (frontend: node build → nginx static; backend: Go binary), nginx reverse proxy for `/api`.
- Production env: `FINANCER_JWT_SECRET` required, optional TLS for HTTP/3.
- **Done when:** `docker compose up` serves the app and API on a clean machine.

---

## Future / Post-1.0 (not blocking merge)

- Rule "run now" result feedback UI (linked/created counts surfaced).
- Budget targets, month-over-month deltas.
- Investments: dividend/interest tracking.
- Reports: export (CSV/PDF), net-worth history chart.
