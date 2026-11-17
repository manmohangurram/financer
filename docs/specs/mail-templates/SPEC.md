# Spec: Template-Driven Mail Ingestion — V1

Status: proposed
Owner: Financer
Date: 2026-11-17

## Objective

Replace fixed hand-written bank-alert parsing with **learned, stored regex
templates**. A template parses future mail from the same sender. When no
template matches, DeepSeek (through a self-hosted LiteLLM proxy) is asked to
**generate a template from 2–3 sample emails**, never to parse a single email as
the primary path. Failures and incomplete extractions land in a redesigned
split-pane Inbox where the user corrects fields and approves.

Users: the self-hosting owner linking their own bank-alert mailboxes.

Success:
- The first email from a new sender produces a provisional template; the next
  matching email validates it (or repairs it) without manual regex work.
- Routine alerts after learning parse with **zero LLM calls**.
- LLM is called only on parse failure/refresh, and never blocks the inbox.
- AUTO creates a transaction only when extraction is complete and ambiguous-free.
- Every unparsed/partial email can be completed and approved in the UI.
- Each user configures their own LiteLLM endpoint/model/key in Settings, and
  mail is polled automatically on login and when the Inbox opens (throttled).

## Assumptions

1. LiteLLM is self-hosted and reachable from the Financer container; full email
   text may be sent to it. Each user supplies their own base URL, model alias and
   API key in Settings; there are no server-wide LLM env vars.
2. The configured model is a DeepSeek model alias exposed by the user's LiteLLM
   proxy and returns JSON (`response_format: json_object`).
3. Databases may be reset (single baseline migration; no upgrade path).
4. Go's RE2 engine is used for all generated regexes (linear time, no
   backtracking).
5. The existing mail rules decide disposition (ignore/review/auto); templates
   only extract. Rules are evaluated before any LLM work.
6. The UI follows the existing ledger design system; this spec covers the Inbox
   page and a Settings section only.

## Tech Stack

- Go 1.27, Gin, `glebarez/sqlite` (WAL), embedded baseline migrations.
- `net/http` for the LiteLLM call (OpenAI-compatible `/chat/completions`).
- Vue 3 + TypeScript + Tailwind/daisyUI, existing `App*` primitives.
- Tests: `go test ./...`, `vitest`, `vue-tsc`, `vite build`.

## Commands

```bash
# backend
go build ./...
go vet ./...
go test ./...
gofmt -l cmd internal migrate.go

# frontend
cd frontend
npm run build
npm test
```

## Project Structure (changes)

```
internal/mail/template.go          # template schema, guards, compiled extraction
internal/mail/template_test.go
internal/mail/llm.go               # LiteLLM client + prompt + validation
internal/mail/llm_test.go
internal/mail/poll.go              # parse-first flow, generation trigger
internal/llmsettings/service.go    # per-user LiteLLM settings (encrypted key)
internal/llmsettings/service_test.go
internal/repo/llm.go               # settings row/repo
internal/repo/sqlite/llm.go
internal/model/llmsettings.go      # wire types (never returns the key)
internal/mailservice/service.go    # account create/update (no allowlist), act overrides, PollUser
internal/repo/mail.go              # template rows/repo
internal/repo/sqlite/mail.go
internal/model/mail.go             # wire types (account, action overrides)
internal/server/mail.go            # PUT account, manual poll, action body
internal/server/llm.go             # GET/PUT /api/me/llm
internal/openapi/spec.go           # update documented mail/settings payloads
db/migrations/000001_initial_schema.up.sql  # mail_templates, user_llm_settings, no allowlist
frontend/src/views/InboxView.vue   # split-pane layout
frontend/src/views/ProfileView.vue # tabbed settings shell
frontend/src/components/settings/* # Account, Appearance, Security, Inbox AI, Mailboxes, API keys tabs
frontend/src/components/mail/*     # queue, review pane, mailbox form
frontend/src/lib/api/client.ts     # updateAccount, manualPoll, llm settings, action overrides
```

## Data Model

`mail_accounts`: **drop `sender_allowlist`** (rules decide disposition).
Add `mail_templates`:

