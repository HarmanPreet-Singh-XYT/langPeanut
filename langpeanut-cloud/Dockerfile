# ── Runtime (server + runner fallback) ────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git wget docker.io \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -r -u 1001 -g 0 langpeanut && \
    git config --system --add safe.directory "*"

WORKDIR /app
COPY bin/langpeanut-cloud /app/langpeanut-cloud
COPY bin/langpeanut-runner /usr/local/bin/langpeanut-runner
COPY bin/langpeanut-runner /app/langpeanut-runner
COPY web/out /app/web/out

RUN chown -R langpeanut /app
USER langpeanut

ENV PATH="/usr/local/bin:/app:${PATH}"
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/langpeanut-cloud"]
