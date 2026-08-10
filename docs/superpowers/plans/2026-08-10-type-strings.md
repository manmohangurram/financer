# String-based Enums Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace int-based `type`/`accountType` values on the wire and in the DB with stable lowercase strings, using Rust enums as the single source.

**Architecture:** `AccountType` and `TransactionType` become `serde::Serialize + Deserialize + sqlx::Type` enums with `rename_all = "snake_case"` for both wire and DB. Request structs declare the enum directly; an axum `JsonRejection` handler maps serde errors to the `400` error envelope. DB migrations `000002`/`000003`/`000007` change `type INTEGER` → `type TEXT` (lowercase values); dev DB reset. Frontend enum maps updated to lowercase; unsupported Crypto Wallet dropped; transfer-counterpart keys normalized to camelCase.

**Tech Stack:** Rust (axum 0.8, sqlx 0.8 sqlite, serde), Go (deprecated, untouched for these columns), Vue 3 + TypeScript.

## Global Constraints

- Naming per `AGENTS.md`: full words (`TransactionType` not `TxnType`), no entity-prefixed fields (`Transaction.type`), real enums with serde renames, strict validation (no sentinel variants), one object no wire structs.
- Wire + DB values: transaction `type` = `"debit"`/`"credit"`; account `accountType` = `"checking"`/`"savings"`/`"credit_card"`/`"loan"`.
- Error envelope stays `{code, message}`; bad enum → `400` `invalid_argument`.
- `cargo clippy -- -D warnings` must pass; `cargo test` green; Go tests still run (unowned-route breakage on renamed columns accepted, not asserted).
- Migrations may be edited in place and dev DB reset (not production). `000007` backfill updated to `'credit'`/`'debit'`.
- All work lands on `feat/rust-type-strings` (stacked after `feat/rust-transactions`).

---

### Task 1: String enums in repo layer

**Files:**
- Modify: `src/repo/account.rs` (AccountType enum + AccountRow)
- Modify: `src/repo/transaction.rs` (TransactionType enum + Transaction row/filter)
- Test: existing `#[cfg(test)]` modules in both files

**Interfaces:**
- Produces: `AccountType { Checking, Savings, CreditCard, Loan }` and `TransactionType { Debit, Credit }`, both `#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type)]` with `#[serde(rename_all = "snake_case")]` + `#[sqlx(rename_all = "snake_case")]`.
- Removes: `#[repr(i64)]`, `TryFrom<i64>`, `from_wire()`, `db_value()` on both enums; `TxnListFilter.r#type` stays `Option<TransactionType>`.

- [ ] **Step 1: Rewrite `AccountType` enum in `src/repo/account.rs`**

Replace the current enum + `TryFrom` + `from_wire` + `db_value` block with:

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type)]
#[serde(rename_all = "snake_case")]
#[sqlx(rename_all = "snake_case")]
pub enum AccountType {
    Checking,
    Savings,
    CreditCard,
    Loan,
}
```

- [ ] **Step 2: Rewrite `TransactionType` enum in `src/repo/transaction.rs`**

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, sqlx::Type)]
#[serde(rename_all = "snake_case")]
#[sqlx(rename_all = "snake_case")]
pub enum TransactionType {
    Debit,
    Credit,
}
```

Keep `TransactionListFilter.r#type: Option<TransactionType>`.

- [ ] **Step 3: Fix all int-typed bindings in the repo**

In `src/repo/transaction.rs`, every SQL `bind`/arg that previously used `.db_value()` or a raw int now binds the enum directly (sqlx `Type` derive encodes it as TEXT). Specifically:
- `create`: `.bind(t.r#type)` (was `.bind(t.r#type.db_value())`)
- `update`: `.bind(input.txn.r#type)` (was `.db_value()`)
- `find_transfer_counterpart` param `r#type: TransactionType`, `.bind(r#type)` (was `.db_value()`)
- `build_transaction_where`: filter arm `args.push(t.to_string())` → `args.push(sql_str(t))` where `sql_str` is `format!("{t:?}").to_lowercase()` — or simpler, keep `t.to_string()` if `Debug` gives `Debit`/`Credit`; use a `pub fn wire_value(self) -> &'static str` on each enum returning `"debit"`/`"credit"`/`"checking"`/etc. and `args.push(wire_value(t).to_string())`.

