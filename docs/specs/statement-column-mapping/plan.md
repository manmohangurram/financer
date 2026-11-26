# Plan — statement column mapping

## Decisions

- **One UI, two models.** The screen is a table of the statement's own columns;
  the amount mode is the only switch. This is Option C from the design review.
- **Explicit mapping is authoritative.** `parseStatement(lines, undefined, mapping)`
  skips auto-correction so a wrong choice surfaces as a balance break instead of
  being quietly replaced.
- **Reparse from a cached extraction.** `readStatementDocument()` extracts the PDF
  once; every role change calls `parseDocument()` synchronously.
- **Roles resolve to x-positions.** Columns are assigned by the printed header's
  x coordinates, because statement amounts are right-aligned inside their column.
- **Native selects inside chips.** The role control is a real `<select>` covering
  a styled chip, giving keyboard and screen-reader behaviour for free.

## Risks / limits

- A single-amount statement without a Dr/Cr marker cannot know the first row's
  direction from the balance alone; the opening is inferred and labelled.
- Statements without a printed header keep the preset fallback.

## Verification

- Unit tests: column detection, default mapping, explicit-mapping authority,
  one-column direction from balance.
- Browser: correct map reconciles 303 rows; swapping Debit/Credit breaks the
  balance and disables Continue; Escape closes and focus returns; mobile has no
  horizontal overflow.
