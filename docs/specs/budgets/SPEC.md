# Budgets — SPEC

Status: approved
Owner: Spending, Categories
Related: `docs/specs/inbox-reader/SPEC.md`

## Objective

Give every category a **recurring monthly spending limit** so the month can be
planned, not just reported. One amount per category, repeating until changed; no
per-month history, no rollover, no overall cap.

## Behaviour

### Categories page (`/accounts/categories`, MANAGE)

Budgets live on the Categories page — one page owns the list and the plan.

- Header: `Categories`, one-line purpose, `Add category`.
- Summary band (visible once any limit exists): **Planned** (sum of limits),
  **Spent** (this month, debit only, budgeted categories), **Left** (planned − spent;
  debit-red and labelled *over* when negative), **Used** bar (capped at 100%).
- Table, one row per category:
  `Category · Share this month · Spent · Entries · Monthly budget (input) · Used bar · Left`.
- Amounts commit on blur or Enter; an empty field means no budget (0). Invalid or
  negative values show an inline error and are not saved.
- Over-limit rows get a debit wash, an `over` badge and a red "₹x over"; unbudgeted
  rows show `—` and *unbudgeted*.
- Income-only categories are listed but can stay unbudgeted; budgets only measure
  **debit** spend, for the current calendar month.
- No categories: the existing empty state, wording mentions limits.

### Spending page

- The **Largest** stat is replaced by a **Budget · <month>** card:
  - remaining amount (or amount *over*) in the display face,
  - a progress bar (accent, debit when over),
  - `<spent> of <planned> · <n>% used`.
  - No budgets yet: "No budgets yet" with a link to Categories.
- The right-hand card becomes **Budget by category** when at least one budget
  exists: budgeted categories first (spent / limit, left/over, progress bar), then
  unbudgeted categories with spend only (grey dot, *unbudgeted*).
- Without budgets, the existing range-based **Where it went** share list stays.
- Budget progress always uses the **current calendar month**, independent of the
  7D/1M/6M/1Y range that drives the chart and ledger.

### API

- `GET /api/categories` includes `monthlyBudget` (number, 0 = none).
- `PUT /api/categories/{id}/budget` with `{ "monthlyBudget": number }`:
  - 200 with the updated `Category`;
  - 400 for negative, NaN/Inf or above 1,000,000,000;
  - 404 for an unknown/foreign category.
  - Value is rounded to two decimals.

### Schema

`000004_category_budgets` adds `categories.monthly_budget REAL NOT NULL DEFAULT 0`.

## Non-goals

- Overall/total budget separate from categories.
- Per-month budget values or history.
- Rollover of unspent amounts.
- Alerts or notifications.
- Budgets on income (credit) amounts.

## Acceptance

- Set Food ₹2,500 once; it persists across reloads and months.
- Spending shows ₹2,500 planned for Food and a month-to-date bar.
- Clearing the input removes the limit and restores the unbudgeted rendering.
- Over-limit rows read as over in text, colour and bar (not colour alone).
