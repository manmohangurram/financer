# Financer — Project Rules

Financer: a personal finance tracker. Rust backend (axum, plain HTTP/JSON) + Vue 3 (Vite, TypeScript, Tailwind CSS v4 + daisyUI v5) frontend, SQLite storage. No protobuf — API types are hand-written Rust structs in `src/service/*.rs` / `src/repo/*.rs`, serialized to JSON by serde.

## Commands

Run verification in this order after backend changes, then frontend changes.

**Backend (Rust, from repo root):**
```bash
cargo run            # starts API on :8080, auto-runs DB migrations on boot
cargo build --locked
cargo clippy -- -D warnings
cargo test           # unit + repository integration tests (in-memory SQLite)
```

**Frontend (from `frontend/`):**
```bash
npm run dev        # vite dev server on :5173, calls the backend directly (CORS, no proxy)
npm run build       # vue-tsc -b (type check) && vite build → static frontend/dist/
npm test            # vitest — pure helpers/composables under src/lib
```

**Verification checklist:** after any change run `cargo build --locked && cargo clippy -- -D warnings && cargo test` (repo root) and `npm run build && npm test` (`frontend/`). Browser smoke test for UI changes (console must be clean).

**Review before every PR:** before opening a pull request for a branch, run these review skills and fold any fixes into the branch:
- `ponytail-review` — over-engineering scan (delete/stdlib/native/yagni/shrink).
- `code-simplification` — reduce complexity without changing behavior.
- `code-review-and-quality` — correctness, security, maintainability.
- `performance-optimization` — measure and fix measurable bottlenecks (skip if nothing to measure).
- `finishing-a-development-branch` — branch completion: verify tests, present merge options.

## Skills

Installed skills are workflows, not suggestions — route the task to the right one before starting (see `using-agent-skills` for the discovery tree). The migration is ROADMAP-driven, so most work starts in the plan/implement/verify columns.

| When you need to… | Use |
|---|---|
| Route/verify which skill applies | `using-agent-skills` |
| Start any session / creative work | `using-superpowers`, `brainstorming` |
| Pull out what's actually wanted | `interview-me`, `idea-refine` |
| Turn a spec into ordered tasks | `planning-and-task-breakdown`, `writing-plans` |
| Execute a written plan (e.g. ROADMAP phase) | `executing-plans`, `subagent-driven-development` |
| Implement a feature slice by slice | `incremental-implementation`, `test-driven-development` |
| Verify against official docs first | `source-driven-development` |
| Stress-test a non-trivial decision | `doubt-driven-development` |
| Design an API / module boundary | `api-and-interface-design` |
| Build or change UI | `frontend-ui-engineering` + `browser-testing-with-devtools` |
| Fix a bug / unexpected behavior | `systematic-debugging`, `debugging-and-error-recovery` |
| Review before PR (merge gate) | `ponytail-review` → `code-simplification` → `code-review-and-quality` → `performance-optimization` |
| Review received feedback | `receiving-code-review` |
| Finish a branch / present merge options | `finishing-a-development-branch` |
| Commit / branch / versioning | `git-workflow-and-versioning`, `caveman-commit`, `using-git-worktrees` |
| Run the migration phases | `deprecation-and-migration` (retire Go phase-by-phase) |
| Security-sensitive code | `security-and-hardening` |
| Harden with logs/metrics | `observability-and-instrumentation` |
| Record decisions / docs | `documentation-and-adrs` |
| Ship / deploy / CI | `shipping-and-launch`, `ci-cd-and-automation` |
| Claim work done (evidence first) | `verification-before-completion` |
| Delegate to subagents | `cavecrew`, `dispatching-parallel-agents` |
| Speak terse / save tokens | `caveman` (lite/full/ultra) |
| Write review comments | `caveman-review` |
| Auditing/debt tooling | `ponytail-audit`, `ponytail-debt`, `ponytail-gain` |

