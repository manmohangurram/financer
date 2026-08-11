# UI Standards

> Follow this document on **every** UI change. It defines the design system, component usage rules, and accessibility baseline. If a change can't follow a rule, say so in the PR/review rather than silently deviating.

Stack: **Vue 3 + Vite + Tailwind v4 + daisyUI v5**. No `tailwind.config.js`; tokens live in the app's `src/assets/main.css` (`@theme` block + daisyUI theme).

---

## 1. Design tokens (never raw values)

All colors, radii, and shadows come from the theme tokens. **Never use inline hex, arbitrary Tailwind values (`bg-[#...]`), or literals.**

### Color semantics

| Token | Use for |
|---|---|
| `bg` / `bg-elevated` | page / elevated background |
| `surface` / `surface-alt` | cards, panels, inputs |
| `border` | borders, dividers, hairline strokes |
| `text` | primary text |
| `text-secondary` | secondary text |
| `text-muted` | labels, placeholders, muted text |
| `subtle` | tertiary / helper text |
| `faint` | disabled / decorative |
| `primary-500` + scale | actions, links, active states, focus |
| `positive` / `negative` | gains vs losses, success vs destructive (green vs red) |
| `track` | progress tracks, chart gridlines |

daisyUI semantic classes (`.btn-primary`, `.badge-outline`, `text-error`, etc.) are also fair game — they resolve to the same theme.

### Radius

- Cards: `rounded-2xl` (16px)
- Buttons / inputs / selects: `rounded-xl` (11px)
- Badges / pills: `rounded-full`
- Small controls (checkboxes, day cells): `rounded-md`

### Shadows

- Cards: `shadow-card` (large, soft — only for elevated cards/dialogs)
- Popovers/dropdowns: `shadow-popover`
- Buttons: **no shadow** (flat).
- Never stack multiple shadows.

### Spacing scale

Use Tailwind's scale (`px`, `0.5`, `1`, `1.5`, `2`, `3`, …). Never off-scale values like `13px` for margin/padding.

### Typography

- Page titles: `text-lg`–`text-2xl font-semibold text-text`
- Section titles: `text-sm font-medium text-text`
- Field labels: `text-[12px] text-text-muted`
- Body: `text-[13px] text-text`
- Helper/secondary: `text-[11.5px]–text-[12px] text-subtle`

---

## 2. Form metrics (mandatory)

Every form field — input, select, textarea, date picker — follows these exact dimensions and spacing. **No per-field variance.**

### Input / select box

| Property | Value |
|---|---|
| Height | `h-10` (40px) — single-line inputs and selects |
| Textarea min-height | `h-24` (96px) |
| Padding (horizontal) | `px-3.5` (14px) |
| Padding (vertical) | centered text (`h-10` handles it) |
| Border radius | `rounded-xl` (11px) |
| Border | `border` token, 1px; `focus` → `primary-500` border + ring |
| Font size | `text-[13px] text-text` |
| Width | fill the container (`w-full`) unless a fixed width is required |

### Layout / spacing between fields

| Property | Value |
|---|---|
| Label → field gap | `mb-1.5` (6px) |
| Between fields (same group) | `space-y-4` (16px vertical) |
| Between fields (side by side) | `gap-4` (16px horizontal) |
| Field group → field group | `space-y-6` (24px) |
| Form → action row | `pt-6` (24px) |
| Action buttons gap | `gap-3` (12px) |
| Section heading → first field | `mt-4` (16px) |

### Inline errors

- Error text: `text-[12px] text-negative`, directly **below** the field, `mt-1.5` (6px).
- Invalid field: red border + red focus ring; pair with an `aria-invalid` attribute.
- Never rely on color alone — pair the error with a message.

### Segmented controls

- Same height as inputs (`h-10`), `rounded-xl`, inner buttons `px-3`, `gap-1`.
- The active segment is `bg-primary text-primary-content`; inactive is transparent.

---

## 3. Components — use the shared ones

**Never re-declare form styles inline.** Use the project's shared form/UI components (in `src/components/`):

| Need | Use | Notes |
|---|---|---|
| Text/number input | the shared labeled input component | styled `input` baked in; leading-icon slot auto-pads left |
| Select | the shared labeled select component | with `<option>` children |
| Date picker | the shared date-picker component | custom calendar; internal selects styled |
| Modal shell | the shared modal component | slot content; optional `wide` prop for large modals |
| Confirm | the shared confirm dialog component | never use native `confirm()` |
| Dropdown/panel | the shared popover component | every filter/popup shell follows this |
| Card shell | `.card` (daisyUI) or `div` with `bg-surface border border-border rounded-2xl` | see section 4 |
| Buttons | daisyUI `.btn` classes only | see section 5 |
| Icons | `@lucide/vue`, `stroke-width="1.5–2"`, size `w-4`/`w-5` | no emoji as icons |

### Duplication rule

If you need the same markup in two places (an input+label pair, an empty state, a segmented control), **extract a component first**, then use it in both. Never copy-paste a class string between files.

---

## 4. Cards & containers

- Card: `card bg-surface border border-border` (daisyUI) or `bg-surface border border-border rounded-2xl shadow-card` when elevated.
- If a domain entity has a recurring card, extract a named component rather than hand-rolling per view.
- List rows: `bg-surface border border-border rounded-xl px-4 py-3`, hover `hover:border-primary-500/40`.
- Avoid uniform card grids as a default layout — let information priority drive the layout.

---

## 5. Buttons — daisyUI `.btn` only

There are **no custom button classes**. Use daisyUI v5 variants:

