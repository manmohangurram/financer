# Architecture — Server-Side Aggregation

All finance math lives in the backend; the frontend renders, never aggregates.

- `GET /api/dashboard` returns the account position plus the current month at a glance:
  - `totalBalance`, `totalIncome`, `totalExpenses` from the cached account columns (all-time).
  - `cash`, `savings`, `debt` split by account type; `totalBalance = cash + savings − debt`.
  - `month`: label/from/to/daysLeft, income/expenses/net for the current calendar month, average/day,
    previous-month expenses and change percent, top debit category, peak day, overall budget
    (planned/spent/left/used/hasBudgets/overCount) and one zero-filled daily bucket per day to date.
  - `recent`: the latest six transactions; `recentTotal` is their signed sum.
  - Month totals come from `TransactionRepo.Totals`, which uses the same transfer scope as
    `SpendingRows`; budget spend comes from the category debit totals in `Service.Spending`; the
    budget plan comes from `categories.monthly_budget`.
- `GET /api/spending?range=7D|1M|6M|1Y` (or `from`/`to`) returns day/month buckets and per-category
  debit/credit/net. Buckets are computed in Go so they honour the configured timezone (a DST-correct
  local bucket cannot be expressed in SQL); ranges over 30 days switch to monthly buckets. Rows come
  from `TransactionRepo.SpendingRows`, which excludes non-debt transfers via `transfer_links`
  (CREDIT_CARD/LOAN accounts remain included).
- `GET /api/transactions` filtering (account, category, type, date range, amount range, name search),
  sorting and cursor pagination are all server-side.
- The dashboard and the spending page share `wireTransactions` for recent/list rows, so clean names,
  transfer links and category links render identically in both.
