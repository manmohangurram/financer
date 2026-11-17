# Todo — Template-Driven Mail Ingestion V1

Spec: `SPEC-mail-templates.md` · Plan: `tasks/plan.md`

## Phase 1 — Schema + template engine
- [x] Schema: mail_templates, user_llm_settings, drop allowlist, template_id
- [x] Template compile/guards/extract + tests
- [x] LiteLLM client/prompt/validation + tests

## Phase 2 — Repos + settings
- [x] Mail repo template/account methods
- [x] LLM settings repo/service/model
- [x] Mail service updates (account update, overrides, PollUser)

## Phase 3 — Poll + HTTP
- [x] Poll parse-first + generation + promotion/cooldown
- [x] HTTP routes + login trigger + OpenAPI
- [x] Backend tests + smoke

## Phase 4 — UI
- [x] Tabbed Settings shell + existing tabs
- [x] Inbox AI + Mailboxes tabs
- [x] Split-pane Inbox
- [x] Client + docs + UI gates

## Phase 5 — Release
- [ ] Docs, changelog, version, tag, arm64 image, release
