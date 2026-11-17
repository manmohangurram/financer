# Plan — manual mail templates

1. **Pattern lists.** `mail.PatternList` (string-or-array JSON); `Template` compiles
   `[]*regexp.Regexp` per field; `captureAny`/`anyMatch` pick the first hit.
2. **Storage.** `mail_templates` rows with `status = 'manual'`, keyed by the rule's
   `fromPattern`; `ListManualTemplates` matches sender patterns as substrings of the
   From address; `UpsertManualTemplate` / `GetManualTemplate` / `DeleteManualTemplate`.
3. **Priority.** `parseWithTemplates` tries manual rows first, then exact-sender rows;
   manual rows are never promoted or sample-counted; `storeTemplate` refuses to
   overwrite.
4. **Rule API.** `template` on `MailRule`/`ReqMailRule`; save/update/delete keeps the
   manual row in sync; templates require a From condition.
5. **Validation endpoint.** `POST /api/mail/templates/validate` → per-pattern checks
   plus the overall extraction; `mail.ValidateSpec` does the work.
6. **UI.** `MailTemplateEditor.vue` inside the rule modal; multi-row fields, sample test
   panel, inline stamps; `mailTemplate.ts` holds the field definitions and validation.
7. **Tests.** pattern lists, first-match wins, invalid-pattern reporting, manual beats
   learned, AI never overwrites, rule CRUD round-trip, editor helpers.
