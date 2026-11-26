# Nocturne — plan

1. **Tokens** — `frontend/src/assets/main.css`: Nocturne light/dark palettes, radii, chart ramp,
   canvas glow, `--on-accent`, `.num` utility; daisyUI theme remap.
2. **Primitives** — `AppPageHeader`, `AppStat`, `AppChart` new; `AppTable`, `AppButton`, `AppStamp`,
   `AppEmpty`, `AppSkeleton`, `AppFolio` restyled on the tokens.
3. **Shell** — `AppSidebar` rail (tint active row, mono balances), `AppLayout` content frame.
4. **Dashboard** — rebuild as net-worth pair (hero + budget ring), full-width spending, recent
   entries. Delete `AccountsPanel` (duplicate of the rail), `NetWorthBand`, `BudgetPanel`,
   `DashboardSpendingLine`.
5. **Pages** — every view adopts `AppPageHeader`; lists move to `AppTable`, charts to `AppChart`,
   action rows to `AppButton`.
6. **Docs** — `frontend/UI_STANDARDS.md` rewritten to describe Nocturne.
7. **Gates** — `npx vue-tsc -b`, `npm test`, `npm run build`.
