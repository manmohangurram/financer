# Changelog

All notable changes to Financer. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions are [SemVer](https://semver.org/spec/v2.0.0.html) with an `-alpha` suffix while the project is pre-1.0.

Entries are grouped by theme. An entry starts at the first tag it appeared in; earlier history lives in the
git log (`git log --oneline v0.2.0-alpha..v0.3.0-alpha`).

## [0.3.0-alpha] — 2026-11-14

This release is about one new feature — mail in, transactions out — and one big deletion: the SurrealDB
backend is gone, and the published image is arm64 only.

### Added

- **Inbox: email → transactions.** Link an IMAP mailbox, poll it on a timer, and turn bank-statement mail
  into transactions.
  - IMAP ingest with implicit TLS, a required sender allowlist, and the mailbox password encrypted at rest
    with `FINANCER_SECRET_KEY`. (#150, #154)
  - Background poll (`FINANCER_MAIL_POLL_SECS`, default 300s) plus on-demand polling, with per-mailbox
    error reporting. (#154)
  - Disposition rules per sender — `IGNORE`, `REVIEW`, `AUTO` — with the first match winning; `AUTO` rows
    become transactions during the poll. (#155)
  - Approve / reject from the Inbox; approving creates the transaction through the normal service path, so
    the rule engine, balances and categories apply. (#155)
  - Inbox UI: link a mailbox, review what needs a person, manage disposition rules, and let the Inbox
    suggest an `AUTO` rule for senders that keep being approved. (#157)
  - Account resolution by the account/card ending digits, with the account picked by hand when they do not
    resolve. (#148, #157)
- **Statement import.** CSV, XLSX and PDF statements are parsed on the backend: upload → map columns →
  commit, for transactions and for investments. (#129, #130, #133, #136, #137)
- **Sidebar** grouped into sections, showing account balances. (#146)
- **Spending** opens on a range that has data. (#139)
- **Docs:** a full Inbox section and the mail env vars. (#163)

### Changed

- **Rust 1.100 everywhere** — `rust-version`, the CI toolchain and the Docker build image are pinned, so a
  new clippy release can no longer break a green `main`. (#151, #158)
- **Dependencies:** sqlx 0.9, utoipa 6, utoipa-axum 0.3, utoipa-swagger-ui 10, calamine 0.38, plus a
  lockfile refresh. `jsonwebtoken` stays on 9 (12 needs a crypto-provider feature for HS256 only). (#158)
- **Docker:** the image is built on a native arm runner and published on tag, cached through the Actions
  cache. (#158)
- Rules preview uses the request date and backfills existing transactions on create/update/run. (#138)

### Fixed

- **Account balances and totals were patched outside the write transaction**, so a crash between the two
  left transactions stored with balances that no longer matched them, and nothing could recompute them.
  They are now recomputed from their own rows inside the same transaction. (#161)
- **Login had no attempt limit.** Five failed attempts per email in 15 minutes now answer `429` before the
  password is hashed; a successful login clears the counter. (#161)
- Bank-statement headers are recognised when guessing import columns. (#128)
- `rustls` bumped for RUSTSEC-2026-0285. (#145)

### Performance

- Transaction list dropped a `COUNT(*) OVER ()` column that made SQLite sort the user's whole history for
  every page: 300 ms → 30 ms at 500k transactions, with the total counted separately. (#159)
- `transfer_links` gained indexes on both sides of the join used by the list and spending queries. (#159)

### Removed

- **The SurrealDB backend**, entirely: the `src/repo/surreal/` implementations, the `surreal` cargo
  feature, the `surrealdb` dependency, the `-surreal` image variant, and the compose file that bundled a
  SurrealDB. SQLite is the only storage. (#158)
- **The `linux/amd64` image.** The published image is `linux/arm64` only. (#158)
- **`docker-compose.minimal.yml`** — `docker-compose.yml` is now that single-container SQLite setup. (#158)
- Unused dependencies and dead code (`utoipa-axum`, `AccountRepo::update_balance`/`apply_totals`,
  `TransactionRepo::get_by_id_batch`, the service-side balance delta bookkeeping). (#158, #161)
- CI: the floating `stable` toolchain, and `cargo install cargo-audit` compiling from source. (#158, #159)

### Upgrading from 0.2.0-alpha

- **SurrealDB users must migrate their data to SQLite.** `FINANCER_DATABASE` and `FINANCER_SURREAL_*` are
  ignored now; there is no compatibility path and no image that reads a SurrealDB instance.
- **x86 hosts can no longer pull the image.** It is arm64 only.
- New env vars for the Inbox: `FINANCER_SECRET_KEY` (64 hex chars, `openssl rand -hex 32`) and optionally
  `FINANCER_MAIL_POLL_SECS`. Keep the secret key stable — changing it makes stored mailbox passwords
  unreadable.
- Mailbox routes require HTTPS (loopback exempt); behind a proxy, forward `X-Forwarded-Proto: https`.
