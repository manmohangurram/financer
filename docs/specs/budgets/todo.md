# Budgets — todo

Spec: `docs/specs/budgets/SPEC.md` · Plan: `docs/specs/budgets/plan.md`

## Backend
- [x] Migration `000004_category_budgets`
- [x] `monthly_budget` in category repo/model/scans
- [x] `SetBudget` validation (0..1e9, two decimals) + service tests
- [x] `PUT /api/categories/{id}/budget` + OpenAPI
- [x] `Category.monthlyBudget` on the wire

## Frontend
- [x] Client `setCategoryBudget`
- [x] Budgets route + sidebar (MANAGE)
- [x] Budgets page: summary band, editable table, over/unbudgeted states
- [x] Spending: Budget stat card (replaces Largest), month fetch
- [x] Spending: budget-by-category rows with fallback to the old share list
- [x] Empty states (no categories, no budgets)

## Release
- [x] Gates: gofmt/vet/build/test, vue-tsc/npm test/build
- [x] Browser smoke: set/edit/clear a budget; card and rows reflect it
- [ ] Docs/changelog + PR
