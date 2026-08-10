# Design: String-based enums (drop int wire/DB values)

Date: 2026-08-10
Phase: `feat/rust-type-strings` (stacked after `feat/rust-transactions`)

## Problem

Transaction `type` and account `accountType` currently travel as **ints**:
`0`/`1` for debit/credit, `1`–`4` for account types. The DB stores the same
ints (`type INTEGER`). This couples the wire contract and stored data to enum
**ordering** — reordering or inserting a variant silently corrupts stored data
and API meanings. The migration no longer needs strict byte-parity with the Go
wire, so these should be clean, stable **strings**.

## Goal

- Wire (request + response) uses lowercase strings: transaction `type` =
  `"debit"`/`"credit"`; account `accountType` = `"checking"`/`"savings"`/
  `"credit_card"`/`"loan"`.
- DB columns store the same lowercase strings (`TEXT`), not ints.
- One Rust enum per domain is the single source: serde serializes the wire
  string, sqlx stores/reads the DB string. No `#[repr(i64)]`, no
  `db_value()`, no `from_wire()`.
- Request bodies deserialize **into the enum**; unknown values → `400`
  `invalid_argument` via the standard error envelope.
- Frontend updated to send/receive the lowercase values; the unsupported
  Crypto Wallet account type is dropped.
- Go is **not** updated for these columns — it is being replaced and its
  int-based queries will silently match nothing on unowned routes (accepted).

## Approach (locked during brainstorming)

1. **Enums** — `AccountType` and `TransactionType` in their repo modules:

   ```rust
   #[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type)]
   #[serde(rename_all = "snake_case")]
   #[sqlx(rename_all = "snake_case")]
   pub enum AccountType { Checking, Savings, CreditCard, Loan }

   #[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type)]
   #[serde(rename_all = "snake_case")]
   #[sqlx(rename_all = "snake_case")]
   pub enum TransactionType { Debit, Credit }
   ```

   Delete `#[repr(i64)]`, `TryFrom<i64>`, `from_wire`, `db_value`, and the
   `type_value()` helper fns (serde + sqlx replace them). `TxnListFilter.r#type`
   stays `Option<TransactionType>` — `None` means "no type filter".

2. **DB migrations, edited in place** (`000002`, `000003`):
   - `accounts.type INTEGER NOT NULL DEFAULT 1` → `type TEXT NOT NULL`
   - `transactions.type INTEGER NOT NULL` → `type TEXT NOT NULL`
   - Dev DB reset so migrations re-run. (`000007` backfill references
     `t.type = 1` — update to `t.type = 'credit'`.)

3. **Request bodies** — request structs declare the enum directly
   (`account_type: AccountType`, `type: TransactionType`). Malformed bodies /
   unknown enum values surface as serde errors; add an axum `JsonRejection`
   `IntoResponse` that maps them to `400 invalid_argument` with the standard
   `{code, message}` envelope (matches Go's `decodeBody`/`typeValue` 400s).

4. **List query param** `type` — `"debit"`/`"credit"` (string), parsed into
   `Option<TransactionType>`; unknown → 400.

5. **Transfer counterpart keys** — normalize to camelCase
   `{"debitTransactionId", "creditTransactionId"}` (Go's untagged PascalCase
   dropped, since wire parity is no longer strict).

6. **Frontend** — `frontend/src/lib/utils/accountType.ts`: option values →
   `checking`/`savings`/`credit_card`/`loan`, remove Crypto Wallet, update
   `LABELS`/emoji maps. `TransactionForm.vue`: type radio values →
   `'debit'`/`'credit'`. Any filter/query building that sends `type` or
   `accountType` uses lowercase. Backend enum maps are keyed by lowercase.

7. **Go** — no changes to type handling. Unowned Go routes (dashboard,
   spending, rules `run`, investments) degrade on these columns until rebuilt
   in Rust. `migration.md` updated to note the contract change.

## Out of scope

- Categories/rules/investments/dashboard/spending migration (later ROADMAP phases).
- `investment_type` column (still Go-owned).
- Keeping Go's unowned routes functional against renamed columns.

## Testing

- Repo tests updated to string enums; add a strict-400 test: `POST /api/accounts`
  with `accountType: "bogus"` → `400` `{"code":"invalid_argument",...}`;
  same for `POST /api/transactions` with `type: "bogus"`.
- `cargo build`, `cargo clippy -- -D warnings`, `cargo test`.
- Frontend: `npm run build && npm test`.
- Live wire check: create account + transaction via Rust, confirm lowercase
  values on wire and in DB; Go unowned breakage accepted (not asserted).

## Files touched

- `db/migrations/000002_accounts.up.sql`, `000003_transactions_transfers.up.sql`,
  `000007_account_totals.up.sql`
- `src/repo/account.rs`, `src/repo/transaction.rs`
- `src/service/account.rs`, `src/service/transaction.rs`, `src/service/transfer.rs`
- `src/http/account.rs`, `src/http/transaction.rs`, `src/http/transfer.rs`
- `src/error.rs` (JsonRejection handler)
- `frontend/src/lib/utils/accountType.ts`,
  `frontend/src/components/workspace/TransactionForm.vue` (+ filters)
- `migration.md`, `AGENTS.md` (if naming rules need updating)
