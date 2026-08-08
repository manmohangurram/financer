# Financer UI Standards

> Follow this document on **every** UI change. It defines the design system, component usage rules, and accessibility baseline for the Financer frontend. If a change can't follow a rule, say so in the PR/review rather than silently deviating.

Stack: **Vue 3 + Vite + Tailwind v4 + daisyUI v5** (dark `financer` theme). There is no `tailwind.config.js`; all tokens live in `src/assets/main.css`.

---

## 1. Design tokens (never raw values)

All colors, radii, and shadows come from the `@theme` block and daisyUI theme in `src/assets/main.css`. **Never use inline hex, arbitrary Tailwind values (`bg-[#...]`), or literals.**

### Color semantics

| Token | Use for |
|---|---|
| `bg` / `bg-elevated` / `bg-auth` | page / elevated / auth background |
| `surface` / `surface-alt` | cards, panels, inputs |
| `border` | borders, dividers, hairline strokes |
| `text` | primary text |
| `text-secondary` | secondary text |
| `text-muted` | labels, placeholders, muted text |
| `subtle` | tertiary / helper text |
| `faint` | disabled / decorative |
| `primary-500` + scale | actions, links, active states, focus |
| `income` | credit / gains (green) |
| `expense` | debit / losses / destructive (red) |
| `track` | progress tracks, chart gridlines |

daisyUI semantic classes (`.btn-primary`, `.badge-outline`, `text-error`, etc.) are also fair game — they resolve to the same theme.

### Radius

- Cards: `rounded-2xl` (or `rounded-card` token = 16px)
- Buttons / inputs / selects: `rounded-xl` (11px)
- Badges / pills: `rounded-full`
- Small controls (checkboxes, day cells): `rounded-md`

### Shadows

- Cards: `shadow-card` (large, soft, only for elevated cards/dialogs)
- Popovers/dropdowns: `shadow-popover`
- Buttons: **no shadow** (flat; daisyUI `.btn` has `box-shadow: none` in `main.css`).
- Never stack multiple shadows.

### Spacing scale

Use Tailwind's scale (`px`, `0.5`, `1`, `1.5`, `2`, `3`, …). Never off-scale values like `13px` for margin/padding. Common rhythm: `gap-2`/`gap-3` between controls, `p-6` inside modals, `space-y-4` between form sections.

### Typography

- Page titles: `text-lg`–`text-2xl font-semibold text-text`
- Section titles: `text-sm font-medium text-text`
- Field labels: `text-[12px] text-text-muted` (via `AppInput`/`AppSelect`)
- Body: `text-[13px] text-text`
- Helper/secondary: `text-[11.5px]–text-[12px] text-subtle`

---

## 2. Components — use the shared ones

**Never re-declare form styles inline.** Use these components (in `src/components/`):

| Need | Use | Notes |
|---|---|---|
| Text/number input | `<AppInput v-model label placeholder>` | daisyUI `input` + financer styling baked in; `#icon` slot for leading icons (auto `pl-9`) |
| Select | `<AppSelect v-model label>` with `<option>` children | daisyUI `select` |
| Date picker | `<DatePicker v-model>` | custom calendar; internal selects already styled |
| Modal shell | `<AppModal title @close>` | slot content; `wide` prop for large modals |
| Confirm | `<ConfirmDialog title message @confirm @cancel>` | never use native `confirm()` |
| Dropdown/panel | `<Popover panel-class @close>` | every filter popup follows this shell |
| Card shell | `.card` (daisyUI) or plain `div` with `bg-surface border border-border rounded-2xl` | see section 3 |
| Buttons | daisyUI `.btn` classes only | see section 4 |
| Icons | `@lucide/vue`, `stroke-width="1.5–2"`, size `w-4`/`w-5` | no emoji as icons |

### Duplication rule

If you need the same markup in two places (an input+label pair, an empty state, a segmented control), **extract a component first**, then use it in both. Never copy-paste a class string between files.

---

## 3. Cards & containers

- Card: `card bg-surface border border-border` (daisyUI) or `bg-surface border border-border rounded-2xl shadow-card` when elevated.
- Do not hand-roll account/balance cards — use `AccountCard` (has a `compact` prop).
- List rows: `bg-surface border border-border rounded-xl px-4 py-3`, hover `hover:border-primary-500/40`.
- Avoid uniform card grids as a default layout — let information priority drive the layout.

---

## 4. Buttons — daisyUI `.btn` only

There are **no custom button classes** (`btn-primary-cta`, `btn-danger-ghost` were removed). Use daisyUI v5 variants:

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

## 5. Forms

- Use `AppInput` / `AppSelect` / `DatePicker` — never raw `input w-full bg-surface ...` class strings.
- Every field gets a visible `<label>` (AppInput/AppSelect render it). Icon-only controls get `aria-label`.
- Errors: show as `text-[12px] text-expense` under the field or in an `alert alert-error`; never color-only.
- Number inputs use `step="0.01"` for money; amounts formatted via `formatCurrency`.
- Segmented controls (e.g. Debit/Credit) are the one allowed custom pattern: `peer sr-only` radio + styled span.

---

## 6. States — never a blank screen

Every list/screen must handle all three:

- **Loading:** skeleton blocks (`animate-pulse bg-surface-alt`) for content, never a bare spinner for a whole screen. `aria-busy="true"`.
- **Empty:** icon (`text-faint`) + title + one-line helper + primary action. e.g. RulesTab's empty state.
- **Error:** message + retry button (`btn btn-outline btn-sm`).

---

## 7. Accessibility (WCAG 2.1 AA)

- **Keyboard:** every interactive element is a real `<button>`/`<a>`/`<input>`/`<select>` (focusable by default). Verify by tabbing through.
- **Labels:** every input has a visible label or `aria-label`.
- **Contrast:** text is `text`/`text-secondary` on `surface`; never `subtle`/`faint` for essential text. All text tokens (`text-muted`, `subtle`, `faint`) are tuned to pass WCAG AA (≥4.5:1) on the dark surfaces (`bg`, `base-200`).
- **Don't rely on color alone:** income/expense must pair color with a sign (`+`/`−`) or icon.
- **Focus:** global `:focus-visible` outline is set in `main.css` — don't override it.
- **Reduced motion:** handled globally via `prefers-reduced-motion` in `main.css`; don't add new animations that bypass it.

---

## 8. Responsive

Mobile-first. Test at **320, 768, 1024, 1440**. Sidebar (`AppLayout`) collapses on small screens; forms go single-column (`grid-cols-1 sm:grid-cols-2`); tables scroll horizontally inside a `overflow-x-auto` container.

---

## 9. What "done" looks like

- [ ] Renders with no console errors.
- [ ] Tab-through works; every action is a real button.
- [ ] Loading / empty / error states present.
- [ ] Uses `AppInput`/`AppSelect`/`AppModal`/`ConfirmDialog`/`Popover` where applicable.
- [ ] Only daisyUI `.btn` button classes + `@theme` tokens (no hex, no `bg-[...]`).
- [ ] No duplicated component code — if you copy-pasted markup, extract a component.
- [ ] Works at 320 / 768 / 1024 / 1440.
