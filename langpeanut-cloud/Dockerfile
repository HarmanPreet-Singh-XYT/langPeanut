# ── Stage 0: Build Next.js Web UI ─────────────────────────────────────────────
FROM node:24-alpine AS web-builder

WORKDIR /web
COPY web/package.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# ── Stage 1: Build Go Backend & Runner ────────────────────────────────────────
# CGO is required: tree-sitter grammars (via langpeanut_local dependency) use C.
# We also need go-sqlite3 which requires CGO.
FROM golang:1.26-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc libc6-dev git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY --from=langpeanut_local . /langpeanut_local
COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 CGO_CFLAGS="-O0" GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/langpeanut-cloud ./cmd/server && \
    CGO_ENABLED=1 CGO_CFLAGS="-O0" GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/langpeanut-runner ./cmd/runner

# ── Stage 2: Runtime (server + runner fallback) ───────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git wget docker.io \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -r -u 1001 -g 0 langpeanut && \
    git config --system --add safe.directory "*"

WORKDIR /app
COPY --from=builder /out/langpeanut-cloud /app/langpeanut-cloud
COPY --from=builder /out/langpeanut-runner /usr/local/bin/langpeanut-runner
COPY --from=builder /out/langpeanut-runner /app/langpeanut-runner
COPY --from=web-builder /web/out /app/web/out

RUN chown -R langpeanut /app
USER langpeanut

ENV PATH="/usr/local/bin:/app:${PATH}"
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/langpeanut-cloud"]
