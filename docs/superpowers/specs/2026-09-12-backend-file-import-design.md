# Backend file import (CSV / XLSX / PDF) — design

Issue: #118 · 2026-09-12

## Problem

Import is split and client-heavy. CSV and XLSX are parsed **in the browser**
(`frontend/src/lib/utils/csv.ts`, `workbook.ts` via `read-excel-file`), mapped to
transactions in JS (`mapCsvRowsToTransactions`), and POSTed to
`/api/transactions`. Only PDF is parsed server-side (`src/pdf.rs`), through a
different, single-shot endpoint.

Consequences: two import paths, two API shapes, parsing/mapping rules that the
API and MCP cannot reuse, and an `read-excel-file` dependency shipped to every
browser.

## Goals

- The **backend owns file parsing and column mapping** for CSV, XLSX and PDF.
- One two-step contract: **upload → (headers, rowCount) → map → commit**.
- Re-importing the same file stays idempotent.

## Non-goals

- Changing how transactions are created (rules, balances, transfers) — the
  commit reuses the existing service.
- Supporting formats beyond CSV / XLSX(XLS/ODS) / PDF.

## Design

### Dependencies

| Purpose | Crate |
|---|---|
| CSV | `csv` |
| XLSX / XLS / ODS | `calamine` |
| PDF | `pdfsink-rs` (already present) |

`read-excel-file` is removed from the frontend.

### Step 1 — upload + preview

`POST /api/transactions/import/file` — `multipart/form-data`, field `file`.

1. Pick a parser by extension: `.csv` → `csv`, `.xlsx`/`.xls` → `calamine`,
   `.pdf` → `pdfsink-rs`. Anything else → `400 unsupported file type`.
2. Persist the raw bytes to **`/tmp/financer-uploads/<uuid>.<ext>`**.
3. Compute `sha256(bytes)` — the dedupe prefix for the whole file.
4. Parse **headers only** (first row) and the **data-row count**.
5. Respond `{ "id": "<uuid>", "headers": [...], "rowCount": <n> }`.

The client never receives the rows, and never supplies a filesystem path.

Errors: unsupported type · no header row · encrypted PDF (`/Encrypt`) ·
unreadable/empty file.

### Step 2 — commit

`POST /api/transactions/import/file/commit` — JSON:

```json
{ "id": "<uuid>", "accountId": "<id>", "mapping": { "date": 0, "description": 1, "amount": 4, "type": -1, "debit": -1, "credit": -1 }, "amountFormat": "single" }
```

1. Resolve the file **by `id`** (unknown/expired → `400`). No client paths.
2. Re-parse the stored file and apply the mapping → transactions, with
   `externalId = "<sha256>:<rowIndex>"`.
3. Create via **`TransactionService::create`** (write-time rules, balances,
   transfer-linking all apply), batching internally to ≤1000 per call.
4. **Delete the temp file**.
5. Respond with the existing bulk result (`created`/`skipped`/`failedIds`).

### Mapping semantics (moved to Rust)

| Field | Meaning |
|---|---|
| `date` | transaction date (day-first `DD/MM/YYYY`, `YYYY-MM-DD`, …) |
| `description` | transaction name |
| `amount` | single signed/unsigned amount (with `type`) |
| `type` | `CREDIT`/`DEBIT` marker column |
| `debit` / `credit` | split amount columns (`amountFormat: "split"`) |

`amountFormat: "single"` uses `amount` (+ optional `type`); `"split"` uses
`debit`/`credit` and turns each into the matching type/amount.

### Housekeeping

Temp files live in `/tmp/financer-uploads/`, are deleted on commit, and are
swept by age (1 hour) at startup and on each upload.

### Frontend

One path for all three formats: drop file → upload → `{id, headers, rowCount}` →
existing mapping UI (showing "Import N rows") → commit. Removed:
`parseCsvText`, `parseCsvLine`, `workbook.ts`, `mapCsvRowsToTransactions`,
`csvFileKey`, and the `read-excel-file` dependency. Kept: `guessMapping`
(headers still arrive from the server).

### Removed endpoints

`POST /api/transactions/import/pdf` (replaced by the two-step flow).

## Security

- The commit takes an **opaque id**; the server maps it to its own stored file.
  A client-supplied path is never accepted (path traversal).
- Ownership is encoded in the filename: the temp file is
  `/<tmp>/financer-uploads/<user_id>_<uuid>.<ext>`. The commit looks up
  `<user_id>_<id>.*` for the authenticated user, so another user's id does not
  resolve. No sidecar/meta file and nothing extra to sweep.

## Edge cases

- Re-import of the same file: identical `sha256` → same `externalId`s → skipped.
- Rows imported **before** this change used a client `fnv1a` key, so they will
  not dedupe against a new import — accepted, one-time.
- Large files: parsing happens twice (upload and commit); files are statements,
  so this is bounded and simpler than caching rows in memory.

## Acceptance

- [ ] `POST /import/file` returns `{id, headers, rowCount}` for CSV, XLSX and PDF.
- [ ] `POST /import/file/commit` creates transactions through the normal path,
      with `externalId = <sha256>:<index>`, and deletes the temp file.
- [ ] Encrypted PDFs, unsupported types and unknown ids return clear 400s.
- [ ] The frontend has a single upload→map→commit path; no CSV/XLSX parsing remains in JS.
- [ ] `cargo clippy --all-targets -- -D warnings` + `cargo test` pass; `vue-tsc` + vitest pass.
