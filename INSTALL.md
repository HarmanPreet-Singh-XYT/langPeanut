# INSTALL.md — Installation Guide

Two things live in this monorepo:

- **[`langpeanut_local/`](langpeanut_local/)** — the CLI, TUI, and zero-build web Studio. Single Go binary. This is what judges should install first.
- **[`langpeanut-cloud/`](langpeanut-cloud/)** — the hosted GitHub-bot service. Only needed if you want to run the bot against real GitHub repos on a VPS.

---

## 1. Local CLI / TUI / Web Studio

### 1.1 Prerequisites

- **Go 1.21+** (developed/tested on `go1.26.2`).
- **CGO enabled** (`CGO_ENABLED=1`, the default almost everywhere) — the AST Scout links real tree-sitter grammars (TSX, Dart, Swift, Kotlin) via cgo, so a C toolchain must be present (Xcode Command Line Tools on macOS, `build-essential` on Debian/Ubuntu).
- **Git** (optional but recommended — checkpoints/rollback and delta-tracking use it).
- No API keys, no database, no Docker required for this path.

### 1.2 One-command install (recommended)

```bash
git clone <this-repo-url>
cd langpeanut/langpeanut_local
./install.sh
```

`install.sh` will:
1. Verify Go and git are present.
2. Run `go mod download`.
3. Build a stripped release binary to `bin/langPeanut` (`go build -ldflags="-s -w"`).
4. Symlink it into `langpeanut_local/` and install a copy onto your `$PATH` (`$GOPATH/bin`, then `~/.local/bin`, then `~/go/bin`, whichever exists first).
5. Copy `.env.example` → `.env` if you don't already have one, ready for you to drop in API keys later (optional — see §1.5).

If the install directory isn't already on your `$PATH`, the script prints the exact `export PATH=...` line to add to your shell profile.

### 1.3 Manual build

```bash
cd langpeanut/langpeanut_local
go mod download
CGO_ENABLED=1 go build -o langPeanut ./cmd/langPeanut
./langPeanut --help
```

Or via `make` (run from `langpeanut_local/`):

```bash
make build     # builds bin/langPeanut and a local symlink
make install   # runs install.sh
make test      # go test -v ./...
make web       # go run ./cmd/langPeanut web
make tui       # go run ./cmd/langPeanut
make benchmark # go run ./cmd/langPeanut benchmark
```

### 1.4 Verify the install

```bash
langPeanut --help
langPeanut benchmark      # runs fully offline, ~seconds, $0.00
```

### 1.5 Optional: API keys for frontier models

