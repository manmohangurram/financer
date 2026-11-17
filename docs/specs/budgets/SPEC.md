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

Budgets live on the Categories page, and the page stays deliberately small: a
category is a **name and a monthly budget**. Analysis belongs to Spending.

- Header: `Categories`, one-line purpose, `Add category`.
- Table, one row per category: `Category · Monthly budget` (read-only figure, or
  "No budget").
- A row opens the edit popup, which carries **Name**, **Monthly budget** and Delete.
  There are no inline inputs and no edit/delete icons.
- Budgets repeat every month, apply to debit spend only, and never roll over.
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
- `POST /api/categories` and `PUT /api/categories` accept `monthlyBudget`, so a
  category and its limit are one call (omitted on update keeps the stored value).
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
