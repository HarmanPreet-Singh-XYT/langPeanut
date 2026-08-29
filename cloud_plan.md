# cloud_plan.md — langPeanut Cloud: GitHub-Integrated Web Interface

> Companion plan to [PLAN.md](PLAN.md). Scope: turn the local `langPeanut` CLI/TUI into a hosted
> service with a web interface and a GitHub App bot that opens localization PRs on demand or on
> triggers, while reusing the existing `pkg/agents`, `pkg/platforms`, `pkg/llm`, and `pkg/types`
> packages as a library — not by shelling out to the compiled binary.

---

## 1. Goal

A team connects their GitHub account (installs the GitHub App), the service lists every repo that
installation has access to, and the team picks any one of them to enable localization on — no
manual repo-URL entry, no per-repo separate connect step. Once a repo is picked, the team
configures localization settings for it (languages, tone, model/provider, token budgets — the same
knobs the CLI wizard already exposes), and either clicks "Run" in the web UI or lets a webhook
trigger a run. The service clones the repo, runs the existing 6-agent pipeline
(`SupervisorAgent.RunEndToEnd`), and opens a PR with the changes. If the autonomous Code Repair
Agent (Session Entry 41) cannot fix a compiler regression introduced by the refactor, the PR is
still opened — never withheld — but flagged clearly for human review.

**Connect → list → pick flow** (confirmed): after the GitHub App install completes, the web UI
calls `ListInstallations` to find the installation(s) tied to the logged-in user, then
`ListInstallationRepos` (paginated, already implemented — see §10) against each installation's
token to render the full repo picker. This mirrors exactly how GitHub's own App-install callback
works: the install redirect carries an `installation_id`, and everything after that is a read of
"what can this installation see" rather than the user typing in a repo name.

## 2. Non-Goals (for this phase)

