# Financer — Project Rules

Financer: a personal finance tracker. Go backend (plain HTTP/JSON over h2c) + Vue 3 (Vite, TypeScript, Tailwind CSS v4 + daisyUI v5) frontend, SQLite storage. No protobuf — API types are hand-written plain Go structs in `api/types.go`, serialized to JSON by `httpserver`.

## Commands

Run verification in this order after backend changes, then frontend changes.

**Backend (Go, from repo root):**
```bash
go run main.go            # starts API on :8080, auto-runs DB migrations on boot
go run ./cmd/seed         # seeds demo user (demo@financer.app / password123)
go build ./...
go vet ./...
go test ./...             # unit + repository integration tests (in-memory DB)
```

**Frontend (from `frontend/`):**
```bash
npm run dev        # vite dev server on :5173, calls the Go backend directly (CORS, no proxy)
npm run build       # vue-tsc -b (type check) && vite build → static frontend/dist/
npm test            # vitest — pure helpers/composables under src/lib
```

**Verification checklist:** after any change run `go build ./... && go vet ./... && go test ./...` (repo root) and `npm run build && npm test` (`frontend/`). Browser smoke test for UI changes (console must be clean).

## Architecture

**Request flow:** `main.go` opens the DB, runs migrations, constructs repositories → services, then registers HTTP handlers via `httpserver.NewAPI`. REST JSON under `/api/...`; routes in `httpserver/server.go` via `a.route(mux, method, path, public, handler)`; path params via `r.PathValue("id")`. JWT auth wraps handlers; only `/api/auth/{signup,login,refresh}` are public; the user id is injected into `context.Context` under `auth.UserIDKey`.

**Layering:** `repository/` (SQL) → `services/` (business logic) → `httpserver/` (JSON mapping + handlers). Repositories embed `*repository.BaseRepository`, which holds **separate write and read `*sql.DB` handles** (`db.OpenDBs`) — write pool capped at 1 connection with `_txlock=immediate` (SQLite WAL, single-writer), read pool allows several. Use the generic `QueryAll[T]` / `QueryOne[T]` / `Exec` helpers in `repository/base.go` — never hand-roll scan loops.

**API types** live in `api/types.go` (package `api`, hand-written, no protobuf): request types, response types, enums with `.String()` + `*_value` maps. Field shapes mirror what the frontend's hand-rolled `fetch` clients send/receive. `httpserver/dto.go` maps wire ↔ structs. No getters (fields are exported and accessed directly). Edit `types.go` when the API shape changes.

**Rule engine** (`services/alias.go`): user rules auto-categorize transactions by conditions (field: name/amount/type/category/account; operator: contains/starts_with/ends_with/equals/gt/lt/regex; AND/OR logic). Each rule carries **output actions** — rename, set category, or **transfer-to-account** (`AliasAction`, `repository/alias.go`). Matching is **non-destructive, applied at read time**: `AliasService.Overlay` loads rules once (`ListForOverlay`, priority-ordered) and applies all matching actions per transaction; later/lower-priority rules override earlier setters. `SET_TRANSFER_ACCOUNT` actions are handled at write time by `services/transfer_rule.go` (`TransferRuleService`): it finds-or-creates the counterpart across accounts and links them; `POST /api/aliases/{id}/run` runs one rule against existing transactions.

**Server-side aggregation:** all finance math lives in the backend. `GET /api/dashboard` returns balance/income/expense sums + portfolio + accounts + instruments; `GET /api/spending?range=…` returns day/month buckets and per-category debit/credit/net computed in SQL (transfer exclusion via `transfer_links` + debt account types). The frontend renders, never aggregates. Transaction filtering/pagination/sorting (`GET /api/transactions`) is server-side.

