# Handoff: langPeanut Cloud — GitHub-Integrated Localization Bot

**Date**: 2026-08-29
**Repo**: `/Users/harmanpreetsingh/Public/Code/langTranslate` (module `github.com/langPeanut/langPeanut`)
**Branch**: `master` (all work below is uncommitted working-tree state — see "Git State" below)

## What This Is

Extending the existing `langPeanut` CLI/TUI (a Go multi-agent localization tool built for the
micro1 Agentic Workflows Hackathon) into a hosted, self-sufficient VPS service ("langPeanut
Cloud"): a team connects GitHub, picks any repo they have access to, and the service
automatically clones it, runs the existing 6-agent localization pipeline, and opens a PR — all
running on the VPS, with the user's PC only ever loading a web page.

**Full architecture, data model, and design rationale live in [`cloud_plan.md`](cloud_plan.md)**
at the repo root — read that first, it is the source of truth and this handoff does not repeat its
content. Key sections: §1 (goal + confirmed connect→list→pick flow), §3 (architecture, single-VPS
diagram with trust zones), §5 (SQLite data model), §6 (job execution flow incl. §6.1 repo mirrors,
§6.2 dedupe, §6.3 sandboxing), §7 (confirmed decisions), §8 (repo layout + Docker/Compose specifics
for the VPS deploy), §9 (open questions), §10 (implementation order / progress checklist).

**Chronological narrative of every session's directive → problem → fix → verification is in
[`CHANGELOG.md`](CHANGELOG.md), Session Entries 42–46** — read those five entries for exactly what
was built, why, and how it was tested. Do not re-derive this from the diff; the entries already
explain it in detail.

## Where We Are Right Now

`pkg/github/` (inside the existing `langTranslate` repo, NOT a new repo yet) is **feature-complete
for its planned scope**:

| File | What it does | Status |
|---|---|---|
| `pr_template.go` | Deterministic PR title/body/labels formatter from `*agents.PipelineResult` — zero LLM calls | Done, tested |
| `app_auth.go` | GitHub App RS256 JWT signing (stdlib only), installation token exchange, `ListInstallations`, paginated `ListInstallationRepos` | Done, tested |
| `http_util.go` | Shared HTTP request helpers | Done |
| `repo_client.go` | Clone/branch/commit/push via system `git` binary (not go-git — deliberate choice) | Done, tested |
| `pr_client.go` | `CreatePullRequest`, `AddLabels`, `PostComment`, and `OpenLocalizationPR` (the single entry point a job worker calls) | Done, tested |
| `webhook.go` | HMAC-SHA256 signature verification + typed push/installation event parsing | Done, tested |

**30 tests passing** in `pkg/github/`, full repo `go build`/`go vet`/`go test ./...` clean with
zero regressions as of the last verification pass.

**Nothing beyond `pkg/github/` has been scaffolded yet.** No `langpeanut-cloud` repo exists. No
server, no worker loop, no SQLite schema, no Dockerfile, no web UI — those are all still just
described in `cloud_plan.md`, not implemented.

## Git State (needs a decision, not yet handled)

Uncommitted in the working tree:
- Modified: `CHANGELOG.md`
- Untracked: `cloud_plan.md`, `pkg/github/` (all files)

Nothing has been committed or pushed this session. The next agent (or the user) needs to decide
whether/how to commit this — it was not asked for explicitly yet, per the project's standing rule
of never committing without being asked.

## Next Step (per `cloud_plan.md` §10, step 5)

Scaffold the actual `langpeanut-cloud` repo — this is new-repo work, not more additions to
`pkg/github/`:
- `cmd/server/main.go` (trusted host process: HTTP API + worker loop as goroutines)
- SQLite schema + migrations (see §5 for the exact `jobs`/`repos`/`repo_settings`/etc. table shapes)
- Minimal HTTP API (health check, settings CRUD)
- In-process worker loop wired to `pkg/agents.SupervisorAgent` (via `github.com/langPeanut/langPeanut/pkg/...` as a Go module import — see §8.4 for the `replace`-directive-during-development pattern)
- `internal/mirror/` — persistent bare git mirror cache (§6.1)
- Sandbox launcher — spawns `langpeanut-runner` Docker containers per job via the Docker socket (§6.3)

After that: `Dockerfile` + `Dockerfile.runner` + `docker-compose.yml` + `Caddyfile` (§8.2–8.3), get
a real end-to-end VPS deploy working API-only (curl-testable) before building the web UI.

## Confirmed Decisions (don't re-litigate these — see `cloud_plan.md` §7 for full rationale)

- **All-cloud execution**: user's PC only loads a web page; every clone/run/commit/push/PR-open
  happens entirely on the VPS. (This was a real point of user confusion earlier in the session —
  worth stating explicitly again if it comes up.)
- **Deployment target**: single self-hosted VPS, `docker-compose up`, no managed cloud services.
- **Datastore**: SQLite (WAL mode) — explicitly chosen over Postgres for hackathon scope.
- **Job queue**: the `jobs` table itself (atomic `UPDATE ... WHERE status='pending'` claims) — no
  Redis.
- **Repo split**: `pkg/github/` stays in `langTranslate` (no web/DB deps); the service itself is a
  new standalone repo, `langpeanut-cloud`, importing `langTranslate`'s `pkg/...` as a dependency.
- **Git mechanism**: shell out to system `git` via `os/exec`, not a Go git library (go-git) —
  simpler auth-token-in-URL push semantics, avoids a large new dependency.
- **PR behavior**: PR always opens, success or partial-failure. Repair-agent failures add a
  `needs-manual-review` label + a standalone review comment, never block the PR.
- **PR content**: deterministic templates only, zero LLM calls for PR title/body (explicit "why
  waste tokens" instruction from the user).
- **Sandboxing**: each job's actual pipeline execution runs in a short-lived, purpose-built
  `langpeanut-runner` Docker container spawned by the trusted host process — not a subprocess on
  the host, not inside the long-running API/worker process. The sandbox gets only its own scratch
  volume, that job's LLM key, and a scoped git token; never the SQLite file, the Docker socket, or
  the GitHub App's private/master keys. PR creation stays in the trusted host process only.
- **Dedupe**: two complementary mechanisms — persistent local git mirrors (avoid re-cloning from
  GitHub every run) and a skip-if-already-processed check keyed on
  `(repo_id, branch, head_commit_sha, repo_settings_hash)`.

## Open Questions (still unresolved — see `cloud_plan.md` §9)

1. Web UI stack — Next.js/React assumed but not confirmed with the user.
2. Trigger model — manual-only for v1, or also webhook-driven auto-detection of new strings on
   push? (Webhook parsing exists in `webhook.go` now, but nothing decides whether/how to act on it
   automatically yet.)
3. VPS provider/specs — no preference stated yet (doesn't block implementation, just deployment).

## Also Worth Knowing

- There's a **separate, standing project convention**: every CHANGELOG.md entry in this repo must
  include the user's directive, the root cause/problem, the actions taken, and verification steps
  — this was explicitly requested by the user earlier and has been followed consistently through
  Session Entry 46. Keep following it for any further work in this repo.
- The base `langPeanut` CLI/TUI project (separate from this cloud extension) is a completed,
  100%-passing hackathon submission — see `README.md`, `PLAN.md`, and CHANGELOG Session Entries
  1–41 for that history. This handoff only concerns the cloud/GitHub-bot extension work
  (Session Entries 42–46).

## Suggested Skills for the Next Session

- **`claude-mem:mem-search`** — if picking this up in a fresh session, search prior work before
  re-deriving anything; this project has 46+ CHANGELOG session entries and a claude-mem memory
  index with detailed observations on every design decision made so far.
- **`claude-mem:make-plan`** — if scaffolding the `langpeanut-cloud` repo feels like it needs its
  own phased plan (new repo, new dependencies, real deploy surface), use this before diving into
  code, the same way `cloud_plan.md` was built up incrementally here.
- **`code-review`** — worth running once the `langpeanut-cloud` scaffold has a first working
  version, especially given the security-sensitive surface (sandboxing, credential handling,
  webhook signature verification).
- **`security-review`** — specifically for the sandbox/credential-boundary work in §6.3 once
  implemented (Docker socket exposure, secrets handling, installation token scoping) — this is
  exactly the kind of boundary where a dedicated security pass earns its cost.
