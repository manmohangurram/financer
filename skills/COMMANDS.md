# Commands & Verification

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
