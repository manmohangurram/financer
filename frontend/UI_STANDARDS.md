# UI Standards — Nocturne

> **This is the current standard.** The Ledger section below describes the previous system and is
> kept only as history; where the two disagree, Nocturne wins. Design contract and rationale:
> `docs/specs/nocturne-redesign/SPEC.md`.

## N.1 The system in one page

**Surfaces.** A cool canvas (`--paper`) with one elevated card layer (`--page`), 14px cards, 1px
hairlines (`--rule`), and a single soft glow at the top right of the page. No card shadows in dark;
light uses `--lift`. Radii: control 8 · card 14 · overlay 18.

**Colour.** One accent for interaction (sky), mint/rose only for money direction, amber only for
warnings. Category colour still does not exist.

| Token         | Dark (default) | Light     |
| ------------- | -------------- | --------- |
| `--paper`     | `#080b10`      | `#f4f6fa` |
| `--page`      | `#111823`      | `#ffffff` |
| `--page-sunk` | `#0d141d`      | `#eef1f7` |
| `--rail`      | `#0b1018`      | `#ffffff` |
| `--ink`       | `#e9eef6`      | `#0f1620` |
| `--ink-soft`  | `#a9b6c7`      | `#46536b` |
| `--ink-mute`  | `#8b99ab`      | `#6b7891` |
| `--accent`    | `#7dd3fc`      | `#0369a1` |
| `--on-accent` | `#08111a`      | `#ffffff` |
| `--credit`    | `#4ade80`      | `#047857` |
| `--debit`     | `#fb7185`      | `#be123c` |
| `--stamp`     | `#fbbf24`      | `#b45309` |

**Type.** Newsreader for page titles and the one figure that matters; IBM Plex Sans for UI;
`ui-monospace` for **every number** via the `.num` utility (mono + tabular). Labels are 10.5px,
uppercase, `0.1em` tracking, `--ink-mute`.

**Money format.** `formatAmount` — Indian grouping, paise only when the amount has them
(`₹4,82,310`, `₹34.10`). `formatCurrency` (always two decimals) stays for statement/PDF output.

## N.2 Components

Every page composes these; a page never re-declares a card, table, chart or page head.

| Component       | Use                                                                                             |
| --------------- | ----------------------------------------------------------------------------------------------- |
| `AppPageHeader` | eyebrow + title + actions. `SectionHeader` is a thin alias of it.                               |
| `AppFolio`      | a card: hairline head, body, optional footer.                                                   |
| `AppMoneyBand`  | the opening band of Accounts and Spending: one figure, a row of facts, one bar, one note.       |
| `AppDateRange`  | the period control: the dates on screen are the button; the popup is a two-click calendar.      |
| `AppTable`      | every list of money: tracked head, margin column, empty slot, footer slot.                      |
| `AppPager`      | the one pager: `Prev` · `Page n of m` · `Next`. No row counts, no numbered strip.               |
| `AppChart`      | every chart: `area` \| `line`, Nocturne grid, axis in mono, accent series, dashed average rule. |
| `AppButton`     | solid / outline / quiet / danger / icon. Solid is the one primary action per view.              |
| `AppStat`       | label + mono figure + note, for stat bands.                                                     |
| `AppStamp`      | the only pill: mono caps, tinted by meaning.                                                    |
| `AppEmpty`      | the empty state of every async surface.                                                         |

## N.3 Rules

1. No number appears twice on a page without a reason. The rail already lists account balances, so
   the dashboard does not repeat them.
2. Numbers are `.num`; prose stays in the sans face; only page titles and the hero figure use the
   serif.
3. One primary button per view.
4. Every async surface ships loading, empty, error and content states.
5. Charts are never the only carrier of a number: the card states the figure in text too.
6. **Nobody counts rows.** No surface shows "N entries" / "N transactions": the figure is money.
7. **Budget bars change colour at 70% and 90%** — `--accent` → `--stamp` → `--debit`, in the band, in a
   category bar and in the dashboard ring alike. One helper (`lib/utils/budget.ts`) decides.

## N.4 Pages (as shipped)

**Money band** — the shape both money pages open with. Left: a tracked label, the one big figure, and an
optional line under it. Right: two to four facts, hairline-separated, each a label over a `.num` value.
Below: a 2px bar with an optional "where the calendar is" tick, and one note line.

- _Accounts_ — three states: nothing selected (Net worth · Cash · Owed · Accounts), one account selected
  (its balance, its type and bank as a wordy fact, its share of net worth), and credit/loan selected
  (`Outstanding`, debit-toned, with the limit and available credit). The bar is a composition bar:
  cash vs owed, `restPercent` carrying the remainder.
