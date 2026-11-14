# Deployment Guide

How this app is built and deployed: the Rust binary serves both the API and the built frontend (single container, no nginx). This doc covers the build pipeline, how frontend↔backend link at build/runtime, the arm64 image build, and the GitHub Actions release flow.

Replace `<PROJECT>` / `<owner>` / `<image>` with your project's values. Apply the same pattern to any Rust + Vue app with this layout.

---

## 1. How frontend and backend link

The app ships as **one artifact** — the server statically serves the built frontend.

### At build time (Docker)

The `Dockerfile` builds the frontend first, then the Rust binary, then copies the built assets into the runtime image:

```
frontend stage:  node → npm ci → npm run build → frontend/dist/
builder stage:   rust + cargo-chef → cargo build --release
runtime stage:   debian-slim ← COPY frontend/dist + app binary
```

`RUN VITE_API_URL= npm run build` — the frontend is built with an **empty** API base, meaning it calls the API on the **same origin** it's served from. No hardcoded host.

### At runtime (env)

| Env var | Purpose |
|---|---|
| `<PROJECT>_DOMAIN_URL` | External URL the app is served at. When set, the server injects `<script>window.__API_BASE__=...` into the served `index.html`, and the frontend uses it as the API base. **Empty/omitted → same-origin** (default). |
| `<PROJECT>_STATIC_DIR` | Where the built frontend lives (default `frontend/dist`). Omit → API-only mode (dev). |

Frontend resolution order (`frontend/src/lib/api/client.ts`):

1. `window.__API_BASE__` (injected at runtime by the server)
2. `VITE_API_URL` env (dev builds)
3. same-origin

**So**: in production, leave `DOMAIN_URL` empty for same-origin, or set it to the public URL if the frontend is served from a different host than the API.

### Dev mode (no Docker)

```bash
# Terminal 1 — backend (API on :8080)
FINANCER_JWT_SECRET=devsecret cargo run

# Terminal 2 — frontend (vite dev, calls the API directly)
cd frontend
VITE_API_URL=http://localhost:8080 npm run dev
```

---

## 2. Files required in the container

The runtime image copies these from the build:

| Path in image | From | Purpose |
|---|---|---|
| `/app/<binary>` | builder stage | the compiled binary |
| `/app/frontend/dist` | frontend stage | the built SPA |
| `/app/config` | repo `config/` | app config templates |
| `/data` (volume) | — | runtime data: DB, uploads |

These must be present at build context (the workflow checks out the whole repo, so they are). Do not gitignore `config/`.

---

## 3. Required repo files for CI/CD

| File | Purpose |
|---|---|
| `.github/workflows/ci.yml` | PR gate: cargo build/clippy/test + frontend type-check/vitest/build |
| `.github/workflows/docker-publish.yml` | Release: build + push the arm64 image to GHCR on tag |
| `.github/workflows/prune-packages.yml` | Weekly: delete old image versions from GHCR (keeps the newest 5 releases) |
| `Dockerfile` | Multi-stage build (frontend + cargo-chef + runtime) |
| `.dockerignore` | Exclude `target/`, `data/`, `node_modules/`, `.git` from build context |
| `config/` | Runtime asset copied into the image |

---

## 4. Building the image (arm64)

### How it works

CI builds `linux/arm64` natively on an arm runner — no QEMU. The Dockerfile also handles `TARGETARCH`, so a cross-build works with the toolchain below:

- **arm64 on an arm runner** → native build
- **arm64 under emulation** → `rustup target add aarch64-unknown-linux-gnu` + apt cross-gcc (`gcc-aarch64-linux-gnu`, `libc6-dev-arm64-cross`) → `cargo build --release --target aarch64-unknown-linux-gnu`

Why a cross-C toolchain when cross-compiling: ring/rustls (reqwest) need a C linker for the final binary on arm64.

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
3. Reuses the Actions build cache (`type=gha`) so cargo-chef dep layers persist across releases — only the app crate recompiles

No repo variable to configure; every `v*` tag ships the image.

### Pruning the registry

`prune-packages.yml` runs weekly (Sunday 03:00 UTC) and keeps the newest 5 release images, deleting everything older. Run it by hand from the Actions tab before a release, optionally with `keep` (how many releases to keep) and `dry_run` (report only).

### Locally with buildx

```bash
# arm64, loaded into the local docker (needs an arm host to run it)
docker buildx build --platform linux/arm64 -t ghcr.io/<owner>/<image>:test --load .
```

If you get `no matching manifest for linux/arm64`, run `docker buildx create --use` once to enable the builder.

### Manual trigger (no tag)

The workflow also supports `workflow_dispatch` — run it from the Actions tab on any branch. Note: manual runs tag the image with branch/sha (no `latest`); `latest` is only applied on tag pushes.

---

## 5. Release workflow (end to end)

1. Merge feature branches to the default branch (each PR runs `ci.yml`)
2. Tag a release: `git tag v1.0.0 && git push origin v1.0.0`
3. `docker-publish.yml` builds the arm64 image, pushes `ghcr.io/<owner>/<image>:v1.0.0` and `:latest`
4. Deploy:

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
| `FINANCER_DOMAIN_URL` | no | set only if the SPA is served from a different host |
| `FINANCER_TIMEZONE` | no | IANA zone (default `Asia/Kolkata`) |
| `FINANCER_SQLITE_PATH` | no | SQLite file (default `/data/financer.db`) |
| `FINANCER_SECRET_KEY` | for the Inbox | 64 hex chars (`openssl rand -hex 32`); encrypts mailbox passwords, and must stay stable — changing it makes stored passwords unreadable |
| `FINANCER_MAIL_POLL_SECS` | no | background mail poll interval, seconds (default `300`, min `30`) |

---

## 7. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Frontend calls `localhost:8080` instead of the site URL | `VITE_API_URL` baked into a dev build; rebuild with `VITE_API_URL=`. In Docker the frontend is built with empty base → same-origin. |
| 404 on `/api/...` after deploy | Wrong `DOMAIN_URL` injection; or unknown API path (SPA returns 404 JSON for unknown `/api/*`). |
| `no matching manifest for linux/arm64` locally | buildx builder missing: `docker buildx create --use`. |
| arm64 build fails on ring/cc | Cross-gcc missing — the Dockerfile installs `gcc-aarch64-linux-gnu` + `libc6-dev-arm64-cross` automatically. |
| Full dependency rebuild on every release | The Actions cache (`type=gha`) expires after 7 days of inactivity; a tag push after that gap rebuilds the deps once. |
| Docker image tag is branch/sha not version | Manual `workflow_dispatch` run; `latest`/version tags apply only on `v*` tag pushes. |
