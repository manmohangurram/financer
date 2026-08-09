# syntax=docker/dockerfile:1.4
# Single container: the Go server serves both the API and the built Vue app
# (no nginx). Frontend dist and the Go binary are both architecture-neutral to
# build (static files / CGO_ENABLED=0 cross-compile), so both stages run on the
# NATIVE build platform via --platform=$BUILDPLATFORM — npm/vue-tsc and Go run
# at full speed with no QEMU emulation. The only emulated step is a tiny apk.
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY frontend/ ./
# Same-origin by default; an absolute API base can be injected at runtime via
# the FINANCER_DOMAIN_URL env (read by the Go server, not baked at build).
RUN VITE_API_URL= npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend
# Declare the buildx automatic platform args so GOARCH/GOOS resolve to the
# TARGET platform (undeclared, ${TARGETARCH} is empty and cross-compiles x86).
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# modernc.org/sqlite is pure Go, so CGO_ENABLED=0 cross-compiles natively.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/financer .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=frontend /app/frontend/dist ./frontend/dist
COPY --from=backend /out/financer ./financer
ENV FINANCER_ADDR=:8080 FINANCER_DATA_DIR=/data FINANCER_DOMAIN_URL=
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/financer"]
