# Financer — Project Rules

Financer: a personal finance tracker. Rust backend (axum, plain HTTP/JSON) + Vue 3 (Vite, TypeScript, Tailwind CSS v4 + daisyUI v5) frontend, SurrealDB storage. No protobuf — API types are hand-written Rust structs in `src/service/*.rs` / `src/repo/*.rs`, serialized to JSON by serde.

## Documentation Index

- **Commands & verification** — [skills/COMMANDS.md](skills/COMMANDS.md)
- **Skills routing table** — [skills/SKILLS.md](skills/SKILLS.md)
- **Architecture** (request flow, layering, API types, rule engine, server aggregation, frontend client, data scoping) — [architecture/REQUEST_FLOW.md](architecture/REQUEST_FLOW.md), [architecture/LAYERING.md](architecture/LAYERING.md), [architecture/API_TYPES.md](architecture/API_TYPES.md), [architecture/RULE_ENGINE.md](architecture/RULE_ENGINE.md), [architecture/SERVER_AGGREGATION.md](architecture/SERVER_AGGREGATION.md), [architecture/FRONTEND_CLIENT.md](architecture/FRONTEND_CLIENT.md), [architecture/DATA_SCOPING.md](architecture/DATA_SCOPING.md)
- **Frontend structure** — [architecture/FRONTEND_STRUCTURE.md](architecture/FRONTEND_STRUCTURE.md)
- **Rust naming conventions** — [architecture/NAMING_CONVENTIONS.md](architecture/NAMING_CONVENTIONS.md)
- **Conventions & gotchas** (Tailwind/daisyUI, migrations, env vars, commits, PR format) — [architecture/CONVENTIONS_GOTCHAS.md](architecture/CONVENTIONS_GOTCHAS.md)
- **Deployment** (frontend↔backend linking, multi-arch builds, GitHub Actions release flow) — [DEPLOY.md](DEPLOY.md)

## Quick Reference

- **Commands:** `cargo run` (API on :8080; connects to SurrealDB at FINANCER_SURREAL_URL, schema applied at boot), `cargo build --locked`, `cargo clippy -- -D warnings`, `cargo test`; frontend: `npm run dev` / `npm run build` / `npm test` (details in [skills/COMMANDS.md](skills/COMMANDS.md)).
- **Architecture:** repo → service → http layering; SurrealDB (server-mode HTTP-RPC client, embedded `Mem` in tests); schema via idempotent `define_tables()` at boot; record ids (`table:<id>`) with plain-string wire ids; per-user scoping via `user` record links (details in [architecture/](architecture/)).
- **API docs:** Swagger UI at `/docs` (utoipa-generated), spec at `/openapi.json`.
- **Deployment:** see [DEPLOY.md](DEPLOY.md).
