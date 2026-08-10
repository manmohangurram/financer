# Financer — Personal Finance Tracker

> **⚠️ Under active development — not ready for production.** The app (v0.0.1, alpha) is a work in progress: expect breaking changes and rough edges. Keep regular backups of your data and use it at your own risk.

A self-hosted personal finance tracker with a Rust backend and a Vue 3 single-page app. Track accounts, transactions, transfers, spending, rules (auto-categorization), and investments — all in one place, with a dark, dense dashboard UI.

## Features

- **Auth** — signup, login, JWT bearer tokens with refresh.
- **Accounts & Categories** — full CRUD; account types (Current, Savings, Credit Card, Loan); balances kept in sync as transactions change.
- **Transactions** — create (single + bulk), edit, delete, CSV import, server-side filters (date range, amount, name, category) and pagination.
- **Transfers** — link two transactions as a transfer, unlink, or create the missing counterpart; unified transfer modal with candidate matching (±5 days, ±10% amount) and fee-tolerant amounts.
- **Rules** — conditions (name/amount/type/category/account with contains/starts-with/ends-with/equals/gt/lt/regex, AND/OR) that auto-categorize at read time (non-destructive). Outputs: rename, set category, or "transfer to account" with a run-now action.
- **Spending analytics** — server-computed bar buckets (7D/1M/6M/1Y/custom, day or month granularity), per-category debit/credit donut pies with hover drill-down, transfer-exclusion (debt accounts count as real spending), and a sortable/paginated transaction drill-down.
- **Dashboard** — single summary endpoint: net worth, income, expenses, portfolio value, account cards, portfolio-by-P&L.
- **Investments** — instruments (stock/mutual fund), buy/sell lots with FIFO cost basis, live Yahoo quotes, price history chart, portfolio summary (invested/current/unrealized/realized P&L).

## Tech Stack

**Backend** — Rust (axum on tokio, HTTP/1.1 + HTTP/2), SQLite (WAL, separate write/read pools, single-writer) via `sqlx`, JWT (`jsonwebtoken`), SQL migrations applied at boot.

**Frontend** — Vue 3 + Vite + TypeScript, Tailwind CSS v4 + daisyUI v5 (dark `financer` theme), `@lucide/vue` icons, hand-rolled fetch API client.

All finance math (aggregation, filtering, transfer resolution, FIFO) lives in the backend. The frontend renders server-computed results.

## Getting Started

```bash
# Backend (from repo root)
cargo run                 # starts API on :8080, auto-runs DB migrations

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
| `FINANCER_DATA_DIR` | `data` | Root for all runtime data: `db/`, `config/`, `certs/`, `avatars/` subfolders |
| `FINANCER_DOMAIN_URL` | — | External domain the app is served at (e.g. `https://financer.local`); injected into the frontend as the API base — empty/omitted → same-origin |
| `FINANCER_DB_PATH` | `<DATA_DIR>/db/financer.db` | SQLite database path |
| `FINANCER_JWT_SECRET` | — | JWT signing secret (set in production) |
| `FINANCER_ADDR` | `:8080` | API listen address |
| `FINANCER_STATIC_DIR` | `frontend/dist` | Built frontend served at `/`; omit/absent → API-only (dev) |
| `FINANCER_YAHOO_CONFIG` | `<DATA_DIR>/config/yahoo.json` | Yahoo endpoints; bundled default copied there on first run |
| `FINANCER_TLS_CERT` / `FINANCER_TLS_KEY` | `<DATA_DIR>/certs/{cert,key}.pem` | Enable HTTP/3 (QUIC) when present |
| `FINANCER_QUOTE_REFRESH_INTERVAL` | `30m` | Background quote refresh |
| `FINANCER_HISTORY_REFRESH_INTERVAL` | `30m` | Background price-history refresh |

Frontend API URL: `VITE_API_URL` (default `http://localhost:8080`, see `frontend/.env.example`).

## Project Structure

```
main.go                    # wiring: DB, migrations, services, HTTP server
auth/                      # JWT creation/validation
db/                        # SQLite open (read/write pools) + embedded migrations
repository/                # SQL data access (accounts, transactions, rules, transfers, investments)
services/                  # business logic (auth, accounts, transactions, rules, transfers, spending, investments)
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

REST JSON under `/api/...`. `POST /api/auth/{signup,login,refresh}` are public; everything else requires `Authorization: Bearer <token>`. Notable endpoints: `GET /api/dashboard`, `GET /api/spending`, `GET/POST/PUT /api/transactions`, `/api/transfers/*`, `/api/rules/*`, `/api/instruments/*`.

## Tests

```bash
cargo test      # backend unit + repository integration tests
npm test        # frontend vitest
```

## Deployment (self-host on a Raspberry Pi)

The GitHub Action (`.github/workflows/docker-publish.yml`) builds the image for `linux/arm64` (64-bit Raspberry Pi OS / Apple Silicon) and `linux/amd64` (x86 hosts) and pushes it to **GHCR** whenever a commit lands on `main`. The image is a single container — the Rust binary serves both the API and the built frontend (no nginx). In production builds the frontend calls the API on the same origin, so it works from any device, not just localhost.

**Public repo:** the GHCR package is public too, so pull with no login.

**Private repo:** the package is private like the repo, so `docker pull` returns `unauthorized` unless you authenticate first. Generate a **classic PAT** (github.com → Settings → Developer settings → Personal access tokens → *Tokens (classic)* → *Generate new token*) with the **`read:packages`** and **`repo`** scopes, then on the Pi:

```bash
echo YOUR_PAT | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

**On the Pi:**

```bash
docker pull ghcr.io/manmohangurram/financer:latest
mkdir -p ~/financer/data

docker run -d --name financer --restart unless-stopped \
  -p 8080:8080 \
  -e FINANCER_JWT_SECRET=change-me \
  -v ~/financer/data:/data \
  ghcr.io/manmohangurram/financer:latest
```

> Optionally, you can make **only the package** public (github.com → your profile → *Packages* → *Financer* → *Package settings* → *Change visibility* → **Public**) while keeping the repo private — then the Pi can pull without a PAT. This only exposes the built image, never the source code.

Open `http://<pi-ip>:8080` and sign up.

**Data layout** — everything persists under the volume mount (`/data`):

- `db/financer.db` — SQLite database
- `config/yahoo.json` — Yahoo endpoint bases (auto-copied on first run; edit to add/swap bases)
- `certs/cert.pem` + `certs/key.pem` — optional HTTP/3 (QUIC) TLS pair
- `avatars/` — uploaded profile pictures

**Build the image yourself (any architecture):**

```bash
docker buildx build --platform linux/arm64 -t financer:local .
```
