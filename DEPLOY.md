# Deployment Guide

How this app is built and deployed: the Rust binary serves both the API and the built frontend (single container, no nginx). This doc covers the build pipeline, how frontend↔backend link at build/runtime, multi-arch builds, and the GitHub Actions release flow.

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
<PROJECT>_JWT_SECRET=devsecret cargo run

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
| `.github/workflows/docker-publish.yml` | Release: build + push multi-arch image to GHCR on tag |
| `Dockerfile` | Multi-stage build (frontend + cargo-chef + runtime) |
| `.dockerignore` | Exclude `target/`, `data/`, `node_modules/`, `.git` from build context |
| `config/` | Runtime asset copied into the image |

---

## 4. Building for different architectures

### How it works

The Dockerfile is **multi-arch**: build stages run on the host arch (fast, no QEMU) and cross-compile to `TARGETARCH`:

- **amd64** → native build, no cross toolchain
- **arm64** → `rustup target add aarch64-unknown-linux-gnu` + apt cross-gcc (`gcc-aarch64-linux-gnu`, `libc6-dev-arm64-cross`) → `cargo build --release --target aarch64-unknown-linux-gnu`

Why a cross-C toolchain: ring/rustls (used by the SurrealDB HTTP client + reqwest) need a C linker for the final binary on arm64.

### Via GitHub Actions (recommended)

Set the **repo variable** `DOCKER_PLATFORMS` (Settings → Secrets and variables → Actions → Variables):

| Value | Builds |
|---|---|
| `linux/amd64` (default) | x86_64 hosts |
| `linux/arm64` | ARM64 — Raspberry Pi (64-bit OS), Apple Silicon |
| `linux/arm64,linux/amd64` | **both** — one multi-arch manifest in GHCR |

Then tag a release — the workflow builds and pushes both:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The workflow (`docker-publish.yml`):
1. Sets up QEMU (only when arm64 is in `DOCKER_PLATFORMS`) for the runtime-stage emulation
2. Sets up buildx, logs into GHCR
3. Builds the platforms via buildx → one manifest
4. Pushes to `ghcr.io/<owner>/<image>` tagged with the git tag + `latest`
5. Reuses the **registry-backed build cache** (`:buildcache` image) so cargo-chef dep layers persist across releases — only the app crate recompiles

### Locally with buildx

```bash
# both arches
docker buildx build --platform linux/arm64,linux/amd64 -t ghcr.io/<owner>/<image>:test .

# just arm64 (Pi)
docker buildx build --platform linux/arm64 -t ghcr.io/<owner>/<image>:arm64 .

# just amd64
docker buildx build --platform linux/amd64 -t ghcr.io/<owner>/<image>:amd64 .
```

If you get `no matching manifest for linux/arm64`, run `docker buildx create --use` once to enable the builder.

### Manual trigger (no tag)

The workflow also supports `workflow_dispatch` — run it from the Actions tab on any branch. Note: manual runs tag the image with branch/sha (no `latest`); `latest` is only applied on tag pushes.

---

## 5. Release workflow (end to end)

1. Merge feature branches to the default branch (each PR runs `ci.yml`)
2. Tag a release: `git tag v1.0.0 && git push origin v1.0.0`
3. `docker-publish.yml` builds all configured platforms, pushes `ghcr.io/<owner>/<image>:v1.0.0` and `:latest`
4. Deploy:

```bash
docker run -d \
  --name <app> \
  -p 8080:8080 \
  -v <app>-data:/data \
  -e <PROJECT>_JWT_SECRET="$(openssl rand -hex 32)" \
  ghcr.io/<owner>/<image>:latest
```

### On ARM64 hardware (e.g. Raspberry Pi)

```bash
docker run -d \
  --name <app> \
  -p 8080:8080 \
  -v <app>-data:/data \
  -e <PROJECT>_JWT_SECRET="$(openssl rand -hex 32)" \
  ghcr.io/<owner>/<image>:latest   # same tag — manifest picks the arm64 image automatically
```

No per-arch tag needed — the multi-arch manifest resolves the right image on the target.

---

## 6. Required env vars in production

| Variable | Required | Notes |
|---|---|---|
| `<PROJECT>_JWT_SECRET` | **yes** | JWT signing secret — set a strong random value |
| `<PROJECT>_ADDR` | no | listen address (default `0.0.0.0:8080`, set in image) |
| `<PROJECT>_DATA_DIR` | no | runtime data root (default `/data`, set in image; bind a volume) |
| `<PROJECT>_DOMAIN_URL` | no | set only if frontend served from a different host |
| `<PROJECT>_STATIC_DIR` | no | built frontend path (default `/app/frontend/dist`, set in image) |
| `<PROJECT>_SURREAL_URL` | no | SurrealDB server (default `127.0.0.1:8000`) |
| `<PROJECT>_SURREAL_USER` / `<PROJECT>_SURREAL_PASS` | no | SurrealDB root credentials (default `root`/`root`) |
| `<PROJECT>_SURREAL_NS` / `<PROJECT>_SURREAL_DB` | no | SurrealDB namespace/database (default `financer`/`financer`) |

---

## 7. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Frontend calls `localhost:8080` instead of the site URL | `VITE_API_URL` baked into a dev build; rebuild with `VITE_API_URL=`. In Docker the frontend is built with empty base → same-origin. |
| 404 on `/api/...` after deploy | Wrong `DOMAIN_URL` injection; or unknown API path (SPA returns 404 JSON for unknown `/api/*`). |
| `no matching manifest for linux/arm64` locally | buildx builder missing: `docker buildx create --use`. |
| arm64 build fails on ring/cc | Cross-gcc missing — the Dockerfile installs `gcc-aarch64-linux-gnu` + `libc6-dev-arm64-cross` automatically. |
| Full dependency rebuild on every release | Check the `:buildcache` image exists in GHCR; registry cache survives across tags (gha cache does not — 7-day eviction). |
| Docker image tag is branch/sha not version | Manual `workflow_dispatch` run; `latest`/version tags apply only on `v*` tag pushes. |