Add `wire_value` to both enums:

```rust
impl AccountType {
    pub fn wire_value(self) -> &'static str {
        match self {
            Self::Checking => "checking",
            Self::Savings => "savings",
            Self::CreditCard => "credit_card",
            Self::Loan => "loan",
        }
    }
}
```

```rust
impl TransactionType {
    pub fn wire_value(self) -> &'static str {
        match self {
            Self::Debit => "debit",
            Self::Credit => "credit",
        }
    }
}
```

- `RawTransaction`/`RawAccount` read `type` back as TEXT: change the raw struct field `r#type: i64` → `r#type: String`, and the `From`/`from_row` conversions use `TransactionType::from_str(&r.r#type).unwrap_or(TransactionType::Debit)` (add `use std::str::FromStr;`). Do the same for account.

- [ ] **Step 4: Update tests in both repo modules**

Test helpers `txn(id, name, amount, r#type, ...)` keep taking `TransactionType`; the `.db_value()` sorting/assert calls become `.wire_value()` or compare enum directly. Account test `types.sort()` on `db_value()` → collect `.wire_value()` strings or just compare `Vec<AccountType>`. Add `use std::str::FromStr;` where needed.

- [ ] **Step 5: Build, clippy, test**

Run: `cargo build && cargo clippy -- -D warnings && cargo test`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add src/repo/account.rs src/repo/transaction.rs
git commit -m "refactor: string enums for account/transaction types in repo layer"
```

---

### Task 2: DB migrations to TEXT

**Files:**
- Modify: `db/migrations/000002_accounts.up.sql`
- Modify: `db/migrations/000003_transactions_transfers.up.sql`
- Modify: `db/migrations/000007_account_totals.up.sql`

- [ ] **Step 1: Change `000002` account type column**

`type INTEGER NOT NULL DEFAULT 1,` → `type TEXT NOT NULL,`

- [ ] **Step 2: Change `000003` transaction type column**

`type INTEGER NOT NULL,` → `type TEXT NOT NULL,`

- [ ] **Step 3: Update `000007` backfill**

In the two `UPDATE accounts SET` statements, change `t.type = 1` → `t.type = 'credit'` and `t.type = 0` → `t.type = 'debit'`.

- [ ] **Step 4: Reset dev DB**

Run: `rm -f data/financer.db data/db/financer.db data/*.db-wal data/*.db-shm data/db/*.db-wal data/db/*.db-shm`

- [ ] **Step 5: Verify migrations apply**

Run: `cargo build && FINANCER_ADDR=127.0.0.1:0 FINANCER_DATA_DIR=data target/debug/financer` (briefly) or `cargo test` (tests run migrations). Expected: schema has `type TEXT`.
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add db/migrations/000002_accounts.up.sql db/migrations/000003_transactions_transfers.up.sql db/migrations/000007_account_totals.up.sql
git commit -m "refactor: store account/transaction type as TEXT in DB"
```

---

### Task 3: Strict request deserialization + JsonRejection → 400 envelope

**Files:**
- Modify: `src/error.rs`
- Modify: `src/http/account.rs`
- Modify: `src/http/transaction.rs`
- Modify: `src/service/account.rs` (delete `type_value`)

**Interfaces:**
- Consumes: `AccountType`, `TransactionType` from Task 1.
- Produces: `impl IntoResponse for axum::extract::rejection::JsonRejection` returning `400` `{code:"invalid_argument", message}`.

- [ ] **Step 1: Add JsonRejection handler to `src/error.rs`**

```rust
use axum::extract::rejection::JsonRejection;

impl IntoResponse for JsonRejection {
    fn into_response(self) -> Response {
        ApiError::bad_request(self.body_text()).into_response()
    }
}
```

Place after the existing `ApiError` impl block.

- [ ] **Step 2: Delete `type_value` from `src/service/account.rs`**

Remove the `pub fn type_value` function entirely (serde now validates).

- [ ] **Step 3: Type the account request field as the enum**

In `src/http/account.rs`:
- `ReqAccount.account_type: serde_json::Value` → `account_type: AccountType` (serde derives Deserialize via snake_case).
- Delete `use crate::service::account::type_value;`.
- In `create`/`update` handlers, replace the `let t = match type_value(...)` block with `let t = req.account_type;` (the extractor already validated; a bad value → JsonRejection → 400).

- [ ] **Step 4: Type the transaction request field as the enum**

In `src/http/transaction.rs`:
- `JsonListTxn.r#type: serde_json::Value` → `r#type: TransactionType`.
- Delete `fn type_value` (the standalone helper).
- In `create`/`update`, replace `let ty = match type_value(&t.r#type)` with `let ty = t.r#type;`.
- Wire response emission: `r#type: if r.txn.r#type == TransactionType::Credit { "CREDIT" } else { "DEBIT" }.to_string()` → `r#type: r.txn.r#type.wire_value().to_string()`.
- List query `type` param: `match TransactionType::from_wire(s)` → `match TransactionType::from_str(s)`; the `Some("")`/`None` case → `None`.

- [ ] **Step 5: Build, clippy, test**

Run: `cargo build && cargo clippy -- -D warnings && cargo test`
Expected: pass.

- [ ] **Step 6: Add strict-400 tests**

In `src/http/transaction.rs` or a `#[cfg(test)]` module: assert that an unknown enum value in a request body produces `400` `invalid_argument`. Since handlers are axum extractors, test via the service boundary instead — or add a unit test on `AccountType::from_str` / `TransactionType::from_str` failing for `"bogus"`.

```rust
#[test]
fn rejects_unknown_type() {
    use std::str::FromStr;
    assert!(TransactionType::from_str("bogus").is_err());
    assert!(AccountType::from_str("bogus").is_err());
}
```

- [ ] **Step 7: Commit**

```bash
git add src/error.rs src/http/account.rs src/http/transaction.rs src/service/account.rs
git commit -m "feat: strict enum request validation with 400 error envelope"
```

---

### Task 4: Transfer counterpart camelCase keys

**Files:**
- Modify: `src/http/transfer.rs`

- [ ] **Step 1: Change the counterpart response keys**

In `counterpart` handler, replace the PascalCase `json!` object keys:

```rust
Json(serde_json::json!({
    "debitTransactionId": r.debit_transaction_id,
    "creditTransactionId": r.credit_transaction_id,
}))
```

- [ ] **Step 2: Build, clippy, test**

Run: `cargo build && cargo clippy -- -D warnings && cargo test`
Expected: pass.

- [ ] **Step 3: Commit**

```bash
git add src/http/transfer.rs
git commit -m "feat: normalize transfer counterpart response keys to camelCase"
```

---

### Task 5: Frontend lowercase enums

**Files:**
- Modify: `frontend/src/lib/utils/accountType.ts`
- Modify: `frontend/src/components/workspace/TransactionForm.vue`
- Modify: `frontend/src/components/workspace/FilterTransactionsPopover.vue`
- Modify: `frontend/src/lib/utils/transactionFilters.ts`
- Test: `frontend/src/lib/utils/accountType.test.ts` (if exists)

- [ ] **Step 1: Rewrite `accountType.ts` to lowercase**

```ts
export const ACCOUNT_TYPE_OPTIONS: AccountTypeOption[] = [
  { value: 'checking', name: 'Checking' },
  { value: 'savings', name: 'Savings' },
  { value: 'credit_card', name: 'Credit Card' },
  { value: 'loan', name: 'Loan' }
];

const LABELS: Record<string, string> = { checking: 'Checking', savings: 'Savings', credit_card: 'Credit Card', loan: 'Loan' };
const ICONS: Record<string, string> = {
  checking: '🏦',
  savings: '💰',
  credit_card: '💳',
  loan: '🏷️'
};

export function accountTypeLabel(type: string): string {
  return LABELS[type] || 'Checking';
}

export function accountTypeIcon(type: string): string {
  return ICONS[type] || '🏦';
}

export function isCredit(type: string): boolean {
  return type === 'credit_card';
}

export function isDebt(type: string): boolean {
  return type === 'loan' || type === 'credit_card';
}
```

(Crypto Wallet removed per spec.)

- [ ] **Step 2: Update `TransactionForm.vue` type radio values**

`[{ v: 0, label: 'Debit' }, { v: 1, label: 'Credit' }]` → `[{ v: 'debit', label: 'Debit' }, { v: 'credit', label: 'Credit' }]`

- [ ] **Step 3: Update transaction filters to lowercase**

`frontend/src/lib/utils/transactionFilters.ts`: `type: '' | 'CREDIT' | 'DEBIT'` → `type: '' | 'credit' | 'debit'`; bubble label check `filters.type === 'CREDIT'` → `'credit'`.
`FilterTransactionsPopover.vue`: `<option value="CREDIT">` → `value="credit"`, `DEBIT` → `debit`.

- [ ] **Step 4: Update any other `ACCOUNT_TYPE_*` string refs**

Run `grep -rn "ACCOUNT_TYPE_" frontend/src` and update remaining literal values to lowercase (e.g. `AccountFormModal.vue`, `TransferModal.vue`, `TransactionTable.vue`, `CategoriesTab.vue` if they compare against the old strings).

- [ ] **Step 5: Build + test frontend**

Run (from `frontend/`): `npm run build && npm test`
Expected: pass.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/utils/accountType.ts frontend/src/components/workspace/TransactionForm.vue frontend/src/components/workspace/FilterTransactionsPopover.vue frontend/src/lib/utils/transactionFilters.ts
git commit -m "feat: frontend sends lowercase account/transaction type values"
```

---

### Task 6: Docs + final verification

**Files:**
- Modify: `migration.md`

- [ ] **Step 1: Update `migration.md`**

Confirm the "Strict enum validation" section already documents lowercase values (done during brainstorming). Add the DB column note (`type TEXT`, lowercase) if not present.

- [ ] **Step 2: Full gate**

Run from repo root: `cargo build && cargo clippy -- -D warnings && cargo test && go build ./... && go test ./...`
Run from `frontend/`: `npm run build && npm test`
Expected: all green.

- [ ] **Step 3: Live wire check**

Run Rust server; `POST /api/accounts` with `{"bankName":"X","accountType":"checking"}` → 201 with `"accountType":"checking"`; with `"accountType":"bogus"` → 400 `invalid_argument`. Same for transactions `type`. Confirm `data/financer.db` stores `checking`/`debit` strings.

- [ ] **Step 4: Commit**

```bash
git add migration.md
git commit -m "docs: migration notes reflect lowercase string enums"
```

---

## Self-Review

- **Spec coverage:** enums (T1), DB TEXT + reset (T2), strict request enum + 400 (T3), camelCase counterpart (T4), frontend lowercase + drop Crypto Wallet (T5), docs/gate (T6). All spec sections mapped.
- **Placeholders:** none — each step has concrete code/commands.
- **Type consistency:** `wire_value() -> &'static str` used consistently (repo filter args, http emission); `from_str` for query params; enums named `AccountType`/`TransactionType` throughout; `TransactionListFilter` naming consistent with the full-word rule.
