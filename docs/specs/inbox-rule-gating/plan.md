# Inbox Rule Gating — plan

Source spec: `docs/specs/inbox-rule-gating/SPEC.md`

## Decisions

- **Rule match is mandatory.** `PollAccount` evaluates rules before storing; no match
  means no row. IGNORE rows are still stored (hidden) so the same Message-ID is not
  refetched every poll.
- **Full poll uses the same pipeline.** A new `HeadersAll` IMAP method returns every
  UID (chunked envelope fetch); bodies are fetched only for unknown IDs. Matched rows
  flush in transactions of 100 (`InsertMessagesBatch`).
- **Cleanup is Go-side.** Rules are substring conditions, not SQL, so cleanup reads
  messages and evaluates rules in memory. One pass per boot, after migrations.
- **AI job is in-process state.** One job per user, guarded by a mutex; approve/reject
  checks the same state and returns 409. Restart drops the job (nothing durable).
- **Templates remain the only LLM output.** The job reuses `GenerateTemplates` with a
  force flag instead of the cooldown gate, then re-parses locally.
- **Re-parse scope:** pending rows for that sender only; decided/ignored rows are
  untouched.

## Work

1. Poll: rule-gated storage + skipped accounting; batch insert helper.
2. IMAP: `HeadersAll` without a date window, chunked envelopes.
3. Full poll service + `POST /api/mail/poll/full` (async).
4. Cleanup repo queries + startup pass wired in `main`.
5. AI job: state, sender grouping, forced generation, re-parse, 409 lock, endpoints.
6. Inbox UI: Full poll and Parse with AI buttons, progress banner, disabled decisions.
7. Tests: unmatched skipped, matched stored, batch insert, cleanup rules, AI lock.
8. Docs/OpenAPI/changelog; gates.

## Risks

- Whole-mailbox scans are unbounded by design; bodies are fetched only for unknown
  IDs and inserts are batched, but a very large mailbox will take time — the job runs
  in the background and logs progress.
- Cleanup deletes data; the no-transaction/no-approval guard is the safety rail.
- AI re-parse can book transactions; it runs under the same lock as approve/reject.

## Verification

Go gates + frontend build/tests; API smoke: unmatched poll stores nothing, full poll
batches, cleanup removes noise and keeps booked, AI job blocks decisions and reports
progress.
