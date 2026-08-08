# Per-Account Credit/Debit Totals Cache — Design

Date: 2026-08-08
Status: Approved (design review)

## Motivation

`GET /api/dashboard` currently computes income/expense with `TransactionRepository.SumByType` — an aggregate `SUM ... CASE WHEN type` over the `transactions` table on every dashboard load. At millions of rows this rescan happens on every visit. Cache the totals so the dashboard (and future per-account analysis) reads one table instead of scanning transactions.

## Current state

- `accounts` already maintains a delta-updated `balance` column (source of truth is `transactions`).
- Dashboard: `accRepo.SumBalance` (SUM over accounts.balance) + `txnRepo.SumByType` (SUM over transactions).
- `transactions(user_id)` is indexed; the SUM is fast today but still a per-user scan.
- Gap: `resolveTransfer` creates counterpart transactions via `txnRepo.Create` directly and never calls `UpdateBalance`, so transfer-created counterparts don't move account balances or any cached totals.

## Approaches considered

1. **Status quo + index** — keep the SUM; relies on `idx_transactions_user_id`. No schema change, always correct, but still scans transactions per dashboard load. Rejected (user wants zero scan at scale).
2. **Per-account cached `total_credit` / `total_debit`** — delta-maintained alongside `balance`, backfilled once. Dashboard reads accounts only; powers per-account analysis. **Chosen.**
3. **User-level stats row** — one cached row per user. Simpler to maintain but no per-account breakdown. Rejected.

## Design

### Schema — migration `000007_account_totals`

Add to `accounts`:
- `total_credit REAL NOT NULL DEFAULT 0`
- `total_debit  REAL NOT NULL DEFAULT 0`

Backfill once from the source of truth:
```sql
UPDATE accounts SET
  total_credit = (SELECT COALESCE(SUM(amount),0) FROM transactions t WHERE t.account_id = accounts.id AND t.type = 1),
  total_debit  = (SELECT COALESCE(SUM(amount),0) FROM transactions t WHERE t.account_id = accounts.id AND t.type = 0);
```

### Maintenance — single choke point

All transaction mutations already funnel through `services/transaction.go` (`CreateTransactions`, `UpdateTransactions`, `DeleteTransactions`), which maintain `balance` via `txnDelta` + `UpdateBalance`. Extend those paths with parallel totals maintenance via a new `AccountRepository.ApplyTotals(accountID, creditDelta, debitDelta float32)`:

- **Create:** credit txn → `total_credit += amount`; debit txn → `total_debit += amount`.
- **Update:** reverse the old txn's credit/debit, apply the new one (mirrors the existing `acctDelta` map).
- **Delete:** reverse the deleted txn's credit/debit.

Totals include all transactions (including internal transfers between own accounts) — decision (a), consistent with today's dashboard numbers. Transfer exclusion stays in Phase 10 (Spending) SQL, which needs date/category aggregates anyway.

### Fix the transfer gap

Route the counterpart creation in `services/transfer.go` `resolveTransfer` through the same balance + totals maintenance (it currently bypasses `UpdateBalance` entirely). This fixes a pre-existing bug (transfer-created counterparts not moving balances) and keeps the cache consistent.

### Dashboard

Replace `SumByType` with `SUM(total_credit)` / `SUM(total_debit)` over `accounts`. Net worth stays `SUM(accounts.balance)`. Result: dashboard reads only the `accounts` table; no transaction scan.

### Seed

`cmd/seed` sets the two columns consistently — derive them from the seeded transactions (or set hardcoded values that match the transaction sums), so post-seed dashboard numbers are self-consistent.

## Trade-offs (accepted)

- These columns are a **delta-maintained cache**; a wrong delta drifts them silently, exactly like `balance` today. `transactions` remains the source of truth; no auto-reconciliation is added.
- The per-account "spent" figure includes internal transfers by design (consistent with current dashboard).

## Testing

- Go unit/E2E: create credit → total_credit grows; create debit → total_debit grows; update (amount/type/account change) → both reverse + apply correctly; delete → reversed; transfer counterpart creation updates the target account's totals + balance.
- Dashboard endpoint returns income/expense matching the seeded transaction sums.
- Migration backfill on a DB with existing transactions matches a direct SQL aggregate.

## Out of scope

- Per-account analysis UI (future).
- Transfer exclusion in totals (Phase 10 Spending handles it in SQL).
