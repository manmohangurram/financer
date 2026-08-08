# Financer — Personal Finance Tracker

A self-hosted personal finance tracker with a Go backend and a Vue 3 single-page app. Track accounts, transactions, transfers, spending, rules (auto-categorization), and investments — all in one place, with a dark, dense dashboard UI.

## Features

- **Auth** — signup, login, JWT bearer tokens with refresh.
- **Accounts & Categories** — full CRUD; account types (Checking, Savings, Credit Card, Loan, Crypto Wallet); balances kept in sync as transactions change.
- **Transactions** — create (single + bulk), edit, delete, CSV import, server-side filters (date range, amount, name, category) and pagination.
- **Transfers** — link two transactions as a transfer, unlink, or create the missing counterpart; unified transfer modal with candidate matching (±5 days, ±10% amount) and fee-tolerant amounts.
- **Rules** — conditions (name/amount/type/category/account with contains/starts-with/ends-with/equals/gt/lt/regex, AND/OR) that auto-categorize at read time (non-destructive). Outputs: rename, set category, or "transfer to account" with a run-now action.
- **Spending analytics** — server-computed bar buckets (7D/1M/6M/1Y/custom, day or month granularity), per-category debit/credit donut pies with hover drill-down, transfer-exclusion (debt accounts count as real spending), and a sortable/paginated transaction drill-down.
- **Dashboard** — single summary endpoint: net worth, income, expenses, portfolio value, account cards, portfolio-by-P&L.
- **Investments** — instruments (stock/mutual fund), buy/sell lots with FIFO cost basis, live Yahoo quotes, price history chart, portfolio summary (invested/current/unrealized/realized P&L).

## Tech Stack

**Backend** — Go, plain `net/http` (HTTP/1.1 + HTTP/2 over h2c, optional HTTP/3 QUIC), SQLite (WAL, separate write/read pools, `_txlock=immediate` single-writer), JWT (`golang-jwt`), embedded SQL migrations.

**Frontend** — Vue 3 + Vite + TypeScript, Tailwind CSS v4 + daisyUI v5 (dark `financer` theme), `@lucide/vue` icons, hand-rolled fetch API client.

All finance math (aggregation, filtering, transfer resolution, FIFO) lives in the backend. The frontend renders server-computed results.

## Getting Started

```bash
# Backend (from repo root)
go run main.go            # starts API on :8080, auto-runs DB migrations
go run ./cmd/seed         # optional: seed a demo user (demo@financer.app / password123)

# Frontend
cd frontend
npm install
npm run dev               # Vite dev server on :5173, calls :8080 directly (CORS)
npm run build             # type-check + static build into frontend/dist/
npm test                  # vitest
```

Open `http://localhost:5173` and sign in.

## Configuration (env vars)

| Variable | Default | Purpose |
|---|---|---|
| `FINANCER_DB_PATH` | `data/financer.db` | SQLite database path |
| `FINANCER_JWT_SECRET` | — | JWT signing secret (set in production) |
| `FINANCER_ADDR` | `:8080` | API listen address |
| `FINANCER_TLS_CERT` / `FINANCER_TLS_KEY` | — | Enable HTTP/3 (QUIC) when set |
| `FINANCER_QUOTE_REFRESH_INTERVAL` | `30m` | Background quote refresh |
| `FINANCER_HISTORY_REFRESH_INTERVAL` | `30m` | Background price-history refresh |

Frontend API URL: `VITE_API_URL` (default `http://localhost:8080`, see `frontend/.env.example`).

## Project Structure

```
main.go                    # wiring: DB, migrations, services, HTTP server
auth/                      # JWT creation/validation
db/                        # SQLite open (read/write pools) + embedded migrations
repository/                # SQL data access (accounts, transactions, aliases, transfers, investments)
services/                  # business logic (auth, accounts, transactions, aliases, transfers, spending, investments)
httpserver/                # REST handlers + JSON wire mapping
gen/financer/v1/           # hand-written API wire types
cmd/seed/                  # demo data seeder
frontend/src/
  views/                   # route pages (Dashboard, Accounts, Spending, Rules, Categories, Investments)
  components/              # shared UI (AppModal, AppInput, AppSelect, Pagination, etc.)
  lib/api/                 # fetch API client
  lib/stores/              # auth + account-selection state
  lib/utils/               # pure helpers (formatting, filters, rule outputs)
```

## API

REST JSON under `/api/...`. `POST /api/auth/{signup,login,refresh}` are public; everything else requires `Authorization: Bearer <token>`. Notable endpoints: `GET /api/dashboard`, `GET /api/spending`, `GET/POST/PUT /api/transactions`, `/api/transfers/*`, `/api/aliases/*`, `/api/instruments/*`.

## Tests

```bash
go test ./...   # backend unit + repository integration tests
npm test        # frontend vitest
```

## Deployment

The frontend is a pure static SPA (`frontend/dist/`) — serve it from any static host (or nginx) pointing `VITE_API_URL` at the Go backend. See the roadmap for a Docker multi-stage recipe.
