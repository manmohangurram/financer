# Single container: the Go server serves both the API and the built Vue app
# (no nginx). Built for arm64 (Raspberry Pi) and amd64 via buildx TARGETARCH.
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# modernc.org/sqlite is pure Go, so CGO_ENABLED=0 yields a static binary.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/financer .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /out/financer ./financer
COPY --from=frontend /app/frontend/dist ./frontend/dist
ENV FINANCER_ADDR=:8080 FINANCER_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/financer"]
