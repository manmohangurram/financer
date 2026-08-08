# Single container: the Go server serves both the API and the built Vue app
# (no nginx). Performance: the frontend dist is pre-built natively in CI (npm
# + vue-tsc is slow under arm64 QEMU), and Go cross-compiles on the native
# build platform — so the only emulated step is a tiny alpine apk add.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# modernc.org/sqlite is pure Go, so CGO_ENABLED=0 cross-compiles without QEMU.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/financer .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /out/financer ./financer
COPY --from=backend /src/frontend/dist ./frontend/dist
ENV FINANCER_ADDR=:8080 FINANCER_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/financer"]
