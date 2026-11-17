# syntax=docker/dockerfile:1.7
# Single container: the Go server serves both the API and the built Vue app
# (no nginx). The image is ARM64 ONLY. Both build stages run on BUILDPLATFORM
# (native speed, no QEMU) and cross-compile the Go binary; the SQLite driver is
# pure Go (glebarez/modernc), so no C toolchain or target linker is needed.

FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY frontend/ ./
# Same-origin by default; an absolute API base can be injected at runtime via
# the FINANCER_DOMAIN_URL setting (not baked at build).
RUN VITE_API_URL= npm run build

FROM --platform=$BUILDPLATFORM golang:1.27 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=arm64
# Fail fast instead of publishing an image this project does not support.
RUN if [ "${TARGETARCH}" != "arm64" ]; then \
        echo "financer publishes arm64 only (requested ${TARGETARCH})" >&2; exit 1; \
    fi
# CGO_ENABLED=0 keeps the binary static and makes the arm64 cross-build a plain
# GOARCH switch. Migrations are embedded, so the binary is self-contained.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/financer ./cmd/financer

FROM alpine:3.22 AS runtime
# ca-certificates is required for outbound IMAP TLS; the IANA timezone database
# is embedded in the binary (time/tzdata), so tzdata is not needed.
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=frontend /app/frontend/dist ./frontend/dist
COPY --from=builder /out/financer ./financer
# Configuration is entirely from environment variables (see README.md);
# there is no config file. The schema is applied at boot from the binary.
ENV FINANCER_STATIC_DIR=/app/frontend/dist \
    FINANCER_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/financer"]
