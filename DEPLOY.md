# Deployment Guide

How this app is built and deployed: the Go binary serves both the API and the built frontend (single container, no nginx). This doc covers the build pipeline, how frontend↔backend link at build/runtime, the arm64 image build, and the GitHub Actions release flow.

---

## 1. How frontend and backend link

The app ships as **one artifact** — the server statically serves the built frontend.

### At build time (Docker)

The `Dockerfile` builds the frontend first, then the Go binary, then copies the built assets into the runtime image:

```
frontend stage:  node → npm ci → npm run build → frontend/dist/
builder stage:   golang:1.27 → go mod download → CGO_ENABLED=0 go build ./cmd/financer
runtime stage:   alpine + ca-certificates ← COPY frontend/dist + financer binary
```

`RUN VITE_API_URL= npm run build` — the frontend is built with an **empty** API base, meaning it calls the API on the **same origin** it's served from. No hardcoded host.

The binary is self-contained: SQL migrations are embedded (`go:embed db/migrations/*.up.sql`) and the IANA timezone database is compiled in (`time/tzdata`).

### At runtime (env)

| Env var | Purpose |
|---|---|
| `FINANCER_DOMAIN_URL` | External URL the app is served at. When set, the server injects `<script>window.__API_BASE__=...` into the served `index.html`, and the frontend uses it as the API base. **Empty/omitted → same-origin** (default). |
| `FINANCER_STATIC_DIR` | Where the built frontend lives (image default `/app/frontend/dist`). Omit → API-only mode (dev). |

Frontend resolution order (`frontend/src/lib/api/client.ts`):

1. `window.__API_BASE__` (injected at runtime by the server)
2. `VITE_API_URL` env (dev builds)
3. same-origin

**So**: in production, leave `DOMAIN_URL` empty for same-origin, or set it to the public URL if the frontend is served from a different host than the API.

### Dev mode (no Docker)

```bash
# Terminal 1 — backend (API on :8080)
FINANCER_JWT_SECRET=devsecret go run ./cmd/financer

# Terminal 2 — frontend (vite dev, calls the API directly)
cd frontend
VITE_API_URL=http://localhost:8080 npm run dev
```

---

## 2. Files required in the container

The runtime image copies these from the build:

| Path in image | From | Purpose |
|---|---|---|
| `/app/financer` | builder stage | the compiled Go binary |
| `/app/frontend/dist` | frontend stage | the built SPA |
| `/data` (volume) | — | runtime data: DB, avatars |

There is no config directory — configuration is env-only. Migrations and the timezone database live inside the binary.

---

## 3. Required repo files for CI/CD

| File | Purpose |
|---|---|
| `.github/workflows/ci.yml` | PR gate: gofmt/vet/build/test/govulncheck + frontend type-check/vitest/build |
| `.github/workflows/docker-publish.yml` | Release: build + push the arm64 image to GHCR when a `v*` tag is created |
| `.github/workflows/prune-packages.yml` | Weekly: delete old image versions from GHCR (keeps the newest 5 releases) |
| `Dockerfile` | Multi-stage build (frontend + Go + runtime) |
| `.dockerignore` | Exclude `target/`, `data/`, `node_modules/`, `.git` from build context |
| `go.mod` / `go.sum` | Pinned Go toolchain version and module checksums |
| `db/migrations/` | Embedded single-baseline schema — applied at boot on a fresh database |

---

## 4. Building the image (arm64)

### How it works

The image is **linux/arm64 only**. The build is a **pure-Go cross-build**: `CGO_ENABLED=0` with `GOARCH=arm64`. The SQLite driver (`glebarez/sqlite`, backed by `modernc.org/sqlite`) is pure Go, so there is **no cross-compiler, no `gcc-aarch64-linux-gnu`, and no target-specific linker**.

- **arm64 on an arm runner** → native build (what CI does)
- **any other build host** → the Dockerfile cross-compiles the arm64 binary; the build fails fast if a different `TARGETARCH` is requested

The final image is Alpine with `ca-certificates` (outbound IMAP TLS needs system roots). Timezones are embedded in the binary, so `tzdata` is not installed.

### Via GitHub Actions (recommended)

Tag a release:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The workflow (`docker-publish.yml`) builds and pushes one image, on an arm runner:

| Platform | Runner |
|---|---|
| `linux/arm64` | `ubuntu-24.04-arm` |

| Tag | Storage |
|---|---|
| `:v1.0.0`, `:latest` | SQLite |

1. Builds `linux/arm64` on the native arm runner
2. Pushes and tags `ghcr.io/<owner>/<image>` as above
3. Reuses the registry build cache (`type=registry`) so unchanged layers — including `go mod download` — persist across releases

No repo variable to configure; every `v*` tag ships the image.

### Pruning the registry

`prune-packages.yml` runs weekly (Sunday 03:00 UTC) and keeps the newest 5 release images, deleting everything older. Run it by hand from the Actions tab before a release, optionally with `keep` (how many releases to keep) and `dry_run` (report only).

### Locally with buildx

