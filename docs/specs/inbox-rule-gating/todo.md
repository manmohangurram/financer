# Inbox Rule Gating — todo

Spec: `docs/specs/inbox-rule-gating/SPEC.md` · Plan: `docs/specs/inbox-rule-gating/plan.md`

## Backend
- [x] Poll stores only rule matches; unmatched counted as skipped
- [x] IMAP `HeadersAll` (no date window, chunked)
- [x] Batch insert of 5 matched rows
- [x] Full poll service + `POST /api/mail/poll/full`
- [x] Startup cleanup of unmatched, transaction-free mail
- [x] AI parse job (per sender, force generation, re-parse)
- [x] `POST/GET /api/mail/ai-parse` + 409 lock on approve/reject
- [x] Tests for gating, cleanup, batch and lock

## Frontend
- [x] Inbox header: Full poll + Parse with AI
- [x] AI progress banner with sender progress
- [x] Disable Approve/Reject while the job runs
- [x] Refresh queue when the job finishes

## Release
- [x] OpenAPI + changelog
- [x] Gates: gofmt/vet/build/test, vue-tsc/npm test/build
- [x] Browser smoke
