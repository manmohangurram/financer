# Nocturne — todo

## Round 1 — the system

- [x] Tokens: Nocturne light + dark, radii, chart ramp, canvas, `formatAmount`
- [x] Primitive layer: `AppPageHeader`, `AppStat`, `AppChart`, `AppTable`, `AppButton`,
      `AppStamp`, `AppFolio`, `AppEmpty`
- [x] Shell: sidebar rail, layout frame, dark as the default theme
- [x] Dashboard rebuild — net-worth pair, budget ring + accounts folio, full-width spending,
      recent entries
- [x] Accounts, Manage accounts, Categories, Rules
- [x] Spending — first pass
- [x] Login / Signup — form on a card surface
- [x] UI kit page header
- [x] `UI_STANDARDS.md` Nocturne section
- [x] Repo cleanup — one screenshot folder (`frontend/design/shots/`, git-ignored)

## Round 2 — the money pages

- [x] `AppMoneyBand` — one figure, a row of facts, one bar, one note. Accounts has three states
      (nothing / account / credit); Spending has the month pace band
- [x] `AppDateRange` — the resolved dates are the control; calendar popup replaces the `Custom` tab
- [x] Keyboard: roving tabindex, arrows across days and weeks (and month edges), focus on open, Esc
- [x] Budget colour rule in one helper (`lib/utils/budget.ts`): `<70%` accent, `>=70%` `--stamp`,
      `>=90%` `--debit` — band, category bars, dashboard ring
- [x] Spending body: line chart + budget folio on a 70/30 split; categories as bubbles
- [x] Ledger = `TransactionTable` on Accounts and Spending; sort in the card head
      (`Amount ↓ Amount ↑ Refunds ↓ Refunds ↑`, one `AppMenu` control on a phone), search right-aligned
- [x] `AppPager` — one pager everywhere (Prev · Page n of m · Next), replacing the numbered strip and the
      36px caret pair; no row counts in it
- [x] Accounts head: `Manage · Import · Add`; dashboard `Open ledger` link removed;
      budget card reads `₹4,723 of ₹58,500 planned` on one line
- [x] No entry counts anywhere in the app
- [x] Email/IMAP feature deleted end to end — Go packages, `/api/mail/*`, MCP mail tools, OpenAPI,
      migration `000009_drop_mail`, Inbox view, mail libs, Settings tabs, docs
- [x] Settings: change email and password
- [x] Bundle: `manualChunks` (charts / pdf / vendor), `index.js` 1741 kB → 208 kB
- [x] Dead code: `columns` kind, `selectedIndex` and the recede-on-pick branch removed from
      `AppChart`; `accentQuiet` dropped from the palette
- [x] Date maths in `lib/utils/spendingRange.ts` with 10 tests (`resolveSpan`, `dayCount`,
      `spanGranularity`, `formatSpan`)
- [x] UI kit now shows the money band, stats, the date-range popup and a line chart
- [x] Design folder: approved mockups deleted; `probe-page.mjs` / `shoot-route.mjs` folded into
      `shoot-pages.mjs` (`SHOOT_CLICK`, `SHOOT_KEY`, `SHOOT_SCROLL`); `shoot-file.mjs` renders a local mockup
      in both themes at any width
- [x] `UI_STANDARDS.md` N.2/N.3/N.4 — components, the 70/90 rule, the pages as shipped

## Gates

- [x] `vue-tsc`, `vitest` (55 tests), `prettier --check`, `vite build` (warning-free)
- [x] `gofmt`, `go vet`, `go build`, `go test`, `govulncheck`

## Deferred

- [ ] Settings: split the section panels into `AppFolio` cards instead of hand-rolled
      `section` + padded `div`
