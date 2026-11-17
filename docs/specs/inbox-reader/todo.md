# Inbox Reader — todo

Spec: `docs/specs/inbox-reader/SPEC.md` · Plan: `docs/specs/inbox-reader/plan.md`

## Backend
- [x] `internal/mail/body.go`: MIME parse, HTML→text, quote strip, sender name
- [x] Migration `000003_mail_body` (`body_text`, `from_name`)
- [x] Repo: insert/select/scan the new columns
- [x] Poll stores parsed body
- [x] Service list projection + `Get` (lazy parse for old rows)
- [x] `GET /api/mail/messages/{id}` + OpenAPI
- [x] Parser + service tests

## Frontend
- [x] Fluid content shell
- [x] Queue column with visible scrollbar + hairline split
- [x] `EmailReader` card (parsed body, original toggle)
- [x] Booking card below the email (daisyUI input/select/btn/badge)
- [x] Detail fetch + client method
- [x] Error summary, mobile stacking, no keyboard-shortcut hints

## Release
- [x] Gates: gofmt/vet/build/test, vue-tsc/npm test/build
- [x] Browser smoke at 1440 (no console errors, no horizontal overflow)
- [ ] Changelog + PR
