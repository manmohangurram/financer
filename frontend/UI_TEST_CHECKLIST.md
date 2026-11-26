# UI Test Checklist

Every run against the running backend before a release or after any backend/frontend change.

## Preconditions

- Backend: `go run ./cmd/financer` (Gin JSON server on :8080; migrations auto-applied on boot). No seed binary — seed demo data via the API or a script.
- Frontend: `npm run dev` (vite on :5173)
- Sign in at `http://localhost:5173` as `demo@financer.app` / `password123`
- Baseline demo data: **3 accounts, 18 transactions, 3+ rules, 8 categories**
  - Accounts: Main Checking (CURRENT) · Emergency Fund (SAVINGS) · Credit Card (CREDIT_CARD)
- Fresh DB before testing (delete the SQLite file under `FINANCER_DATA_DIR` — default `/data/financer.db` — restart backend, reseed via API)

## Authentication

- Login with valid credentials lands on Dashboard; sidebar shows Demo User / demo@financer.app
- Login with a wrong password shows a **single** inline error banner (colored, not a daisyUI alert) and the invalid fields get red borders; stays on /login
- Signup with a new email lands on /accounts showing 0 accounts and an empty state
- Signup with an existing email shows the same single-message error banner
- Hard-refresh an authenticated page and stay signed in (session reload via stored token)
- Logout returns to /login
- `/api/auth/refresh` returns a fresh access token + rotated refresh token

## App shell / navigation

- Sidebar shows **Dashboard**, **Accounts** and **Spending**
- User popup (bottom avatar) shows **Settings** and **Logout**
- **Sidebar collapse:** the Collapse/Expand button (above the user box) narrows the sidebar to an icon rail and back; state persists on reload; auto-collapses below 1024px (resize the window); active nav highlight is a rounded rectangle in both states
- **Theme toggle:** Settings → Appearance → Light/Dark switches instantly and persists on reload (no flash); both themes pass Lighthouse accessibility 100 on /accounts and /login
- `/` redirects to `/dashboard`
- Rules and Categories are **sub-pages under /accounts** (`/accounts/rules`, `/accounts/categories`) reached via the header buttons on the Accounts page
- Settings opens `/settings` → Profile page
- Mobile (narrow viewport): hamburger toggles the sidebar, overlay closes it

## Dashboard (`/dashboard`)

- Login lands on Dashboard; the page loads from one `GET /api/dashboard` request
- **Net worth pair:** `totalBalance` matches the sum of account balances; `Savings` = current + savings accounts, `Owed` = loans + credit cards; the composition bar and legend show both amounts and percentages; `In`, `Out` and `Net` match `month.income`, `month.expenses` and `month.net`; `Out` shows the previous-month delta
- **Budget card:** `Left to spend` equals `planned − spent`; the progress bar width equals `used`; the `N over` badge matches the number of over-limit categories; `Planned` and `Spent` align under the left-to-spend and progress-bar columns; over-budget shows the debit state
- **Accounts card:** one row per account, correct type/ending/balance; `Manage` goes to `/accounts/manage`
- **Spending:** total/average/day, largest category, previous-month note and the line chart match the server buckets; the average rule and highest-day callout are present; `Details →` opens Spending
- **Recent entries:** 6 rows, newest first, entry + date + account, amount right-aligned; no selection/margin column; the carried-forward total equals `recentTotal`
- Loading shows daisyUI skeletons, not a blank page; a failed request shows the error banner + Retry; no accounts shows the Add account empty state
- Light and dark themes pass; 1440 and 390 have no horizontal scroll; console clean

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
- **Statement import:** the Import button opens a three-step modal (Upload → Map columns → Review); the PDF is parsed in the browser and no file is uploaded
- **Column mapping:** the Map step lists the statement's printed columns with a role chip on each; changing a role re-reads the rows; “Debit + Credit” and “One amount” switch the amount model; **Reset to detected** restores the header mapping
- **Wrong mapping is rejected:** swap Debit/Credit (or unmap one amount column) → the error banner reports a balance break, the validation strip says the balances do not reconcile, and Continue is disabled
- Statements with no printed header show the layout fallback select instead of the column table
- `POST /api/transactions` with >1000 transactions is rejected with 400
- `PUT /api/transactions` with >1000 transactions is rejected with 400
- **Categories on a transaction:** create a transaction with a category; edit it and change the category — the row shows the new category (always-replace on update)

