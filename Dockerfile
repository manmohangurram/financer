# syntax=docker/dockerfile:1.4
# Single container: the Rust server serves both the API and the built Vue app
# (no nginx). Build stages run on BUILDPLATFORM (native speed, no QEMU) and
# cross-compile to TARGETARCH via rustup target + a cross-gcc from apt.
# cargo-chef caches the dependency build: only the app crate recompiles when
# source changes. Runtime is Debian (glibc), matching the cross-gcc toolchain.

FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY frontend/ ./
# Same-origin by default; an absolute API base can be injected at runtime via
# the FINANCER_DOMAIN_URL setting (not baked at build).
RUN VITE_API_URL= npm run build

# We only pay the cargo-chef install cost once (cached from the second build).
# Pinned to the same Rust as local dev (rustc 1.97.1).
FROM --platform=$BUILDPLATFORM rust:1.97.1 AS chef
RUN cargo install --locked cargo-chef
WORKDIR /app

# Planner: snapshot manifests + full source → recipe (cache key = Cargo.toml/lock).
FROM --platform=$BUILDPLATFORM chef AS planner
COPY . .
RUN cargo chef prepare --recipe-path recipe.json

# Builder: build dependencies once (cached layer), then the app crate.
FROM --platform=$BUILDPLATFORM chef AS builder
ARG TARGETARCH
# amd64 builds natively (glibc). Cross-compiles (e.g. arm64) add the target
# triple and a cross-gcc for the final link (ring/rustls need a C linker).
RUN case ${TARGETARCH} in \
        arm64) rustup target add aarch64-unknown-linux-gnu && \
               apt-get update && apt-get install -y --no-install-recommends gcc-aarch64-linux-gnu libc6-dev-arm64-cross && \
               rm -rf /var/lib/apt/lists/*;; \
    esac
COPY --from=planner /app/recipe.json recipe.json
RUN case ${TARGETARCH} in \
        arm64) CC_aarch64_unknown_linux_gnu=aarch64-linux-gnu-gcc \
               CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-linux-gnu-gcc \
               cargo chef cook --release --target aarch64-unknown-linux-gnu --recipe-path recipe.json;; \
        *)     cargo chef cook --release --recipe-path recipe.json;; \
    esac
COPY . .
RUN mkdir -p /out && \
    case ${TARGETARCH} in \
        arm64) CC_aarch64_unknown_linux_gnu=aarch64-linux-gnu-gcc \
               CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-linux-gnu-gcc \
               cargo build --release --locked -p financer --target aarch64-unknown-linux-gnu && \
               cp target/aarch64-unknown-linux-gnu/release/financer /out/financer;; \
        *)     cargo build --release --locked -p financer && \
               cp target/release/financer /out/financer;; \
    esac

FROM debian:trixie-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=frontend /app/frontend/dist ./frontend/dist
COPY --from=builder /out/financer ./financer
# Configuration is entirely from environment variables (see README.md);
# there is no config file. SurrealDB is a separate server (see
# docker-compose.yml). The schema is applied at boot from the binary.
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/financer"]
