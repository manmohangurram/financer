# Nocturne — SPEC

Status: approved by the owner (option 1 of four dashboard directions) · 2026-11-26

## Problem

The app ships "the Ledger": paper surfaces, hairline rules, 9–13px type, no depth. The dashboard
repeats the account list the rail already shows, and the type is too small to read at a glance.
Every page re-assembles its own card, table and chart markup, so one visual decision has to be made
in a dozen places.

## Objective

One design system, applied to every page:

1. **Nocturne surfaces** — a dark canvas, one elevated card layer, 14px card radius, hairline borders,
   a single sky accent for interaction, mint/rose for money direction, amber for warnings.
2. **Readable scale** — 14px body, 10.5px tracked labels, display serif only for page titles and the
   one figure that matters, mono + tabular for every number.
3. **Reusable primitives** — `AppPageHeader`, `AppCard`, `AppTable`, `AppChart`, `AppButton`, `AppStat`,
   `AppStamp`. A page composes these; it does not hand-roll a card, table or chart.
4. **No duplicated data** — the dashboard drops the Accounts card; balances live in the rail.
5. **Both themes** — Nocturne Light is the same system on paper; dark is the default.

## Non-goals

- No new runtime dependency. ApexCharts stays the chart engine; lucide stays the icon set;
  Newsreader + IBM Plex Sans stay the type.
- No route, API, data-model or MCP changes.
- No re-layout of the ledger/Inbox work flows beyond their visual layer.

## Success criteria

- Every page renders on the primitive layer; no page declares its own card radius, table head
  or chart palette.
- Dashboard matches the approved direction: net worth + budget pair, full-width spending, recent
  entries; accounts appear only in the rail.
- Light and dark pass 4.5:1 for body text; charts are legible in both.
- `vue-tsc`, `npm test`, `npm run build`, `go vet`, `go build`, `go test` pass.

## Decisions

| Decision | Value |
|---|---|
| Default theme | dark (`financer-theme` unset → dark) |
| Accent | sky `#7dd3fc` dark / `#0369a1` light |
| Money | mint `#4ade80` / `#047857` credit, rose `#fb7185` / `#be123c` debit |
| Warning | amber `#fbbf24` / `#b45309` |
| Radii | control 8 · card 14 · overlay 18 |
| Depth | no card shadow in dark; one soft shadow in light |
| Numbers | system mono, tabular, `−` for negatives |