- No multi-tenant billing/metering beyond raw token/cost display (BYO API key model).
- No support for self-hosted GitHub Enterprise in v1 (github.com only).
- No real-time collaborative editing of translations in the web UI — review happens on the PR itself.
- No new LLM-generated prose for PR titles/descriptions — deterministic templates only (see §7,
  reaffirming the project's existing "Zero-Generation Principle" hot take in README.md §4).

## 3. High-Level Architecture — Self-Sufficient Single-VPS Unit

The deployment target is **one VPS, one `docker-compose up`**, no managed cloud services. The
whole cloud unit is designed to be self-sufficient: it brings its own DB, its own queue (DB-backed,
no Redis), its own TLS termination, and ships as a small number of containers.

```
                              ┌─────────────────────────────┐
                    HTTPS     │  Caddy (reverse proxy + TLS) │
     (your PC's browser)  ───►│  auto Let's Encrypt cert     │
                              └──────────────┬───────────────┘
                                             │
                     ┌───────────────────────┼───────────────────────┐
                     │                                                │
            ┌────────▼────────┐                              ┌────────▼────────┐
            │  Web UI (static  │                              │  langpeanut-    │
            │  build, served   │─────HTTP (same-origin)──────►│  cloud (Go       │
            │  by Caddy)       │                              │  binary: API +   │
            └──────────────────┘                              │  worker, single  │
                                                                │  process — trusted│
                                                                │  host process,     │
                                                                │  holds App private │
                                                                │  key + master key) │
                                                                └────────┬─────────┘
                                     ┌──────────────────┬─────────────────┼──────────────────────┐
                                     │                  │                 │                       │
                           ┌──────────▼──────────┐ ┌─────▼──────┐ ┌───────▼────────┐  ┌────────────▼────────────┐
                           │ SQLite (single file, │ │ Repo mirror│ │ /var/run/       │  │ GitHub App               │
                           │ WAL mode)            │ │ cache      │ │ docker.sock     │  │ (installation tokens,    │
                           │ - teams/repos/settings│ │ (data/    │ │ (spawns sibling │  │  webhooks, PR/comment    │
                           │ - jobs table doubles  │ │ mirrors/) │ │  runner          │  │  API calls — called ONLY │
                           │   as the job queue    │ └────────────┘ │  containers)    │  │  from the host process)  │
                           │   (dedupe by commit +  │               └────────┬────────┘  └───────────────────────────┘
                           │   settings hash)       │                        │
                           └─────────────────────────┘                       │
                                                                              ▼
                                                          ┌─────────────────────────────────────┐
                                                          │  langpeanut-runner (per-job, sandboxed│
                                                          │  Docker container — untrusted zone):  │
                                                          │  - only this job's scratch volume     │
                                                          │  - only this job's LLM key (env var)  │
                                                          │  - only a pre-authenticated git remote │
                                                          │  - CPU/memory/wall-clock capped        │
                                                          │  - runs pkg/agents.SupervisorAgent,    │
                                                          │    commits + pushes, then exits        │
                                                          │  - NO SQLite access, NO Docker socket, │
                                                          │    NO App private key                  │
                                                          └─────────────────────────────────────┘
```

**Why this shape:**
- **One Go binary** runs both the HTTP API and the job worker as goroutines in the same process —
  no separate worker fleet to deploy or coordinate on a single box.
- **SQLite, not Postgres** — per user decision, this is a hackathon-scoped deployment; SQLite in
  WAL mode handles the expected job volume (manual/webhook-triggered runs, not high concurrency)
  with zero extra containers. The whole app's persistent state is one file, trivial to back up
  (`cp`/`litestream`) and trivial to inspect.
- **DB-backed job queue, not Redis** — the `jobs` table itself is the queue. A poller claims the
  oldest `pending` row with a `SELECT ... WHERE status='pending' ORDER BY created_at LIMIT 1` inside
  a transaction (SQLite serializes writers naturally, so no `SKIP LOCKED` trick is even needed at
  this scale — a single-writer mutex around claim+update is sufficient). No extra service, no extra
  failure mode to operate.
- **Caddy** for the reverse proxy: automatic HTTPS via Let's Encrypt with a few lines of Caddyfile,
  no manual certbot cronjobs — the right tool for a bare VPS.
- **Standalone repo** (`langpeanut-cloud`, separate from `langTranslate`) that depends on
  `github.com/langPeanut/langPeanut/pkg/...` as a Go module import. Keeps the hackathon CLI
  submission repo focused, and lets the cloud unit have its own Dockerfile, compose file, and
  release cadence. See §8.4 for the exact module/replace setup.

Everything under "langpeanut-cloud" imports the existing Go module as a library:
`github.com/langPeanut/langPeanut/pkg/agents`, `/pkg/platforms`, `/pkg/llm`, `/pkg/types`,
`/pkg/orchestrator`, plus the new `pkg/github/` package for GitHub-specific glue (App auth, repo
clone/push, PR creation, comment/label formatting, webhook verification). No behavior of the
existing CLI or TUI changes — `pkg/github/` lives in the `langTranslate` repo since it has zero
web/DB dependencies and is equally usable from a future CLI `langPeanut pr` command.

## 4. New Package: `pkg/github/`

| File | Responsibility |
| :--- | :--- |
| `app_auth.go` | GitHub App JWT signing + installation-token exchange (App ID, private key from env/secret store). |
| `repo_client.go` | Clone target repo to a scratch dir (go-git, shallow clone), create branch, commit, push using the installation token. |
| `pr_client.go` | Create PR via GitHub REST/GraphQL API, add labels, post review/issue comments. |
| `pr_template.go` | **Pure, deterministic formatter.** Takes `*agents.PipelineResult` (+ run metadata: locales, tone, provider/model, token usage) and returns `(title string, body string, labels []string)`. No network calls, no LLM calls — fully unit-testable with table-driven tests. |
| `webhook.go` | Verifies GitHub webhook signatures (HMAC-SHA256) and dispatches events (e.g. `push`, `installation`) to job triggers. |

### 4.1 `pr_template.go` contract (first concrete artifact to implement)

```go
type RunMetadata struct {
    Locales      []string
    TonePreset   string
    Provider     string
    Model        string
    PromptTokens int64
    OutputTokens int64
    EstimatedCostUSD float64
}

// BuildPullRequest deterministically formats a PR title, body, and label set
// from a completed pipeline run. Never calls an LLM — see README.md's
// "Zero-Generation Principle" hot take.
func BuildPullRequest(result *agents.PipelineResult, meta RunMetadata) (title, body string, labels []string)
```

Behavior:
- **Title**: `i18n: localize {N} string(s) across {M} file(s) ({locales joined})`. If
  `len(result.UnresolvedErrors) > 0`, append ` — {K} file(s) need review`.
- **Body sections**: Summary (counts, tone, provider/model, token/cost), Files touched (from
  `result.RefactoredFiles` + per-file extracted-string counts), Verification (pass/fail per tier
  from `result.VerificationReport.Diagnostics`, plus `result.CodeRepairs` if non-empty), and —
  only rendered when `result.UnresolvedErrors` is non-empty — a `## ⚠️ Needs manual review`
  section listing each `types.CompilerDiagnostic` (file, line, message, source).
- **Labels**: always `i18n-automation`; add `needs-manual-review` iff `UnresolvedErrors` non-empty.
- The PR is opened in **both** cases (success and partial-failure) — the function never returns
  an error for "repair failed," only for malformed input (e.g. nil result).

## 5. Data Model (SQLite, via a new `db/migrations/` dir using plain versioned `.sql` files)

Single file, WAL mode enabled (`PRAGMA journal_mode=WAL`) so the API can read while the worker
writes. `modernc.org/sqlite` (pure-Go, no CGO) is the preferred driver so the cloud binary keeps
the CLI's easy cross-compilation story — CGO is currently only needed for tree-sitter grammars,
which the cloud binary depends on transitively anyway, so plain `mattn/go-sqlite3` is also fine if
that's already a build requirement; worth a quick check once implementation starts.

- `teams` (id, name, created_at)
- `github_installations` (id, team_id, installation_id, account_login)
- `repos` (id, installation_id, owner, name, default_branch)
- `repo_settings` (repo_id, locales_json, tone_preset, provider, model, safety_mode,
  chunk_word_budget, chunk_key_ceiling — mirrors the CLI wizard's 4 steps + Session 37/38 tunables;
  `locales_json` stores the []string as a JSON array column since SQLite has no native array type)
- `api_credentials` (team_id, provider, encrypted_key) — BYO key, encrypted at rest with a server-
  side master key (age or libsodium secretbox; key itself lives outside the SQLite file, e.g. an
  env var injected at container start — never logged, never returned in API responses after creation)
- `jobs` (id, repo_id, trigger_type [manual|webhook], status
  [pending|running|succeeded|needs_review|failed|skipped_no_changes], head_commit_sha,
  repo_settings_hash, pr_url, error_message, created_at, started_at, finished_at) — **this table
  doubles as the job queue**; the worker loop polls for `status='pending'` rows. `head_commit_sha` +
  `repo_settings_hash` back the dedupe check in §6.2 (skip a run if the same commit was already
  processed under the same settings); `error_message` surfaces genuine infra failures (clone/push/
  GitHub API/sandbox killed for exceeding limits) in the web UI.
- `job_token_usage` (job_id, provider, model, input_tokens, output_tokens, cost_usd) — same shape
  as `pkg/llm/tracker.go`'s `ModelUsage`, just keyed per-job/per-team instead of per-machine JSON file.

**Backups**: since there's no managed DB, back up the SQLite file directly — either a cron `cp`
of the file after a `PRAGMA wal_checkpoint` or `litestream` streaming continuous replication to
object storage (S3-compatible, e.g. Backblaze B2) if zero-data-loss matters more than simplicity.
For a hackathon-scoped deployment, a nightly cron copy is enough; note `litestream` as the upgrade
path if this becomes a real production service later.

## 6. Job Execution Flow

1. Trigger arrives (web UI click → API → `INSERT INTO jobs (status='pending')`; or webhook push
   event with new strings detected).
2. In-process worker loop (a goroutine started alongside the HTTP server, polling every few seconds
   for `status='pending'`) claims the job: `UPDATE jobs SET status='running', started_at=now()
   WHERE id=? AND status='pending'` — the `WHERE status='pending'` guard makes claiming atomic
   even if a second worker goroutine or process instance is running, with no separate queue needed.
3. Worker resolves `repo_settings` + decrypts the team's API key for the chosen provider.
4. **Dedupe check** (see §6.2): if the target branch's current HEAD commit SHA already has a
   `succeeded` or `needs_review` job on record for the same repo+settings, skip straight to
   `status='skipped_no_changes'` — no clone, no pipeline run, no PR. Otherwise continue.
5. **Update local mirror** (see §6.1) and clone the working copy from it, checkout new branch
   `langpeanut/i18n-{timestamp}-{shortsha}`.
6. **Launch the sandboxed job container** (see §6.3) — the actual pipeline execution
   (`agents.SupervisorAgent.RunEndToEnd`) happens inside this container, not in the API/worker
   process itself.
7. Construct `agents.SupervisorAgent` exactly as the CLI does today (`OnProgress` wired to a
   per-job progress channel → web UI via SSE/WebSocket, reusing the Session 36 streaming design;
   from inside the sandbox, progress is streamed back to the host worker over the container's
   stdout as newline-delimited JSON, since the sandbox has no direct DB access).
8. Run `RunEndToEnd(ctx, sourceLocale, targetLocales, dryRun=false)` inside the sandbox.
9. Commit + push branch using installation token (push happens from inside the sandbox, which has
   network egress to github.com but nothing else — see §6.3).
10. Call `github.BuildPullRequest(result, meta)` → open PR with returned title/body/labels (this
    API call is made by the **host** worker process after reading the sandbox's result, not by the
    sandbox itself — keeps the GitHub App private key and installation-token minting entirely
    outside the sandboxed, less-trusted execution environment).
11. Persist `job_token_usage` from the tracker; mark job `succeeded` or `needs_review` (never `failed`
    purely due to unresolved compiler errors — that's a review state, not a job failure). Genuine
    infrastructure failures (clone failed, push rejected, GitHub API error, sandbox crashed/killed
    for exceeding its resource limits) mark the job `failed` with an error message column, surfaced
    in the web UI.
12. Destroy the sandbox container and its scratch volume unconditionally (success, failure, or
    timeout) — nothing about a job's execution persists past its own run except the pushed branch
    on GitHub and the row in `jobs`.

### 6.1 Persistent Repo Mirrors (avoid re-cloning from GitHub every run)

Rather than a fresh `git clone` from `github.com` on every job, the VPS keeps one **bare mirror**
per connected repo under `data/mirrors/{repo_id}.git`, created once with `git clone --mirror` and
kept current with `git fetch` before each job. The per-job working copy is then cloned **from that
local mirror** (`git clone /data/mirrors/{repo_id}.git workdir`, a fast local filesystem operation,
followed by `git remote set-url origin <authenticated-github-url>` before push) instead of pulling
the full repo over the network each time. This cuts GitHub API/bandwidth usage and start-up latency
per job, especially for large repos, without changing anything about correctness — the mirror is
just a cache of what GitHub already has.

### 6.2 Skip Redundant Runs on Unchanged Commits

Before doing any cloning or pipeline work, the worker checks: has this exact `(repo_id, branch,
head_commit_sha, repo_settings_hash)` combination already produced a `succeeded` or `needs_review`
job? If yes, the new job is marked `skipped_no_changes` immediately and the web UI reports "already
up to date as of commit `{sha}`" instead of silently doing nothing or wastefully re-running an
identical localization pass and opening a duplicate PR. `repo_settings_hash` is included in the key
because a settings change (e.g. adding a new target language) legitimately means the same commit
needs a fresh run. This requires one new column on `jobs`: `head_commit_sha`, populated right after
the mirror fetch in step 5, before the dedupe check in step 4 can run for the *next* job.

### 6.3 Sandboxed Execution (per-job Docker container)

Each job's actual pipeline execution — the AST scout, patch engine, translator LLM calls, and the
final `git commit`/`git push` — runs inside its **own short-lived Docker container**, not directly
on the VPS host or inside the long-running API/worker process. This matters because jobs operate on
arbitrary third-party repository content (build scripts, dependency manifests, source files this
service doesn't control), and a compromised or buggy input shouldn't be able to reach the host
filesystem, the SQLite database, other jobs' scratch directories, or the GitHub App's private key.

- **How it's launched**: the host worker process talks to the Docker daemon over the mounted
  `/var/run/docker.sock` (the API/worker container needs this socket mounted — the one deliberate
  exception to "no host access" since something has to be able to start sibling containers) and
  starts a container from a purpose-built `langpeanut-runner` image (same Go binary, CGO/tree-sitter
  included, but with `ENTRYPOINT` set to just run one job and exit).
- **What the sandbox gets**: a single bind-mounted scratch volume for that job's clone (nothing else
  from `/data`), the decrypted API key for this job only (passed as an env var at container start,
  never written to disk), and the pre-authenticated git remote URL (installation token already
  embedded — the sandbox never sees the GitHub App's private key or JWT-signing capability, only a
  short-lived, single-purpose token good for this one push).
- **What the sandbox does NOT get**: access to the SQLite file, the Docker socket itself (no nested
  container spawning), any other job's volume, or the App's private key/master encryption key.
- **Resource limits**: `--memory`, `--cpus`, and a wall-clock timeout (`context.WithTimeout` around
  the `docker run`, killing the container if the pipeline hangs) enforced by the host worker at
  launch time — caps a single runaway job from starving the VPS.
- **Network egress**: unrestricted at the Docker level for this phase (needs to reach the LLM
  provider's API and github.com) — tightening this to an explicit allowlist (e.g. via a custom
  Docker network with egress filtering) is a reasonable hardening step later but adds real
  operational complexity; noted as a follow-up, not required for the hackathon-scoped deploy.
- **Result handoff**: the sandbox writes its final `PipelineResult` JSON to stdout as the last line
  before exiting; the host worker captures it, persists `job_token_usage`, and calls
  `github.BuildPullRequest` — the sandbox itself never calls the GitHub PR-creation API, keeping
  that credential-bearing call in the trusted host process.

## 7. Design Decisions Confirmed

- **PR always opens**, success or partial-failure — repair failures become a labeled, commented
  review item, not a blocked PR.
- **Deterministic templates only** for PR title/body — no LLM token spend on prose generation,
  consistent with the project's existing anti-hallucination stance.
- **GitHub App**, not OAuth App or raw PAT — scoped installation tokens, acts as its own bot
  identity, supports org-wide install.
- **Embed as Go library**, not subprocess-per-job — reuses `OnProgress` streaming hooks already
  built for the TUI, avoids binary-packaging overhead per job.
- **BYO API key per team**, stored encrypted — avoids the service eating inference cost and sidesteps
  rate-limit sharing across tenants.
- **Deployment target: a single self-hosted VPS**, not a managed PaaS/cloud provider — the cloud
  unit must be self-sufficient (own DB, own queue, own TLS), deployable via `docker-compose up`.
- **SQLite** as the datastore (hackathon-scoped decision — simplest possible ops story: one file,
  one thing to back up, no separate DB container).
- **DB-backed job queue** (the `jobs` table itself, polled by an in-process worker loop) — no
  Redis/asynq; unnecessary operational surface for the expected job volume.
- **Standalone repo** (`langpeanut-cloud`) importing `langTranslate`'s `pkg/...` as a Go module
  dependency — keeps the hackathon CLI submission repo unpolluted by web/DB/deploy infra.
- **Per-job sandboxed Docker containers** for actual pipeline execution (§6.3) — the host API/worker
  process stays trusted and holds the App private key + master encryption key; the sandbox that
  touches arbitrary third-party repo content gets none of that, plus capped CPU/memory/wall-clock.
- **Persistent local repo mirrors + commit-based dedupe** (§6.1, §6.2) — avoid both re-cloning from
  GitHub on every run and re-running/re-opening-a-PR for a commit that's already been processed
  under the same settings.
- **Web UI: Next.js (React)** — confirmed by user (Session Entry 49). Static export served by Caddy
  from `web/` in the `langpeanut-cloud` repo. The Go API is consumed from the same origin via
  fetch/SWR; no separate frontend hosting needed.
- **Trigger model: manual + webhook, manual-first** — both modes will be supported (confirmed Session
  Entry 49), but v1 only ships the manual "Run" button in the web UI. Webhook-on-push auto-trigger
  is planned for v2 once the manual path is stable end-to-end. `webhook.go`'s
  `PushEvent.IsDefaultBranchPush()` already exists for when that gate opens.

## 8. Repo Layout & VPS Deployment Details

### 8.1 `langpeanut-cloud` repo structure

```
langpeanut-cloud/
├── cmd/
│   ├── server/
│   │   └── main.go              # starts HTTP API + worker loop in one process (trusted host)
│   └── runner/
│       └── main.go              # sandbox entrypoint: one job, runs SupervisorAgent, exits
├── internal/
│   ├── api/                     # HTTP handlers (settings CRUD, job trigger, SSE progress)
│   ├── worker/                  # job-claim loop, dedupe check, spawns runner containers via
│   │                             # the Docker socket, reads result, calls pkg/github for the PR
│   ├── mirror/                  # repo mirror cache management (git clone --mirror / fetch)
│   ├── db/                      # SQLite access layer + db/migrations/*.sql
│   └── auth/                    # GitHub OAuth (user login) session handling
├── web/                         # frontend (static build served by Caddy)
├── Dockerfile                   # builds the server image (CGO for tree-sitter) + copies web build
├── Dockerfile.runner            # builds the langpeanut-runner sandbox image (same Go binary base,
│                                 # different entrypoint, no DB/Docker-socket access at runtime)
├── docker-compose.yml           # app container + Caddy container
├── Caddyfile
├── go.mod                       # requires github.com/langPeanut/langPeanut v0.x.y
└── data/
    ├── langpeanut.db            # SQLite file
    ├── mirrors/                 # persistent bare git mirrors, one per connected repo (§6.1)
    └── jobs/                    # per-job scratch volumes, one bind-mounted into each runner
                                  # container and deleted unconditionally after that job exits
```

### 8.2 Dockerfile shape

Multi-stage build: stage 1 builds the Go binary with `CGO_ENABLED=1` (tree-sitter grammars need
CGO, same requirement the CLI already has per REPRODUCE.md) on a `golang:1.26` base with a C
toolchain; stage 2 is a minimal runtime image (`debian:bookworm-slim`, not `scratch`, since CGO
binaries need libc) that copies the binary, the built frontend static assets, and runs as a
non-root user with the `data/` volume mounted for the SQLite file and job scratch clones.

### 8.3 docker-compose.yml shape

```yaml
services:
  app:
    build: .
    volumes:
      - ./data:/data
      - /var/run/docker.sock:/var/run/docker.sock   # lets the worker spawn sandboxed runner containers
    environment:
      - DATABASE_PATH=/data/langpeanut.db
      - GITHUB_APP_ID=...
      - GITHUB_APP_PRIVATE_KEY_PATH=/data/github-app.pem
      - MASTER_KEY=...          # encrypts api_credentials at rest
      - RUNNER_IMAGE=langpeanut-runner:latest
    restart: unless-stopped
  caddy:
    image: caddy:2
    ports: ["80:80", "443:443"]
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
    depends_on: [app]
    restart: unless-stopped
volumes:
  caddy_data:
```

Mounting the host's Docker socket into `app` is what lets the trusted worker process launch
sibling `langpeanut-runner` containers per job — those sandboxes do **not** get the socket
mounted into them (see §6.3), so a compromised job container can't use it to spawn further
containers or escape via the Docker API. `langpeanut-runner:latest` is built once from
`Dockerfile.runner` (`docker build -f Dockerfile.runner -t langpeanut-runner .`) and reused for
every job — the worker just runs `docker run --rm --memory=... --cpus=... -v <job-scratch>:/work
-e LLM_API_KEY=... langpeanut-runner <args>` per job.

One `docker compose up -d` on the VPS brings up the whole unit. Updates are `git pull && docker
build -f Dockerfile.runner -t langpeanut-runner . && docker compose up -d --build`.

### 8.4 Dependency wiring: `langpeanut-cloud` → `langTranslate`

Since `langTranslate`'s `pkg/...` packages aren't published to a registry, `langpeanut-cloud`'s
`go.mod` requires them via the GitHub path:

```
require github.com/langPeanut/langPeanut v0.0.0-<commit-pseudo-version>
```

During active co-development, a `replace` directive pointing at a local checkout
(`replace github.com/langPeanut/langPeanut => ../langTranslate`) avoids the tag/push cycle; drop
the `replace` and pin to a tagged commit before each VPS deploy so the cloud unit builds
reproducibly from a clean clone.

### 8.5 Secrets on the VPS

No secret manager assumed — a `.env` file outside git (referenced by `docker-compose.yml`'s
`env_file:`) holding `GITHUB_APP_ID`, the App's private key (mounted as a file, not an env var,
since PEM keys are multi-line), and `MASTER_KEY` for credential encryption. Restrict file
permissions (`chmod 600`) and exclude `.env`/`data/` from any backup that leaves the box unencrypted.

## 9. Open Questions (remaining)

1. **VPS provider/specs** — any preference (Hetzner, DigitalOcean, existing box)? Doesn't block
   implementation; it's just Docker. Determine before the first deploy.

*(Web UI stack and trigger model were resolved in Session Entry 49 — see §7 confirmed decisions.)*

## 10. Implementation Order

1. `pkg/github/pr_template.go` + unit tests — pure, no external deps, fastest to verify. **(done — Session Entry 42)**
2. `pkg/github/app_auth.go` + `repo_client.go` — GitHub App auth and repo clone/push. **(done — Session Entry 44)**
   - `app_auth.go`: RS256 App JWT signing (stdlib `crypto/rsa`, no JWT dependency), installation
     token exchange, `ListInstallations` + `ListInstallationRepos` (paginated) — the direct backer
     for "connect GitHub → see all repos → pick one."
   - `repo_client.go`: shells out to system `git` (chosen over go-git — see §7) for clone (token
     embedded in the HTTPS remote URL), branch, commit, push, and change-detection; redacts the
     installation token from any error output.
3. `pkg/github/pr_client.go` — PR creation, labels, comments. **(done — Session Entry 46)**
   - `CreatePullRequest`, `AddLabels`, `PostComment` as independent, individually-testable calls.
   - `OpenLocalizationPR`: the single entry point the job worker calls — formats via
     `BuildPullRequest`, creates the PR, applies labels, and posts a standalone review-request
     comment (distinct from the PR body's own section, since GitHub notifies on new comments but
     not on the initial PR body) only when `UnresolvedErrors` is non-empty. The PR itself is never
     un-created by a downstream labels/comment failure.
4. `pkg/github/webhook.go` — signature verification + event dispatch. **(done — Session Entry 46)**
   - `VerifySignature`: HMAC-SHA256 constant-time check against `X-Hub-Signature-256`.
   - `ParseWebhook`: typed decode for `push` and `installation`/`installation_repositories` events;
     unrecognized event types return `(EventUnhandled, nil, nil)` rather than erroring, since GitHub
     sends many event types this service doesn't act on.
   - `PushEvent.IsDefaultBranchPush()`: gates automatic triggering to default-branch pushes only,
     pending the still-open trigger-model decision in §9.
5. Scaffold `langpeanut-cloud` repo: `cmd/server/main.go`, SQLite schema + migrations, minimal
   HTTP API (health check, settings CRUD), in-process worker loop wired to `pkg/agents`. **(next)**
6. Dockerfile + docker-compose.yml + Caddyfile — get a real deploy working end-to-end on the VPS
   early, even before the web UI exists (API-only, testable via curl), to de-risk deployment
   mechanics separately from feature work.
7. Web UI — after the API contract is stable and deployable.