Nothing below is required to use langPeanut — see [§6 of the README](README.md#6-zero-cost-offline-mode-no-api-key-required) for the fully offline path. If you want to use a hosted frontier model for translation quality or the chat copilot, edit `langpeanut_local/.env`:

```env
GEMINI_API_KEY=
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
DEEPL_API_KEY=
```

`langPeanut` auto-detects whichever key is present, in this priority order: Anthropic → OpenAI → Gemini → Hugging Face (`HF_TOKEN`) → local (NLLB/Ollama, no key needed). You only need one.

### 1.6 Optional: fully offline local model

To exercise the zero-cost, zero-API-key path explicitly:

```bash
langPeanut models download        # fetches ~380MB NLLB-200 GGUF (Hugging Face)
langPeanut models install-runner  # installs llama.cpp via Homebrew, no sudo required
langPeanut models list            # confirm status
```

Alternatively, if you already run [Ollama](https://ollama.com) locally, langPeanut auto-detects it — no extra install step needed.

### 1.7 First run

```bash
langPeanut                        # interactive Bubble Tea TUI, instant launch
langPeanut web                    # browser Studio at http://localhost:3000
langPeanut chat                   # agentic chat copilot in the terminal
langPeanut run ./examples/nextjs-app   # 1-click localization on a bundled demo app
```

`langPeanut` (no subcommand) and `langPeanut web` both do **zero work on launch** — no scanning, no network calls — until you trigger an action, so startup is sub-10ms regardless of project size.

### 1.8 Uninstall

```bash
cd langpeanut/langpeanut_local
./uninstall.sh           # removes system binaries and local build outputs
./uninstall.sh --purge   # also cleans user configuration directory
```

---

## 2. `langpeanut-cloud` — hosted GitHub bot (optional, VPS deploy)

Only needed if you want a hosted service that clones real repos and opens PRs automatically. Not required to evaluate the core hackathon submission.

### 2.1 Prerequisites

- A VPS (1 vCPU / 2GB RAM / 20GB SSD minimum — e.g. Hetzner CX22 or a $12/mo DigitalOcean droplet).
- Ubuntu 22.04/24.04 or Debian 12.
- Docker Engine + Docker Compose v2.
- A GitHub App you control (App ID + private key + webhook secret).
- `langpeanut_local/` and `langpeanut-cloud/` must stay siblings, as they already are in this monorepo (the runner image build context references `../langpeanut_local`).

### 2.2 Quick start

```bash
cd langpeanut/langpeanut-cloud
cp .env.example .env
# edit .env: MASTER_KEY (openssl rand -hex 32), GITHUB_APP_ID, GITHUB_APP_PRIVATE_KEY_PATH, GITHUB_WEBHOOK_SECRET
mkdir -p data/mirrors data/jobs
cp /path/to/your-github-app-private-key.pem data/github-app.pem
chmod 600 data/github-app.pem .env

# Build the sandboxed per-job runner image (needs ../langpeanut_local as a sibling checkout)
docker build -f Dockerfile.runner -t langpeanut-runner:latest \
  --build-context langpeanut_local=../langpeanut_local .

# Launch the stack (add --profile caddy if you want bundled automatic HTTPS)
docker compose up -d --build
```

Verify:

```bash
curl -i http://localhost:8080/health   # {"status":"ok"}
```

### 2.3 Live Hosted Cloud Testing (No Server Needed)

If you want to test the full cloud service and GitHub bot without running Docker or a VPS:
- **Live Cloud Dashboard**: [https://34.135.83.146.sslip.io](https://34.135.83.146.sslip.io)
- **Public GitHub App**: [https://github.com/apps/langpeanut-localization-bot](https://github.com/apps/langpeanut-localization-bot)
- **Verified Production Test Repository**: [https://github.com/HarmanPreet-Singh-XYT/pingroute-web](https://github.com/HarmanPreet-Singh-XYT/pingroute-web) (Full Next.js 15 / React project tested and calibrated end-to-end).

### 2.4 Full guide

The complete step-by-step — creating the GitHub App, permission scopes, domain/SSL options (bundled Caddy vs. an existing nginx), backups, and the one-command update flow — is in [`langpeanut-cloud/DEPLOYMENT.md`](langpeanut-cloud/DEPLOYMENT.md).

---

## 3. Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `cgo: C compiler "cc" not found` | Install Xcode Command Line Tools (`xcode-select --install`) on macOS, or `build-essential` on Linux. `CGO_ENABLED=1` is required — the tree-sitter grammars are C. |
| `langPeanut: command not found` after install | The install directory isn't on `$PATH`. Re-run `./install.sh` (from `langpeanut_local/`) and follow the printed `export PATH=...` line, or run the binary directly via `./bin/langPeanut`. |
| `0 candidates found` when scanning the monorepo root | Neither the monorepo root nor `langpeanut_local/` itself is a frontend project root. Point at a real app directory, e.g. `langPeanut audit ./examples/nextjs-app` (run from inside `langpeanut_local/`), or use `[p]` in the TUI / the project switcher in the web Studio. |
| Benchmark's zero-shot baseline column shows a fixed `42.0%` | No `GEMINI_API_KEY` is set — that column falls back to a labeled historical estimate. Add the key to `langpeanut_local/.env` to live-measure it (rate-limited on the free tier, ~20 req/min). |
| Translation falls back to offline/local quality unexpectedly | No API key was detected in `.env` or the environment; this is by design (§1.5/§1.6), not a bug — set a key if you want a frontier-model translation instead. |
| `docker build -f Dockerfile.runner` fails looking for `../langpeanut_local` | Run the build from inside `langpeanut-cloud/` with `langpeanut_local/` as its sibling directory (as it already is in this monorepo) — the `--build-context` flag depends on that relative path. |
