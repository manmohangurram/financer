# Standards — Commands, Verification & Code Quality

## Commands

Run verification in this order after backend changes, then frontend changes.

**Backend (Go, from repo root):**

```bash
go run ./cmd/financer  # starts API on :8080; config from FINANCER_* env (schema applied at boot)
go build ./...
go vet ./...
go test ./...          # unit + repo (SQLite tempfile) + HTTP/MCP integration tests
go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
```

**Frontend (from `frontend/`):**

```bash
npm run dev        # vite dev server on :5173, calls the backend directly (CORS, no proxy)
npm run build       # vue-tsc -b (type check) && vite build → static frontend/dist/
npm test            # vitest — pure helpers/composables under src/lib
```

## Verification checklist

After any change run `gofmt -l cmd internal migrate.go` (must print nothing), `go vet ./...`, `go build ./...`, `go test ./...`, `govulncheck ./...` (repo root) and `npm run format:check && npm run build && npm test` (`frontend/`). Formatting is mandatory before a push — CI fails the PR otherwise. Browser smoke test for UI changes (console must be clean).

## Review before every PR

Before opening a pull request for a branch, run these review skills and fold any fixes into the branch:

- `ponytail-review` — over-engineering scan (delete/stdlib/native/yagni/shrink).
- `code-simplification` — reduce complexity without changing behavior.
- `code-review-and-quality` — correctness, security, maintainability.
- `performance-optimization` — measure and fix measurable bottlenecks (skip if nothing to measure).
- `finishing-a-development-branch` — branch completion: verify tests, present merge options.

## Code standards

### No noise comments

**Default to no comment.** A comment is maintenance cost the moment it is written — it can go stale, and it competes with the code for attention. Before adding one, ask _does the code already say this?_ If yes, don't write it. When in doubt, leave it out.

- **Cut** comments that restate the code: `// increment count`, `// Create a user`.
- **Cut** doc comments that rename the function/field they sit above: `// Set the rule-resolved name` above `ApplyRuleResult`.
- **Keep** a comment only when it carries non-obvious _why_ or intent the code can't express: return-meaning on an opaque type, data-scoping/shape notes, security rationale, a load-bearing edge case.
- **Keep** package comments (`// Package x ...`) and section separators where they aid navigation — terse, not prose.

Rule of thumb: if deleting the comment loses no understanding, delete it.

### Go style

- **Errors, not panics.** Return `error`; map infrastructure failures to `*errors.Error` (`internal/errors`) at the service boundary. `panic` only for impossible init-time states (e.g. `crypto/rand` failure).
- **Wrap with context** (`fmt.Errorf("…: %w", err)`) and keep the message lowercase without trailing punctuation; never leak SQL internals to clients.
- **Interfaces live with the consumer.** A service declares the narrow interface it needs (`RuleApplier`, `TransferApplier`); SQLite repos are concrete types. Widen only when a second implementation exists.
- **Constructors + optional deps.** `New(...) *Service`, with `WithRule` / `WithTransferRule`-style setters for optional collaborators.
- **Nil means "not found".** Repo getters return `(*Row, error)` with `nil, nil` for a missing row. Lists return slices; use `[]T{}` where the wire contract requires `[]` rather than `null`.
- **Timestamps stay strings in storage.** Use `internal/time` (`NowGoTS`, `TSRFC3339`, `LocalDayStart/End`, `LocalKey`); never `time.Now()` inline for stored values.
- **Pointers for nullable rows, values for required.** `*string` for optional columns; scan into `sql.NullString` then promote.
- **Let `gofmt` and `go vet` be the arbiters.** Prefer stdlib; keep imports grouped stdlib / third-party / module.

### Modules & structure

- **One package = one responsibility.** `internal/repo` holds interfaces + row types, `internal/repo/sqlite` the SQL, `internal/<domain>service` the business logic, `internal/server` the HTTP layer. Keep files focused; split when a file does more than one thing.
- **Layering is one-way**: `server` → service → repo → storage. No lower layer imports a higher one; services never touch Gin; repos never touch HTTP.
- **Repo interfaces are backend-agnostic** — no storage or HTTP types in signatures.
- **Public surface is intentional.** Export only what other packages use; keep helpers unexported.
- **No import cycles.** Optional collaborators are expressed as consumer-side interfaces, not package imports.

### Declarations & types

- **Wire vs storage types are separate.** `internal/model` = camelCase JSON; `internal/repo` = plain row/input structs. Translate in services or handlers; don't tag repo rows with JSON.
- **String enums for closed sets**, validated at the boundary (`AccountType`, `TransactionType`, rule enums). Unknown → 400, never a silent default.
- **Struct fields mirror the DB column meaning**; migrations own column renames.
- **Constructors**: `New(...) *Service` returns a ready value; fail early at boot for required config.

### Interfaces

- **Define the smallest interface the caller uses**, in the consuming package.
- **One interface per domain** where a seam is genuinely needed (`TransactionRepo`, `MailRepo`, `RuleRepo`); don't create interfaces for single-use structs.
- **No speculative methods** — add a method when a caller needs it.

### Functions

- **Short, single-purpose.** Extract when a function exceeds ~50 lines or does more than one thing.
- **Arguments**: small; group 4+ related params into a struct. Booleans as params are a smell — split the function or pass an enum.
- **Return `(T, error)`** for fallible work; `(*T, error)` when absence is meaningful. Map infra errors at the service boundary.
- **Name by behavior**: verbs (`Create`, `Link`, `IsTransferLinked`), not implementation.

### Error handling

- **Domain error type** (`*errors.Error`) with stable status mapping (400/401/403/404/409/429/502/500). Services return it; `internal/server.writeError` renders `{code, message}`.
- **Never swallow errors silently.** Log server-side detail (`slog`) and return a generic message; don't leak internals or secrets.
- **Check `rows.Err()`** after every query loop; close rows via `defer`.

### Variables & naming

- See `architecture/NAMING_CONVENTIONS.md` (full words, no stutter, role-not-shape, predicate booleans).
- **Locals** short but clear; `row`, `req`, `repo`, `svc`, `txn`, `uid` are fine when unambiguous.
- **Acronyms keep their case**: `userID`, `HTTPClient`, `APIKey`.

### Testing

- **Repos tested** against a SQLite tempfile via `db.Open(t.TempDir()/…)`.
- **Behavior over implementation**; test the public service/repo API, table-driven where cases multiply.
- **`go test ./...` must pass** before any PR (see Verification above).
- One smoke/assert test minimum for non-trivial logic; every HTTP route change should be covered by the MCP/HTTP integration tests or a handler test.

### General

- **No `unsafe`** unless absolutely required and justified.
- **No new dependency without reason** — prefer stdlib/existing deps (see review `ponytail-review`). CGO stays off: the SQLite stack is pure Go.
- **`gofmt`, `go vet` and `govulncheck` are law.**
