# Standards — Commands, Verification & Code Quality

## Commands

Run verification in this order after backend changes, then frontend changes.

**Backend (Rust, from repo root):**
```bash
cargo run            # starts API on :8080; reads config from config.toml (schema applied at boot)
cargo build --locked
cargo clippy -- -D warnings
cargo test           # unit + repository integration tests (embedded SurrealDB Mem)
```

**Frontend (from `frontend/`):**
```bash
npm run dev        # vite dev server on :5173, calls the backend directly (CORS, no proxy)
npm run build       # vue-tsc -b (type check) && vite build → static frontend/dist/
npm test            # vitest — pure helpers/composables under src/lib
```

## Verification checklist

After any change run `cargo build --locked && cargo clippy -- -D warnings && cargo test` (repo root) and `npm run build && npm test` (`frontend/`). Browser smoke test for UI changes (console must be clean).

## Review before every PR

Before opening a pull request for a branch, run these review skills and fold any fixes into the branch:
- `ponytail-review` — over-engineering scan (delete/stdlib/native/yagni/shrink).
- `code-simplification` — reduce complexity without changing behavior.
- `code-review-and-quality` — correctness, security, maintainability.
- `performance-optimization` — measure and fix measurable bottlenecks (skip if nothing to measure).
- `finishing-a-development-branch` — branch completion: verify tests, present merge options.

## Code standards

### No noise comments

Never add noise comments. A comment earns its place only when it explains something the code does not already say.

- **Cut** comments that restate the code: `// increment count`, `/// Create a user`, `/// Build the X backend` above `build_x_backend()`.
- **Cut** `/// X payload.`-style doc lines that just repeat the struct/function name.
- **Keep** a comment only when it carries non-obvious *why* or intent the code can't express: return-meaning on an opaque type, data-scoping/shape notes, security rationale, "mirrors Go's X" references, an edge-case reason, a batch-vs-N+1 query rationale.
- **Keep** module-level (`//!`) context and section separators where they aid navigation — but terse, not prose.

Rule of thumb: if deleting the comment loses no understanding, delete it. Write code that needs no comment; comment only the part that isn't obvious.
