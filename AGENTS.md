# Financer — Project Rules

Financer: a personal finance tracker. Go backend (Gin, plain HTTP/JSON, pure-Go SQLite) + Vue 3 (Vite, TypeScript, Tailwind CSS v4 + daisyUI v5) frontend, SQLite storage. API types are hand-written Go structs in `internal/model/*.go`, serialized by `encoding/json`.

## Documentation Index

- **Commands & verification & standards** — [skills/STANDARDS.md](skills/STANDARDS.md)
- **For skill routing & stack/PR workflows** — see the global pi skills (`~/.pi/agent/skills/`) as needed
- **Architecture** (request flow, layering, API types, rule engine, server aggregation, frontend client, data scoping) — [architecture/REQUEST_FLOW.md](architecture/REQUEST_FLOW.md), [architecture/LAYERING.md](architecture/LAYERING.md), [architecture/API_TYPES.md](architecture/API_TYPES.md), [architecture/RULE_ENGINE.md](architecture/RULE_ENGINE.md), [architecture/SERVER_AGGREGATION.md](architecture/SERVER_AGGREGATION.md), [architecture/FRONTEND_CLIENT.md](architecture/FRONTEND_CLIENT.md), [architecture/DATA_SCOPING.md](architecture/DATA_SCOPING.md)
- **Frontend structure** — [architecture/FRONTEND_STRUCTURE.md](architecture/FRONTEND_STRUCTURE.md)
- **Naming conventions** — [architecture/NAMING_CONVENTIONS.md](architecture/NAMING_CONVENTIONS.md)
- **Conventions & gotchas** (Tailwind/daisyUI, migrations, env vars, commits, PR format) — [architecture/CONVENTIONS_GOTCHAS.md](architecture/CONVENTIONS_GOTCHAS.md)
- **Feature specs, plans & tasks** — one folder per feature under [docs/specs/](docs/specs/) (`SPEC.md`, `plan.md`, `todo.md`)
- **Deployment** (frontend↔backend linking, the arm64 image build, GitHub Actions release flow) — [DEPLOY.md](DEPLOY.md)

## Quick Reference

- **Commands:** `go run ./cmd/financer` (API on :8080; config from `FINANCER_*` env, schema applied at boot), `go build ./...`, `go vet ./...`, `go test ./...`, `govulncheck ./...`; frontend: `npm run dev` / `npm run build` / `npm test` (details in [skills/STANDARDS.md](skills/STANDARDS.md)).
- **Architecture:** repo → service → server layering; SQLite via `glebarez/sqlite` with WAL (write + read pools in `internal/db/sqlite.go`); migrations embedded from `db/migrations/` and applied at boot; plain-string ids on the wire; per-user scoping by `user_id` (details in [architecture/](architecture/)).
- **API docs:** Swagger UI at `/docs`, hand-written spec at `/openapi.json`; MCP at `/mcp` (API-key auth).
- **Deployment:** see [DEPLOY.md](DEPLOY.md).

## Workflow rules

- **Always search with the codebase-memory MCP, not grep.** Start every code lookup with `search_graph` (symbols), `trace_path` (callers/callees), `get_code_snippet` (source) and `query_graph` (multi-hop). Use `rg`/grep only for string literals, config values, non-code files, or when the graph has a recorded coverage gap — and read the reported ranges when it does. `check_index_coverage` validates evidence paths before claims.
- **Stacks, not one-off branches.** Multi-stage work ships as a GitHub stack via `gh stack` (`init` / `add` / `submit`), one branch per stage, each PR based on the one below. Never a bare `git checkout -b` inside a stack. Simple single-commit changes stay a plain branch + PR.
- **Never force-push.** No `gh stack rebase`, no `--force`/`--force-with-lease`. Update a branch by merging `main` into it. `gh stack submit` only for creating PRs on branches that have not been pushed yet.
- **After creating a branch, refresh the code graph before reading code:** refresh the `codebase-memory-mcp` project index (`index_repository`) and use its graph tools. Read code through MCP, not whole files: `search_graph`/`search_code` return the file and line, `get_code_snippet` returns just the enclosing construct, and `get_file_outline` pages members with `offset`/`limit`. Use `read` with `offset`/`limit` only when the graph has no entry for the file. Never read whole files unless they are under ~120 lines.
- **Gates before a PR:** `gofmt -l cmd internal migrate.go` (must print nothing), `go vet ./...`, `go build ./...`, `go test ./...`, `govulncheck ./...`, plus frontend `vue-tsc` / `npm test` / `npm run build` when the UI changed.
- **Specs, plans and tasks live together.** Any feature that spans more than a trivial fix gets `docs/specs/<feature>/SPEC.md`, `plan.md` and `todo.md` (see [docs/specs/README.md](docs/specs/README.md)). Write the spec before code, get it approved, then plan and implement from it. Update the spec first when a decision changes, keep the todo checkboxes current, and commit all three with the feature branch.
