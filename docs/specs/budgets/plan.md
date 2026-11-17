# Budgets — plan

Source spec: `docs/specs/budgets/SPEC.md`

## Decisions

- **One column, not a history table.** `categories.monthly_budget` is the single
  source of truth; the amount repeats until edited. No per-month rows.
- **Budget progress is month-scoped.** The Spending range tabs keep driving the
  chart and ledger; the budget card and rows always use the current calendar month,
  because a monthly limit cannot be compared to a 7-day window.
- **Debit only.** Credits are not spend; a category's budget progress sums debits.
- **Dedicated endpoint.** `PUT /api/categories/{id}/budget` keeps the existing bulk
  category update shape untouched.
- **Editing commits on blur/Enter.** No save button per row; clearing the field
  removes the limit. Invalid input is rejected client-side and server-side.
- **Fallback is the old view.** With no budgets the Spending page renders exactly as
  before, so the feature is invisible until a limit is set.

## Work

1. Migration `000004_category_budgets`.
2. Repo/model/service: `monthly_budget`, `CategoryResponse.monthlyBudget`,
   `SetBudget` with validation + rounding.
3. HTTP `PUT /api/categories/{id}/budget`; OpenAPI schema and route.
4. Budgets view, route, sidebar item.
5. Spending: month fetch, budget computeds, stat card, category rows.
6. Tests: service lifecycle + validation, migration expectation.
7. Docs, UI checklist, standards; gates and smoke.

## Risks

- Month boundaries use local time; the spending API takes explicit `dateFrom`/`dateTo`
  so the client and server agree.
- Users with many categories: the table grows; it scrolls with the page.
- Editing while offline shows the row error and keeps the draft.

## Verification

`gofmt`, `go vet`, `go build`, `go test ./...`; frontend `vue-tsc`/`npm test`/
`npm run build`; browser smoke: set, edit and clear a budget; Spending card and
rows match; over-limit row reads correctly.
