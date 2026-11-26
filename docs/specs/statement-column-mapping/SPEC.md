# Statement column mapping

## Objective

Let the reader tell the importer what each printed column in a bank statement is,
instead of picking one of five fixed layouts. A mapping that produces a broken
running balance must never be importable.

## Behaviour

- The Map step lists the columns printed in the statement's own header, left to
  right, keyed `A`, `B`, … Each column gets one role:
  **Date, Name, Reference, Debit, Credit, Amount, Balance, Not used**.
- The detected roles are pre-filled from the header labels. The reader can change
  any role; the PDF is re-read immediately with the new mapping.
- **Amount mode** is a toggle:
  - *Debit + Credit*: two amount columns, one each.
  - *One amount*: a single amount column plus a direction source — the Dr/Cr
    marker text, or the change in the closing balance.
- `Date` and `Balance` cannot be unmapped. Unmapped columns are listed in an
  *Unmapped* tray and ignored.
- The live table previews the first five rows using the current mapping.
- The validation strip and the error banner report opening, closing, row count,
  missing refs and whether the balances reconcile. **Continue is disabled while
  the mapping is invalid.**
- Statements with no printed header keep the legacy layout fallback.
- The mapping is used exactly as chosen — unlike the old preset select, an
  explicit mapping is never silently replaced by auto-detection.

## Rules that stay

- Reference numbers are preferred from the mapped Reference column, then from the
  narration fallback.
- A re-import is still checked for duplicates by reference, then by
  date + type + amount.

## Boundaries

- Frontend only (`statementPdf.ts`, `StatementImportModal.vue`); no API change.
- The first row's direction is inferred when a single-amount statement has no
  Dr/Cr marker; the UI says so next to the direction control.