| column | type | notes |
|---|---|---|
| id | TEXT PK | uuid |
| user_id | TEXT NOT NULL | FK users, cascade |
| sender | TEXT NOT NULL | lowercased From address |
| name | TEXT NOT NULL | human label from LLM |
| guards_json | TEXT NOT NULL | `{subject_contains, body_contains, body_not_contains}` |
| patterns_json | TEXT NOT NULL | field → RE2 pattern (named captures) |
| status | TEXT NOT NULL | `provisional` \| `active` \| `disabled` |
| sample_count | INTEGER NOT NULL DEFAULT 1 | distinct messages learned/validated on |
| successes | INTEGER NOT NULL DEFAULT 0 | |
| failures | INTEGER NOT NULL DEFAULT 0 | |
| created_at / last_used_at / last_attempt_at | TIMESTAMP | |

Index `(user_id, sender, status)`. `mail_messages` gains nullable
`template_id` (FK → templates, `ON DELETE SET NULL`).

`user_llm_settings` (one row per user):

| column | type | notes |
|---|---|---|
| user_id | TEXT PK | FK users, cascade |
| base_url | TEXT NOT NULL | e.g. `http://litellm:4000/v1` |
| model | TEXT NOT NULL | DeepSeek alias configured in LiteLLM (required; no default) |
| api_key_enc | TEXT NOT NULL DEFAULT '' | same ChaCha20-Poly1305 scheme as mailbox passwords |
| timeout_secs | INTEGER NOT NULL DEFAULT 20 | |
| enabled | INTEGER NOT NULL DEFAULT 1 | |
| min_samples | INTEGER NOT NULL DEFAULT 3 | samples required to promote a template to active |
| failure_threshold | INTEGER NOT NULL DEFAULT 3 | failures before regeneration |
| cooldown_minutes | INTEGER NOT NULL DEFAULT 60 | minimum gap between generations per sender |
| auto_poll_throttle_minutes | INTEGER NOT NULL DEFAULT 5 | login/Inbox poll throttle |
| updated_at | TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP | |

Template JSON:

```json
{
  "name": "HDFC debit alert",
  "guards": {"subject_contains": "debited", "body_contains": "INR", "body_not_contains": ["OTP", "statement"]},
  "patterns": {
    "amount": "INR\\s*(?P<amount>[\\d,]+(?:\\.\\d{1,2})?)",
    "debit": "(?i)\\bdebited\\b",
    "credit": "(?i)\\bcredited\\b",
    "account": "(?:A/c|account|card)[^.\\n]{0,24}?(?:[Xx*]{2,})?(?P<account>\\d{4})\\b",
    "merchant": "(?i)\\bat\\s+(?P<merchant>[A-Za-z0-9][A-Za-z0-9 .&_'-]{1,40})",
    "date": "(?P<date>\\d{1,2}\\s+[A-Z]{3}\\s+\\d{4}|\\d{1,2}[-/]\\d{1,2}[-/]\\d{2,4})"
  }
}
```

## Behavior

### Parse-first poll

1. Fetch recent INBOX mail (no sender search; window unchanged, hard cap 200).
2. Skip known Message-IDs; evaluate **IGNORE rules first** (no LLM, no template).
3. For each sender, load `active`/`provisional` templates ordered by specificity
   (guards present > broader) and first-match:
   - all guard conditions must hold (case-insensitive contains; `body_not_contains`
     must not match).
   - apply patterns; captures: amount>0, direction from debit/credit match,
     date parsed from common formats (`17 NOV 2026`, `17/11/2026`, `2026-11-17`),
     account last-4, merchant.
4. Extraction completeness: `amount + direction + date` required; `account`
   required for unattended AUTO; merchant optional but defaults to "Unknown".
5. Regex **pre-hints** extract the same fields independently; if a hint and the
   template disagree on amount/date/account, keep the template value but mark
   the extraction `needs_review`.

### Template generation (LLM)

Trigger when no template matches or the match is incomplete:

1. Collect up to 3 recent messages from that sender (current + stored), including
   `approved_json` corrections from resolved rows.
2. Load the user's LiteLLM settings. If missing/disabled or no base URL → log
   once at debug, mark `UNPARSED`, stop. No generation without user settings.
3. Decrypt the stored proxy key and call LiteLLM with a prompt requiring JSON:
   `{"templates":[{"name","guards","patterns"}]}`; temperature 0; timeout from
   the user's settings (default 20 s); no retries beyond one.