| Purpose | Classes |
|---|---|
| Primary action | `btn btn-primary` (optionally `btn-sm`) |
| Secondary / neutral | `btn btn-ghost` or `btn btn-outline` |
| Destructive | `btn btn-outline btn-error` |
| Dense/toolbar | `btn btn-ghost btn-sm` |
| Icon-only | `btn btn-ghost btn-sm` + `aria-label` |

Rules:
- Add `gap-1.5` (or `gap-2`) when a button contains an icon + label.
- `:disabled` for in-flight states; swap the label to "…" / spinner (`Loader2` with `animate-spin`).
- Loading: replace the CTA label (e.g. "Saving…"), keep the button disabled.
- Never use a `span`/`div`/`a` with `role="button"` where a real `<button>` works (keyboard semantics are free).
- `btn` has no shadow by design.

---

## 6. Forms

- Use the shared labeled input/select/date-picker components — never raw `input w-full bg-surface ...` class strings.
- Every field gets a visible `<label>` (the shared input/select render it). Icon-only controls get `aria-label`.
- Errors: show as `text-[12px] text-negative` under the field or in an error banner; never color-only.
- Segmented controls (e.g. binary toggles) are the one allowed custom pattern: `peer sr-only` radio + styled span.

---

## 7. States — never a blank screen

Every list/screen must handle all three:

- **Loading:** skeleton blocks (`animate-pulse bg-surface-alt`) for content, never a bare spinner for a whole screen. `aria-busy="true"`.
- **Empty:** icon (`text-faint`) + title + one-line helper + primary action.
- **Error:** message + retry button (`btn btn-outline btn-sm`).

---

## 8. Accessibility (WCAG 2.1 AA)

- **Keyboard:** every interactive element is a real `<button>`/`<a>`/`<input>`/`<select>` (focusable by default). Verify by tabbing through.
- **Labels:** every input has a visible label or `aria-label`.
- **Contrast:** text is `text`/`text-secondary` on `surface`; never `subtle`/`faint` for essential text. All text tokens (`text-muted`, `subtle`, `faint`) are tuned to pass WCAG AA (≥4.5:1) on the app surfaces. Any palette used by badges/cells must pass AA on its tinted background in all supported themes.
- **Theme support:** the app ships `system` / `dark` / `light` modes (`data-theme`, persisted in `localStorage`). UI that must match the screen uses the resolved theme, never the raw mode. Tokens have light overrides; every color must pass AA in every theme.
- **Don't rely on color alone:** positive/negative values must pair color with a sign (`+`/`−`) or icon.
- **Focus:** global `:focus-visible` outline is set in `main.css` — don't override it.
- **Reduced motion:** handled globally via `prefers-reduced-motion` in `main.css`; don't add new animations that bypass it.

---

## 9. Responsive

Mobile-first. Test at **320, 768, 1024, 1440**. Navigation collapses on small screens; forms go single-column (`grid-cols-1 sm:grid-cols-2`); tables scroll horizontally inside an `overflow-x-auto` container.

### What changes with viewport width

**Sizes never change** — a 40px input stays 40px, `gap-4` stays 16px at every width. Only **layout** reflows. This is the rule set for that reflow, in compromise order (what gives up space first):

| Breakpoint | Behavior |
|---|---|
| **≥ 1440** (extra space) | Content `max-w` caps the column (~1200px) and centers it; the extra space goes to margins, never stretching controls. Side-by-side fields may go 2–3 columns. |
| **1024–1440** | Multi-column grids (2–3) stay; sidebar (if any) stays expanded. |
| **768–1024** | Grids drop to 2 columns (`lg:grid-cols-2`); sidebars collapse to icon rail / overlay. |
| **< 768** (tight) | Everything goes single-column (`grid-cols-1`); action rows stack; tables scroll horizontally (`overflow-x-auto`) instead of squeezing; dense toolbars wrap or collapse into a "more" menu. |

### What to compromise on, in order, as space shrinks

1. **Column count** — columns collapse first (3 → 2 → 1). Content stays readable.
2. **Side-by-side fields** — two-column form rows become stacked; fields stay full-width and 40px tall.
3. **Non-essential chrome** — hide secondary text, helper copy, decorative icons; keep labels and primary actions.
4. **Inline toolbars** — shrink to icon-only buttons, then to an overflow/"more" popover.
5. **Tables** — scroll horizontally rather than shrinking cell padding or text size below `text-[12px]`.

**Never compromise:** readable font size (`text` ≥ 12px), tap targets (buttons/inputs stay ≥ 40px tall), field spacing below 8px, or hiding the primary action.

### What absorbs extra space, in order, as it grows

1. **Card/panel width** — content column grows up to the `max-w` cap; controls stay fixed-size.
2. **Column count** — grids add columns (1 → 2 → 3) before any single element widens.
3. **Whitespace** — remaining space becomes margins/gaps between sections, not bigger controls.
4. **Charts/media** — the only elements allowed to scale freely with width.

Rule of thumb: **controls are fixed, layout is fluid.** If a design needs larger controls on a big screen, that's an explicit accessibility decision (`clamp()` or media query), not the default.

---

## 10. What "done" looks like

- [ ] Renders with no console errors.
- [ ] Tab-through works; every action is a real button.
- [ ] Loading / empty / error states present.
- [ ] Uses the shared components (input/select/date-picker/modal/confirm/popover) where applicable.
- [ ] Only daisyUI `.btn` button classes + theme tokens (no hex, no `bg-[...]`).
- [ ] Form fields follow the section 2 metrics exactly (40px height, 16px vertical gap, 6px label gap, etc.).
- [ ] No duplicated component code — if you copy-pasted markup, extract a component.
- [ ] Works at 320 / 768 / 1024 / 1440.