Codebase questions: `semble` search to locate symbols, `graphify` query to trace architecture/relationships (see the global `~/.config/opencode/AGENTS.md`).

## Architecture

**Request flow:** `src/main.rs` opens the DB, runs migrations, constructs repositories → services, then registers HTTP handlers via `src/http/mod.rs` `router()`. REST JSON under `/api/...`; routes per feature in `src/http/<feature>.rs` via axum `Router::route`; path params via `Path`/`Path(String)` extractors. JWT auth via `require_user(headers, &jwt)`; only `/api/auth/{signup,login,refresh}` are public.

**Layering:** `src/repo/` (SQL) → `src/service/` (business logic) → `src/http/` (JSON mapping + handlers). Repos use `sqlx` against a `Db` holding **separate write (1 conn) and read pools** (`src/db.rs`, `open_pools` — SQLite WAL, single-writer). Migrations are raw SQL in `db/migrations/`, applied by a runner mirroring Go's (`src/db.rs` `run_migrations`, tracked in `schema_migrations`). Use `sqlx::query`/`query_as` with bound params — never string-concatenate values.

**API types** are hand-written Rust structs in the repo/service layer, serialized by serde: `#[serde(rename_all = "camelCase")]` + `#[serde(rename)]` + `serialize_with` map Rust fields ↔ wire. Enums use `strum::Display`/`EnumString` + serde `rename_all` for the wire string (e.g. `TransactionType::Debit` ↔ `"DEBIT"`). No getters — fields are exported and accessed directly.

**Rule engine** (`src/service/rule.rs` + `src/repo/rule.rs`): user rules auto-categorize transactions by conditions (field: name/amount/type/category/account; operator: contains/starts_with/ends_with/equals/gt/lt/regex; AND/OR logic). Each rule carries **output actions** — rename, set category, or **transfer-to-account** (`RuleAction`). Matching is **non-destructive, applied at read time**: `RuleService::overlay` loads rules once (`list_for_overlay`, priority-ordered) and applies all matching actions per transaction; later/lower-priority rules override earlier setters. `SET_TRANSFER_ACCOUNT` actions are handled at write time by `src/service/transfer_rule.rs` (`TransferRuleService`): it finds-or-creates the counterpart across accounts and links them; `POST /api/rules/{id}/run` runs one rule against existing transactions.

**Server-side aggregation:** all finance math lives in the backend. `GET /api/dashboard` returns balance/income/expense sums + portfolio + accounts + investments; `GET /api/spending?range=…` returns day/month buckets and per-category debit/credit/net computed in SQL (transfer exclusion via `transfer_links` + debt account types). The frontend renders, never aggregates. Transaction filtering/pagination/sorting (`GET /api/transactions`) is server-side.

**Frontend client:** the per-service clients in `frontend/src/lib/api/client.ts` (`accounts()`, `transactions()`, `analytics()`, etc.) are hand-written wrappers around raw `fetch` (an `api(method, path)` helper fills `{id}` params and serializes GET query strings), attaching the JWT from `localStorage` via `frontend/src/lib/api/transport.ts` (`setTokens`/`clearTokens`/`loadTokens`/`getAccessToken`). `frontend/src/lib/stores/auth.ts` calls `fetch` directly for auth. Do not assume generated TS clients exist.

**Data is scoped per user** — every resource table has a `user_id` column and repository queries filter by it. Follow that precedent when adding queries.

## Frontend structure

