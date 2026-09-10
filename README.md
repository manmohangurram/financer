<div align="center">

# Financer

**A self-hosted personal finance tracker.**

Rust backend · Vue 3 frontend · single container · SQLite (default) or SurrealDB

[![CI](https://github.com/manmohangurram/financer/actions/workflows/ci.yml/badge.svg)](https://github.com/manmohangurram/financer/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/manmohangurram/financer?display_name=tag)](https://github.com/manmohangurram/financer/releases)
[![License: 0BSD](https://img.shields.io/badge/license-0BSD-blue)](LICENSE)
[![GHCR](https://img.shields.io/badge/ghcr.io-financer-2496ED?logo=docker&logoColor=white)](https://github.com/manmohangurram/financer/pkgs/container/financer)

</div>

> [!WARNING]
> **Alpha — still under active development, not ready for production.** Expect breaking changes and rough edges. Keep regular backups of your data and use it at your own risk.

Financer tracks accounts, transactions, transfers, spending, rules (auto-categorization), and investments — in one place, with a dark, dense dashboard UI. It ships as **one container**: the Rust binary serves both the API and the built frontend (no nginx).

---

## Table of contents

- [Features](#features)
- [Tech stack](#tech-stack)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Data, backups & upgrades](#data-backups--upgrades)
- [Reverse proxy & HTTPS](#reverse-proxy--https)
- [API](#api)
- [MCP server](#mcp-server)
- [Development](#development)
- [Deployment reference](#deployment-reference)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Auth** — signup, login, JWT access + refresh tokens.
- **Accounts & categories** — full CRUD; account types (Current, Savings, Credit Card, Loan); balances stay in sync as transactions change.
- **Transactions** — create (single + bulk), edit, delete, CSV/XLSX import, server-side filters (date, amount, name, category) and cursor pagination.
- **Transfers** — link two transactions as a transfer, unlink, or create the missing counterpart; candidate matching (±5 days, ±10% amount) and fee-tolerant amounts.
- **Rules** — conditions (name/amount/type/category/account; contains/starts-with/ends-with/equals/gt/lt/regex; AND/OR) with outputs (rename, set category, transfer to account). Rules are applied **at write time** and snapshot the resolved name + category onto the transaction; the raw bank string is preserved.
- **Spending analytics** — server-computed buckets (7D/1M/6M/1Y/custom, day or month granularity), per-category debit/credit pies with drill-down, and transfer exclusion (debt accounts count as real spending).
- **Dashboard** — one summary endpoint: net worth, income, expenses, portfolio value, account cards, P&L.
- **Investments** — instruments (stock/mutual fund), buy/sell lots with FIFO cost basis, live Yahoo quotes, price-history chart, portfolio summary (invested / current / unrealized / realized P&L).
- **API keys + MCP** — per-user API keys (scoped read / read_write) and an MCP server at `/mcp` for external AI agents.

All finance math (aggregation, filtering, transfer resolution, FIFO) lives in the backend; the frontend renders server-computed results.

## Tech stack

| Layer | Stack |
|---|---|
| Backend | Rust (axum on tokio), JWT (`jsonwebtoken`), OpenAPI via `utoipa` |
| Storage | **SQLite** (default, `sqlx` + migrations in `db/migrations/`) or **SurrealDB** (server-mode HTTP-RPC) |
| Frontend | Vue 3 + Vite + TypeScript |
| Styling | Tailwind CSS v4 + daisyUI v5 (dark `financer` theme), `@lucide/vue` icons |
| Packaging | Single container — Rust binary serves API **and** built SPA |

## Quick start

The image is published to GitHub Container Registry for `linux/amd64` and `linux/arm64`.

### Docker Compose (recommended)

The repo ships two compose files:

```bash
# Default: SurrealDB + the app
docker compose up -d

# Minimal: one container, SQLite, no external database
docker compose -f docker-compose.minimal.yml up -d
```

Copy [`.env.example`](.env.example) to `.env` to set the port, data directory, image tag, timezone, and (for the default setup) the SurrealDB credentials:

```bash
cp .env.example .env
```

### Docker run

```bash
docker run -d --name financer --restart unless-stopped \
  -p 8080:8080 \
  -v financer-data:/data \
  -e FINANCER_JWT_SECRET="$(openssl rand -hex 32)" \
  ghcr.io/manmohangurram/financer:latest
```

Open **http://localhost:8080** and sign up.

> [!IMPORTANT]
> `FINANCER_JWT_SECRET` is **required** — the server refuses to start without it. Set a stable value (e.g. `openssl rand -hex 32`) so sessions survive restarts.

### Pulling a private package

If the GHCR package is private, authenticate with a **classic PAT** that has the `read:packages` scope:

```bash
echo $CR_PAT | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

Alternatively, make only the package public (GitHub → your profile → *Packages* → *financer* → *Package settings* → *Change visibility*) while keeping the repo private.

## Configuration

Configuration is **entirely environment variables** — there is no config file. Every setting has a built-in default except `FINANCER_JWT_SECRET`, which is required. Blank/unset vars keep the default.

| Variable | Default | Purpose |
|---|---|---|
| `FINANCER_JWT_SECRET` | **required** | Token signing secret (e.g. `openssl rand -hex 32`) |
| `FINANCER_ADDR` | `0.0.0.0:8080` | Listen address |
| `FINANCER_DATA_DIR` | `/data` | Runtime data root |
| `FINANCER_DOMAIN_URL` | *(empty)* | External URL, if the SPA is served from a different origin |
| `FINANCER_TIMEZONE` | `Asia/Kolkata` | IANA timezone for day/month bucketing and date ranges |
| `FINANCER_DATABASE` | `sqlite` | `sqlite` \| `surreal` |
| `FINANCER_SQLITE_PATH` | `/data/financer.db` | SQLite file |
| `FINANCER_SURREAL_URL` | `127.0.0.1:8000` | SurrealDB endpoint |
| `FINANCER_SURREAL_USER` | `root` | SurrealDB user |
| `FINANCER_SURREAL_PASS` | `root` | SurrealDB password |
| `FINANCER_SURREAL_NS` | `financer` | SurrealDB namespace |
| `FINANCER_SURREAL_DB` | `financer` | SurrealDB database |

**Frontend API base:** in the container the frontend is built same-origin, so it talks to the API on whatever host serves the page. For local dev set `VITE_API_URL` (see [`frontend/.env.example`](frontend/.env.example)).

### Storage backends

- **SQLite (default)** — zero-dependency, single file at `/data/financer.db`. Migrations in `db/migrations/` run automatically at boot.
- **SurrealDB (optional)** — set `FINANCER_DATABASE=surreal` and point `FINANCER_SURREAL_*` at a running SurrealDB. The schema is applied idempotently at boot. The default [`docker-compose.yml`](docker-compose.yml) bundles a SurrealDB for you.

## Data, backups & upgrades

Everything persists under the mounted `/data` volume:

| Path | Contents |
|---|---|
| `/data/financer.db` (+ `-wal`, `-shm`) | SQLite database |
| `/data/avatars/` | Uploaded profile pictures |

### Backup

SQLite with WAL is a single-writer file; stop the container (or use `sqlite3 .backup`) for a consistent copy:

```bash
docker compose stop
tar czf financer-backup-$(date +%F).tar.gz -C ./data .
docker compose start
```

### Restore

```bash
docker compose stop
rm -rf ./data && mkdir ./data
tar xzf financer-backup-YYYY-MM-DD.tar.gz -C ./data
docker compose start
```

### Upgrading

```bash
docker compose pull
docker compose up -d
```

Schema migrations run automatically at startup.

## Reverse proxy & HTTPS

Financer serves plain HTTP on `:8080`. Put it behind a reverse proxy for TLS and a hostname. Set `domain_url` only if the frontend is served from a different origin than the API (leave it empty for same-origin).

<details>
<summary>Caddy</summary>

```caddy
financer.example.com {
    reverse_proxy 127.0.0.1:8080
}
```
</details>

<details>
<summary>nginx</summary>

```nginx
server {
    listen 443 ssl;
    server_name financer.example.com;

    ssl_certificate     /etc/letsencrypt/live/financer.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/financer.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```
</details>

## API

REST/JSON under `/api/*`.

- Interactive docs (Swagger UI): **`/docs`**
- OpenAPI spec: **`/openapi.json`**
- `POST /api/auth/{signup,login,refresh}` are public; everything else requires `Authorization: Bearer <access-token>`.

Manage personal API keys (for the MCP server and other clients) at `GET/POST /api/me/keys` and `DELETE /api/me/keys/{id}`, or in the app under **Settings → API keys**.

## MCP server

Financer exposes a **Model Context Protocol** server at **`/mcp`** (streamable HTTP) so any MCP-capable client (Claude, Gemini, ChatGPT, …) can read your data. No AI code runs inside Financer — the agent is on your side.

Authenticate with a per-user API key (created in **Settings → API keys**):

```json
{
  "mcpServers": {
    "financer": {
      "url": "https://financer.example.com/mcp",
      "headers": { "Authorization": "Bearer fin_live_<your-key>" }
    }
  }
}
```

Tools include `list_accounts`, `list_transactions`, `get_balance_summary`, `get_portfolio`, `list_categories`, plus write tools when the key has `read_write` scope.

## Development

**Prerequisites:** Rust ≥ 1.85 (edition 2024) and Node 22.

```bash
# Backend — API on :8080 (set FINANCER_JWT_SECRET, see Configuration)
cargo run

# Frontend — Vite dev server on :5173, calls the API directly (CORS)
cd frontend
npm install
VITE_API_URL=http://localhost:8080 npm run dev
```

Open http://localhost:5173.

### Tests

```bash
# Backend: unit + repo (SurrealDB Mem) + HTTP integration tests (SQLite)
cargo test

# Frontend
cd frontend && npm test
```

Quality gate (all must pass before a PR):

```bash
cargo build --locked
cargo clippy --all-targets -- -D warnings
cargo test
cargo audit          # advisories ignored are documented in .cargo/audit.toml
```

### Project structure

```
src/
  main.rs                 # wiring: repo set, services, HTTP server
  auth.rs                 # JWT creation/validation
  config.rs               # env config + defaults
  error.rs                # ApiError → HTTP status mapping
  http/                   # REST handlers (account, transaction, rule, investment, analytics, settings, …)
  mcp/                    # MCP server tools + models
  repo/
    traits/               # repo traits + row/input types (shared)
    sqlite/               # SQLite implementation (sqlx)
    surreal/              # SurrealDB implementation
  service/                # business logic (rules, transfers, FIFO, Yahoo, …)
  tests/                  # HTTP integration tests (SQLite, in-process)
db/migrations/            # SQLite migrations (applied at boot)
frontend/src/
  views/                  # route pages
  components/             # shared UI
  lib/api/                # fetch API client
  lib/stores/             # auth + account selection state
  lib/utils/              # pure helpers (formatting, filters)
architecture/             # layering, data flow, naming, conventions
```

## Deployment reference

Full build/deploy reference — frontend↔backend linking, multi-arch builds, the GitHub Actions release flow, and troubleshooting — is in **[DEPLOY.md](DEPLOY.md)**.

Releases are cut by pushing a `v*` tag; `.github/workflows/docker-publish.yml` builds the image for the configured platforms and pushes it to GHCR with the version tag and `latest`.

## Contributing

Issues and pull requests are welcome. Please run the [quality gate](#tests) before opening a PR, keep commits in [Conventional Commits](https://www.conventionalcommits.org/) format, and fill in the PR template.

## License

[**0BSD**](LICENSE) (Zero-Clause BSD) — an OSI-approved, no-conditions license. Anyone may use, copy, modify, and/or distribute this software for any purpose, commercial or otherwise, with **no attribution required**. Provided "as is", without warranty of any kind.
