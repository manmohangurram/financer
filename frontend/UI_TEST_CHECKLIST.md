# UI Test Checklist

Every run against the running backend before a release or after any backend/frontend change.

## Preconditions

- Backend: `go run .` (JSON server on :8080), then `go run ./cmd/seed` for a fresh demo user
- Frontend: `npm run dev` (vite on :5173)
- Sign in at `http://localhost:5173` as `demo@financer.app` / `password123`
- Baseline demo data: **3 accounts, 16 transactions, 3 rules, 8 categories**
  - Accounts: Main Checking ₹5,42,050 · Emergency Fund ₹12,30,000 · Credit Card ₹-85,025
  - Stat cards: Total Cash ₹17,72,050 · Credit Owed ₹85,025 · Net Worth ₹16,87,025
- Fresh DB before testing (delete `data/financer.db`, restart backend, reseed)

## Authentication

- Login with valid credentials lands on **/accounts**; sidebar shows Demo User / demo@financer.app
- Login with a wrong password shows an inline error and stays on /login
- Signup with a new email lands on /accounts showing 0 accounts and an empty state
- Signup with an existing email shows an error
- Hard-refresh an authenticated page and stay signed in (session reload via stored token)
- Logout returns to /login
- `/api/auth/refresh` returns a fresh access token + rotated refresh token

## App shell / navigation

- Sidebar shows only **Dashboard**, **Accounts** and **Investments** (no Spending/Transactions/Rules/Categories items)
- User popup (bottom avatar) shows **Settings** and **Logout**
- `/` redirects to `/dashboard`
- Rules and Categories are **sub-pages under /accounts** (`/accounts/rules`, `/accounts/categories`) reached via the header buttons on the Accounts page
- Settings opens `/settings` → Profile page
- Mobile (narrow viewport): hamburger toggles the sidebar, overlay closes it

## Dashboard (`/dashboard`)

- Login lands on Dashboard
- Stat cards show Net Worth (accounts + portfolio), Total Income, Total Expenses, and Account count matching the backend (`GET /api/dashboard` returns all values in one request; zero client-side math)
- Portfolio Snapshot lists accounts (compact cards) with correct balances
- Investments Portfolio lists investments sorted by P&L (or the empty state when none)
- "View all" links to /investments
- Net Worth = account balance sum + portfolio current value

## Spending (`/accounts/spending`)

