# Dashboard redesign — plan

## Stage 1 — backend

- `internal/repo/transaction.go`: add `Totals(userID, from, to)`.
- `internal/repo/sqlite/transaction.go`: implement `Totals` with the same transfer scope as
  `SpendingRows`; share the scope clause.
- `internal/transactionservice/service.go`: add `MonthTotals` (local date strings → stored bounds).
- `internal/transactionservice/spending_test.go`: test month bounds and transfer exclusion.
- `internal/model/transaction.go`: `WireDashboardMonth`, `WireDashboardBudget`,
  `WireDashboardTopCategory`; extend `WireDashboard`.
- `internal/server/analytics.go`: aggregate cash/savings/debt, month totals, previous-month delta,
  daily buckets, peak bucket, top category, budget totals/over count, recent entries.
- `internal/server/transactions.go`: share `wireTransactions` with the dashboard.
- `internal/openapi/spec.go`: document the payload.

## Stage 2 — daisyUI audit and standard

- Script the current direct daisyUI usage across `frontend/src`.
- Write `docs/audits/daisyui-usage.md` with counts, gaps and migration order.
- Add the daisyUI-first rule to `frontend/UI_STANDARDS.md` and `AGENTS.md`.

## Stage 3 — dashboard UI

- `frontend/src/components/dashboard/NetWorthBand.vue`
- `frontend/src/components/dashboard/BudgetPanel.vue`
- `frontend/src/components/dashboard/AccountsPanel.vue`
- `frontend/src/components/dashboard/DashboardSpendingLine.vue` (Apex smooth line, no fill)
- `frontend/src/components/dashboard/RecentEntries.vue`
- `frontend/src/views/DashboardView.vue`: fetch, loading/empty/error, composition.
- Use daisyUI `card`, `progress`, `table`, `badge`, `alert`, `skeleton`, `btn`, `link` as the
  starting classes; ledger tokens override the look.

## Stage 4 — docs and verification

- Update `frontend/UI_STANDARDS.md` §6.3 and §8.
- Update `frontend/UI_TEST_CHECKLIST.md`.
- Update `architecture/SERVER_AGGREGATION.md` and `architecture/FRONTEND_STRUCTURE.md`.
- Run all gates and verify the real app at 1440/390 in both themes.
