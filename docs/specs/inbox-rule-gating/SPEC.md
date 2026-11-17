# Inbox Rule Gating — SPEC

Status: approved
Owner: mail ingest, Inbox

## Objective

The Inbox must contain only mail that matched a rule. Unmatched mail is noise and
is never stored. Existing noise is removed at startup, a full poll can backfill an
entire mailbox, and an explicit AI action clears pending mail by learning templates
per sender.

## Behaviour

### Normal poll

- For each message in the recent window: fetch the body, evaluate the user's mail
  rules (IGNORE first, then REVIEW/AUTO).
- **No rule matches → the message is skipped and not stored.** It is counted as
  `skipped`.
- IGNORE matches are stored as `IGNORED` so they are not reprocessed; REVIEW and
  AUTO matches keep today's behaviour (templates, AUTO requires active template +
  account hint, otherwise review).

### Full poll

- New action `POST /api/mail/poll/full` (background, 202) scans the **whole INBOX**
  with no date window.
- Envelopes are fetched in chunks; bodies only for Message-IDs not already stored.
- Only rule matches are stored; matched rows are inserted in **batches of 100**.
- Same dispositions and template pipeline as a normal poll.

### Startup cleanup

- On boot, after migrations, delete stored messages that:
  - match **no current rule**, and
  - have **no transaction** and no approved extraction, and
  - are not `REJECTED`.
- Booked/approved rows and rejected history are kept. Logged: users scanned,
  messages deleted.
- Runs once per boot, before the poller starts.

### AI parse job

- `POST /api/mail/ai-parse` starts one background job per user; `GET` returns its
  state.
- Groups pending mail (`PENDING` / `UNPARSED`) by sender. For each sender with
  pending rows:
  1. take up to **5 newest unrejected bodies** (any status except `REJECTED`),
  2. call the user's LLM for a regex template (cooldown bypassed — the user asked),
  3. validate every candidate against every sample (existing rule),
  4. store the template, then re-parse that sender's pending rows locally,
  5. AUTO-eligible rows are booked; rows that still fail stay in review.
- **Progress lock:** while the job runs, `POST /api/mail/messages` (approve/reject)
  returns `409` for that user, and the Inbox shows a banner with sender progress and
  disables Approve/Reject. The lock clears when the job ends (success or error).
- Status shape: `{ running, totalSenders, doneSenders, currentSender, startedAt,
  finishedAt, error }`.

## API

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/mail/poll/full` | start a whole-mailbox background poll (202 `{status:started}`) |
| POST | `/api/mail/ai-parse` | start the per-sender AI parse job (202) |
| GET | `/api/mail/ai-parse` | job state |

## Non-goals

- Recovering deleted mail.
- Per-email LLM extraction (the model only generates templates).
- Changing rule matching semantics or the disposition order.
- Cleaning messages that produced a transaction.

## Acceptance

- Polling a mailbox with no matching rules stores nothing.
- A REVIEW rule match is stored and appears in review; an IGNORE match is stored as ignored.
- Full poll ingests old matched mail in 100-row batches.
- Restart removes existing unmatched, transaction-free messages and keeps booked/rejected ones.
- AI parse shows progress, blocks approve/reject, then leaves a clear queue (booked or review).