- Reached via the "Spending Tracker" button on /accounts; "Back to accounts" returns
- Range buttons 7D/1M/6M/1Y/Custom switch the bar chart; custom from/to opens a date picker
- Account filter narrows to one account
- Bar chart buckets match the backend; values are transfer-excluded (non-debt transfers don't count; debt-account transfers do)
- Categories tab: donut pies + table of DEBITED / CREDITED / NET SPENT per category, values match a hand-checked SQL query
- Transactions tab: drill-down list of transactions in range with debit/credit sort + pagination

## Accounts (`/accounts`)

- Header buttons: Rules, Categories, Manage accounts — each navigates to the right sub-page
- Stat cards show Total Cash, Credit Owed, Net Worth matching the baseline
- Account filter bar lists All Accounts + each account with the correct balance (Credit Card shows negative)
- Add Account creates a card, stat cards update
- Edit Account opens a pre-filled modal; changing the nickname updates the card
- Edit Account → Delete → Confirm dialog → Delete removes the card, stat cards update

## Transactions (embedded on `/accounts`)

- List shows rows with date, name, category, and +/- amount; counter matches the account filter
- Add Transaction (debit and credit) appears in the list, counter increments, linked account balance moves
- Edit Transaction opens a pre-filled modal; amount shows a clean value (19.99, not 19.98999977…) and saves without retyping
- Delete Transaction removes the row, counter decrements, balance moves back
- Account filter buttons (All / each account) narrow the list
- Pagination — next/prev work when more rows than page size
- **CSV import:** Add → Import CSV → upload a file → column mapping auto-guesses (Date/Description/Debit/Credit); both "Single amount column" and "Debit + Credit columns" formats work; preview shows the row count; Import creates the transactions, the list refreshes, and the account balances on the page update without a reload
- Imports larger than 500 rows are split into 500-per-request batches; a >1000-row CSV still imports (chunked)
- **Idempotent retry:** importing the same CSV twice creates rows only the first time — the second reports `skipped` and balances do not move again (crash-resume safe)
- `POST /api/transactions` with >1000 transactions is rejected with 400
- `PUT /api/transactions` with >1000 transactions is rejected with 400
- Invalid CSV (no header, zero rows) shows an inline error
- **Categories on a transaction:** create a transaction with a category; edit it and change the category — the row shows the new category (always-replace on update)

## Filters

- Filters popover opens with From/To date, Min/Max amount, Category, Name, match mode
- Applying a filter narrows the transaction list; clearing restores it
- Filtering by a category only returns transactions with that category

## Transfers

- Transfer modal: pick a debit and a credit transaction from different accounts → link succeeds
- Linked transactions show a shared link badge/indicator
- Unlink restores both transactions to standalone (linkedTransferId cleared)
- Create counterpart: link one side to a target account → the matching counterpart is created and linked
- Linking two transactions from the same account is rejected
- Linking an already-linked transaction is rejected

## Categories (`/accounts/categories`)

- Lists the 8 seeded categories
- Add Category appears, count increments
- Edit Category renames and updates the row
- Delete Category removes it, count decrements
- Deleting a category in use does not crash the transactions list (categories are display-only on the row)

## Rules (`/accounts/rules`)

- Lists the 3 seeded rules with priority badge, logic (OR/AND), conditions, and outputs
- Empty state "No rules yet" shows **only** when there are no rules (regression: it previously showed alongside the list)
- Add Rule appears, count increments
- Edit Rule updates it
- Delete Rule removes it, count decrements
- **Run now** runs a rule and reports matched/linked/created

### Rule actions (set name / set category)
- Rule with "Set name" action → matching transactions display the canonical name (stored value untouched)
- Rule with "Set category" action → matching transactions show the chosen category
- Rule with both set-name + set-category applies both
- Two rules match the same transaction → lower-priority rule's set name/category wins (applied last)
- Delete a rule with an action → transactions revert to their stored name/category (non-destructive overlay)
- Rule with no output action → matches but leaves name/category unchanged
- Rule with "Transfer to account" action → on matching transaction create, the counterpart is auto-created and linked
- RuleForm "Set category" select is populated with the user's categories

### Rule → Transactions propagation
- Add a rule (set-name action) → the transactions list on /accounts **immediately shows the updated name** (same page, no reload)
- Add a rule (set-category action) → matching transactions show the new category
- Delete the rule → matching rows revert in the list

## Investments (`/investments`, `/investments/:id`)

- Empty state shows "Add your first investment" when no investments
- Add Investment (Stock requires a symbol; Mutual Fund uses Manual NAV) → appears in the table
- Invalid investment type is rejected client-side
- Edit Investment opens a pre-filled modal and saves
- Delete Investment removes it (with confirmation)
- **Lots:** add buy lots → quantity / avg cost update in the table
- **Sell lots:** add a sell lot → FIFO realized P&L appears; selling more than held is rejected
- Manual NAV mutual fund shows the NAV as its current price
- Portfolio summary cards (invested, current value, unrealized, realized P&L) match the table
- Price refresh button calls `/api/investments/refresh-prices` and updates prices (offline: error shown, cached prices kept)
- Investment detail page lists lots with buy/sell side; delete lot removes it and updates P&L
- Price history chart loads (range selector); no data shows an empty state

## Settings (`/settings`, user popup)

- User popup → Settings opens the profile page
- Profile shows current name/email and avatar initials
- Edit name/email → Save → sidebar + popup reflect the new name
- Upload avatar (PNG/JPG/WebP) → avatar image shows; invalid file type rejected
- Change password: wrong current password → inline error; valid → success, "signed out other sessions" notice, session stays valid
- Sign out all sessions → confirm dialog → success; other sessions' refresh tokens revoked

## Errors / console

- Browser DevTools console shows no errors or uncaught exceptions across any flow
- No net::ERR_FAILED / CORS / access-control-allow-headers errors on any backend request
- All network requests return 2xx for happy paths; 4xx for intentional invalid input

## Backend smoke (curl)

- Login returns accessToken + refreshToken
- Authed `GET /api/dashboard` returns `totalBalance`, `totalIncome`, `totalExpenses`, `portfolioValue`, `accounts`, `investments` in one request (no client math)
- Authed `GET /api/spending?range=1M` returns `buckets` and `categories`; linking a non-debt transfer removes both sides from the aggregates
- Dashboard income/expense come from the cached per-account `total_credit`/`total_debit` and stay consistent with the transaction list (create/edit/delete a transaction, then reload dashboard)
- Transfer counterpart creation moves the target account's balance and totals (regression: was skipped)
- Authed `GET /api/me/profile` returns userId/name/email/avatarUrl
- Authed `GET /api/accounts` returns accounts with balances
- Authed `GET /api/transactions` returns clean cent amounts (19.99, not 19.98999977…) with `categoryIds`, `linkedTransferId`, `totalCount`
- Authed `POST /api/transactions` returns **201**; `DELETE /api/transactions` returns **204** (no body)
- `POST /api/transfer-links` returns **201**; `DELETE /api/transfer-links` returns **204**
- `GET /api/rules` returns rules; `POST /api/rules` returns **201**; `DELETE /api/rules/{id}` returns **204**
- `POST /api/investments` returns **201**; `GET /api/portfolio/summary` returns the 4 totals
- Rule overlay: `GET /api/transactions` returns the overlaid name/categoryId for matched transactions (no DB write)
- Wrong-password login → 401 `{code:"unauthenticated", message}` (single error shape)

## Data hygiene

- Demo data is back to baseline (3 accounts, 16 transactions, 3 rules, 8 categories) after testing
- Any account/transaction/rule/category/investment/lot created during testing is deleted before shipping
