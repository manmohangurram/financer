# Inbox Reader — plan

Source spec: `docs/specs/inbox-reader/SPEC.md`

## Decisions

- **Parse at ingest, not on every read.** The poll already has the raw message open;
  deriving `body_text`/`from_name` there costs one pass and keeps `GET /messages` cheap.
- **Text, not HTML.** Rendering remote HTML would need a sanitizer and lets tracking
  pixels through. Server-side HTML→text with block semantics keeps structure without
  the risk.
- **Raw text leaves the list payload.** The list carried the full RFC 822 source for up
  to 300 rows. The reader gets one message on selection instead.
- **Breaking change to the list payload is acceptable**: `rawText` is dropped from
  `GET /api/mail/messages`; the frontend is the only consumer.
- **daisyUI for form controls.** `select`, `input`, `btn`, `badge` base classes are
  already mapped to the project theme in `main.css`; the old native selects in the
  Inbox are replaced by these.
- **Fluid shell.** The 1180px cap is removed; pages own their measure.

## Work

1. `internal/mail/body.go` — MIME parse, HTML→text, quote stripping, `fromName`.
2. `000003_mail_body` migration + repo columns (insert/select/scan).
3. Poll stores `body_text`/`from_name`.
4. `mailservice`: list projection (no raw text, preview), `Get` for one message.
5. HTTP: `GET /api/mail/messages/:id`; OpenAPI.
6. `AppLayout`: fluid content column.
7. `InboxView`: full-bleed queue + stacked email/booking cards, daisyUI controls,
   detail fetch, scrollbar styling; extract `EmailReader` component.
8. Tests: parser table tests, service list/detail, migration count.
9. Gates + browser smoke (1440 and 390), docs.

## Risks

- **Charset decoding** depends on `go-message`; the fallback path must never return
  the raw header block.
- **HTML→text** can flatten tables; use line breaks between cells/rows so statement
  mail stays legible.
- **Existing rows** have empty `body_text`; detail parsing must be lazy but bounded.
- **Payload change** could surprise cached clients; documented in the changelog.

## Verification

`gofmt`, `go vet`, `go build`, `go test ./...`, frontend `npm test` + `npm run build`,
browser smoke with no console errors, screenshots at 1440 and 390.
