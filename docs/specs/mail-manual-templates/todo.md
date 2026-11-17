# Todo — manual mail templates

- [x] `PatternList` accepting a string or an array; compile per field
- [x] First matching pattern wins per field (`captureAny`, `anyMatch`)
- [x] `status = 'manual'` storage keyed by the rule's `fromPattern`
- [x] Manual templates outrank learned ones; AI never overwrites them
- [x] Skip "account differs from regex hint" when no account pattern is set
- [x] Rule API carries and persists the template (create/update/delete)
- [x] `POST /api/mail/templates/validate` + `mail.ValidateSpec`
- [x] Rule modal: `MailTemplateEditor` with multi-pattern fields and sample test
- [x] OpenAPI schemas and path
- [x] Tests: pattern lists, priority, overwrite guard, rule CRUD, editor helpers
- [ ] Optional: `{…}` shorthand sugar
- [ ] Optional: reorder patterns
- [x] Editing a template re-parses queued mail and applies the rule action
- [x] AUTO books only when every declared field extracts cleanly
- [x] Re-application reads the full raw message (list rows omit bodies)
