# syntax=docker/dockerfile:1

ARG SING_BOX_IMAGE=ghcr.io/sagernet/sing-box:latest

# -------- Stage 1: build the frontend static export --------
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
# Install deps from the lockfile first for better layer caching.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# output:'export' produces /app/frontend/out
RUN npm run build

# -------- Stage 2: build the Go single binary (frontend embedded) --------
FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Embed the freshly built frontend export. This populates internal/web/dist,
# which embed.go pulls in via //go:embed all:dist.
COPY --from=frontend /app/frontend/out/ ./internal/web/dist/
# CGO disabled: the SQLite driver (glebarez) is pure Go, so we get a fully
# static binary that runs on a bare alpine image.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/sing-box-admin ./cmd/server

# -------- sing-box binary (official multi-arch image) --------
FROM ${SING_BOX_IMAGE} AS singbox

# -------- Stage 3: runtime --------
FROM alpine:3.20 AS runtime
# ca-certificates for outbound TLS; procps provides `pgrep -x` used to detect
# whether sing-box is running (busybox's pgrep lacks -x).
RUN apk add --no-cache ca-certificates procps
COPY --from=singbox /usr/local/bin/sing-box /usr/local/bin/sing-box
COPY --from=backend /out/sing-box-admin /usr/local/bin/sing-box-admin
ENV PORT=8080 \
    DB_PATH=/data/sing-box-admin.db
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sing-box-admin"]