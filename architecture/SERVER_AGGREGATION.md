# Architecture — Server-Side Aggregation

All finance math lives in the backend; the frontend renders, never aggregates.

- `GET /api/dashboard` returns `totalBalance`, `totalIncome`, `totalExpenses` and the account list. Totals come from the cached `accounts.balance` / `total_credit` / `total_debit` columns, which are recomputed from transactions on every create/update/delete.
- `GET /api/spending?range=7D|1M|6M|1Y` (or `from`/`to`) returns day/month buckets and per-category debit/credit/net. Buckets are computed in Go so they honour the configured timezone (a DST-correct local bucket cannot be expressed in SQL); ranges over 30 days switch to monthly buckets. Rows come from `TransactionRepo.SpendingRows`, which excludes non-debt transfers via `transfer_links` (CREDIT_CARD/LOAN accounts remain included).
- `GET /api/transactions` filtering (account, category, type, date range, amount range, name search), sorting and cursor pagination are all server-side.
