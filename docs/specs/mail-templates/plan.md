# Implementation Plan: Template-Driven Mail Ingestion V1

Source spec: `docs/specs/mail-templates/SPEC.md`

## Overview

Add learned regex templates, per-user LiteLLM (DeepSeek) settings, async polling,
manual extraction overrides, editable mailboxes, and a split-pane Inbox plus a
tabbed Settings page. Templates parse every new email; the LLM only generates a
template when parsing fails, and generated regexes are validated against samples
before storage.

## Architecture Decisions

- **Template storage in SQLite** (`mail_templates`), guarded variants per sender;
  regex strings are data, compiled at parse time with RE2.
- **LLM is a template generator, not a parser.** The client posts 1–3 sample
  emails and expects a JSON template; every candidate must compile and parse all
  samples, otherwise it is discarded.
- **Per-user LLM config** (`user_llm_settings`) with the API key encrypted using
  the existing ChaCha20-Poly1305 helper; no LLM environment variables.
- **Async polling** on login/Inbox open with a per-user throttle; the existing
  background poller remains.
- **Manual corrections** are sent with the approve action and stored in
  `approved_json`, then used as samples when regenerating templates.
- **Single baseline migration** updated in place (fresh DB policy already
  documented); no incremental migration.

## Task List

### Phase 1 — Schema + template engine
- [ ] Task 1: Baseline schema — `mail_templates`, `user_llm_settings`, remove
      `sender_allowlist`, add `mail_messages.template_id`.
- [ ] Task 2: Template types/validation/extraction (`internal/mail/template.go`)
      with guard matching, amount/account/date parsing, hint disagreement.
- [ ] Task 3: LiteLLM client + prompt + candidate validation
      (`internal/mail/llm.go`), tested against a fake HTTP server.

### Checkpoint 1
- [ ] `go test ./internal/mail/...` passes; schema test passes.

### Phase 2 — Repos + settings
- [ ] Task 4: Mail repo: template CRUD/stats/listing, recent-message samples,
      account update (no allowlist).
- [ ] Task 5: LLM settings repo + service + wire types; encrypted key, test call.
- [ ] Task 6: Mail service: account update, manual action overrides, `PollUser`
      (async + throttle), auto-create demotion when ambiguous.

### Checkpoint 2
- [ ] `go test ./...` passes for mail/mailservice/db.

### Phase 3 — Poll pipeline + HTTP
- [ ] Task 7: Poll-first parse flow: rules → templates → generator → disposition,
      provisional/active promotion, failure counters/cooldown.
- [ ] Task 8: HTTP: `PUT /api/mail/accounts/{id}`, `POST /api/mail/poll`,
      `GET/PUT /api/me/llm`, `POST /api/me/llm/test`; login async poll; OpenAPI.
- [ ] Task 9: Tests: poll with fake source + fake LLM, auto gating, throttling,
      settings endpoints, action overrides.

### Checkpoint 3
- [ ] backend gates green; live smoke of settings + poll 202.

### Phase 4 — UI
- [ ] Task 10: Tabbed Settings shell + Account/Appearance/Security/API keys
      components (URL-addressable, keyboard-accessible).
- [ ] Task 11: Inbox AI + Mailboxes tabs (LLM settings, thresholds, mailbox
      add/edit/poll/delete; no allowlist).
- [ ] Task 12: Split-pane Inbox: queue, email pane, extraction form, inline
      validation, keyboard shortcuts, async poll indicator.
- [ ] Task 13: API client + OpenAPI docs + UI tests/build.

### Checkpoint 4
- [ ] `npm run build`, `npm test`, browser smoke of Inbox + Settings.

### Phase 5 — Release
- [ ] Task 14: docs (README/DEPLOY/AGENTS), changelog, version bump, tag,
      arm64 image, GitHub release.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| LLM emits unsafe/expensive regex | High | RE2 only, length cap, compile + parse all samples before storing |
| LLM duplicates/rewrites working templates | Med | Add variants only; never overwrite; dedupe by sender+guards |
| Cost/latency from repeated failures | Med | failure threshold, cooldown, per-user throttle |
| Ambiguous account auto-posts money | High | AUTO requires complete extraction + unique account; failures demote to PENDING |
| Schema reset breaks old DB | Med | documented fresh-DB policy; startup fails clearly on pre-0.6 DB |
| Settings UI regression | Med | tab extraction preserves existing behavior; vitest + manual smoke |

## Open Questions

None (defaults configurable in Settings).
