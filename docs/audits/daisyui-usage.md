# daisyUI usage audit

Date: 2026-11-18 · Scope: `frontend/src/**/*.vue` (69 files)

## Method

A script reads every `class="..."` and `:class="..."` binding, extracts exact class tokens, and
matches them against the daisyUI 5.7.16 component names plus their modifiers. Counts are direct
usages only; a page that renders `AppButton` does not count as a daisyUI usage even if the wrapper
uses one internally.

## Headline

- **69** Vue files scanned.
- **27** files use at least one daisyUI component class directly.
- **224** direct daisyUI class occurrences (up from 16 files / 196 before the first primitive pass).
- The new dashboard is fully daisyUI-based: `card`, `stat`, `progress`, `table`, `badge`, `alert`,
  `skeleton`, `btn`, `link` in all six dashboard files.
- The shared primitive pass has landed: `AppButton`, `AppFolio`, `AppTable`, `AppStamp`,
  `AppSkeleton`, `AppField`/`AppInput`, `AppSelect`, `AppTabs`, `StatCard` and `AccountCard` now
  start from daisyUI base classes; pages that render those wrappers inherit the change.
- The first page-level pass has landed: `ProfileView` (custom `AppSelect`, `AppButton`, daisyUI
  checkbox, dialog actions), `AccountsView`, `ConfirmDialog` and the `SpendingView` sort buttons.
  No `ns-primary` / `ns-quiet` / `ns-danger` classes remain.

## Files with the most direct daisyUI usage

| File                                             | Occurrences | Components                                  |
| ------------------------------------------------ | ----------: | ------------------------------------------- |
| `views/DashboardView.vue`                        |          22 | `alert`, `btn`, `card`, `link`, `skeleton`  |
| `components/dashboard/BudgetCard.vue`            |          16 | `badge`, `card`, `progress`, `stat`, `link` |
| `components/dashboard/NetWorthCard.vue`          |          14 | `card`, `stat`                              |
| `components/workspace/StatementImportModal.vue`  |          14 | `btn`, `checkbox`, `select`                 |
| `components/dashboard/RecentEntries.vue`         |           7 | `card`, `table`, `link`                     |
| `views/ProfileView.vue`                          |           7 | `btn`, `btn-circle`, `checkbox`             |
| `components/dashboard/DashboardSpendingLine.vue` |           5 | `card`, `link`                              |

Top classes: `btn` (27), `btn-sm` (19), `card` (11), `btn-ghost` (11), `btn-primary` (10),
`link` / `link-hover` (8 each), `select` (7), `badge` (5), `stat` (5), `skeleton` (5),
`progress` (1 — the budget bar).

## Gaps

## Migration progress

Landed in the first primitive pass:

- [x] `AppButton` → `btn`, with `btn-ghost` for quiet/icon variants, `shadow-none`, and `p-0` for
      icon-only buttons (daisyUI's default padding was shrinking the 14px icons).
- [x] `AppFolio` → `card` + `card-body`.
- [x] `AppTable` → `table table-sm`.
- [x] `AppStamp` → `badge badge-outline`.
- [x] `AppSkeleton` → `skeleton` (keeps the ledger's pulse timing).
- [x] `AppField` / `AppInput` → `input` / `textarea`.
- [x] `AppSelect` → `select select-bordered` (native wrapper and the custom trigger).
- [x] `AppTabs` → `tabs`.
- [x] `StatCard` / `AccountCard` → `card` / `stat`, icon action → `btn btn-ghost`.
- [x] Dashboard cards and tables (already daisyUI).
- [x] `ProfileView` — native selects replaced with the custom `ui/AppSelect`; `ns-*` buttons replaced
      with `AppButton`; checkboxes use `checkbox`; avatar action uses `btn btn-ghost btn-circle`.
- [x] `AccountsView` and `ConfirmDialog` — `ns-*` buttons replaced with `AppButton`; the destructive
      dialog action uses the new `dangerSolid` variant (solid debit fill).
- [x] `SpendingView` — the ledger sort controls and `AppPager` both start from `btn btn-ghost`.

Still deliberate custom surfaces / remaining work:

- [ ] `AppModal` — two custom focus-trapped dialogs (`components/AppModal.vue` and
      `components/ui/AppModal.vue`); keep the focus traps and teleport, then adopt `modal-box` if the
      daisyUI shell can be used without losing the behaviour.
- [ ] `AppMenu` — custom keyboard menu; can use `menu` list styling once the menu item markup is
      converted to `<ul><li>`.
- [x] `AppPager` — the one pager (Prev / “Page n of m” / Next) on `btn btn-ghost` with a `border-rule`
      outline; replaced the old `Pagination` bar, whose numbered strip and 1–10-of-47 counts are gone.
- [ ] `AppEmpty` — empty state composition, not a daisyUI component; it can stay a composition.
- [ ] `TransactionsPanel` shell and the account filter chips.
- [ ] Residual raw controls in the settings subcomponents; the main pages now render the
      migrated wrappers or direct daisyUI classes.

## Migration order

1. **Primitives first.** Done for the shared wrappers above; every page inherits the base classes.
2. **Cards and stats.** Done for `StatCard`, `AccountCard`; remaining panel shells next.
3. **Page markup.** Done for `ProfileView`, `AccountsView`, `ConfirmDialog` and the
   `SpendingView` sort controls. Next: `TransactionsPanel` shell, account filter chips, overlays,
   then the residual settings subcomponents.
4. Keep the ledger token overrides in every step: daisyUI supplies behaviour/structure, the
   `@theme` tokens supply the look.

## Charts

daisyUI 5.7.16 has **no chart component**. The project uses ApexCharts through
`AppApexChart` + `src/lib/chartPalette.ts`. The dashboard's new spending line is a smooth Apex
line chart with no area fill (`DashboardSpendingLine.vue`), loaded async. Do not add a second chart
library or hand-roll a chart that Apex already provides.

## Rule added

- `frontend/UI_STANDARDS.md` §8 — daisyUI is the component layer; start from its classes.
- `AGENTS.md` — the same rule is in the project workflow rules, so it survives a new session.
