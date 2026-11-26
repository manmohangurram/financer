# Dashboard redesign — SPEC

Status: approved · implemented in `DashboardView.vue` · 2026-11-18

## Problem

The dashboard answers "what is my total balance?" and lists accounts. It does not answer, at a
glance, what the month cost, whether the budget is on plan, and what the net movement was.

## Objective

`/dashboard` becomes the month-at-a-glance page:

1. **Net worth** and what it is made of (cash, savings account, owed).
2. **This month**: in, out, net.
3. **Budget**: what is left, how much of the plan is used, planned and spent.
4. **Accounts**: the individual balances behind net worth.
5. **Spending**: the daily shape of the month as a line chart.
6. **Recent entries**: the evidence, without a selection column.

"Simple and information loaded" means one card answers one question and the reading order is fixed.
It does not mean fewer numbers; it means no number appears twice without a reason.

## Non-goals

- No savings card. Removed by request; the savings-account balance remains only as a net-worth
  composition segment.
- No period switcher, month navigation, forecasting, goals or investments.
- No new colour, font, radius, shadow or spacing token.
- No new chart library. daisyUI has no chart component; the shared `AppApexChart` engine is used
  for the spending line, with the ledger palette.
- No changes to transaction write paths or MCP tools.

## Success criteria

- At 1440×900 the first screen names net worth, this month's spend, budget left and accounts.
- Every figure is server-computed and arrives in one `GET /api/dashboard` request.
- Loading, empty, error and content states exist for the page.
- Light and dark pass contrast; 390px has no horizontal page scroll.
- `go test ./...`, `go vet ./...`, `go build ./...`, `npm test`, `npm run build` pass.
- Every interactive/structural primitive in the new dashboard starts from a daisyUI class
  (`card`, `progress`, `table`, `badge`, `alert`, `skeleton`, `btn`, `link`).

## Layout

```
┌ page head ─────────────────────────────────────────────────────────────────┐
│ Dashboard · the month so far                         November 2026  Ledger →│
├ net-worth pair ────────────────────────────────────────────────────────────┤
│ NET WORTH              ₹5,17,810.00 │ COMPOSITION                         │
│ as of 18 Nov                        │ Savings vs owed                     │
│ IN +₹1,20,000  OUT −₹84,500  NET …  │ [savings █████████ | owed █]         │
│                                     │ SAVINGS ₹6,09,200 87.0%             │
│                                     │ OWED    ₹91,390   13.0%             │
├ budget (2×2) ──────────────────────┼ accounts (right) ──────────────────────┤
│ BUDGET · NOVEMBER 2026      1 OVER │ Accounts                    Manage      │
│ LEFT TO SPEND        │ USED        │ Main Checking     ₹2,10,000.00         │
│ ₹11,800              │ [=======]   │ Emergency Fund    ₹3,20,000.00         │
│ 12 days left         │ 80.3% used  │ Credit Card      −₹47,690.00           │
│ ─────────────────────┴──────────── │                                         │
│ PLANNED ₹60,000      │ SPENT ₹48,200│                                        │
├ spending (full width) ─────────────────────────────────────────────────────┤
│ Spent this month ₹84,500 · ₹4,694/day · 8.2% less than October             │
│ [daily line, average rule, highest-day callout]                            │
├ recent entries (full width) ───────────────────────────────────────────────┤
│ Entry (date · account)                                      Amount          │
│ 6 rows + carried forward                                                   │
└────────────────────────────────────────────────────────────────────────────┘
```

Mobile stacks in the same order: net worth, budget, accounts, spending, recent. The budget top
row stacks (text, then bar) and planned/spent remain a two-column strip. The recent table drops
the account column; the account moves into the date line. No horizontal page scroll.

## Data contract

`GET /api/dashboard`:

```json
{
  "totalBalance": 482310.0,
  "totalIncome": 120000.0,
  "totalExpenses": 84500.0,
  "cash": 210000.0,
  "savings": 320000.0,
  "debt": 47690.0,
  "accounts": [ ... ],
  "month": {
    "label": "November 2026",
    "from": "2026-11-01",
    "to": "2026-11-30",
    "daysLeft": 12,
    "income": 120000.0,
    "expenses": 84500.0,
    "net": 35500.0,
    "averagePerDay": 4694.44,
    "previousLabel": "October 2026",
    "previousExpenses": 92000.0,
    "expenseChangePct": -8.15,
    "topCategory": { "name": "Groceries", "amount": 12400.0, "share": 14.67 },
    "peakBucket": { "key": "2026-11-14", "label": "14 Nov", "amount": 9600.0 },
    "budget": { "planned": 60000.0, "spent": 48200.0, "left": 11800.0,
                "used": 80.33, "hasBudgets": true, "overCount": 1 },
    "buckets": [ { "key": "2026-11-01", "label": "1 Nov", "amount": 0.0 } ]
  },
  "recent": [ ... ]
}
```

Rules:

- `totalBalance = cash + savings − debt` for the four account types.
- `month.income` / `month.expenses` exclude internal transfers with the same scope as
  `/api/spending`; `month.net = income − expenses`.
- `month.buckets` covers every local day from the 1st through today, with zero days present.
- `month.budget.spent` is spend in categories with a positive monthly limit; `left` may be negative.
  `overCount` is the number of budgeted categories whose spend exceeds their limit.
- `month.peakBucket` is the highest daily debit in the month to date, or absent.
- `recent` is the latest 6 transactions, newest first.
- `totalIncome` / `totalExpenses` stay all-time for the MCP summary.

## Components and state

| Surface | Loading | Empty | Error |
|---|---|---|---|
| Page | daisyUI `skeleton` bars in the real grid | no accounts: one ruled page + Add account | `alert alert-error` + retry |
| Net-worth band | skeleton amount + three bars | hidden when no accounts | banner |
| Budget | skeleton amount + bar | “No monthly budget yet.” + Set budgets | banner |
| Spending | skeleton chart | “Nothing spent yet this month.” | banner |
| Recent | 3 skeleton rows | “Nothing booked yet.” + Add entry | banner |

## daisyUI rule

daisyUI is the component layer. Start every interactive or structural primitive from its class —
`card`, `btn`, `input`, `select`, `table`, `progress`, `badge`, `alert`, `skeleton`, `link` — then
override the visual properties with the ledger tokens. daisyUI has no chart component; charts use
the shared `AppApexChart` engine and `src/lib/chartPalette.ts`. A hand-rolled replacement of a
daisyUI primitive is a bug in the same way an unstyled `<div>` is.

## Accessibility

- Budget progress: `role="progressbar"`, `aria-valuenow/min/max`, label “Budget used”.
- Net-worth composition bar: `role="img"` with a full text alternative.
- The line chart is accompanied by total, average and largest category as text.
- Credit/debit figures keep `+`/`−`; over-budget has the `1 over` word plus the count.
- h1 page, h2 cards, no skipped levels. Focus rings unchanged. Reduced motion respected.

## Verification

- Go: `gofmt -l cmd internal migrate.go`, `go vet ./...`, `go build ./...`, `go test ./...`,
  `govulncheck ./...`.
- Frontend: `npx vue-tsc -b`, `npm test`, `npm run build`.
- Browser: 1440×900 and 390×844, light and dark; console clean; no horizontal overflow.
- The daisyUI audit report is recorded in `docs/audits/daisyui-usage.md`.