## Filters

- Filter bar sits between the **Transactions** heading and the **Add** button
- Typing a name and pressing Enter adds a name bubble inside the field and searches immediately
- Multiple name bubbles combine with OR (results match any name); Backspace on an empty input removes the last name bubble
- The filter button (icon + chevron) at the right of the bar opens the popup with From/To date, Min/Max amount, Category, and Type (Both/Credit/Debit)
- Applying the popup adds bubbles for each set filter; the popup's Clear resets only the popup fields (name bubbles stay)
- Each bubble is removable via its ✕; removing a filter re-applies
- Filtering by a category only returns transactions with that category; Type=Debit/Credit narrows by type
- The bar looks like a normal input and shows the placeholder "Search" when no filters are active
- **Account filter:** with many accounts the bar scrolls horizontally (invisible scrollbar); left/right arrows appear on hover and scroll ~55px; arrows hide at the scroll ends
- **Responsive:** at <1024px the sidebar auto-collapses, the Spending Tracker button and the Categories column hide (below `lg`), and the Accounts sub-heading hides (below `md`); no horizontal overflow at 1280/1920/2560 widths (16:9 and 16:10)

## Transfers

- Transfer modal: pick a debit and a credit transaction from different accounts → link succeeds
- Linked transactions show a shared link badge/indicator
- Unlink restores both transactions to standalone (linkedTransferId cleared)
- Create counterpart: link one side to a target account → the matching counterpart is created and linked
- Linking two transactions from the same account is rejected
- Linking an already-linked transaction is rejected

## Categories (`/accounts/categories`)

- Lists the seeded categories (9 baseline)
- Add Category appears, count increments
- Edit Category renames and updates the row
- Delete Category removes it, count decrements
- Deleting a category in use does not crash the transactions list (categories are display-only on the row)

## Rules (`/accounts/rules`)

- Lists the seeded rules with priority badge, logic (OR/AND), conditions, and outputs
- RuleForm dropdowns are populated with **string enum values** (`OR`/`AND`, `NAME`, `CONTAINS`, etc.) — regression: previously defaulted to empty/numeric
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

## Settings (`/settings`, user popup)

- User popup → Settings opens the tabbed settings page; tabs are Account, Appearance, Security, API keys
- Tabs are URL-addressable (`/settings?tab=keys`), arrow-key navigable, and keep their panel state while switching
- Account: profile shows current name/email and avatar initials; edit + save updates sidebar/popup; upload avatar works for PNG/JPG/WebP and rejects other files
- Security: wrong current password shows an inline error; a valid change succeeds, signs out other sessions, and the session stays valid; sign out all asks for confirmation

## Errors / console

- Backend down on the Transactions panel → the panel shows an error banner + **Retry** button (never the "Add your first transaction" empty state); Retry reloads the list once the backend is back
- Failed account/rule/category create/update/delete shows an inline `text-expense` error banner in the modal/view (AccountFormModal, Manage Accounts, Rules, Categories) instead of a silent console log
- Browser DevTools console shows no errors or uncaught exceptions across any flow
- No net::ERR_FAILED / CORS / access-control-allow-headers errors on any backend request
- All network requests return 2xx for happy paths; 4xx for intentional invalid input

## Performance (audit)

- Production bundle < 200KB gzip (~80KB total: index + AppLayout)
- Dashboard LCP < 2.5s and CLS < 0.1 (measured 297ms / 0.00)
- `/api/dashboard` is one request with server-side math; the transaction list is a single SQL query
- No N+1 in hot paths

## Theme sweep — contrast on every screen (light + dark)

Run this for **every** page, popup/modal, dropdown, and component state, in **both** Light and Dark themes. Toggle via Settings → Appearance (persists; no reload needed). Every text/color introduced on the screen must pass WCAG AA (≥ 4.5:1 body/labels, ≥ 3:1 large text/UI):