**Frontend client:** the per-service clients in `frontend/src/lib/api/client.ts` (`accounts()`, `transactions()`, `analytics()`, etc.) are hand-written wrappers around raw `fetch` (an `api(method, path)` helper fills `{id}` params and serializes GET query strings), attaching the JWT from `localStorage` via `frontend/src/lib/api/transport.ts` (`setTokens`/`clearTokens`/`loadTokens`/`getAccessToken`). `frontend/src/lib/stores/auth.ts` calls `fetch` directly for auth. Do not assume generated TS clients exist.

**Data is scoped per user** — every resource table has a `user_id` column and repository queries filter by it. Follow that precedent when adding queries.

## Frontend structure

- `frontend/src/views/*.vue` — one per route (`DashboardView`, `AccountsView`, `AccountsManageView`, `SpendingView`, `RulesView`, `CategoriesView`, `InvestmentsView`, `InstrumentDetailView`, `LoginView`, `SignupView`), wired in `frontend/src/router/index.ts`. Views compose components; no raw popup/card markup in views.
- `frontend/src/components/` — shared UI: `AppLayout` (sidebar shell), `AppModal` (modal shell: `title`/`wide`/`xwide` props, `close` emit, default slot), `Popover` (dropdown shell: backdrop + positioned panel, `panel-class`, `close` emit), `ConfirmDialog` (replaces native `confirm()`), `AppInput` / `AppSelect` (labeled form controls — never hand-roll input/select class strings), `Pagination` (first/prev/current/next/last, centered), `DatePicker`, `AccountCard` (the one account-card component, has `compact` prop), `StatCard`. `frontend/src/components/workspace/` holds transactions/rules workspace components (`TransactionsPanel`, `TransactionsTab`, `TransactionTable`, `AddTransactionModal`, `EditItemModal`, `TransferModal`, `RuleForm`, `RulesTab`, `CategoriesTab`, `CategoryForm`, `TransactionForm`).
- `frontend/src/lib/utils/` — pure helpers (formatting, CSV parsing, filters, rule outputs, category colors) factored out of views; each has a co-located `*.test.ts` under vitest.
- See `frontend/UI_STANDARDS.md` for the design system: tokens, spacing, buttons, forms, accessibility — follow it on every UI change.

## Conventions & gotchas

- **Tailwind v4 + daisyUI v5:** no `tailwind.config.js`/`postcss.config.js`. Config lives in `frontend/src/assets/main.css`: `@import "tailwindcss"`, `@plugin "daisyui"`, the custom `financer` theme, and tokens in an `@theme` block. Use tokens (`bg-surface`, `text-income`, `bg-primary-500`) — never inline hex or `bg-[#...]` arbitrary values. daisyUI v5 notes: `input`/`select` bordered by default (no `-bordered`), `form-control`/`label-text` gone, `btn-group` replaced by `join` + `join-item`.
- **DB migrations:** raw SQL in `db/migrations/`, embedded (`db.migrationsFS`), auto-applied at startup in filename order, tracked in `schema_migrations`. Add `NNNNNN_description.up.sql` / `.down.sql` pairs.
- **Env vars:** `FINANCER_DB_PATH` (default `data/financer.db`), `FINANCER_JWT_SECRET`, `FINANCER_ADDR` (`:8080`), `FINANCER_TLS_CERT`/`KEY` (enable HTTP/3 QUIC), quote/history refresh intervals. Frontend API URL: `VITE_API_URL` (default `http://localhost:8080`).
- **`occurred_at` storage:** the SQLite driver stores `time.Time` as `"YYYY-MM-DD HH:MM:SS +0000 UTC"` — `date()`/`strftime()` can't parse it. Extract date parts with `substr(t.occurred_at, 1, 10)` (day) or `substr(t.occurred_at, 1, 7)` (month).
- **Debt accounts** (Loan=4, Credit Card=3) count as real spending in analytics; non-debt transfers are excluded.
- **Commits:** Conventional Commits (`feat:`/`fix:`/`docs:`/`refactor:`/`test:`/`chore:`/`perf:`); one logical change per commit. Feature branches → PR → `main`. See `ROADMAP.md` for the build order.