4. Validate every returned template:
   - all patterns compile on RE2, length ≤ 500 chars each;
   - guards plus patterns parse **every** sample into a complete extraction;
   - reject duplicates of an existing template (same sender+guards).
5. Store as `provisional` when learned from one sample, `active` once it parses
   the user's `min_samples` distinct samples (default 3, 2 accepted as a lower
   bound when only that many exist). Re-parse the triggering email with the new
   template.
6. Never overwrite a working template; add a variant. If a sender already has a
   failing template, increment `failures`; after `failure_threshold` failures
   (default 3) or when new samples arrive, attempt regeneration — but no more
   often than `cooldown_minutes` (default 60) per sender.

### Disposition

- `IGNORE` rule → `IGNORED`, no extraction.
- Template parse complete + AUTO rule + **active** template + exactly one account
  matches the last-4 → create transaction automatically (`AUTO`).
- Provisional template, partial extraction, ambiguity (0 or >1 account), or
  hint disagreement → `PENDING`/`UNPARSED` for review.
- No auto-create when the LLM/template is unavailable.

### Manual correction and approve

`POST /api/mail/messages` items gain optional overrides:
`{name, amount, type, occurredAt, accountId, categoryIds}`.
Overrides win over stored extraction; the effective extraction is persisted in
`approved_json` and used as future training evidence. Approve still creates the
transaction with `externalId = mail:<Message-ID>`.
Validation errors return per-row failures; the UI keeps the row open.

### Mailbox management

- Create/update no longer accept `senderAllowlist`.
- `PUT /api/mail/accounts/{id}`: label, host, port, username, enabled, optional
  password (empty keeps the stored secret). Validates host/port/username as today.
- Create still requires the password and `FINANCER_SECRET_KEY`.

### Per-user LLM settings

`GET /api/me/llm` → `{baseUrl, model, timeoutSecs, enabled, apiKeySet, minSamples, failureThreshold, cooldownMinutes, autoPollThrottleMinutes, updatedAt}`
(never the key). `PUT /api/me/llm` accepts the same fields plus `apiKey?`;
an omitted/empty `apiKey` keeps the stored one, and `apiKey = null` clears it.
Saving requires `FINANCER_SECRET_KEY` (same encryption as mailbox passwords).
`baseUrl` must be http(s), a bare origin/path (no credentials), ≤ 300 chars;
`model` ≤ 100 chars; timeout 5–120 s; `minSamples` 1–10; `failureThreshold`
1–10; `cooldownMinutes` 5–1440; `autoPollThrottleMinutes` 1–60. A “Test” action
calls `POST /api/me/llm/test` with a tiny prompt and reports success/error
without touching mail.

Settings UI: Settings → **Inbox AI** — base URL, model, API key (write-only,
shows “configured”), timeout, enabled toggle, Test and Save.

### Automatic polling (async)

- **On login:** after a successful login response is written, the server starts
  an asynchronous poll of that user's enabled mailboxes. It never blocks or fails
  the login.
- **On Inbox open:** the UI calls `POST /api/mail/poll`; the server accepts the
  job and returns **202** immediately with `{status: "started"|"throttled"}`.
  Nothing waits on IMAP.
- The Inbox shows a quiet “Checking mail…” indicator while the job runs and
  refreshes the message list every few seconds until it completes (or the user
  navigates away).
- A per-user throttle (`autoPollThrottleMinutes`, default 5, in-memory) makes
  both triggers a no-op when a poll ran recently; `?force=1` bypasses it for the
  explicit “Poll now” button.
- The existing background poller (`FINANCER_MAIL_POLL_SECS`) keeps running for
  all users; the login/Inbox triggers only add freshness.

## UI — split-pane reviewer (selected option B)

`/inbox`:

- Header: Inbox, mailbox tabs (All + each mailbox), “Poll now”, “Edit mailboxes”.
- Left queue: status filter chips, newest-first rows with sender, subject,
  extracted merchant/amount, status stamp, confidence dot.
- Right review pane: email header, full text (scrollable), form for name, amount,
  type, date, account, categories; missing/uncertain fields highlighted with
  inline errors; error summary on failed approve; Approve / Reject; keyboard
  `A` approve, `R` reject, `J`/`K` next/prev; responsive single-column < 900px.
