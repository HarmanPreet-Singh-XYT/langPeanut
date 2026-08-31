# JUDGES.md — Everything a Judge Needs, in One Place

`langPeanut` was built for the **Agentic Workflows Hackathon**. This document is a single entry point that pulls together the problem, the proof, the architecture, and exactly how to verify every claim — so you don't have to piece it together from the READMEs, changelog, and code yourself.

This monorepo has two parts: [`langpeanut_local/`](langpeanut_local/) (the CLI/TUI/web engine — start here) and [`langpeanut-cloud/`](langpeanut-cloud/) (the hosted GitHub bot, optional VPS deploy).

---

## 1. 30-second summary

Retrofitting localization onto an existing codebase is manual and dangerous: hardcoded strings are scattered everywhere, and naive tools (regex, single-prompt LLMs) corrupt code when they try to fix it — deleting comments, breaking JSX/widget trees, mistranslating variable placeholders. `langPeanut`'s core is a 6-agent pipeline that treats this as an **AST boundary problem, not a translation problem**: a deterministic tree-sitter scout finds strings, a deterministic byte-range patch engine rewrites code, and an LLM is used *only* for the narrow judgment calls (which words are ambiguous, what's the right translation) — never to regenerate a whole file. The result: **100% AST compilation pass rate, 0% formatting drift**, measured live on a 10-case adversarial benchmark, reproducible with one command, at $0.00.

But that's one of **three cooperating agentic systems**, not the whole submission:

- **System A — Localization Engine**: the 6-agent pipeline above, plus a Code Repair Agent, a Directive Agent for free-form post-localization edits, and three standalone maintenance agents (Doctor, Pruner, Persona Scout).
- **System B — Central AI Copilot** (`langPeanut chat`): a conversational control plane with **19 registered tools** that can drive System A, System C, checkpoint/config management, diagnostics, and the cloud job queue — all from one chat session, with a deterministic offline fallback when there's no network.
- **System C — SEO & Growth Studio** (`langPeanut seo`): an independent 5-agent pipeline (Scout → Keywords → Weaver → Simulator → Growth Critic) that optimizes the already-translated copy for local search — something no commercial localization tool touches.

