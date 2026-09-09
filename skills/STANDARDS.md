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

### Rust style: references & strings

Follow Rust's type-driven convention — **use the method the receiver's type requires; never reach for `to_string()`/`clone()` when a borrow works.** Pick by the type, not by habit:

| Value | Pass to a call taking `&str` | To an owned `String` |
|---|---|---|
| `s: &str` | `s` (already a `&str`) | `s.to_string()` |
| `s: String` | `s.as_str()` | `s` |
| `s: Option<String>` | `s.as_deref()` | `s` |
| `&String` | `&s` or `s.as_str()` (prefer `as_str()`) | `s.clone()` |
| enum (`AccountType`, `TransactionType`, `InvestmentType`) | — (no borrow method) | `x.to_string()` only |

Key rules:
- **`String` → `&str`**: use `.as_str()`, not `&s` (moves nothing, clearer).
- **`Option<String>` → `Option<&str>`**: `.as_deref()`; never `.as_str()` on an `Option`, never `.map(|s| s.as_str())`.
- **`&str` already**: pass it as-is; don't add `.to_string()` or `&` around a `&str`.
- **Only `to_string()` when the target needs an owned `String`** (e.g. enum → string, `format!`). For `&str`/`String` that already have what you need, `to_string()` is an avoidable allocation.
- **`&String` params**: prefer accepting `&str` in the signature; if you have a `&String`, pass `.as_str()`.
- **No `.clone()` to satisfy a borrow** — if the callee takes `&str`, pass `.as_str()`, don't clone a `String`.

Real example — SurrealDB query binds:
```rust
.bind(("name", t.name.as_str()))        // String -> &str
.bind(("clean", t.clean_name.as_deref())) // Option<String> -> Option<&str>
.bind(("acc", rid("account", &t.account_id))) // borrow a String into a fn that takes it by ref
.bind(("type", t.transaction_type.to_string())) // enum: only path
```

**SurrealDB bind types** (surreal value): `&str` and `Option<&str>` both serialize; `&String` and `Option<String>` do not bind directly. That's why the `.as_str()`/`.as_deref()` forms are required there.

### Rust style: general

- **Ownership by default; borrow only when needed.** Prefer passing `&str`/`&[T]`/`&Struct` over owned values or `Arc` clones when the callee only reads.
- **`Arc` only for genuine sharing** (services, concurrent state), never as a workaround to avoid a lifetime.
- **Avoid `.clone()` on hot/read paths** unless the value is genuinely moved. Bind-by-borrow (`as_str`, `as_deref`) over clone.
- **Prefixed enum->string via `strum`** (`Display`/`as_str` if derived), not hand `match`.
- **`Option` unwraps**: prefer `let Some(x) = ... else { return Err(...) }` / `?` over `.unwrap()` in prod code; `.unwrap()` only in tests.
- **Builder setters** for optional service deps (`with_rule`, `with_transfer_rule`) instead of many-arg constructors.
- Let `cargo clippy -- -D warnings` be the final arbiter; where clippy and a local preference differ, follow clippy.
