# Roadmap

The backend migration is complete: the deployed server is the Go implementation in `cmd/financer` + `internal/`, serving the API and the built Vue SPA from one container. The Rust implementation that previously powered the API has been removed; its behavior lives on in the Go contract and tests.

## Done

- **Foundation** — config, SQLite pools with WAL, a single-baseline embedded schema, error envelope, JWT auth.
- **Accounts & categories** — CRUD, cached balances, cursor-paginated categories.
- **Transactions & transfers** — bulk create/update/delete, server-side filters + cursor pagination, recomputed balances, transfer linking and counterpart creation.
- **Rules** — write-time name/category snapshots, transfer-to-account actions, preview, run-on-existing, auto-backfill.
- **Analytics** — dashboard totals and timezone-aware day/month spending buckets with transfer exclusion.
- **Platform** — API keys, MCP server at `/mcp` (read/read_write scopes), OpenAPI at `/openapi.json`, Swagger UI at `/docs`.
- **Delivery** — Go CI (gofmt/vet/build/test/govulncheck), frontend gate, arm64-only image build validated on PRs, tag-driven GHCR publish, weekly registry prune.

## Post-cutover backlog

- Self-host the Swagger UI assets if `/docs` must work without outbound internet.
- Optional parity polish: bcrypt cost to match the old hashes' work factor, and align the MCP transaction payload with the HTTP list shape.

## Dropped

- **Mail ingest** — removed in 0.16.0 along with the Inbox, mailboxes, templates, rule gating and the LLM
  settings. Imports come from statement PDFs, not from a mailbox.