```bash
# arm64, loaded into the local docker (needs an arm host to run it)
docker buildx build --platform linux/arm64 -t ghcr.io/<owner>/<image>:test --load .
```

If you get `no matching manifest for linux/arm64`, run `docker buildx create --use` once to enable the builder. Building with `--platform linux/amd64` is rejected on purpose — this project ships arm64 only.

### Trigger

The image is built **only** when a `v*` tag is created (`git push origin v1.0.0` or a GitHub Release that creates the tag). Pull requests and branch pushes never build or publish an image.

---

## 5. Release workflow (end to end)

1. Merge feature branches to the default branch (each PR runs `ci.yml`)
2. Tag a release: `git tag v1.0.0 && git push origin v1.0.0`
3. `docker-publish.yml` builds the arm64 image, pushes `ghcr.io/<owner>/<image>:v1.0.0` and `:latest`
4. Deploy on an **arm64 host** (the image has no amd64 variant):

```bash
docker run -d \
  --name <app> \
  -p 8080:8080 \
  -v <app>-data:/data \
  -e FINANCER_JWT_SECRET="$(openssl rand -hex 32)" \
  ghcr.io/<owner>/<image>:latest
```

### On ARM64 hardware (e.g. Raspberry Pi)

```bash
docker run -d \
  --name <app> \
  -p 8080:8080 \
  -v <app>-data:/data \
  -e FINANCER_JWT_SECRET="$(openssl rand -hex 32)" \
  ghcr.io/<owner>/<image>:latest   # arm64 image
```

No per-arch tag needed — the published image **is** the arm64 one.

---

## 6. Required env vars in production

Configuration is env-only (no config file). See the table in `README.md`.

| Env var | Required | Notes |
|---|---|---|
| `FINANCER_JWT_SECRET` | **yes** | token signing secret; the server refuses to start without it |
| `FINANCER_ADDR` | no | listen address (default `0.0.0.0:8080`) |
| `FINANCER_DATA_DIR` | no | runtime data root (default `/data`, bind a volume) |
| `FINANCER_STATIC_DIR` | no | built SPA location (image default `/app/frontend/dist`) |
| `FINANCER_DOMAIN_URL` | no | set only if the SPA is served from a different host |
| `FINANCER_TIMEZONE` | no | IANA zone (default `Asia/Kolkata`); resolved from embedded tzdata |
| `FINANCER_SQLITE_PATH` | no | SQLite file (default `/data/financer.db`); WAL mode is enabled at boot |
| `FINANCER_SECRET_KEY` | for the Inbox | 64 hex chars (`openssl rand -hex 32`); encrypts mailbox passwords, and must stay stable — changing it makes stored passwords unreadable |
| `FINANCER_MAIL_POLL_SECS` | no | background mail poll interval, seconds (default `300`, min `30`) |
| `FINANCER_LOG_LEVEL` | no | `debug`, `info` (default), `warn`, `error` |
| `FINANCER_LOG_FILE` | no | append logs to a file instead of stderr; rotates at 10 MB and keeps at most 3 files (`financer.log`, `.1`, `.2` — the oldest is deleted). With the default volume use `/data/financer.log`, otherwise read `docker logs` |
| `FINANCER_ALLOW_INSECURE_HTTP` | no | set to `1` only behind a trusted proxy that terminates TLS; otherwise mailbox/auth endpoints require HTTPS (loopback is exempt) |

---

## 7. Resetting the database (pre-0.6 → 0.6+)

The schema is a **single baseline**. Databases created by the Rust backend or
by 0.5.x are not migrated: delete the database and start fresh.

```bash
# stop the app first
docker compose down

# bind-mounted data directory
rm -f ./data/financer.db ./data/financer.db-wal ./data/financer.db-shm

# or, for a named volume (this also removes avatars and log files)
docker volume rm <project>_data

docker compose up -d
```

Back up the old file first if you may need it. The new container applies
`000001_initial_schema` on first start and creates `_financer_applied_migrations`.

## 8. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Frontend calls `localhost:8080` instead of the site URL | `VITE_API_URL` baked into a dev build; rebuild with `VITE_API_URL=`. In Docker the frontend is built with empty base → same-origin. |
| 404 on `/api/...` after deploy | Wrong `DOMAIN_URL` injection; or unknown API path (SPA returns 404 JSON for unknown `/api/*`). |
| `no matching manifest for linux/arm64` locally | buildx builder missing: `docker buildx create --use`. |
| `cgo: C compiler not found` while building | Something re-enabled CGO; the SQLite stack is pure Go — keep `CGO_ENABLED=0`. |
| Wrong timestamps / `UTC` instead of the configured zone | The zone name is invalid; the IANA database is embedded, so only the name can be wrong. |
| IMAP polling fails with `x509: certificate signed by unknown authority` | The runtime image is missing `ca-certificates` (the provided Dockerfile installs it). |
| Full dependency rebuild on every release | The registry build cache expires after 7 days of inactivity; a tag push after that gap rebuilds the deps once. |
