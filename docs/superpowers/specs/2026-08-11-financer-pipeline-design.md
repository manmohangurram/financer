# Financer Pipeline — Workflow Engine + Email Ingestion

Date: 2026-08-11
Status: Approved design (pending spec review)
Repo: new standalone repo (moved from this workspace at dev time; sibling under `~/projects/`)

## Purpose

A workflow-centric application (separate from financer) that runs N-step pipelines.
Each workflow = a trigger (starter/dispatcher) + a template (ordered module steps).
The first real use case: email → decrypt PDF → parse → submit to financer's API.
Decoupled from financer: financer is only a downstream consumer of the submitter script.

## Non-Goals (explicitly out)

- No HTTP-callback submitter support (script-only — avoids remote-execution attack surface)
- No workflow branching/DAG (linear steps now; model left open for branches later)
- No financer-credential coupling in the pipeline DB (submitter scripts own their financer login)
- No IMAP IDLE push (poll-based only)
- No multi-user/tenant model (single-user self-hosted)

## Stack

- Rust backend: axum + sqlx (SQLite), tokio; single binary serving REST + SPA + scheduler
- Frontend: Vue 3 + Vite + TypeScript + Tailwind v4 + daisyUI v5 (mirrors financer frontend setup)
- SQLite storage in `data/`; run artifacts (attachments, logs, manifests) on disk under `data/runs/{run_id}/`
- Migrations: raw SQL in `db/migrations/`, applied at boot (reuse financer's migration-runner pattern)
- Own GitHub repo, Conventional Commits, CI = cargo build/clippy/test + frontend build/test (mirror financer's `ci.yml`)

## Architecture

```
financer-pipeline/
├── Cargo.toml
├── frontend/                # Vue 3 + Vite SPA
├── src/
│   ├── main.rs              # boot: DB, migrations, trigger scheduler, axum server
│   ├── db.rs                # SQLite pools + migration runner
│   ├── auth.rs              # local login (own users table)
│   ├── engine/              # workflow engine
│   │   ├── mod.rs           # Module trait, registry, Step, Workflow
│   │   ├── runner.rs        # executes a run: iterate steps, state machine, log stream, timeout
│   │   ├── router.rs        # trigger → run dispatch (route-order matching)
│   │   └── replay.rs        # re-run from stored payload
│   ├── modules/             # module implementations (each implements Module)
│   │   ├── listener/imap.rs # (used by email trigger's filter/download)
│   │   ├── decrypter/pdf.rs
│   │   ├── parser/{groww_mf.rs, hdfc_bank.rs, xlsx.rs}
│   │   └── submitter/script.rs
│   ├── http/                # REST API (axum)
│   ├── repo/                # SQL repos
│   └── scheduler.rs         # background + cron trigger loops (tokio)
├── data/                    # SQLite + runs/{run_id}/...
└── db/migrations/
```

### Module trait (pluggability seam)

```rust
#[async_trait]
trait Module {
    fn id(&self) -> String;
    fn kind(&self) -> ModuleKind;  // listener | decrypter | parser | submitter
    async fn run(&self, ctx: &RunContext) -> Result<StepOutput>;
}
```

- `RunContext`: run id, working dir (attachments/decrypted/parsed), config (serde JSON), log handle
- `StepOutput`: data passed to next step (attachment list, parsed rows, etc.)
- Registry: `HashMap<(ModuleKind, String), Box<dyn Module>>`; modules self-register at boot
- New module types = implement trait + register — no engine changes

## Data Model (SQLite)

```
users
  id, email, password_hash, created_at
  -- pipeline's OWN login only; no financer coupling

accounts (IMAP)
  id, name, host, port, username, password, folder (default INBOX), tls, created_at

triggers
  id, name, type ('email'|'cron'|'manual'), config (JSON), enabled, created_at
  -- email: { account_id, poll_interval_secs, routes: [...] }
  -- cron:  { cron_expr, routes: [...] }
  -- manual: { } (button-triggered; routes optional)

workflows
  id, name, enabled, timeout_secs, trigger_id, created_at, updated_at
  -- a workflow = trigger + template; template = its steps

workflow_steps
  id, workflow_id, position, module_kind, module_instance_id, config (JSON)
  -- ordered steps; position unique per workflow

routes
  id, trigger_id, filter (JSON), workflow_id, position
  -- email route filter: sender / subject / attachment-name matchers (OR/AND)
  -- evaluated in position order; first match spawns that workflow's run
  -- no match → email skipped silently

runs
  id, workflow_id, trigger_id, trigger ('auto'|'cron'|'manual'|'rerun'),
  status ('running'|'passed'|'failed'|'timed_out'),
  payload_dir, started_at, finished_at, timeout_secs
  -- one run per trigger event; concurrent runs of same workflow allowed

run_steps
  id, run_id, position, module_kind, status ('running'|'passed'|'failed'),
  error, started_at, finished_at

settings
  key, value        -- retention_days (default 30), poll defaults, etc.
```

- **Trigger = dispatcher**: a trigger can route to many workflows; each route's filter decides which workflow a matched email starts. Cron/manual triggers may also route (single route = trivial case).
- **Re-run**: replay from step 1 of the template using the run's stored payload dir (trigger + listener skipped; no re-poll). The stored `attachment/` dir is copied to a fresh working dir; steps 2..N (decrypter → parser → submitter) re-execute against those files, regenerating `decrypted/` and `parsed/`.

## Runner Semantics

- Strict chain: iterate `workflow_steps` in order; any step fails → run `failed`, stop (no continuation)
- Whole-run timeout (`timeout_secs`): kill the tokio task → `timed_out`, manual re-run only
- Runs of the same workflow never overlap for a given trigger event, but different events run concurrently
- Step output flows through `RunContext` in memory; working dir holds files on disk

## Scheduler / Trigger Runtime

- At boot, spawn one background task per enabled trigger:
  - **email**: poll loop every `poll_interval_secs` → IMAP fetch → apply routes' filters → for each match download attachments to a fresh payload dir → spawn run
  - **cron**: ticker evaluates `cron_expr` → spawn run (only on next matching tick; no catch-up after downtime)
  - **manual**: no task; `POST /api/triggers/{id}/run` spawns immediately
- Manual force-pull per trigger (button in UI)

## Submitter Module (script runner)

- Config: `script_path`, `args` (template with `{payload_dir}` / `{rows_file}` tokens), `timeout_secs`
- Runner writes parsed rows as JSON to a temp file, invokes script, streams stdout+stderr into run log
- exit 0 = passed; non-zero = failed (chain stops)
- Financer login + import logic lives entirely inside the script (fully decoupled)
- No HTTP-callback variant (explicitly out)

## Logging & Observability

Per-run folder `data/runs/{run_id}/`:
```
run.log               — JSON-lines: {ts, run_id, step, module, level, message}
errors.log            — failures only: {ts, step, module, error, payload_ref}
payload-manifest.json — step-by-step summary
attachment/           — raw downloaded files
decrypted/            — decrypter output
parsed/               — parser output (rows JSON)
```

manifest.json:
```json
{
  "runId": "...", "workflow": "...", "trigger": "email", "status": "failed",
  "startedAt": "...", "finishedAt": "...", "timeoutSecs": 300,
  "steps": [
    {"position": 1, "module": "decrypter:pdf", "status": "passed",
     "summary": {"inputFiles": 2, "decrypted": 2, "passwordsTried": 3}},
    {"position": 2, "module": "parser:groww_mf", "status": "failed",
     "summary": {"filesRead": 2, "rowsParsed": 0}, "error": "..."}
  ],
  "payloadDir": "attachment/"
}
```

Analysis helpers:
- `GET /api/runs?status=failed&workflow_id=…` filter
- `GET /api/runs/{id}/manifest`, `/log`, `/errors` (log tail-streamed live)
- Run list shows failure step + error summary inline

Retention: runs + payload/attachment dirs kept (for re-run); `run.log`/`errors.log` pruned after `retention_days` (configurable).

## REST API (draft)

```
POST   /api/auth/login, /api/auth/signup          # local pipeline auth
GET/POST/PUT/DELETE /api/accounts                 # IMAP accounts
GET/POST/PUT/DELETE /api/triggers                 # triggers (config embedded)
GET/POST/PUT/DELETE /api/workflows                # workflow = trigger + template
GET/PUT/DELETE /api/workflows/{id}/steps          # ordered module steps
POST   /api/triggers/{id}/run                     # manual / force-pull
POST   /api/runs/{id}/rerun                       # replay from stored payload
GET    /api/runs?workflow_id=&status=&page=       # run list
GET    /api/runs/{id}/manifest
GET    /api/runs/{id}/log
GET    /api/runs/{id}/errors
GET/PUT /api/settings
```

Module instances are registered at boot (built-in set); step config selects a `module_kind` + instance id.

## UI

- **Workflows**: create = name → pick trigger (email/cron/manual) + configure → pick template steps (add/remove/reorder modules) → configure each step
- Email trigger config: IMAP account (pick or create) + filter rules + route→workflow mapping
- Cron config: cron expression
- **Runs**: list per workflow with status badges (running/passed/failed/timed_out), trigger badge, duration
- **Run detail**: step timeline from manifest; each step expandable → its log lines; failed step highlighted with error + link to errors.log; live tail while running; re-run button
- **Accounts**: CRUD
- **Settings**: retention days, etc.

## Error Handling

- Step failure → `failed`, error logged to run.log + errors.log, manifest records error, chain stops
- Timeout → `timed_out` (killed task), manual re-run
- Decryption failure (no password works) → step `failed`, attachment kept, skipped per workflow choice
- No route match → email skipped silently
- Submitter non-zero exit → step failed with script output captured

## Testing

**Backend**:
- Engine: chain semantics (fail → stop), re-run skips trigger, timeout kills, router order-matching, no-overlap
- Repo: SQLite CRUD (triggers/workflows/steps/runs/manifest) — in-memory SQLite pattern
- Modules: decrypter (password PDF + wrong-password fixtures), parsers (groww xlsx + HDFC PDF fixtures → rows), submitter (fake scripts exit 0/1)
- Cron expression edge cases

**Frontend**: vitest for pure helpers (manifest formatting, trigger route forms); browser smoke test.

**Integration**:
- Email trigger against a local IMAP test server (fixture mailbox): poll → filter → route → spawn run
- E2E: fake submitter script asserts it received rows JSON from a parsed fixture email
- Manual: real IMAP account + real financer

## Initial Module Set

- listener/imap (used by email trigger filter+download)
- decrypter/pdf (password-protected PDF, per-workflow password list)
- parser/groww_mf (Groww mutual-fund order history — xlsx and PDF variants)
- parser/hdfc_bank (HDFC bank statements, PDF password-protected)
- parser/xlsx (generic xlsx → rows, for the submitter)
- submitter/script (execute customer script; rows JSON in)

New parsers/formats = new code implementing `Module` + registering (per design: fixed built-in parsers, no UI templates).
