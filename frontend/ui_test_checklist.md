# UI Test Checklist

Every run against the running backend before a release or after any backend/frontend change:

- Backend: `go run .` (JSON server on :8080), `go run ./cmd/seed` for a fresh demo user
- Frontend: `npm run dev` (vite on :5173)
- Sign in at `http://localhost:5173` as `demo@financer.app` / `password123`
- Baseline demo data: 3 accounts, 18 transactions, 3 rules, 8 categories, net worth $16,777.76

## Authentication
- Login with valid credentials lands on Dashboard; sidebar shows Demo User / demo@financer.app
- Login with a wrong password shows an inline error and stays on /login
- Signup with a new email lands on a Dashboard showing $0.00 and 0 accounts
- Signup with an existing email shows an error
- Hard-refresh an authenticated page and stay signed in (session reload via stored token)
- Logout returns to /login
- RefreshToken endpoint returns a fresh access token + rotated refresh token

## Dashboard (/)
- Net Worth = $16,777.76, Total Income = $5,800.00, Total Expenses = $2,765.46, Accounts = 3
- Portfolio Snapshot lists the 3 accounts with correct balances
- Recent Activity shows transactions with name, amount, date

## Accounts (/accounts)
- Counter shows 3 accounts; Total Cash = $17,648.00, Credit Owed = $870.24, Net Worth = $16,777.76
- Each account shows the correct icon, bank name, nickname, balance (Credit Card = -$870.24)
- Add Account creates a row, counter increments, Net Worth updates
- Edit Account opens a pre-filled modal; changing the nickname updates the row
- Edit Account → Delete → Confirmation dialog → Delete removes the row, counter decrements
- Creating a transaction updates the linked account balance

## Workspace Transactions (Transactions tab)
- Tab shows "Transactions (18)"; rows show date, type, category, +/- amount
- Add Transaction (debit and credit) appears in the list, counter increments, account balance moves
- Edit Transaction opens a pre-filled modal; amount shows a valid value like 19.99 (not float noise like 19.98999977…) and saves without retyping it
- Delete Transaction removes the row, counter decrements, account balance moves back
- Account filter buttons (All / each account) narrow the list
- Pagination — next/prev pages work when more rows than page size
- CSV Import loads transactions into the list and updates balances

## Filters
- Filters popover opens with From/To date, Min/Max amount, Category, Name, match mode
- Applying a filter narrows the transaction list

## Rules / Aliases
- Tab shows "Rules (3)", each with priority + "N condition · OR/AND"
- Add Rule appears, counter increments, transactions re-categorize
- Edit Rule updates it
- Delete Rule removes it, counter decrements

### Rule actions (set name / set category)
- Add a rule with an action "Set name" → matching transactions display the canonical name (transaction row name changes, original stored value untouched)
- Add a rule with an action "Set category" → matching transactions show the chosen category
- Rule with both set-name + set-category applies both to a matching transaction
- Two rules match the same transaction → lower-priority rule's set name/category wins (applied last in priority order)
- Delete a rule with an action → transactions revert to their stored name/category (no persisted alias rows)
- Rule with no action set → matches but leaves name/category unchanged
- RuleForm shows a "Set category" select populated with the user's categories (8)

### Rule → Transactions propagation
- Add a rule (with a set-name action) → the Workspace **Transactions tab immediately shows the updated name** on every matching row (same page, no reload; read-time overlay)
- Add a rule (with a set-category action) → matching transactions show the new category
- Switch to Transactions tab **before** adding the rule, add the rule, then return → matching rows still show the overlaid name/category
- Delete the rule → matching rows revert to the stored name/category in the list

## Categories
- Tab shows "Categories (8)"
- Add Category appears, counter increments
- Edit Category renames and updates the row
- Delete Category removes it, counter decrements

## Errors / Console
- Browser DevTools console shows no errors or uncaught exceptions across any flow
- No net::ERR_FAILED / CORS / access-control-allow-headers errors on any backend request

## Backend smoke (curl)
- Login returns accessToken + refreshToken
- Authed ListTransactions (Bearer token) returns amounts as clean cents (19.99, not 19.98999977…)
- Authed ListAliases returns rules with `action` (`setName`/`setCategoryId`) when set
- Authed ListTransactions reflects rule actions: a transaction matched by a rule with an action comes back with the overlaid name/categoryId (read-time overlay, no DB write)

## Data hygiene
- Demo data is back to baseline (3 accounts, 18 transactions, 3 rules, 8 categories) after testing
- Any account/transaction/rule/category created during testing is deleted before shipping