- **Every page:** Dashboard, Accounts, Transactions (embedded), Accounts Manage, Rules, Categories, Spending, Settings, Login, Signup
- **Every popup/modal:** Add/Edit Transaction, Add/Edit Account, Transfer modal, ConfirmDialog, Add/Edit Rule, Add/Edit Category, Filter popup, user popup
- **Every state on each screen:** normal, hover, focus, active, disabled, loading/spinner, empty state, and **error states** (inline error banners, red-bordered invalid inputs, toast/notice messages)
- **Text classes:** `text-text`, `text-text-secondary`, `text-subtle`, `text-faint`, `text-muted` on both themes (regression: light-theme error messages previously rendered near-black on a light banner → failed AA)
- **Colored text:** income/expense/p&l values (`text-income`/`text-expense`), category badges (all 16 `CATEGORY_PALETTE` colors), primary/error/success buttons and their text
- **Systematic method per screen:** toggle theme → screenshot or eyeball every visible text+background pair → repeat for dark. Check each new component the moment it mounts (modal opens, dropdown expands, error banner appears, empty state shows)
- **Known-good references:** Primary button `primary`/`primary-600` + white text on both themes; error banner = `text-expense` on `bg-expense/10` in both themes; `--color-primary-400` (#1d4ed8) is the AA-safe accent in light
- Lighthouse a11y 100 on both themes across the sweep pages

## Accessibility (WCAG 2.1 AA)

- Lighthouse accessibility score is 100 on the authenticated pages and /login (color contrast, landmarks, labels)
- Sidebar/user text uses AA-compliant tokens (`text-muted`, `subtle`, `faint` ≥ 4.5:1 on dark)
- Primary buttons use `primary`/`primary-600` with white text (≥ 4.5:1)
- Category badges use the AA-compliant `CATEGORY_PALETTE` (all 16 colors ≥ 4.5:1 on dark)
- Every page has a `<main>` landmark (Login/Signup are standalone `<main>`; app pages render in the shell's `<main>`)
- Forms use labeled controls (`AppInput`/`AppSelect`/`AppModal`); interactive elements are keyboard-reachable
- Modals (`AppModal`): ESC closes, focus moves into the dialog on open and returns to the trigger on close; the × button has a "Close" label
- Mobile: when the off-canvas sidebar is closed its links are not focusable (inert); the hamburger has `aria-expanded`; the overlay is `aria-hidden`
- File pickers (avatar upload) are real keyboard-openable buttons, not hidden-label tricks
- Credit amounts are prefixed with `+` (not color-only)
- Hover-revealed controls (edit icons, HScroll arrows, transfer buttons) stay visible on keyboard focus (`focus-visible:opacity-100`)

## Backend smoke (curl)

- Login returns accessToken + refreshToken
- Authed `GET /api/dashboard` returns `totalBalance`, `totalIncome`, `totalExpenses`, `accounts` in one request (no client math)
- Authed `GET /api/spending?range=1M` returns `buckets` and `categories`; linking a non-debt transfer removes both sides from the aggregates
- Dashboard income/expense come from the cached per-account `total_credit`/`total_debit` and stay consistent with the transaction list (create/edit/delete a transaction, then reload dashboard)
- Transfer counterpart creation moves the target account's balance and totals (regression: was skipped)
- Authed `GET /api/me/profile` returns userId/name/email/avatarUrl
- Authed `GET /api/accounts` returns accounts with balances
- Authed `GET /api/transactions` returns clean cent amounts (19.99, not 19.98999977…) with `categoryIds`, `linkedTransferId`, `totalCount`
- Authed `POST /api/transactions` returns **201**; `DELETE /api/transactions` returns **204** (no body)
- `POST /api/transfer-links` returns **201**; `DELETE /api/transfer-links` returns **204**
- `GET /api/rules` returns rules; `POST /api/rules` returns **201**; `DELETE /api/rules/{id}` returns **204**
- Rule overlay: creating a transaction that matches a rule stores the resolved display name in `clean_name` (the raw `name` is preserved) and `GET /api/transactions` returns the resolved name
- Wrong-password login → 401 `{code:"unauthenticated", message}` (single error shape)
- `GET /api/me/keys` returns masked keys; `POST /api/me/keys` returns **201** with the plaintext key once; `DELETE /api/me/keys/{id}` returns **204**
- `GET /openapi.json` returns the spec and `/docs` loads Swagger UI

## Data hygiene

- Demo data is back to baseline (3 accounts, 18 transactions, 5 rules, 9 categories) after testing
- Any account/transaction/rule/category created during testing is deleted before shipping
