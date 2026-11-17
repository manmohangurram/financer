<div align="center">

# Financer

**A self-hosted personal finance tracker.**

Go backend · Vue 3 frontend · single container · SQLite

[![CI](https://github.com/manmohangurram/financer/actions/workflows/ci.yml/badge.svg)](https://github.com/manmohangurram/financer/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/manmohangurram/financer?display_name=tag)](https://github.com/manmohangurram/financer/releases)
[![License: 0BSD](https://img.shields.io/badge/license-0BSD-blue)](LICENSE)
[![GHCR](https://img.shields.io/badge/ghcr.io-financer-2496ED?logo=docker&logoColor=white)](https://github.com/manmohangurram/financer/pkgs/container/financer)

</div>

> [!WARNING]
> **Alpha — still under active development, not ready for production.** Expect breaking changes and rough edges. Keep regular backups of your data and use it at your own risk.

Financer tracks accounts, transactions, transfers, spending, and rules (auto-categorization) — in one place, with a dark, dense dashboard UI. It ships as **one container**: the Go binary serves both the API and the built frontend (no nginx).

---

## Table of contents

- [Features](#features)
- [Tech stack](#tech-stack)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Inbox (email → transactions)](#inbox-email--transactions)
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
- **Transactions** - create (single + bulk), edit, delete, server-side filters (date, amount, name, category) and cursor pagination.
- **Transfers** — link two transactions as a transfer, unlink, or create the missing counterpart; matching looks for the opposite leg in the target account within ±3 days for the same amount.
- **Rules** — conditions (name/amount/type/category/account; contains/starts-with/ends-with/equals/gt/lt/regex; AND/OR) with outputs (rename, set category, transfer to account). Rules are applied **at write time** and snapshot the resolved name + category onto the transaction; the raw bank string is preserved.
- **Spending analytics** — server-computed buckets (7D/1M/6M/1Y/custom, day or month granularity), per-category debit/credit pies with drill-down, and transfer exclusion (debt accounts count as real spending).
- **Dashboard** — one summary endpoint: net worth, income, expenses, and account cards.
- **Inbox** — link an IMAP mailbox (app password, encrypted at rest), poll it on a timer, and turn bank-statement mail into transactions: disposition rules send each sender to Ignore / Review / Auto, and the Inbox approves or rejects what needs a person.
- **API keys + MCP** — per-user API keys (scoped read / read_write) and an MCP server at `/mcp` for external AI agents.

All finance math (aggregation, filtering, transfer resolution) lives in the backend; the frontend renders server-computed results.

## Tech stack

| Layer | Stack |
|---|---|
| Backend | Go (Gin) — JWT (`golang-jwt`), OpenAPI at `/docs` + `/openapi.json`, MCP at `/mcp` |
| Storage | **SQLite** (`glebarez/sqlite`, pure Go; migrations in `db/migrations/`) |
| Frontend | Vue 3 + Vite + TypeScript |
| Styling | Tailwind CSS v4 + daisyUI v5 (dark `financer` theme), `@lucide/vue` icons |
| Packaging | Single container — Go binary serves API **and** built SPA |

## Quick start

The image is published to GitHub Container Registry for `linux/arm64`:

| Tag | Storage |
|---|---|
| `ghcr.io/manmohangurram/financer:latest` | **SQLite** |

[`docker-compose.yml`](docker-compose.yml) runs it as a single container.

### Docker Compose (recommended)

```bash
docker compose up -d
```

Copy [`.env.example`](.env.example) to `.env` to set the port, data directory, image tag and timezone:


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
| `FINANCER_STATIC_DIR` | `frontend/dist` | Built SPA location (the image sets `/app/frontend/dist`) |
| `FINANCER_DOMAIN_URL` | *(empty)* | External URL, if the SPA is served from a different origin |
| `FINANCER_TIMEZONE` | `Asia/Kolkata` | IANA timezone for day/month bucketing and date ranges |
| `FINANCER_SQLITE_PATH` | `/data/financer.db` | SQLite file (WAL mode is enabled at boot) |
| `FINANCER_SECRET_KEY` | *(unset)* | 32-byte hex key (64 chars) encrypting mailbox passwords; the mail feature needs it |
| `FINANCER_MAIL_POLL_SECS` | `300` | How often the background poll runs (minimum 30) |
| `FINANCER_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `FINANCER_LOG_FILE` | *(empty)* | Append logs to this file instead of stderr; rotates at 10 MB and keeps at most 3 files (older ones are deleted) |
| `FINANCER_ALLOW_INSECURE_HTTP` | `false` | `1`/`true`/`yes` only behind a TLS-terminating proxy; otherwise credential endpoints require HTTPS (loopback exempt) |

**Frontend API base:** in the container the frontend is built same-origin, so it talks to the API on whatever host serves the page. For local dev set `VITE_API_URL` (see [`frontend/.env.example`](frontend/.env.example)).

### Storage

**SQLite** — zero-dependency, single file at `/data/financer.db`. Migrations in `db/migrations/` run automatically at boot.

## Inbox (email → transactions)

The server reads bank-statement mail over IMAP and turns it into transactions.

1. Generate the credential key once — it encrypts the stored mailbox passwords and is **not** the JWT secret:

   ```bash
   openssl rand -hex 32     # -> FINANCER_SECRET_KEY, 64 hex characters
   ```

   Without it the mailbox endpoints refuse to store credentials (`FINANCER_SECRET_KEY is not set`).
2. In the app: **Inbox → Link mailbox** — IMAP host, port, username, password, and the senders to read.
   - **Port 993 only** (implicit TLS). STARTTLS on 143 is not supported yet and is rejected with a clear message.
   - Use an **app password** from the provider, not the account password.
   - The **sender allowlist** is required; mail from anyone else is never fetched.
   - Private/LAN hosts are allowed, cloud metadata addresses are not.
3. The poll runs in the background every `FINANCER_MAIL_POLL_SECS` (default 300s, minimum 30), and on demand from the Inbox.
4. Disposition rules decide what happens to each match, first rule wins:

   | Disposition | Effect |
   |---|---|
   | `AUTO` | The poll creates the transaction itself and the row is marked `AUTO` |
   | `REVIEW` | The row waits in the Inbox for approve / reject |
   | `IGNORE` | Seen, never surfaced |

   A row that needs a decision it cannot make (ambiguous account, nothing extracted) lands in `PENDING` with the reason rather than guessing.
5. Approving creates the transaction through the normal service path, so the rule engine, balance recalculation and category handling all apply. The account is picked from the last 4 digits in the mail, or chosen by you when they do not resolve.

**These routes require HTTPS** (loopback exempt): the mailbox password travels in the request body. Behind a reverse proxy, forward `X-Forwarded-Proto: https` — see [Reverse proxy & HTTPS](#reverse-proxy--https).

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

### Statement import

Statement import is **not supported**: the Go backend exposes no CSV/XLSX/PDF
import routes, and the app has no import UI. Add transactions manually, via the
API, or from bank-alert mail (see the inbox).

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

Tools include `list_accounts`, `list_transactions`, `get_balance_summary`, `list_categories`, `list_rules`, `preview_rule`, plus write tools when the key has `read_write` scope.

## Development

**Prerequisites:** Go ≥ 1.27 and Node 22.

```bash
# Backend — API on :8080 (set FINANCER_JWT_SECRET, see Configuration)
go run ./cmd/financer

# Frontend — Vite dev server on :5173, calls the API directly (CORS)
cd frontend
npm install
VITE_API_URL=http://localhost:8080 npm run dev
```

Open http://localhost:5173.

### Tests

```bash
# Backend: unit + repo (SQLite tempfile) + HTTP/MCP integration tests
go test ./...

# Frontend
cd frontend && npm test
```

Quality gate (all must pass before a PR):

```bash
gofmt -l cmd internal migrate.go   # must print nothing
go vet ./...
go build ./...
go test ./...
go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
```

### Project structure

```
cmd/financer/main.go        # wiring: config → DB → repos → services → Gin server
internal/
  config/                   # env config + defaults
  db/                       # SQLite pools (WAL) + migration runner
  errors/                   # API error envelope {code, message}
  auth/                     # JWT creation/validation
  model/                    # camelCase wire types
  repo/                     # repo interfaces + row types
    sqlite/                 # SQLite implementations
  authservice/ userservice/ accountservice/ categoryservice/
  transactionservice/ transferservice/ ruleservice/ mailservice/ keyservice/
  mail/                     # IMAP source, alert extractor, credential crypto
  mcp/                      # MCP tool handlers
  openapi/                  # /openapi.json spec
  server/                   # Gin router, handlers, middleware
  time/                     # stored-timestamp + timezone helpers
migrate.go                  # embeds db/migrations/*.up.sql
db/migrations/              # SQLite migrations (applied at boot)
frontend/src/
  views/                    # route pages
  components/               # shared UI
  lib/api/                  # fetch API client
  lib/stores/               # auth + account selection state
  lib/utils/                # pure helpers (formatting, filters)
architecture/               # design notes: layering, request flow, naming, conventions
```

## Deployment reference

Full build/deploy reference — frontend↔backend linking, the arm64 build, the GitHub Actions release flow, and troubleshooting — is in **[DEPLOY.md](DEPLOY.md)**.

Releases are cut by pushing a `v*` tag; `.github/workflows/docker-publish.yml` builds the image for the configured platforms and pushes it to GHCR with the version tag and `latest`.

## Contributing

Issues and pull requests are welcome. Please run the [quality gate](#tests) before opening a PR, keep commits in [Conventional Commits](https://www.conventionalcommits.org/) format, and fill in the PR template.

## License

[**0BSD**](LICENSE) (Zero-Clause BSD) — an OSI-approved, no-conditions license. Anyone may use, copy, modify, and/or distribute this software for any purpose, commercial or otherwise, with **no attribution required**. Provided "as is", without warranty of any kind.
