# ── Stage 0: Build Next.js Web UI ─────────────────────────────────────────────
FROM node:22-alpine AS web-builder

WORKDIR /web
COPY web/package.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# ── Stage 1: Build Go Backend ─────────────────────────────────────────────────
# CGO is required: tree-sitter grammars (via langpeanut_local dependency) use C.
# We also need go-sqlite3 which requires CGO.
# langpeanut_local is supplied as a named additional build context (see
# docker-compose.yml `additional_contexts`); when building with plain
# `docker build`, pass --build-context langpeanut_local=../langpeanut_local.
FROM golang:1.26-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc libc6-dev git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY --from=langpeanut_local . /langpeanut_local
COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/langpeanut-cloud ./cmd/server

# ── Stage 2: Runtime (server) ─────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git wget \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -r -u 1001 -g 0 langpeanut

WORKDIR /app
COPY --from=builder /out/langpeanut-cloud /app/langpeanut-cloud
COPY --from=web-builder /web/out /app/web/out

RUN chown -R langpeanut /app
USER langpeanut

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/langpeanut-cloud"]
