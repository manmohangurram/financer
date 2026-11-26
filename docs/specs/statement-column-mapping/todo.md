# Todo — statement column mapping

- [x] Detect printed columns (`detectColumns`) and the mapping they imply.
- [x] Parse with an explicit `ColumnMapping` (two columns, or one + direction).
- [x] Expose `columns` / `mapping` on `StatementParse`.
- [x] Cache the PDF extraction (`readStatementDocument` / `parseDocument`).
- [x] Rebuild the Map step as a column table with role selects and mode toggle.
- [x] Keep validation blocking Continue; preview the first five rows.
- [x] Keep the preset fallback for statements with no header.
- [x] Unit tests for mapping behaviour (44 tests passing).
- [x] Browser check: correct map reconciles, wrong map is rejected, Escape/focus.
- [x] Mobile check: no horizontal overflow, 44px targets on the modal.
- [ ] Import a fresh statement end-to-end and confirm the Review step.
