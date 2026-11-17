# Changelog

All notable changes to Financer. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions are [SemVer](https://semver.org/spec/v2.0.0.html) and release tags are `vX.Y.Z`.

Entries are grouped by theme. An entry starts at the first tag it appeared in; earlier history lives in the
git log (`git log --oneline v0.7.0..v0.8.0`).

## [0.9.1] — 2026-11-17

### Added

- **Rule-gated Inbox:** a normal poll stores only mail that matches a rule; unmatched
  mail is skipped and never appears. Ignored matches stay stored (hidden) for dedupe.
- **Full poll** (`POST /api/mail/poll/full`): background scan of the whole mailbox with
  no date window; matched rows insert in batches of 100.
- **Startup cleanup:** unmatched, transaction-free mail is deleted on boot; booked and
  rejected history stays.
- **AI parse** (`POST/GET /api/mail/ai-parse`): per-sender background job using up to
  five unrejected bodies, forcing template generation past the cooldown, then
  re-parsing that sender's pending mail. While it runs, approve/reject returns 409 and
  the Inbox disables the buttons and shows sender progress.
- **Create the missing account:** when no account matches the mail's ending digits,
  approving offers to create one with that number and books automatically. Existing
  matches are selected without asking.

## [0.9.0] — 2026-11-17

### Fixed

- **SSRF in Inbox AI settings:** the base URL is validated for link-local/metadata
  targets and the resolved addresses are checked again immediately before the
  request; mailbox hosts use the same shared guard.
- Inbox polling no longer blanks the queue: background refreshes are silent, and
  mailboxes/categories/accounts are fetched once rather than on every 3-second tick.
- Search is debounced (300 ms) and stale list/detail responses are dropped, so an
  older request cannot overwrite a newer one.
- Creating or renaming a category together with its budget is one request; the
  fragile create-then-find-by-name path is gone.
- Email bodies render at most 400 paragraph blocks; the reader keeps its fixed
  height.
- New mail rules join the end of the list instead of all sharing position 0, and
  deleting one is a two-step inline confirm rather than a native dialog.

### Added

- Cursor pagination on every growable list endpoint: accounts, rules, mailboxes
  and mail rules now accept `pageSize`/`pageToken` and return `nextPageToken`, in
  addition to the existing cursor pages on transactions, categories and mail
  messages. Transaction views with an explicit `sortBy`/`sortDir` are cursor-
  paginated too; `offset` remains only for the numbered ledger pages. API keys stay
  unpaginated because the service caps them at 50.
- Cursor-pagination tests for the mail queue (disjoint pages, final page, invalid
  token, status union, search) and accounts.
- Mail rules can target an account and categories (AUTO pre-booking), matching the
  backend fields.
- `idx_mail_messages_user_sender` (migration `000005`) for LLM sample lookups.

### Changed

- **Inbox AI wording is provider-neutral.** The settings tab now talks about an
  OpenAI-compatible base URL and model (LiteLLM, DeepSeek direct, OpenAI, Ollama)
  instead of naming LiteLLM. Pasting a provider's full `…/chat/completions` URL is
  accepted as-is; a base URL still gets `/chat/completions` appended. The connection
  test prompt contains the word “json” so JSON-mode providers accept it.
- Inbox queue is cursor-paginated: `GET /api/mail/messages` takes `pageToken`,
  `pageSize` (default 20; `limit` still accepted), `accountId` and `search`, and
  returns `total`/`totalCount` + `nextPageToken`. Filters (status, mailbox, search)
  now run in SQL, so each page is a real 20-row fetch; approve/reject refreshes and
  returns to the first page.
- Inbox account picker lists the user's financial accounts (with ending digits), not
  their mailboxes.
- The email body has a fixed-height scroll area, so long mail never pushes the
  booking form off screen.
- Settings has a **Mail rules** tab: sender/subject/body conditions with
  review / auto / ignore, editable in a popup with Delete.

## [0.8.0] — 2026-11-17

### Added

- **Per-category monthly budgets** — one recurring limit per category, edited from the
  **Categories** page (MANAGE). The table is a name and a budget; a row opens the popup
  where name, budget and Delete live. Spending owns the analysis: a Budget card
  (left/over, bar, "spent of planned · % used") replaces `Largest`, and
  `Where it went` becomes `Budget by category` when limits exist. Budgets apply to
  debit spend only and never roll over.
- **Budget progress on Spending** — the `Largest` stat is replaced by a Budget card
  (left/over, bar, “spent of planned · used”) and `Where it went` becomes
  `Budget by category` when limits exist, falling back to the range share list when
  they do not. Budget progress always uses the current calendar month.
- `GET /api/categories` returns `monthlyBudget`; `PUT /api/categories/{id}/budget`
  sets or clears it (validated, rounded to paise).
- Migration `000004_category_budgets`.
- **Inbox email reader** — the stored message is parsed at ingest (MIME decoded,
  quoted-printable/base64, charsets) and shown as readable text above the booking
  form; `text/html` is converted to text so remote markup is never rendered. An
  "Original message" toggle reveals the raw source.
- `GET /api/mail/messages/{id}` returns one message with `bodyText` and `rawText`;
  rows stored before the parser existed are parsed on demand.
- `mail_messages.body_text` and `mail_messages.from_name` (migration `000003_mail_body`).

### Changed

- The content column is fluid; pages are no longer capped at 1180px.
- The Inbox is a full-bleed split view: queue with its own visible scrollbar, and a
  detail column that stacks the email card over the booking card (email on top,
  fields below). Form controls use daisyUI `input`/`select`/`btn`/`badge`.
- `GET /api/mail/messages` no longer returns the raw message body; it returns
  `fromName` and a one-line `bodyPreview` instead.
- Keyboard shortcut hints (`A` / `R` / `J` / `K`) and their global handlers were
  removed from the Inbox; the actions are the named buttons.

### Fixed

- Dashboard → Recent entries no longer reserves an empty margin column, and its
  empty state has an `Add account` action.
- Rules showed `OR` for `ALL` logic and a meaningless `AND`/`OR` stamp on
  single-condition rules; the label is now correct and hidden when there is one
  condition. Rule rows open the edit popup (Delete lives there) and the columns are
  grid-aligned; `Run` is the only inline action.
- Category rows are a name and a monthly budget; analysis stays on Spending. A row
  opens the edit popup (name + budget + Delete); no inline inputs or icons.
- The Transactions search field no longer stretches across the toolbar.
- ApexCharts received an ignored top-level `type`; the chart type is now set via
  `chart.type`.
- The Spending carried-forward line now sits at the bottom of the ledger instead of
  floating above empty space.

## [0.7.0-alpha] — 2026-11-17

Mail ingestion is now template-first: stored, guarded regex templates parse bank alerts, and the
user's self-hosted LiteLLM (DeepSeek) is asked for a template only when parsing fails. The Inbox is a
split-pane reviewer and Settings is tabbed.

### Added

- **Learned parsing templates** (`mail_templates`): one sender can have several guarded variants. A
  generated template starts `provisional` and becomes `active` after the configured sample count; a
  working template is never overwritten. Every candidate must compile on RE2 and parse every sample
  before it is stored.
- **Per-user Inbox AI settings** (`user_llm_settings`): LiteLLM base URL, model alias (required, no
  default), API key encrypted with `FINANCER_SECRET_KEY`, timeout, enabled toggle, and thresholds for
  samples, failures, generation cooldown and auto-poll throttle. Keyed endpoints: `GET/PUT
  /api/me/llm`, `POST /api/me/llm/test`; the key is never returned (only `apiKeySet`).
- **Asynchronous polling**: `POST /api/mail/poll` returns `202` with `started` or `throttled`, starts
  on login and when the Inbox opens, and is throttled per user (`?force=1` to force). The background
  poller still runs every `FINANCER_MAIL_POLL_SECS`.
- **Editable mailboxes**: `PUT /api/mail/accounts/{id}` changes label/host/username/password
  (optional, empty keeps the stored one) and polling on/off.
- **Field overrides on approve**: name, amount, type, date, account and categories can be corrected
  before approving; the corrected extraction is stored in `approved_json` as template evidence.
- **Split-pane Inbox**: mailbox tabs, status filters, email pane plus editable extraction with inline
  validation, and `A` / `R` / `J` / `K` keyboard shortcuts.
- **Tabbed Settings**: Account, Appearance, Security, Inbox AI, Mailboxes and API keys, URL-addressable
  via `/settings?tab=…`.

### Changed

- **Mail is fetched from all senders.** The mailbox-level sender allowlist is removed; mail rules
  (`IGNORE` / `REVIEW` / `AUTO`) decide disposition, first match wins.
- **`AUTO` is gated**: a transaction is created automatically only with an active template, a complete
  extraction (amount, direction, date) and a resolvable account hint. Ambiguous or partial rows stay in
  `PENDING` / `UNPARSED` for review.
- Parsing order: `IGNORE` rules → stored templates → template generation → deterministic regex hints for
  pre-filling the review form.

### Migrations

- `000002_mail_templates` adds `mail_templates` and `user_llm_settings`, adds
  `mail_messages.template_id` and drops `mail_accounts.sender_allowlist`. Existing 0.6 databases upgrade
  in place; back up `/data` first.

## [0.6.0-alpha] — 2026-11-17

The schema is now a single baseline. **This release requires a fresh database**;
databases created by 0.5.x or the Rust backend are not migrated.

### Changed (breaking)

- **One migration describes the final schema.** The 18 incremental migrations
  (and their `ALTER`/drop-history replay) are replaced by
  `000001_initial_schema.up.sql`, including every final column and all query
  indexes. The runner is a plain apply-pending loop.
- **Reset procedure:** stop the container, delete `/data/financer.db` plus
  `-wal`/`-shm`, start again. Named-volume users can remove the volume for a
  full reset (this also removes avatars/logs). Back up first if needed.

### Added

- Indexes for `rule_conditions.rule_id` and `rule_actions.rule_id`.
- `user_keys.user_id` now cascades on user deletion.

## [0.5.1-alpha] — 2026-11-17

Patch release: startup on an existing (Rust-era) database no longer fails, plus
an index audit.

### Fixed

- **A partially recorded migration tracker no longer blocks boot.** The sqlx
  migration history is adopted on every start (not only when the Go tracker is
  empty), and `ALTER TABLE … ADD COLUMN` steps are skipped when the column
  already exists or its table was dropped by a later migration. This recovers
  databases where an earlier boot recorded only the first few migrations and
  then failed on `duplicate column name: avatar_url`. (#176)

### Performance

- **Query-shape indexes** (migration `000018`): transactions by
  `user_id, occurred_at, id` and by `user_id, account_id, occurred_at`; inbox
  messages by `user_id, created_at`; rules by `user_id, priority, name`; and
  `transaction_categories(category_id)` for category filters. (#176)

## [0.5.0-alpha] — 2026-11-17

The backend rewrite release. The API is now a clean-slate Go service; the Rust
implementation is gone and the database is adopted in place.

### Changed

- **The backend is Go.** Gin router, `glebarez/sqlite` (pure Go, WAL), one writer
  + read pool, embedded migrations and timezone data. The `/api` contract,
  SQLite schema, env vars and JWT/API-key formats are unchanged; an existing
  database is adopted via its sqlx migration history. (#174)
- **Rules still snapshot at write time**, and transfer rules create/link the
  missing counterpart on create.
- **Mail ingest, API keys, MCP (`/mcp`) and OpenAPI (`/docs`, `/openapi.json`)
  are implemented in Go**, including the arm64 image path.
- **Images are built only when a `v*` tag is created**, for `linux/arm64` only.

### Added

- **Structured logging** via `FINANCER_LOG_LEVEL` and `FINANCER_LOG_FILE`:
  request lines with status/duration/user, 5xx causes, panic stacks, startup and
  migration summaries. A file-backed log rotates at 10 MB and keeps 3 files.
- Graceful startup summary of the resolved configuration (never secrets).

### Removed

- **Statement import** (CSV/XLSX/PDF) from the backend and UI.
- The Rust implementation, `Cargo.toml`/`Cargo.lock`, `clippy.toml` and
  `.cargo/`; the architecture docs and standards now describe the Go service.

### Fixed

- Duplicate signup returns `409 already_exists` instead of a 500 leaking the
  SQLite unique-constraint message.
- Timestamps render in the configured timezone for driver-returned RFC3339
  values; the IANA database is embedded.
- 5xx responses log their underlying cause.

## [0.4.0-alpha] — 2026-11-15

A redesign release. The frontend was rebuilt on one design system — tokens, primitives, and then every
page — and the investments feature was deleted with it.

### Changed

- **The UI is one system now: "the ledger".** Paper surfaces, hairline rules, one petrol accent, a ruled
  margin column that carries state (the reconciliation tick, the priority number, the row's type), and
  two families: Newsreader for titles and the carried-forward figure, IBM Plex Sans everywhere else.
  Light is the default; dark is the same ledger at night. (#166, #167)
- **The primitives were rebuilt, not restyled:** `AppButton`, `AppField`, `AppSelect` (a real listbox —
  there is no native `<select>` left in the app), `AppTabs`, `AppStamp`, `AppTick`, `AppModal`,
  `AppMenu`, `AppToaster`, `AppFolio`, `AppTable`, `AppEmpty`, `AppSkeleton`. (#167)
- **Spending is a bento page:** four fact tiles (spent, largest, categories, average per day), spend per
  day as columns you click to narrow the ledger, and a full-width allocation strip whose segments show
  name · amount · share on hover. The ledger below scrolls on its own; the page fills the window. (#169,
  #171, #172)
- **Charts are ApexCharts**, loaded asynchronously on Spending only so the entry bundle stays ~90 kB
  gzipped; the palette is read from the CSS tokens and re-read when the theme flips. (#171)
- Every other page — Dashboard, Accounts, Manage, Categories, Rules, Inbox, Settings, Login, Signup —
  and every modal and form behind them is on the same components. (#172)
- `frontend/UI_STANDARDS.md` was rewritten from scratch: tokens, type scale, the margin, component
  states, per-page composition, and the rules the code is checked against. (#166, #172)

### Removed

- **Investments.** The pages, the API, the service and the repository are gone, and migration
  `000017_drop_investments` drops the tables on boot. (#170)
- Dead frontend components with nothing importing them: `ExpenseChart`, `TimeRange`, `AppBars`,
  `AppLineChart`, `SpendingTrendChart`, and the per-category colour module that painted chips from 24
  hex hues. (#172)

### Fixed

- **Inbox: `?status=pending` answered `400 unknown status`.** The filter uppercases before parsing the
  enum now. (#172)
- **Inbox: a paged mail query answered `500 datatype mismatch`** — the statement mixed numbered and
  positional placeholders; it uses plain `?` throughout. (#172)
- Mobile: the Inbox queue clipped the sender and the Reject button, the Dashboard ledger table drew its
  header across a third of the card (a four-cell footer for a three-column table), and the Spending
  columns were drawn at 28% opacity with no day selected. All three were caught at 390px and fixed.
  (#172)

## [0.3.0-alpha] — 2026-11-14

This release is about one new feature — mail in, transactions out — and one big deletion: the SurrealDB
backend is gone, and the published image is arm64 only.

### Added

- **Inbox: email → transactions.** Link an IMAP mailbox, poll it on a timer, and turn bank-statement mail
  into transactions.
  - IMAP ingest with implicit TLS, a required sender allowlist, and the mailbox password encrypted at rest
    with `FINANCER_SECRET_KEY`. (#150, #154)
  - Background poll (`FINANCER_MAIL_POLL_SECS`, default 300s) plus on-demand polling, with per-mailbox
    error reporting. (#154)
  - Disposition rules per sender — `IGNORE`, `REVIEW`, `AUTO` — with the first match winning; `AUTO` rows
    become transactions during the poll. (#155)
  - Approve / reject from the Inbox; approving creates the transaction through the normal service path, so
    the rule engine, balances and categories apply. (#155)
  - Inbox UI: link a mailbox, review what needs a person, manage disposition rules, and let the Inbox
    suggest an `AUTO` rule for senders that keep being approved. (#157)
  - Account resolution by the account/card ending digits, with the account picked by hand when they do not
    resolve. (#148, #157)
- **Statement import.** CSV, XLSX and PDF statements are parsed on the backend: upload → map columns →
  commit, for transactions and for investments. (#129, #130, #133, #136, #137)
- **Sidebar** grouped into sections, showing account balances. (#146)
- **Spending** opens on a range that has data. (#139)
- **Docs:** a full Inbox section and the mail env vars. (#163)

### Changed

- **Rust 1.100 everywhere** — `rust-version`, the CI toolchain and the Docker build image are pinned, so a
  new clippy release can no longer break a green `main`. (#151, #158)
- **Dependencies:** sqlx 0.9, utoipa 6, utoipa-axum 0.3, utoipa-swagger-ui 10, calamine 0.38, plus a
  lockfile refresh. `jsonwebtoken` stays on 9 (12 needs a crypto-provider feature for HS256 only). (#158)
- **Docker:** the image is built on a native arm runner and published on tag, cached through the Actions
  cache. (#158)
- Rules preview uses the request date and backfills existing transactions on create/update/run. (#138)

### Fixed

- **Account balances and totals were patched outside the write transaction**, so a crash between the two
  left transactions stored with balances that no longer matched them, and nothing could recompute them.
  They are now recomputed from their own rows inside the same transaction. (#161)
- **Login had no attempt limit.** Five failed attempts per email in 15 minutes now answer `429` before the
  password is hashed; a successful login clears the counter. (#161)
- Bank-statement headers are recognised when guessing import columns. (#128)
- `rustls` bumped for RUSTSEC-2026-0285. (#145)

### Performance

- Transaction list dropped a `COUNT(*) OVER ()` column that made SQLite sort the user's whole history for
  every page: 300 ms → 30 ms at 500k transactions, with the total counted separately. (#159)
- `transfer_links` gained indexes on both sides of the join used by the list and spending queries. (#159)

### Removed

- **The SurrealDB backend**, entirely: the `src/repo/surreal/` implementations, the `surreal` cargo
  feature, the `surrealdb` dependency, the `-surreal` image variant, and the compose file that bundled a
  SurrealDB. SQLite is the only storage. (#158)
- **The `linux/amd64` image.** The published image is `linux/arm64` only. (#158)
- **`docker-compose.minimal.yml`** — `docker-compose.yml` is now that single-container SQLite setup. (#158)
- Unused dependencies and dead code (`utoipa-axum`, `AccountRepo::update_balance`/`apply_totals`,
  `TransactionRepo::get_by_id_batch`, the service-side balance delta bookkeeping). (#158, #161)
- CI: the floating `stable` toolchain, and `cargo install cargo-audit` compiling from source. (#158, #159)

### Upgrading from 0.2.0-alpha

- **SurrealDB users must migrate their data to SQLite.** `FINANCER_DATABASE` and `FINANCER_SURREAL_*` are
  ignored now; there is no compatibility path and no image that reads a SurrealDB instance.
- **x86 hosts can no longer pull the image.** It is arm64 only.
- New env vars for the Inbox: `FINANCER_SECRET_KEY` (64 hex chars, `openssl rand -hex 32`) and optionally
  `FINANCER_MAIL_POLL_SECS`. Keep the secret key stable — changing it makes stored mailbox passwords
  unreadable.
- Mailbox routes require HTTPS (loopback exempt); behind a proxy, forward `X-Forwarded-Proto: https`.
