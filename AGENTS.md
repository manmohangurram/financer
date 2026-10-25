# Financer — Project Rules

Financer: a personal finance tracker. Rust backend (axum, plain HTTP/JSON) + Vue 3 (Vite, TypeScript, Tailwind CSS v4 + daisyUI v5) frontend, SurrealDB storage. No protobuf — API types are hand-written Rust structs in `src/service/*.rs` / `src/repo/*.rs`, serialized to JSON by serde.

## Documentation Index

- **Commands & verification & standards** — [skills/STANDARDS.md](skills/STANDARDS.md)
- **For skill routing & stack/PR workflows** — see the global pi skills (`~/.pi/agent/skills/`) as needed
- **Architecture** (request flow, layering, API types, rule engine, server aggregation, frontend client, data scoping) — [architecture/REQUEST_FLOW.md](architecture/REQUEST_FLOW.md), [architecture/LAYERING.md](architecture/LAYERING.md), [architecture/API_TYPES.md](architecture/API_TYPES.md), [architecture/RULE_ENGINE.md](architecture/RULE_ENGINE.md), [architecture/SERVER_AGGREGATION.md](architecture/SERVER_AGGREGATION.md), [architecture/FRONTEND_CLIENT.md](architecture/FRONTEND_CLIENT.md), [architecture/DATA_SCOPING.md](architecture/DATA_SCOPING.md)
- **Frontend structure** — [architecture/FRONTEND_STRUCTURE.md](architecture/FRONTEND_STRUCTURE.md)
- **Rust naming conventions** — [architecture/NAMING_CONVENTIONS.md](architecture/NAMING_CONVENTIONS.md)
- **Conventions & gotchas** (Tailwind/daisyUI, migrations, env vars, commits, PR format) — [architecture/CONVENTIONS_GOTCHAS.md](architecture/CONVENTIONS_GOTCHAS.md)
- **Deployment** (frontend↔backend linking, multi-arch builds, GitHub Actions release flow) — [DEPLOY.md](DEPLOY.md)

## Quick Reference

- **Commands:** `cargo run` (API on :8080; config from `FINANCER_*` env, schema applied at boot), `cargo build --locked`, `cargo clippy --all-targets -- -D warnings`, `cargo test`, `cargo audit`; frontend: `npm run dev` / `npm run build` / `npm test` (details in [skills/STANDARDS.md](skills/STANDARDS.md)).
- **Architecture:** repo → service → http layering; SurrealDB (server-mode HTTP-RPC client, embedded `Mem` in tests); schema via idempotent `define_tables()` at boot; record ids (`table:<id>`) with plain-string wire ids; per-user scoping via `user` record links (details in [architecture/](architecture/)).
- **API docs:** Swagger UI at `/docs` (utoipa-generated), spec at `/openapi.json`.
- **Deployment:** see [DEPLOY.md](DEPLOY.md).

## Workflow rules

- **Stacks, not one-off branches.** Multi-stage work ships as a GitHub stack via `gh stack` (`init` / `add` / `submit`), one branch per stage, each PR based on the one below. Never a bare `git checkout -b` inside a stack. Simple single-commit changes stay a plain branch + PR.
- **Never force-push.** No `gh stack rebase`, no `--force`/`--force-with-lease`. Update a branch by merging `main` into it. `gh stack submit` only for creating PRs on branches that have not been pushed yet.
- **After creating a branch, refresh the code graph before reading code:** run `graphify . --update --code-only` (add `--update` without `--code-only` only when an LLM key is set; the wrapper enriches the graph with function end lines either way), then locate code with the `code-read` skill rather than reading whole files: `semble search` when the file is unknown, `rg -n` when it is known, then `read-ranges.py --file <f> --at <line>` for just the enclosing construct. Never `read` without `offset`/`limit` unless the file is under ~120 lines.
- **Gates before a PR:** `cargo fmt --check`, `cargo clippy --all-targets -- -D warnings` and with `--features surreal`, `cargo test` both feature sets, `cargo audit`, plus frontend `vue-tsc` / `npm test` / `npm run build` when the UI changed.
