# Roadmap — Financer (Personal Finance Tracker)

How this project would be built if started from scratch today, in development order, with how each piece lands and merges.

## Conventions (apply to every phase)

- **Commit style:** [Conventional Commits](https://www.conventionalcommits.org) — `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, `perf:`. One logical change per commit; imperative subject line.
- **Branching:** every phase ships on a feature branch (`feat/<phase>`) merged to `main` via a pull request after tests pass. No direct commits to `main`.
- **Merge gate:** `go build ./...`, `go vet ./...`, `go test ./...`, `npm run build`, `npm test` all green, plus a manual browser smoke test.
- **Review before merge:** run a code-quality pass (ponytail-review for over-engineering, performance-optimization for measurable bottlenecks) before each PR.

---

## Phase 0 — Foundation
`feat/foundation` → `main`
- Go module + `net/http` server skeleton on `FINANCER_ADDR`, graceful shutdown.
- SQLite integration: WAL mode, separate write pool (1 conn, `_txlock=immediate`) and read pool, `FINANCER_DB_PATH`, `FINANCER_JWT_SECRET` env config.
- Embedded migrations (`db/migrations/*.sql`) auto-applied at boot, tracked in `schema_migrations`.
- `GET /api/health`.
- **Done when:** server boots, migrations run, health endpoint responds.

## Phase 1 — Auth
`feat/auth` → `main`
- `users` table, signup/login with bcrypt, JWT access + refresh tokens, `GET /api/me`.
- Auth middleware: bearer-token validation, `user_id` injected into request context; every future query is scoped by user.
- Frontend: login/signup pages, token store (localStorage), route guard.
- **Done when:** signup → login → session persists; all `/api` routes reject unauthenticated requests.

## Phase 2 — Accounts & Categories
`feat/accounts-categories` → `main`
- Accounts CRUD with account types (Checking, Savings, Credit Card, Loan, Crypto Wallet) and stored balance.
- Categories CRUD (bulk create/update/delete).
- Frontend: accounts list/cards, account filter bar, category management.
- **Done when:** create/edit/delete accounts and categories end-to-end.

## Phase 3 — Transactions
`feat/transactions` → `main`
- Transactions CRUD: create (single + bulk), list with server-side filters (date range, amount, name, category) and cursor pagination, update, delete.
- Account balance recalc on create/update/delete (reverse old, apply new per account).
- Transaction → category mapping (many-to-many).
- Frontend: transactions table with sort, filters, pagination, add/edit modals.
- **Done when:** transaction lifecycle works and balances stay correct.

## Phase 4 — CSV Import
`feat/csv-import` → `main`
- Client-side CSV parsing + column mapping + preview; bulk create through the transactions API.
- **Done when:** a bank CSV imports with correct category mapping and balances update.

## Phase 5 — Frontend Shell & Design System
`feat/shell` → `main`
- App layout (sidebar nav, responsive), routing, and the `financer` theme (Tailwind v4 + daisyUI v5 tokens).
- **Light + dark theme support:** theme tokens are mapped per theme (`color-scheme` toggles `light`/`dark`), every surface/text/border/state is token-driven (no hardcoded hex), and contrast is verified independently in both themes (primary text ≥ 4.5:1, secondary ≥ 3:1, separators and interaction states distinguishable in both). A theme toggle switch persists the user's choice.
- Reusable components: `AppModal`, `AppInput`, `AppSelect`, `Pagination`, `ConfirmDialog`, `Popover`, `StatCard`, `AccountCard`.
- `UI_STANDARDS.md` — tokens, spacing, typography, button variants, accessibility baseline.
- **Done when:** every page renders in the shared shell with consistent components and passes contrast checks in both light and dark mode.

## Phase 6 — Transfers
`feat/transfers` → `main`
- Transfer links table (debit ↔ credit), link/unlink endpoints, counterpart creation.
- Unified Transfer modal: From/To accounts, searchable source transaction, candidate preview (±5 days, ±10% amount), link or create counterpart (fee-tolerant amounts).
- Transfer-aware analytics exclusion (debt accounts count as real spending).
- **Done when:** link, unlink, and create-counterpart all work; spending excludes non-debt transfers.

## Phase 7 — Rules
`feat/rules` → `main`
- Alias/rule engine: conditions (field/operator/pattern, AND/OR), outputs (rename / set category / transfer-to-account), priority.
- Read-time non-destructive overlay (matching + applying outputs on every list/get); SQL-based preview; "run now" action for transfer rules.
- Frontend: rule builder, preview table, run-now per rule.
- **Done when:** a rule auto-categorizes on the next load and deletes cleanly (no persisted mutation).

## Phase 8 — Dashboard
`feat/dashboard` → `main`
- `GET /api/dashboard` summary: balance/income/expense sums (SQL), portfolio value, accounts, instruments.
- Frontend dashboard: stat cards, account snapshot, portfolio-by-P&L.
- **Done when:** the page loads from a single request, zero client-side math.

## Phase 9 — Spending Analytics
`feat/spending` → `main`
- `GET /api/spending`: day/month buckets + per-category debit/credit/net, computed in SQL with transfer exclusion.
- Frontend spending tracker: bar chart (7D/1M/6M/1Y/custom), donut pies with hover, Transactions/Categories tabs, server-sorted drill-down.
- **Done when:** buckets/categories match a hand-checked SQL query for the same data.

## Phase 10 — Investments
`feat/investments` → `main`
- Instruments (stock/mutual fund), buy/sell lots, FIFO cost basis and realized P&L, symbol search, Yahoo quote fetch + cache, price-history chart, portfolio summary.
- **Done when:** add lots, refresh prices, and see portfolio P&L match a manual FIFO calculation.

## Phase 11 — Cross-Cutting Quality
`feat/quality` → `main`
- Test coverage pass: repository integration tests (in-memory DB harness), service unit tests, frontend vitest for pure helpers.
- Accessibility pass (WCAG 2.1 AA): keyboard nav, labels, focus, reduced motion, contrast — verified in **both light and dark themes** (never inferred from one theme).
- Performance pass: server-side logic audit, bundle review, list virtualization/`ListView`-style rendering, avoid N+1 in new code.
- **Done when:** full test suite green; ponytail-review and performance-optimization audits have no open high-severity findings.

## Phase 12 — Deployment
`feat/deploy` → `main`
- Multi-stage Docker build (frontend: node build → nginx static; backend: Go binary), nginx reverse proxy for `/api`.
- Production env: `FINANCER_JWT_SECRET` required, optional TLS for HTTP/3.
- **Done when:** `docker compose up` serves the app and API on a clean machine.

---

## Future / Post-1.0 (not blocking merge)

- Rule "run now" result feedback UI (linked/created counts surfaced).
- Spending: budget targets, month-over-month deltas.
- Investments: dividend/interest tracking.
- Reports: export (CSV/PDF), net-worth history chart.