- `frontend/src/views/*.vue` — one per route (`AccountsView` (account cards + stat cards + embedded `TransactionsPanel`), `AccountsManageView`, `RulesView`, `CategoriesView`, `InvestmentsView`, `InvestmentDetailView`, `ProfileView`, `LoginView`, `SignupView`), wired in `frontend/src/router/index.ts`. Sidebar has only **Accounts** and **Investments**; **Rules and Categories are sub-routes under `/accounts`** (`/accounts/rules`, `/accounts/categories`); **Settings is in the user popup** next to Logout (`/settings` → `ProfileView`). Dashboard/Spending are hidden until their phases. Views compose components; no raw popup/card markup in views.
- `frontend/src/components/` — shared UI: `AppLayout` (sidebar shell), `AppModal` (modal shell: `title`/`wide`/`xwide` props, `close` emit, default slot), `Popover` (dropdown shell: backdrop + positioned panel, `panel-class`, `close` emit), `ConfirmDialog` (replaces native `confirm()`), `AppInput` / `AppSelect` (labeled form controls — never hand-roll input/select class strings), `Pagination` (first/prev/current/next/last, centered), `DatePicker`, `AccountCard` (the one account-card component, has `compact` prop), `StatCard`. `frontend/src/components/workspace/` holds transactions/rules workspace components (`TransactionsPanel`, `TransactionsTab`, `TransactionTable`, `AddTransactionModal`, `EditItemModal`, `TransferModal`, `RuleForm`, `RulesTab`, `CategoriesTab`, `CategoryForm`, `TransactionForm`).
- `frontend/src/lib/utils/` — pure helpers (formatting, CSV parsing, filters, rule outputs, category colors) factored out of views; each has a co-located `*.test.ts` under vitest.
- **UI conventions on every UI change:** follow `frontend/UI_STANDARDS.md` (tokens, spacing, buttons, forms, accessibility) — if a change introduces a new token, spacing, or pattern, **update UI_STANDARDS.md to match**; and **add any new testable flow to `frontend/UI_TEST_CHECKLIST.md`** so the smoke-test procedure stays complete. Both are part of the merge gate.

## Naming conventions (Rust)

- **Full words, no abbreviations.** Types/structs/fns/fields spell out domain nouns: `TransactionType`, `TransactionRepo`, `TransactionService`, `account_repo`, `transaction_repo` — never `TxnType`/`TxnRepo`/`txn_repo`. Short names allowed only for universally-clear locals (`id`, `req`, `row`, `svc`, `repo`).
- **No entity-prefix on fields.** A field on its own struct doesn't repeat the struct's noun: `Transaction.type` not `Transaction.transaction_type`, `Account.type` not `Account.account_type`. Use a raw identifier (`r#type`) for keyword collisions — never `type_`/`type_enum`.
- **No entity-prefix on functions.** Within a module, helpers don't repeat the module name: `wire()` not `txn_wire()`, `type_value()` not `account_type_value()`. Cross-module disambiguation lives in the module path (`http::transaction::wire`), not the name.
- **Enums over ints/string-maps.** Real Rust enums with `#[repr(i64)]` for the DB column, `#[serde(rename = "...")]` for the exact wire name. No protobuf-style `PREFIX_ENUM_NAME` variant names — the Rust variant is `Checking`, wire `"ACCOUNT_TYPE_CHECKING"` is a serde rename.
- **Strict validation, no sentinel variants.** No `Unspecified`/zero-value default. A missing or unknown enum value is a 400 at the boundary. Valid values are exactly what the enum defines.
- **One object, no wire structs.** The domain/service struct IS the JSON response — `#[serde(rename_all = "camelCase")]` + `#[serde(rename)]` + `serialize_with` cover Go's wire mapping. No separate `WireXxx` type unless the wire shape genuinely differs (e.g. transactions flatten a join row, null-vs-empty `categoryIds`). Request structs stay (they parse `any`-typed fields).
- **SQL column and wire names are sacred but standard.** Column and JSON names match the Go contract (`type` for accounts/transactions, `accountType`/`occurredAt` on the wire) and are NOT re-prefixed to look like Rust. `#[serde(rename)]`/`#[sqlx(rename)]` maps Rust field ↔ wire/column. Schema lives in `db/migrations/`; it's not production, so a column rename may update the migration in place and reset the dev DB.
- **Repo field names short and type-qualified when ambiguous:** a service holding two repos uses `account_repo`/`transaction_repo` (the suffix disambiguates); a field of a single type is `repo`.
- **Name by role, not by shape:** `repo`/`svc`/`id`/`req`/`row` are fine; `string`/`vec`/`list`/`data`/`value` for a typed thing are not.
- **Booleans read as predicates** (`is_linked`, `inserted`, `success`); verbs for actions (`create`, `link`, `delete`).
- **snake_case** for everything in Rust (fields, locals, fns, files). Types/structs PascalCase, consts SCREAMING_SNAKE.