- Mailbox modal: add/edit (label, host, port, username, password, enabled),
  poll, delete. No allowlist field.
- Settings → **Inbox AI**: LiteLLM base URL, model, write-only API key
  (“configured” indicator), timeout, enabled toggle, advanced thresholds
  (samples/failures/cooldown/throttle), Test, Save.
- Accessibility: labelled controls, visible focus, status not color-only,
  reduced motion respected.

### Settings (tabbed redesign)

Reorganize `/settings` into tabs instead of one long page, using the existing
`AppTabs` primitive and the ledger tokens:

| Tab | Contents |
|---|---|
| **Account** | name, email, avatar upload |
| **Appearance** | light/dark theme |
| **Security** | change password, sign out all sessions |
| **Inbox AI** | LiteLLM base URL, model, API key, timeout, enabled, advanced template thresholds, Test/Save |
| **Mailboxes** | linked mailbox list, add/edit/poll/delete |
| **API keys** | existing key list/create/revoke |

- Tabs are URL-addressable (`/settings?tab=inbox`), arrow-key navigable,
  `aria-selected`/`aria-controls` wired, with visible focus and a labelled panel.
- One tab panel is mounted at a time; unsaved Inbox AI changes warn before
  leaving the tab.
- Responsive: tabs scroll horizontally on narrow widths; no sticky bar overlaps
  content; reduced-motion users get no tab transition animation.
- Each tab uses existing form primitives (`AppInput`, `AppSelect`, `AppField`,
  `AppButton`) — no new styling system and no hardcoded colors.

## Configuration

LLM endpoint/model/key are **per-user settings**, not server env vars. Server
env used by this feature:

| env | default | purpose |
|---|---|---|
| `FINANCER_SECRET_KEY` | _(unset)_ | required to store mailbox passwords and LLM API keys |
| `FINANCER_MAIL_POLL_SECS` | `300` | background poll interval for all users |

Without `FINANCER_SECRET_KEY`, mailbox linking and saving an LLM key are
refused; templates still parse.

## Testing Strategy

- `internal/mail/template_test.go`: guards, amount (`INR 378.64`), account
  (`XX2467`), dates (`17 NOV 2026`, `17/11/2026`), direction, incomplete cases.
- `internal/mail/llm_test.go`: fake LiteLLM (httptest) returns valid/invalid
  templates; invalid regex/sample mismatch rejected; timeout handled.
- `poll_test.go`: no template → generation from one sample (provisional);
  second sample promotes to active; active template → no LLM call; AUTO gating;
  ambiguous account → review; IGNORE rule skips LLM.
- `mailservice` tests: account update (password optional), remove allowlist,
  approve with overrides persists `approved_json`.
- `llmsettings` tests: settings CRUD, key encryption/decryption, key never
  returned by GET, invalid base URL/timeout rejected.
- Auto-poll tests: login triggers an async poll; `POST /api/mail/poll` returns
  202 and respects the per-user throttle; `force=1` bypasses it.
- Settings tests: Inbox AI validation/round-trip, thresholds persist, API key
  never echoed; tab navigation is keyboard-accessible and URL-addressable.
- `internal/server` tests updated for new payloads; OpenAPI spec updated.
- Frontend: helper tests for extraction display/validation; `vue-tsc`, vitest,
  build; manual browser smoke of split-pane.

## Boundaries

- Always: run Go/frontend gates; validate generated regex before storing; keep
  user scoping on every query; never create money rows from partial guesses.
- Ask first: adding a dependency (none planned), changing non-Inbox pages,
  changing the single-baseline reset policy.
- Never: send email content anywhere except the configured LiteLLM base; log
  email bodies or secrets; execute LLM-supplied code; overwrite a working
  template silently.

## Out of Scope (Deferred)

- Per-email LLM extraction as the primary path.
- Fine-tuning or prompt-managed model training.
- Template management UI or public template APIs (templates are learned
  internally; a debug view can come later).
- Sender allowlists and IMAP folder selection.
- Changes to Dashboard/Accounts/Spending/Rules pages.
- Multi-user template sharing.

## Open Questions

None. Defaults are configurable per user in Settings → Inbox AI; the Inbox poll
is asynchronous by design.