- _Spending_ — `Left to spend · <month>`, the month's pace bar, and the note
  `92% of the plan · 90% through November · 3 days left`. Facts: Spent · Planned · Average / day · Peak day.
  The band always speaks for the **current calendar month**; the period control below drives the chart, the
  categories and the ledger only, and says so in a muted line.

**Period control (Spending)** — `7D 1M 6M 1Y` as quiet tabs, then the resolved dates as an `AppDateRange`
button (`20–26 Nov 2026`). There is no `Custom` tab: clicking the dates opens the calendar, first click sets
the start, second the end, `Apply` commits. Arrow keys move by day/week, Esc closes. Presets and the popup
are mutually exclusive: picking one clears the other.

**Spending body** — 70/30 split on `lg`: left the line chart (`AppChart kind="line"`, dashed average rule,
click a point to open that day's ledger below), right the budget folio: one row per category, `spent` over
`planned`, a 70/90 bar, no "left" figure. Under 1024px the two stack. Categories are **bubbles** (pill
stamps with the category name and its share), not a legend list.

**Ledger (Spending and Accounts)** — the same `TransactionTable` on both pages: margin glyph, date, name +
account, category, amount. Sort lives in the card head as four quiet buttons — `Amount ↓ Amount ↑
Refunds ↓ Refunds ↑` — never on a row of its own; on a phone the four fold into one `AppMenu` button, because
four labels would eat the card title. Search is right-aligned in the same head. The footer is one `AppPager`
(`‹ Prev` · `Page 2 of 5` · `Next ›`) in every ledger, at every width: words instead of carets, targets at the
card's edges, and no `1–10 of 47` counts.

**Accounts** — the band, then the transaction panel (`All accounts` + one chip per account with its
balance, search right, `Manage · Import · Add` in the page head). The account cards are gone.

**Dashboard** — net-worth pair, budget ring + accounts folio, full-width spending line, recent entries.
The budget line reads `₹4,723 of ₹58,500 planned` on one line; there is no `Open ledger →` link.

---

# UI Standards — the Ledger (superseded)

Rewritten from scratch. This replaces the old daisyUI-default standards: nothing in here is inherited,
and no component from the old system is kept on the grounds that it already exists.

Read this as one document. Tokens, components and pages are three views of the same decision.

---

## 1. Intent

**Who.** One person, at a laptop or a phone, usually on a Sunday evening or on the 1st of the month, with a
bank app open in another tab. They are not exploring — they are **closing the books**: check what came in,
what went out, whether a statement was booked, whether the numbers agree.

**What they must do.** Reconcile. Read a number, trust it, and move on.

**How it should feel.** Like writing in a well-made ledger: quiet paper, one ink, ruled lines that keep
columns straight, and a tick in the margin when something is settled. Not a trading terminal (no neon, no
density for density's sake), not a consumer finance app (no confetti, no mascots, no gradient cards).

The test for every choice: _would a bookkeeper recognise this as their desk, or as a startup's dashboard?_

---

## 2. Where the design comes from

**Domain concepts** (the territory, not the features): the ruled ledger page · the ruled margin column ·
double-entry columns · the bank statement in its perforated envelope · the passbook and its page bands ·
reconciliation ticks in the margin · the till's compartments · index tabs and box files · iron-gall ink and
red correction ink.

**Colour world** (what is physically in that world): cream ledger paper · pale steel-blue rule lines ·
blue-black iron-gall ink · red correction ink · green bookbinding cloth · brass of the till and the clip ·
manila folders.

**Signature.** _The margin._ Every ruled surface in the app has a narrow left margin column separated by a
vertical rule, and state lives there: the entry's type glyph, the reconciliation tick for a matched
transfer, the priority stamp on a rule. It is the one structural idea
carried from the logo into every table, card and list — and it is where colour is allowed to mean something.

**Defaults rejected**

1. _Slate + blue sidebar-with-KPI-boxes dashboard_ → the **folio page**: a ruled content column with a
   margin, and one "balance carried forward" band instead of four equal metric boxes.
2. _Category pie chart + coloured chips + round avatars_ → **allocation strips** and **horizontal ruled
   bars**; categories carry a hairline and a monogram tab, not a hue.
3. _Purple/indigo gradients, glassmorphism, drop shadows everywhere_ → **paper, hairlines, one petrol
   accent**, shadows only on things that float.

---

## 3. Brand

### The mark

`frontend/src/assets/logo-mark.svg` — a folio page: ruled entries to the right, a filled margin band carrying
the clerk's tick.

- Ink `#12232b`, margin band filled ink, tick in paper.
- Clear space: the width of the margin band on all sides. Never place it on a colour; it is ink on paper or
  paper on ink.
- Minimum size 24px for the detailed mark; below that use the favicon variant.
- `logo-app.svg` — the same mark on an ink field with a petrol tick, for app icon / avatar use.
- `logo-favicon.svg` — 16px: two shapes only (margin rule + tick), no page outline, no entries.

### The wordmark

`financer` set lowercase in **Newsreader 500**, tracking `-0.02em`, ink. The descender of the _f_ is not
decorated — the mark carries the metaphor, the wordmark stays a word.

Tagline, used once (login screen and README only): `STATEMENTS, BOOKED` in labels style, petrol.

### Voice

Sentence case everywhere. Verbs are bookkeeping verbs: _approve, book, reconcile, carried forward, unmatched,
overdue_. No exclamation marks, no emoji, no "Oops!". Empty states state the fact and the next action in one
line: `No accounts yet. Add one to start booking statements.`

---

## 4. Foundations

### 4.1 Colour

Two themes, one palette. **Light is the default** — the paper world is the product; dark is the same ledger
at night, not a different design. Everything is `oklch` so lightness steps are perceptually even.

```css
:root {
  /* paper & ink — light, the default */
  --paper: oklch(0.965 0.008 85); /* desk / canvas */
  --page: oklch(0.995 0.003 85); /* the page you write on */
  --page-sunk: oklch(0.945 0.01 85); /* inputs, wells */
  --ink: oklch(0.255 0.02 245); /* primary text */
  --ink-soft: oklch(0.435 0.018 245); /* secondary */
  --ink-mute: oklch(0.585 0.014 245); /* tertiary, metadata */
  --ink-faint: oklch(0.7 0.01 245); /* placeholder, disabled */

  /* rules — the structure */
  --rule: oklch(0.255 0.02 245 / 0.12);
  --rule-soft: oklch(0.255 0.02 245 / 0.07);
  --rule-strong: oklch(0.255 0.02 245 / 0.22);
  --margin-rule: oklch(0.7 0.045 225); /* the steel margin line */

  /* meaning */
  --accent: oklch(0.455 0.075 205); /* petrol: actions, links, focus */
  --accent-hover: oklch(0.395 0.075 205);
  --accent-wash: oklch(0.94 0.02 205);
  --credit: oklch(0.505 0.1 155); /* income */
  --debit: oklch(0.52 0.15 28); /* expense, destructive */
  --stamp: oklch(0.66 0.105 78); /* brass: carried forward, pending */
  --overlay: oklch(0.255 0.02 245 / 0.32);
}

:root[data-theme='dark'] {
  --paper: oklch(0.215 0.012 245);
  --page: oklch(0.255 0.014 245);
  --page-sunk: oklch(0.19 0.012 245);
  --ink: oklch(0.935 0.01 85); /* paper-coloured text */
  --ink-soft: oklch(0.79 0.01 85);
  --ink-mute: oklch(0.64 0.012 85);
  --ink-faint: oklch(0.52 0.012 85);

  --rule: oklch(1 0 0 / 0.1);
  --rule-soft: oklch(1 0 0 / 0.06);
  --rule-strong: oklch(1 0 0 / 0.18);
  --margin-rule: oklch(0.48 0.035 225);

  --accent: oklch(0.72 0.08 205);
  --accent-hover: oklch(0.79 0.08 205);
  --accent-wash: oklch(0.3 0.03 205);
  --credit: oklch(0.72 0.1 155);
  --debit: oklch(0.68 0.14 28);
  --stamp: oklch(0.76 0.09 78);
  --overlay: oklch(0.1 0.01 245 / 0.6);
}
```

**Rules of use**

- One accent (petrol). It marks _interaction_: links, primary buttons, focus, active nav. Not decoration.
- `--credit` / `--debit` mark _direction of money_ only. Never use them for actions.
- `--stamp` is brass, used for one thing: the "carried forward" figure and `PENDING` state. Nothing else.
- Category colour: **none**. Categories are identified by name, monogram and position, never by hue — the
  colourblind and the twelve-colour-pie-chart case are the same case.
- Data-viz palette, when a chart genuinely needs more than one series: `--accent`, `--accent` at 60% opacity,
  `--ink-mute`, and `--stamp` — same-hue steps plus one warm, all chart series drawn at 8% opacity fills with
  a 1.5px solid top stroke.

### 4.2 Type

Two families, both self-hosted (no CDN, the app is offline-capable):

| Role                                                     | Family                       | Weights         | Why                                                                                                        |
| -------------------------------------------------------- | ---------------------------- | --------------- | ---------------------------------------------------------------------------------------------------------- |
| Display, page titles, the carried-forward figure         | **Newsreader** (variable)    | 400 / 500       | A text serif with a statement's calm; makes the one number that matters look printed rather than computed  |
| Everything else                                          | **IBM Plex Sans** (variable) | 400 / 500 / 600 | Engineered, slightly humanist, excellent at 12–14px, real tabular figures — the invoice/ledger temperament |
| Machine strings (IDs, hashes, key prefixes, poll stamps) | `ui-monospace` (system)      | 400             | Never a download for something that appears four times in the product                                      |

```css
:root {
  --font-display: 'Newsreader', Georgia, serif;
  --font-ui: 'IBM Plex Sans', system-ui, sans-serif;
  --font-mono: ui-monospace, 'SF Mono', Menlo, monospace;

  /* Major Third (1.25) from a 14px base — an interface scale, not an editorial one */
  --text-xs: 0.75rem; /* 12 — labels, stamps */
  --text-sm: 0.8125rem; /* 13 — table cells, captions */
  --text-base: 0.875rem; /* 14 — body */
  --text-md: 1rem; /* 16 — emphasised body, form values */
  --text-lg: 1.125rem; /* 18 — section titles */
  --text-xl: 1.375rem; /* 22 — page titles */
  --text-2xl: 1.75rem; /* 28 — carried-forward figure */
  --text-3xl: 2.25rem; /* 36 — dashboard hero figure */

  --tracking-tight: -0.015em;
  --tracking-label: 0.06em;
}
```

**Levels** (size, weight, family, colour — never size alone):

| Level           | Spec                                                                                         |
| --------------- | -------------------------------------------------------------------------------------------- |
| Page title      | `--text-xl` / Newsreader 500 / `--tracking-tight` / `--ink`                                  |
| Section title   | `--text-lg` / Plex 600 / `--ink`                                                             |
| Body            | `--text-base` / Plex 400 / `--ink` / leading 1.55                                            |
| Secondary body  | `--text-base` / Plex 400 / `--ink-soft`                                                      |
| Label           | `--text-xs` / Plex 500 / uppercase / `--tracking-label` / `--ink-mute`                       |
| Table cell      | `--text-sm` / Plex 400 / `--ink`                                                             |
| Money           | Plex 500 + `font-variant-numeric: tabular-nums`; negatives prefixed `−` (U+2212), never `()` |
| Carried forward | `--text-2xl` / Newsreader 500 / `--ink`                                                      |
| Machine string  | `--text-xs` / mono / `--ink-mute`                                                            |

**Rules.** Tabular figures everywhere a column of numbers exists. Line length for prose ≤ 72ch. Never centre
a number in a table. Numbers right-aligned, one currency format per page.

### 4.3 Space & layout

Base unit **4px**, and every value is on the scale: `4 8 12 16 20 24 32 40 48 64 80`.

| Context               | Value                       |
| --------------------- | --------------------------- |
| Icon ↔ text           | 6                           |
| Inside a control      | 10 / 14                     |
| Between form fields   | 16                          |
| Card padding          | 20 (compact) / 24 (default) |
| Between cards         | 16                          |
| Between page sections | 40                          |
| Page padding          | 32 desktop / 16 mobile      |

**Shell.** Sidebar 244px (fixed, `--paper`, hairline right border, no card, no contrast jump) + content that
is a page: **fluid width** (no max-width cap), 24px padding on tablet and 32px on desktop (the page does not
float centred — it sits on the desk like a sheet of paper). Prose blocks cap their own measure at 72–76ch;
queue and ledger pages use the full width.

**The margin column.** Every ruled surface reserves a margin: 56px on desktop, 40px on mobile, closed by
`--margin-rule`. Contents, left to right: state glyph · entry · right-aligned numbers. On mobile the margin
stays (it is the signature), the glyph stacks above the title instead of beside it.

**Breakpoints** `640 / 768 / 1024 / 1280`. Below 768 the sidebar becomes a bottom bar of 5 destinations
(Dashboard, Accounts, Spending, More) and the rest moves under "More".

### 4.4 Depth

**Committed strategy: surface shift + hairline rules.** No card shadows. Shadows exist only where something
actually floats (menu, modal, toast), where they are a single soft layer plus a hairline ring:

```css
--lift: 0 0 0 0.5px var(--rule), 0 6px 20px oklch(0.255 0.02 245 / 0.1);
```

Surfaces: `--paper` (canvas) → `--page` (cards, tables, sheets) → `--page-sunk` (inputs, wells, table head).
A page card is `--page` on `--paper` with a 1px `--rule` border and 6px radius. Radius scale: **4** (controls)
· **6** (cards, sheets) · **10** (modals) · **999** (only stamps and the account monogram tab).

Never: pure white on paper, coloured card backgrounds, borders darker than `--rule-strong`, radius > 10 on
anything but pills.

### 4.5 Motion

- Micro (hover, tick, toggle): **120ms**, `cubic-bezier(0.2, 0, 0, 1)`.
- Enter/leave (menu, modal): **200ms** fade + 4px rise; modal scrim 160ms.
- The **tick** — the one signature animation: confirming a suggested entry draws the check with
  `stroke-dashoffset` over 180ms, then the row collapses over 200ms.
- Nothing loops. No skeleton shimmer sweep — skeletons are static `--page-sunk` bars with the text rhythm.
- `prefers-reduced-motion: reduce` removes every transform and every non-essential transition.

---

## 5. Components

Every component below is written as its states, because a component without its states is a mock.

### 5.1 Button — `.btn` (rewritten)

| Variant | Use                                     | Spec                                                   |
| ------- | --------------------------------------- | ------------------------------------------------------ |
| Solid   | the one primary action per view         | `--accent` fill, paper text, 34px height, 4px radius   |
| Outline | secondary                               | transparent, `--rule-strong` border, ink text          |
| Quiet   | tertiary, table row actions             | no border, ink-soft text, `--page-sunk` on hover       |
| Danger  | destructive, only in confirm flows      | `--debit` text, `--debit` 20% border, transparent fill |
| Icon    | row actions, 32×32, always `aria-label` | quiet styling, 18px glyph                              |

States: hover (darken 8% / wash), active (inset 1px sense via `--page-sunk`), focus-visible (2px
`--accent` ring, 2px offset), disabled (`--ink-faint`, no hover), loading (spinner replaces label, width
held). Minimum hit area 44×44 on touch, padding may exceed the visual box.

### 5.2 Field — input, select, date, textarea

The input reads as a **ruled slot on the page**, not a floating box: 38px height, `--page-sunk` fill,
1px `--rule` border, no radius above 4px, label above in labels style, hint below in `--ink-mute`.
Focus: border `--accent` + 2px `--accent` 18% halo — never a glow.

States: filled · focus · disabled · invalid (`--debit` border + one-line reason below, never a tooltip-only
error) · read-only (no fill, hairline only).

Selects and dates are custom popovers on `--page` with `--lift`; never a native `<select>` dropdown.

### 5.3 Ledger table — the core surface

```
| margin | entry                                     | category | account | amount |
|   ⌐    | Sainsbury's  · 12 Nov                     | Food     | Barcl…42|  −34.10|
```

- Header row: labels style, `--page-sunk`, hairline bottom `--rule`.
- Body rows 44px (dense: 36px), hairline `--rule-soft` between rows, **no zebra striping**.
- Hover: row background `--page-sunk` at 50%. Selected: 2px `--accent` left edge in the margin.
- Margin cell: 56px wide, `--page-sunk` at 45%, closed by `--margin-rule`, glyph centred — `✓` petrol when a transfer is matched, `◌` stamp when
  pending, `↕` ink-mute when linked to a transfer. Empty when nothing to say.
- Money right-aligned, tabular, `--credit`/`--debit`. Amounts are the only coloured text in a normal row.
- Footer: the "carried forward" line — `--page-sunk`, Newsreader, with the page total: this is not decoration,
  it is how the user checks the table against their own arithmetic.
- Loading: 3 static skeleton rows. Empty: the ruled page — a hairline box with one line of text and one
  action. Error: hairline `--debit` banner above the table with a retry button, table keeps its last data.

### 5.4 Folio card — `.folio`

`--page` surface, 1px `--rule`, 6px radius, no shadow. One card-header pattern across every page:
h2 title at 13.5px/600 + 10.5px `--ink-mute` subtitle on the left; optional badge, meta or one action
on the right; `px-4 pt-3 pb-2.5 md:px-5`, closed by a hairline. Body padding is
`px-4 py-3 md:px-5`; a bottom strip is `border-t border-rule px-4 py-2.5 md:px-5`. No hover lift — a
card is a page, not a button. Metric cards (net worth, composition, budget) centre their figures,
bars and fact strips; list and table cards keep data rows left/right.

### 5.5 Stamp — `.stamp`

The only pill in the product: 11px mono uppercase, 1px hairline border, 2px 8px padding, radius 999.
Tones: neutral (`--ink-mute`), pending (`--stamp`), credited (`--credit`), debited (`--debit`), linked
(`--accent`). Used for: `PENDING`, `AUTO`, `REVIEW`, `PAID`, account ending digits (`···42`), API key prefix.

### 5.6 Margin tick — `.tick`

18px glyph button, hit area 44px, sits in the table margin.
States: idle `◌` (`--ink-faint`) · hover `◌` `--accent` · done `✓` (`--accent`, drawn 180ms) · refused `✕`
(`--debit`, 120ms). Match transfer in Spending. This is the signature interaction.

### 5.7 Tabs, menus, modals

- **Tabs**: underline rule only — label + 2px `--accent` underline on the active tab, hairline `--rule`
  beneath the whole row. No filled pills, no segmented control chrome. `AppTabs` is the only switch
  control: view switches (Manual / Import file, theme) are tabs; a two-way choice inside a form
  (Debit / Credit) is a pair of ruled slots from §5.2, selected state = `--accent-wash` + `--accent/50`
  border.
- **Menu / popover**: `--page`, `--lift`, 4px radius, 4px padding, rows 32px, keyboard navigable.
- **Modal**: `--page`, radius 10, `--lift`, scrim `--overlay`; max-width 520 (form) / 760 (preview);
  header with title + close, body scroll, footer right-aligned actions with the primary on the right.
  Esc and scrim-click close; focus is trapped and returned.
- **Toast**: bottom-right, `--page`, hairline, 4px radius, 3s, one action max.

### 5.8 Charts

ApexCharts is the engine, loaded async on the pages that need it (~300 kB stays out of the entry
bundle). It is only ever given the tokens: `src/lib/chartPalette.ts` reads `--accent`, `--ink-mute`,
`--rule-soft` and `--chart-0..7` off the document and re-reads them when `data-theme` flips, so a chart
never holds a colour of its own. daisyUI has no chart component; Apex is the project's chart layer.

Three primitives, all paper-like:

1. **Day columns** — vertical, discrete bars for spend per day, `--accent` fill, 2px top radius, no
   gradient, no shadow, hairline gridlines on the y axis only, y labels `₹` at ten pixels in
   `--ink-mute`, x labels rotated 45° in labels style. Clicking a column narrows the ledger to that day.
   When a day is picked the others drop to `accentQuiet` (accent at 28%) rather than the picked one
   brightening — nothing in the default state is ever drawn faint.
   _Why columns, not a line:_ spend per day is a count over discrete buckets; a line implies
   interpolation between days.
2. **Allocation strip** — one 30px full-bleed horizontal bar, segments proportional to spend, 2px paper
   gaps, colours from the ink ramp `--chart-0..7` (darkest = biggest share, grey tail for "Others").
   Hovering a segment shows `name · amount · share`. The legend below lists the same order with amounts
   only — never percentages, never a hue per category.
3. **Month line (dashboard only)** — the one line chart in the product, chosen because the dashboard
   asks "what shape did the month have?" rather than "which day was biggest". Apex `line`, smooth
   curve, 2px `--accent` stroke, 3px paper-filled markers, y grid only, a dashed average rule and
   one callout on the highest day. No area fill: the line is the data, and the page already has enough
   paper. The card carries total, average/day and largest category as text, so the chart is never the
   only carrier.

No pie, no donut, no exploded slices, no 3D, no more than eight segments (top 6 + Others).

### 5.9 States, always

Every async surface ships four: **loading** (static skeleton rows) · **empty** (ruled page, one line, one
action) · **error** (hairline `--debit` banner + retry, data preserved) · **content**. A blank screen is a
bug.

---

## 6. Pages

Each page states its focal point first — the one thing the person came for.

### 6.1 Login — `/login`

Focal: the form. Split screen: left 42% a `--paper` field with the mark at 40px, the wordmark, and the
tagline; right the form on a `--page` sheet, max 380px, titled "Sign in" (page title), two ruled fields, one
solid button full width, then a hairline and `Create an account` as a quiet link. No marketing copy, no
testimonial, no image.

### 6.2 Signup — `/signup`

Same frame, three fields, one sentence under the title: `One ledger per person. Nothing is shared.`
Password rules appear only on focus, as a single line.

### 6.3 Dashboard — `/dashboard`

Focal: **the month at a glance**. Composition, top to bottom:

1. Page head: title `Dashboard`, subtitle `The month so far.`, right side the month label and
   `Open ledger →`.
2. **Net-worth pair** — two cards with the standard card header (§5.4). The left card is `Net worth`
   with a centred figure and a centred `In` / `Out` / `Net` strip at its bottom. The right card is
   `Composition`: a 48px two-segment bar (Savings = current + savings accounts, Owed = loans + credit
   cards) and a centred two-column legend of label, amount and percentage at its bottom. Both cards
   share the header / centred-main / bottom-strip rows, so their headers, figures and bottom edges
   line up without tall empty boxes.
3. **Budget + Accounts row** (≈1.6 : 1):
   - **Budget** is a 2×2 aligned card, not a stack of boxes. A header row carries
     `BUDGET · <month>` and the `N over` badge. The top row is `Left to spend` on the left and the
     progress bar under a `Used` label on the right; the labels, the big figure, the bar and the meta
     line share the same rows. The bottom strip is `Planned` and `Spent`, aligned under those columns.
     Left goes `--debit` and reads `Over budget` when the plan is exceeded.
   - **Accounts** is a folio card on the right: monogram tab, name + type + `···42` badge, balance right;
     `Manage` in the header.
4. **Spending**, full width: `Spent this month`, average/day, previous-month change and largest category;
   the month line of §5.8 below. `Details →` opens Spending.
5. **Recent entries**, full width: a 6-row `table` with **no selection or margin column** — entry and date
   on the left, account name in the date line, amount right — and the carried-forward line. The page
   deliberately has no savings card; the savings-account balance lives only in the net-worth composition.

Below 1024px the budget and accounts stack; below 768px the band, budget top row and tables stack,
the budget bottom strip stays two columns, and the page has no horizontal scroll.

### 6.4 Accounts — `/accounts`

Focal: **the ledger**. A summary band of three facts (Total cash · Credit owed · Net worth) on
`--page-sunk` strips, then the account filter row (`All accounts` + one chip per account with its
balance), then the transactions panel — search, filter popover, `Add`, and the ledger table with its
carried-forward footer and pagination. The account _cards_ are gone: with three accounts the chips carry
the same information in one line and the table gets the height.

### 6.5 Manage accounts — `/accounts/manage`

Focal: correction. A single ledger table: margin (type glyph), account name + bank, type as stamp, ending
digits, balance, row actions (`Edit`, `Delete`). Add account is a solid button in the page head, opens a
520px modal with four ruled fields. Delete is a danger confirm with the account balance recited in the body.

### 6.6 Spending — `/accounts/spending`

Focal: **the ledger**, framed by four facts. A bento on a 12-column grid:

1. Page head: title `Spending`, the period tabs (`7D 1M 6M 1Y Custom`) and the resolved date range on
   the right of the title, `Add entry` as the one solid button.
2. Four fact tiles across the top — `Budget`, `Spent`, `Categories`, `Average / day` — each a folio
   with a label, one figure, and one line of context. The Budget tile is wider, shows what is left
   (or over) this month with a progress bar, and falls back to a link to Budgets when no limits exist.
3. Left (7/12): `Spending over time`, the day columns of §5.8 filling the panel.
4. Right (5/12): with budgets, `Budget by category` — budgeted rows first (spent / limit, left or over,
   bar), unbudgeted rows in grey; without budgets, `Where it went`'s allocation strip plus legend.
5. Full width below: the ledger (§5.3) — account filter, four-way sort, pagination, carried-forward
   footer pinned to the bottom of the ledger area. No bulk-selection column: this page is for reading,
   not for batch editing.

The page fills the viewport at 940px and above; only the ledger scrolls. No empty band under the table.

### 6.6a Budgets — `/accounts/budgets`

Focal: **the plan**. One page, MANAGE nav, whatever the month:

1. Head: `Budgets`, one-line purpose, the current month on the right.
2. Summary band: Planned · Spent · Left · Used bar (debit-red and "over" past 100%).
3. Table: Category · Monthly budget (input, commits on blur/Enter) · This month · Left · Used bar.
   Over-limit rows get a debit wash, an `over` badge and "₹x over"; unbudgeted rows read `—` and
   _unbudgeted_. Income categories are listed but need no limit.
4. Amounts repeat every month, apply to debit spend only, and never roll over.

### 6.7 Rules — `/accounts/rules`

Focal: what is automatic. Left: the rule list — numbered rows in the margin (priority is literal here), name,
conditions line in `--ink-soft`, action line with category/account stamps, and `Run` as the only inline action.
A row (the first four columns) opens the edit popup; Delete lives in that popup. No edit/delete icons.
Right (or below on mobile): the preview panel — the last 5 transactions the rule matched, ruled rows.
Rule form is a modal, 760px, in two columns: conditions (AND/OR tab) and actions; a live "matches N of the
last 200 entries" line at the bottom updates as you edit.

### 6.9 Categories — `/accounts/categories`

Focal: order **and the monthly plan**. One page owns both:

1. Header: `Categories`, the purpose line, `Add category`.
2. Summary band once a limit exists: Planned · Spent · Left · Used bar.
3. Ledger table: Category · Share this month · Spent · Entries · Monthly budget (input,
   commits on blur/Enter) · Used bar · Left. Over-limit rows get a debit wash, `over`
   badge and "₹x over"; unbudgeted rows read `—` / _unbudgeted_. A row opens the
   edit popup (name + budget), which also carries Delete.
4. Amounts repeat every month, apply to debit spend only, and never roll over.

### 6.12 Settings — `/settings`

Focal: account hygiene. One folio of hairline-separated sections, each with a 15px title and a one-line
purpose: `Profile` (monogram tab with a camera action, name, email), `Change password`, `Appearance`
(the theme as underline tabs), `API keys` (a four-column grid — name flexes, expires, scope as an
`AppSelect`, `Create key` — then the key rows with prefix stamps), `Sign out all sessions` (danger,
outline). No left nav: five sections fit one scroll and the page reads as a form.

---

## 7. Accessibility

- Text contrast ≥ 4.5:1 (`--ink` on `--page` is ~13:1, `--ink-mute` ~5.2:1 — `--ink-faint` is for
  placeholders only, never for information).
- Colour is never the only carrier: every credit/debit figure also carries `+`/`−`; every stamp carries its
  word; every tick has an `aria-label`.
- Focus visible on everything focusable: 2px `--accent` ring, 2px offset, never removed.
- Keyboard: skip-to-content link, sidebar reachable, tables navigable row by row (`j`/`k` optional, arrow
  keys required), Esc closes overlays, focus returns to the trigger.
- Targets ≥ 44×44 on touch; icon buttons carry `aria-label`; decorative glyphs are `aria-hidden`.
- Motion respects `prefers-reduced-motion`. Tables use real `<table>` semantics with `<th scope>`.
- Live regions: poll results, toast, and the review queue announce changes politely.

---

## 8. Implementation notes

**Stack.** Tailwind v4 + daisyUI v5 are the component layer. Start every interactive or structural
primitive from a daisyUI class — `btn`, `input`, `select`, `textarea`, `checkbox`, `table`, `card`,
`modal`, `tabs`, `progress`, `badge`, `alert`, `skeleton`, `link`, `stat` — then override the visual
properties with the tokens above. daisyUI supplies behaviour, density and keyboard semantics; the
ledger supplies paper, ink and rules. A hand-rolled replacement of a daisyUI primitive is a bug in
the same way an unstyled `<div>` is. The current gap and migration order are recorded in
`docs/audits/daisyui-usage.md`; the wrappers (`AppButton`, `AppField`/`AppInput`/`AppSelect`,
`AppTable`, `AppFolio`, `AppStamp`, `AppSkeleton`, `AppTabs`, `Pagination`) are converted first so
every page inherits the change.

**Charts.** daisyUI 5.7.16 has no chart component. Charts use the shared ApexCharts engine
(`AppApexChart`) and `src/lib/chartPalette.ts`; the dashboard line is an async Apex `area` chart.
No second chart library.

**Tokens in code.** `frontend/src/assets/main.css` becomes: `@import "tailwindcss"`, `@plugin "daisyui"`
with a **custom theme per mode** built from the variables in §4.1, an `@theme` block mapping them to Tailwind
utilities (`bg-page`, `text-ink-soft`, `border-rule`, `text-credit`), and no raw hex anywhere else in the
codebase. One file owns colour.

**Fonts.** `@fontsource-variable/newsreader` and `@fontsource-variable/ibm-plex-sans` imported in
`main.ts`; `font-display: swap`; no CDN link.

**Component inventory to rebuild** (replacing the current ones, not editing them): `AppButton`, `AppField`,
`AppSelect`, `AppDatePicker`, `AppTable` (with margin slot + footer slot), `AppFolio` (card), `AppStamp`,
`AppTick`, `AppTabs`, `AppModal`, `AppMenu`, `AppToaster`, `AppEmpty`, `AppSkeleton`,
`AppApexChart` (the engine), `SpendingColumnsChart`, `CategoryShareBar`. Pagination lives in
`components/Pagination.vue` — it was always a page control, not a primitive.

**Migration, in four stations** — each one shippable on its own:

1. Foundations: tokens, fonts, logo, `@theme` mapping, `AppLayout` shell + sidebar + mobile bar.
2. Primitives: button, field, select, date, stamp, tick, tabs, modal, menu, toast.
3. Surfaces: `AppTable` + folio card + empty/skeleton, then Dashboard and Spending (the two pages that prove
   the system).
4. The rest: Accounts, Manage, Rules, Categories, Settings, Login, Signup, and the modals
   and forms behind them.

**Definition of done for a page:** all four states, keyboard-only pass, 390px width pass (no horizontal
scroll), light and dark pass, no raw hex, no colour that is not one of the meaning tokens, the margin
column present where the surface uses one, daisyUI classes as the base of every interactive/structural
primitive, and a screenshot at 1440 and 390 read before the claim is made.

**Known limits.** `AppSelect` and the calendar are custom popovers; the month and year pickers inside
`DatePicker` open `AppMenu` panels. `main.css` is the only file with literals in it (tokens + the
daisyUI theme map). Charts load async on Spending and the dashboard; the dashboard chart is an Apex
area line, not a daisyUI component (daisyUI has none).

---

## 9. Never

- Never a coloured card background, a gradient, or a shadow on a page surface.
- Never a second accent colour; never a category hue.
- Never a number without tabular figures; never a table without its carried-forward footer.
- Never a modal for something that fits inline; never a native `<select>`.
- Never a blank loading screen; never "Oops"; never an exclamation mark.
- Never hand-roll a daisyUI primitive (`btn`, `input`, `select`, `table`, `card`, `modal`, `tabs`,
  `progress`, `badge`, `alert`, `skeleton`) when the daisyUI class plus token overrides will do.
- Never break the margin column to fit content — the margin is the product, except on the dashboard's
  recent table, which deliberately has no selection column.
