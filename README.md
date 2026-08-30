# langPeanut Cloud

> **Self-Sufficient GitHub-Integrated Multi-Agent Localization Service**  
> Universal multi-agent localization bot and web dashboard for automated repository localization across mobile, web, and backend frameworks.

---

## Architecture

- **Trusted Host Binary (`cmd/server`)**: Go REST API + In-Process Worker Queue + Embedded SQLite (WAL mode).
- **Sandboxed Runner (`cmd/runner`)**: Ephemeral Docker container spawned per job via the Docker socket, running the full 6-agent localization pipeline (`pkg/agents.SupervisorAgent`), isolated with CPU/memory limits, auto-destroyed on completion.
- **Web UI (`web/`)**: Next.js 15 App Router dashboard with Tailwind CSS v4 and SWR real-time job polling.
- **Reverse Proxy (`Caddyfile`)**: Caddy with automatic Let's Encrypt TLS.

---

## VPS Deployment

See the complete step-by-step instructions in [DEPLOYMENT.md](DEPLOYMENT.md).

### Quick Start
```bash
# 1. Configure secrets
cp .env.example .env
# Place your GitHub App private key at data/github-app.pem

# 2. Build sandboxed runner image (../langpeanut_local must be a sibling checkout)
docker build -f Dockerfile.runner -t langpeanut-runner:latest \
  --build-context langpeanut_local=../langpeanut_local .

# 3. Launch stack
docker compose up -d --build
```
