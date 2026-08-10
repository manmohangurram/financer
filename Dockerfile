# syntax=docker/dockerfile:1.4
# Single container: the Rust server serves both the API and the built Vue app
# (no nginx). No CGO (sqlx's bundled SQLite + rustls TLS are pure Rust), so
# build stages run on BUILDPLATFORM (native speed, no QEMU) and cross-compile
# to TARGETARCH via rustup target. cargo-chef caches the dependency build:
# only the app crate recompiles when source changes. The prebuilt
# lukemathwalker/cargo-chef image (Rust + cargo-chef) avoids the per-build
# `cargo install`.
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY frontend/ ./
# Same-origin by default; an absolute API base can be injected at runtime via
# the FINANCER_DOMAIN_URL env (read by the Rust server, not baked at build).
RUN VITE_API_URL= npm run build

FROM --platform=$BUILDPLATFORM lukemathwalker/cargo-chef:latest-rust-1.88-alpine3.21 AS chef
WORKDIR /app

# Planner: snapshot manifests + full source → recipe (cache key = Cargo.toml/lock).
FROM --platform=$BUILDPLATFORM chef AS planner
COPY . .
RUN cargo chef prepare --recipe-path recipe.json

# Builder: build dependencies once (cached layer), then the app crate.
FROM --platform=$BUILDPLATFORM chef AS builder
ARG TARGETARCH
# rust:*-alpine targets musl natively, so amd64 needs no --target. Only
# cross-compiles (e.g. arm64) add the target triple (Docker arch → rustc).
RUN case ${TARGETARCH} in arm64) rustup target add aarch64-unknown-linux-musl;; esac
COPY --from=planner /app/recipe.json recipe.json
RUN case ${TARGETARCH} in \
        arm64) cargo chef cook --release --target aarch64-unknown-linux-musl --recipe-path recipe.json;; \
        *)     cargo chef cook --release --recipe-path recipe.json;; \
    esac
COPY . .
RUN mkdir -p /out && \
    case ${TARGETARCH} in \
        arm64) cargo build --release --locked -p financer --target aarch64-unknown-linux-musl && \
               cp target/aarch64-unknown-linux-musl/release/financer /out/financer;; \
        *)     cargo build --release --locked -p financer && \
               cp target/release/financer /out/financer;; \
    esac

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=frontend /app/frontend/dist ./frontend/dist
COPY --from=builder /out/financer ./financer
# Migrations are read from disk at startup (Rust mirrors Go's runner).
COPY db/migrations ./db/migrations
# Yahoo endpoint config (bases/chart/search templates).
COPY config ./config
ENV FINANCER_ADDR=0.0.0.0:8080 FINANCER_DATA_DIR=/data FINANCER_DOMAIN_URL= FINANCER_STATIC_DIR=/app/frontend/dist
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/financer"]