All three share one Go core and are reachable identically from CLI, TUI, a zero-build web Studio, and — in the sibling `langpeanut-cloud` repo — a self-hosted GitHub App that runs them automatically and opens PRs. See [README §3](README.md#3-three-agentic-systems-one-shared-core) for the architecture diagram tying them together.

---

## 2. How to verify the core claim in under a minute

```bash
cd langpeanut_local
./install.sh                 # or: CGO_ENABLED=1 go build -o langPeanut ./cmd/langPeanut
./langPeanut benchmark       # fully offline, ~seconds, $0.00
```

This runs the same 10 adversarial test cases through three approaches — a naive regex tool, a single-prompt LLM baseline, and the full multi-agent pipeline — and prints a live comparison table. The `langPeanut` column and the naive-regex column are **live-measured on every run**, not hardcoded; only the zero-shot LLM column falls back to a labeled historical estimate if you don't supply a `GEMINI_API_KEY`. Full walkthrough: [langpeanut_local/REPRODUCE.md](langpeanut_local/REPRODUCE.md).

To see it work on something bigger than the 10 unit cases, point it at one of the four bundled example apps (real React/Next.js, Flutter, SwiftUI, and Android projects, not toy snippets):

```bash
./langPeanut run ./examples/nextjs-app
./langPeanut reset ./examples/nextjs-app   # restore to pristine state afterward
```

Or drive the whole platform conversationally — ask it to scan, translate, run an SEO pass, or list checkpoints, all in one session:

```bash
./langPeanut chat ./examples/flutter-app
```

Or exercise the independent SEO studio directly:

```bash
./langPeanut seo ./examples/nextjs-app --locales ja,de --goal traffic
```

Or in the browser, with zero build step (surfaces all three systems in one UI):

```bash
./langPeanut web
```

---

## 3. What to look for (mapped to typical judging criteria)

| Criterion | Where the evidence is |
|---|---|
| **Problem clarity & bottleneck** | [README §1](README.md#1-the-4-core-questions) — who has this problem, why naive approaches fail. |
| **Agentic architecture (not a single prompt)** | [README §3](README.md#3-three-agentic-systems-one-shared-core) — three cooperating systems, not one script: 8 agents in the localization engine, a 19-tool conversational copilot, a 5-agent SEO pipeline, all coordinated with DAGs/checkpointing rather than a single monolithic prompt. |
| **Measured improvement over a fair baseline** | [README §10](README.md#10-measured-improvement) and [langpeanut_local/CHANGELOG.md §2](langpeanut_local/CHANGELOG.md) — baseline vs. naive-regex vs. multi-agent, with a documented iteration-by-iteration progression (`Baseline → Iteration 1 → 2 → 3 → 4 → Final`) showing *which* change moved *which* metric. |
| **Reproducibility** | [langpeanut_local/REPRODUCE.md](langpeanut_local/REPRODUCE.md) — one command, offline, deterministic pipeline column. |
| **Self-correction / reflection loops** | The 4-Tier Verifier Critic (`langpeanut_local/pkg/agents/verifier_critic.go`) feeds structured diagnostics back to the translator/patch engine on failure; the Code Repair Agent (`pkg/agents/repair.go`) does the same against real compiler output (`tsc --noEmit`, `dart analyze`), only ever flagging what it introduced; the copilot itself (System B) falls back to deterministic keyword-based tool routing when the LLM planning step is unavailable. |
| **Human-in-the-loop safety** | TUI review queue (approve/skip/batch), atomic pre-run checkpoints, one-command rollback with byte-for-byte restoration — all independently reachable from the copilot's `manage_checkpoints` tool too. |
| **Hot take / what would you build differently next time** | [README §11](README.md#11-hot-takes--practical-insights) — the "Zero-Generation Principle," why AST matchers beat prompt engineering, why localization is a scoping problem not a translation problem, and why a chat copilot is only as trustworthy as the deterministic tools it's restricted to calling. |
| **Depth beyond the MVP** | [README §5](README.md#5-system-b--the-central-ai-copilot) and [§6](README.md#6-system-c--the-seo--growth-studio) — a 19-tool conversational control plane and a fully independent 5-agent SEO pipeline are not bullet-point extras, they're peer systems with their own architecture, sharing the localization engine's locale files and translation memory directly on disk. Plus: doctor, pruner, persona scout, TMX/XLIFF interop, a zero-build web Studio, offline local-model inference, and a hosted GitHub-bot cloud service. All of this is real, working code, not aspirational — see §5 below for how to confirm that. |
| **Engineering judgment under real constraints** | 174+ session entries in the changelog documenting actual failure modes hit against real production codebases (not just the benchmark), root-caused and fixed — e.g. apostrophes in JSX breaking naive syntax validators, Flutter ARB files being misidentified as Dart source, translation memory cache pollution, first-launch latency from a blocking network call in the TUI constructor. |

---

## 4. Architecture at a glance

```
System A: Localization Engine                    System C: SEO & Growth Studio
Supervisor Orchestrator (DAG, checkpoints)        StudioOrchestrator (pkg/seo/)
   │                                                 │
   ├─► AST Scout — finds strings, filters noise      ├─► SERP Scout — competitor discovery
   ├─► Semantic Context Agent — disambiguates         ├─► Keyword Intelligence — volume/intent
   ├─► AST Range Patch Engine — byte-offset refactor  ├─► Semantic Copy Weaver — ICU-safe rewrite
   ├─► Cultural Translator — ICU/plural-safe           ├─► SERP Simulator — mock search preview
   └─► 4-Tier Verifier Critic — syntax/ICU/parity      └─► Growth Predictor Critic — CTR/traffic
          │ (fail → diagnostic feedback, retry)
          ▼                                        Both read/write the SAME locale files on disk —
   Code Repair Agent — fixes compiler regressions   no export/import step between A and C.
          ▼
   Human Checkpoint (TUI/web) ── or ── GitHub PR (langpeanut-cloud)

                    ▲                                        ▲
                    └──────────────┬─────────────────────────┘
                                    │
                     System B: Central AI Copilot (pkg/chat + pkg/genkit)
                     19 tools spanning A, C, checkpoints, config, diagnostics,
                     and the cloud job queue — one conversational control plane.
```

Every one of these is a real Go package (`langpeanut_local/pkg/agents/`, `pkg/orchestrator/`, `pkg/platforms/`, `pkg/chat/`, `pkg/seo/`), not a prompt template — see the file map in [langpeanut_local/PLAN.md §1](langpeanut_local/PLAN.md#1-system-architecture--module-structure) and the full three-system breakdown in [README §3](README.md#3-three-agentic-systems-one-shared-core)–[§6](README.md#6-system-c--the-seo--growth-studio).

---

## 5. Proving the "beyond MVP" surface is real, not decorative

It's fair for a judge to be skeptical that a hackathon project has *all* of: a chat copilot, an SEO studio, offline local models, and a hosted cloud bot. Here's how to check each in under a minute (run from `langpeanut_local/`):

```bash
go build ./...            # whole module compiles cleanly, including all of the above
go test ./...             # unit tests across pkg/agents, pkg/chat, pkg/seo, pkg/github, pkg/platforms, ...
langPeanut --help          # full command list: chat, seo, doctor, prune, persona, models, stats, test-model, export/import, ...
langPeanut chat --help
langPeanut seo --help
langPeanut models list
```

Every command listed in [README §4.3](README.md#43-full-command-surface) corresponds 1:1 to a `cmd/langPeanut/*.go` Cobra command wired to a package under `pkg/`. Nothing is a stub returning canned output — the SEO studio has its own 5-agent pipeline (`pkg/seo/`: scout → keywords → weaver → simulator → critic) independent of the localization agents, and the chat copilot's 19 tools (`pkg/chat/tools.go`) each call into the real agent code, not a mocked response.

---

## 6. Honest caveats (things we'd flag ourselves before you find them)

- **The zero-shot LLM baseline column** in the benchmark is a labeled historical estimate unless you supply a `GEMINI_API_KEY` — we chose to keep the fully-offline path as the default rather than force a judge to configure an API key just to run the benchmark. Add the key to `.env` if you want that column live-measured too (see [langpeanut_local/REPRODUCE.md §3](langpeanut_local/REPRODUCE.md#3-running-the-10-case-adversarial-benchmark)).
- **No top-level LICENSE file** currently exists in this monorepo (only third-party vendored grammars carry their own licenses under `langpeanut_local/pkg/platforms/thirdparty/`) — the README badge says MIT as the intended license; treat it as declared-but-not-yet-committed.

---

## 7. Where everything lives

| Need | File |
|---|---|
| Full README, feature list, architecture | [README.md](README.md) |
| Complete architecture diagrams (Local & Cloud) | [architecture-diagram.md](architecture-diagram.md) |
| Install locally, or deploy the cloud bot | [INSTALL.md](INSTALL.md) |
| Reproduce the benchmark exactly | [langpeanut_local/REPRODUCE.md](langpeanut_local/REPRODUCE.md) |
| Full chronological build log (170+ entries, every directive → root cause → fix → verification) | [langpeanut_local/CHANGELOG.md](langpeanut_local/CHANGELOG.md) → [CHANGELOG1.md](langpeanut_local/CHANGELOG1.md) → [CHANGELOG2.md](langpeanut_local/CHANGELOG2.md) |
| 5-minute video script | [langpeanut_local/DEMO_SCRIPT.md](langpeanut_local/DEMO_SCRIPT.md) |
| Original product spec / hackathon alignment | [langpeanut_local/idea.md](langpeanut_local/idea.md) |
| Cloud/GitHub-bot architecture in full | [langpeanut_local/cloud_plan.md](langpeanut_local/cloud_plan.md), [langpeanut-cloud/README.md](langpeanut-cloud/README.md) |
| Per-case agent reasoning traces | [langpeanut_local/trajectories/](langpeanut_local/trajectories/) |