## Conventions & gotchas

- **Tailwind v4 + daisyUI v5:** no `tailwind.config.js`/`postcss.config.js`. Config lives in `frontend/src/assets/main.css`: `@import "tailwindcss"`, `@plugin "daisyui"`, the custom `financer` theme, and tokens in an `@theme` block. Use tokens (`bg-surface`, `text-income`, `bg-primary-500`) — never inline hex or `bg-[#...]` arbitrary values. daisyUI v5 notes: `input`/`select` bordered by default (no `-bordered`), `form-control`/`label-text` gone, `btn-group` replaced by `join` + `join-item`.
- **DB migrations:** raw SQL in `db/migrations/`, embedded (`db.migrationsFS`), auto-applied at startup in filename order, tracked in `schema_migrations`. Add `NNNNNN_description.up.sql` / `.down.sql` pairs.
- **Env vars:** `FINANCER_DB_PATH` (default `data/financer.db`), `FINANCER_JWT_SECRET`, `FINANCER_ADDR` (`:8080`), `FINANCER_TLS_CERT`/`KEY` (enable HTTP/3 QUIC), `FINANCER_LOG_LEVEL` (`info` default, `debug` adds per-request logs — uses `logx`, a `log/slog` wrapper in `logx/logx.go`; bind request context once via `logx.Request(method, path, user)`), `FINANCER_YAHOO_CONFIG` (default `config/yahoo.json` — bases + chart/search path templates with `{symbol}`, `{interval}`, `{range}`, `{period1}`, `{period2}`, `{query}` tokens; the client retries across bases and adds `.NS` for bare symbols), quote/history refresh intervals. Frontend API URL: `VITE_API_URL` (default `http://localhost:8080`).
- **`occurred_at` storage:** the SQLite driver stores `time.Time` as `"YYYY-MM-DD HH:MM:SS +0000 UTC"` — `date()`/`strftime()` can't parse it. Extract date parts with `substr(t.occurred_at, 1, 10)` (day) or `substr(t.occurred_at, 1, 7)` (month).
- **Debt accounts** (Loan=4, Credit Card=3) count as real spending in analytics; non-debt transfers are excluded.
- **Commits:** Conventional Commits (`feat:`/`fix:`/`docs:`/`refactor:`/`test:`/`chore:`/`perf:`); one logical change per commit. Feature branches → PR → `main`. See `ROADMAP.md` for the build order.
- **Markdown filenames:** project `.md` docs use ALL_CAPS filenames (`README.md`, `AGENTS.md`, `ROADMAP.md`, `UI_STANDARDS.md`, `UI_TEST_CHECKLIST.md`). Tooling-generated docs (`.superpowers/`, `docs/superpowers/`, `graphify-out/`) keep their generator's convention.
- **PR titles:** follow the same Conventional Commits format: `<type>: <scope>: <short summary>` where scope is the phase number, e.g. `feat: phase 3: transactions CRUD, balance recalc, transfer linking` or `fix: phase 3: dead transfer result cleanup`. Use this exact format for every PR title.
- **PR bodies:** fill in `.github/PULL_REQUEST_TEMPLATE.md` and pass it with `gh pr create/edit --body-file <path>`. Never pass the body inline in the shell (PowerShell mangles escapes/quotes).
