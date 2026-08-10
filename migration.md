# Backend Migration Notes (Go → Rust)

Status of the backend migration from Go to Rust (see `ROADMAP.md` for the plan).
The wire contract is unchanged — Rust serves the same JSON/status codes the
frontend already consumes. This file tracks what the backend now runs as and
any behavior the frontend should know about. The frontend does **not** change
for a migration phase unless this file says the contract changed.

## Runtime

- **Two processes during migration:** the Rust server is the entry point on
  `:8080` (serves the frontend + owns migrated routes). Everything under
  `/api/*` that Rust does not own yet is proxied to the Go backend on `:8081`.
  Both share the same SQLite file (`data/financer.db`). No data copy — the file
  is the contract.
- **Migrated (Rust-owned) routes:** auth, profile, accounts, transactions,
  transfer-links. Everything else (categories, rules, investments, dashboard,
  spending, user settings/avatar) still runs on Go and is proxied.
- **Frontend:** unchanged. `VITE_API_URL` / `window.__API_BASE__` points at the
  Rust server; the Rust server proxies what it doesn't own.

## Wire contract (unchanged, but noted)

- Error envelope everywhere: `{"code": "...", "message": "..."}`.
  `code` values: `invalid_argument` (400), `unauthenticated` (401),
  `not_found` (404), `already_exists` (409), `internal` (500).
- JSON keys are **camelCase** unless noted (`accountType`, `occurredAt`,
  `linkedTransferId`, `categoryIds`, `nextPageToken`, `totalCount`).

### Strict enum validation — lowercase string enums

`accountType` and transaction `type` are **strict lowercase string enums** on
the wire and in the DB. A missing, unknown, or out-of-range value is a
`400 invalid_argument`. There is no "unspecified" sentinel and no implicit
default, and no positional ints (`0`/`1`) are ever sent or stored — values are
stable strings so enum reordering can't corrupt stored data.

- `accountType` accepts `checking` / `savings` / `credit_card` / `loan`
  (or their numeric DB-era equivalents during transition). Anything else → 400.
- Transaction `type` accepts `"debit"` / `"credit"`. Anything else → 400.

### Enum responses

Responses use the same lowercase strings (`"checking"`, `"credit"`, `"debit"`).
The frontend's enum maps are keyed by these lowercase values.

## Endpoints owned by Rust

### Auth
- `POST /api/auth/signup` → 201 `{accessToken, refreshToken, userId, email, name}`
- `POST /api/auth/login` → 200 same shape
- `POST /api/auth/refresh` → 200 same shape
- `GET /api/me/profile` → `{userId, name, email, avatarUrl}`

### Accounts
- `GET /api/accounts` → `{accounts: [...]}`
- `POST /api/accounts` → 201 `{id, bankName, accountNickname, accountType, balance, createdAt}`
- `PUT /api/accounts/{id}` → 201 same shape
- `DELETE /api/accounts/{id}` → 204

### Transactions
- `GET /api/transactions` — filters via query params: `pageSize`, `pageToken`,
  `accountId`, `type`, `categoryId` (comma list), `names` (comma list, OR-match),
  `dateFrom`, `dateTo`, `minAmount`, `maxAmount`, `sortBy` (`debit`/`credit`),
  `sortDir` (`asc`/`desc`), `offset`. Returns
  `{transactions: [...], nextPageToken, totalCount}`.
- `POST /api/transactions` → 201 bulk result
  `{success, message, failedIds?, skipped?}`; body `{transactions: [...]}`,
  each `{name, amount, type, occurredAt, accountId, categoryIds?, externalId?}`.
  `occurredAt` accepts RFC3339, `YYYY-MM-DD`, or `{seconds, nanos}`.
- `PUT /api/transactions` → 200 bulk result; body carries full `categoryIds`
  (replaces the set).
- `DELETE /api/transactions` → 204; body `{ids: [...]}`.

### Transfer links
- `POST /api/transfer-links` → 201 bulk result; body `{links: [...]}` each
  `{debitTransactionId, creditTransactionId}`.
- `DELETE /api/transfer-links` → 204; body `{ids: [...]}`.
- `POST /api/transfer-links/counterpart` → 201
  `{"DebitTransactionId": "...", "CreditTransactionId": "..."}` —
  **note PascalCase keys** (Go's response type has no JSON tags; Rust mirrors
  it byte-for-byte). Body `{transactionId, toAccountId}`.

## Transaction shape (list/create response item)

```json
{
  "id": "uuid",
  "name": "Coffee",
  "amount": 5.5,
  "type": "DEBIT",
  "occurredAt": "2024-01-02T10:00:00Z",
  "accountId": "uuid",
  "createdAt": "2026-08-10T15:24:52Z",
  "linkedTransferId": "uuid-or-empty",
  "categoryIds": null
}
```

- `categoryIds` is `null` when the transaction has no categories (mirrors Go's
  nil slice) — treat empty as no categories.
- `linkedTransferId` is `""` when the transaction is not part of a transfer.
- Balances and per-account credit/debit totals are kept in sync by the backend
  on create/update/delete; the frontend should re-fetch after mutations.

## Database schema notes

- Migrations live in `db/migrations/` (SQL, applied at boot in filename order).
  Both backends share the schema and the `schema_migrations` tracker.
- Account type column is named `type` (not `account_type`); transaction type
  column is also `type`. Rust fields use `r#type` and map via serde/sqlx
  renames — the DB column and wire key stay as documented here.
- Dev DB can be deleted to re-run migrations from scratch (not production).

## Not yet migrated (still Go, proxied)

Categories, rules, investments (incl. Yahoo quotes, FIFO lots, import), user
settings (profile edit, avatar upload, password, logout-all), dashboard,
spending. Their wire shapes are unchanged while on Go.

## For UI work

Use this file as the reference for what the backend currently serves. If a
change is needed to the wire contract (e.g. a new field), update the Go API
types + handlers first (or the Rust side once that route is migrated), then
update this file. Keep `frontend/UI_TEST_CHECKLIST.md` in sync with any new
flow.
