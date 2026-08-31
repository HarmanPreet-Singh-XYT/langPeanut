# 🥜 langPeanut — Universal Multi-Agent Localization & Growth Platform

[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](https://golang.org)
[![Benchmark](https://img.shields.io/badge/10--Case%20Benchmark-100%25%20Pass-brightgreen.svg)](langpeanut_local/benchmark/)

> **A multi-system agentic platform, not a single pipeline: a 6-agent localization engine that refactors real codebases with byte-exact AST precision, a central AI copilot that can drive the entire toolchain conversationally, and an independent SEO growth studio — all sharing one Go core, reachable from CLI, TUI, a zero-build web Studio, or a hosted GitHub bot.**

Built for the **Agentic Workflows Hackathon**. What started as a single localization pipeline grew into three cooperating agentic systems (§3 below). This monorepo has two parts:

- **[`langpeanut_local/`](langpeanut_local/)** — the CLI, interactive TUI, and zero-build web Studio. Single Go binary. Start here.
- **[`langpeanut-cloud/`](langpeanut-cloud/)** — turns the same engine into a hosted GitHub App (bot that clones repos and opens localization PRs). See [§12](#12-langpeanut-cloud-the-hosted-github-bot).

---

## 1. The 4 Core Questions

| # | Question | Answer |
|---|---|---|
| **01** | **Who has this problem?** | Mobile, web, and backend developers, product teams, and open-source maintainers who need to internationalize an existing codebase — and then keep it healthy, translated, discoverable, and up to date, not just localized once. |
| **02** | **What bottleneck makes it worth solving?** | Retrofitting localization is manual and error-prone: developers must hunt down hardcoded strings across thousands of lines, hand-write locale catalogs (`.arb`, `.json`, `.xcstrings`, `strings.xml`), and rewrite source code to call the right framework API. Naive regex and single-prompt LLMs make it worse — they hallucinate syntax, delete comments, mangle nested JSX/widget trees, and corrupt ICU placeholders (`{userName}`). And once localized, projects still need someone to keep locale keys clean, catch missing i18n setup, and make the translated content actually rank in local search — work that's normally three separate tools. |
| **03** | **Does the agent solve it well?** | **Yes.** The core refactor pipeline achieves a **100% compilation pass rate** and **0.0% formatting drift** across a 10-case adversarial benchmark by pairing real tree-sitter AST parsers (TSX, Dart, Swift, Kotlin) with a 4-tier verification critic and deterministic byte-range patching — never letting an LLM rewrite a whole file. That same rigor extends outward: a central agentic copilot with 19 tools can drive the whole platform conversationally, and an independent 5-agent SEO studio optimizes the translated copy for local search — both built on the identical "deterministic tool + narrow LLM judgment" pattern. |
| **04** | **Can another person reproduce the result?** | **Yes.** `./langPeanut benchmark` runs the full 10-case evaluation harness from a clean checkout in seconds, entirely offline, at $0.00. See [langpeanut_local/REPRODUCE.md](langpeanut_local/REPRODUCE.md). |

---

## 2. Quickstart

```bash
cd langpeanut_local

# 1-click install: checks Go, builds the binary, installs to PATH, initializes .env
./install.sh
# or: make install

# Launch the interactive terminal app (Bubble Tea TUI)
langPeanut

# Launch the zero-build browser Studio (pure Go HTTP server, no Node/Next.js needed)
langPeanut web

# Talk to the ENTIRE platform in plain English — localization, SEO, checkpoints, diagnostics
langPeanut chat

# 1-click autonomous localization on a bundled example app
langPeanut run ./examples/nextjs-app

# Optimize the translated copy for local search
langPeanut seo ./examples/nextjs-app --locales ja,de --goal traffic

# Run the 10-case adversarial benchmark
langPeanut benchmark
```

No API key is required to try any of the above — see [§9 Zero-Cost Offline Mode](#9-zero-cost-offline-mode-no-api-key-required). Full install instructions (including the cloud/VPS path): [INSTALL.md](INSTALL.md).

---

## 3. Three Agentic Systems, One Shared Core

This is the most important thing to understand about the project's current state: **langPeanut is not one pipeline with extra commands bolted on — it's three independent agentic systems that all read/write the same project state**, so anything one does is immediately visible to the other two.

```
┌───────────────────────────────────────────────────────────────────────────────────┐
│                         pkg/types · pkg/platforms · pkg/memory · pkg/llm            │
│                    (shared AST layer, locale files, translation memory,             │
│                     multi-provider LLM client — every system below sits on this)    │
└───────────────────────────────────────────────────────────────────────────────────┘
        ▲                                  ▲                                   ▲
        │                                  │                                   │
┌───────┴────────────┐   drives all three  │  ┌─────────────────────────────────┴────┐
│ A. LOCALIZATION     │ ◄────────────────┐ │  │ C. SEO & GROWTH STUDIO                │
│    ENGINE           │                  │ │  │ (pkg/seo/) — independent 5-agent      │
│ 6 pipeline agents +  │                  │ │  │ pipeline: Scout → Keywords → Weaver   │
│ Repair + Directive   │                  │ │  │ → Simulator → Growth Critic           │
│ + Doctor/Pruner/     │                  │ │  └────────────────────────────────────────┘
│ Persona (§4)         │                  │ │
└──────────┬───────────┘                  │ │
           │                     ┌────────┴─┴─────────┐
           │                     │ B. CENTRAL AI       │
           └────────────────────►│    COPILOT          │
                                  │ (pkg/chat + pkg/genkit)│
                                  │ 19 tools, one         │
                                  │ conversational surface│
                                  │ over A + C + jobs (§5) │
                                  └────────────────────────┘
```

- **System A — the Localization Engine** (§4) is the original hackathon pipeline: scan, disambiguate, patch, translate, verify, repair. It's the deepest and most rigorously tested part of the platform.
- **System B — the Central AI Copilot** (§5) is a chat-driven control plane (`langPeanut chat`) that sits *on top of* the other two systems and the cloud job queue, with 19 registered tools spanning localization, SEO, checkpoints, config, and diagnostics. It is the "one interface to drive everything" layer — you can ask it to scan, translate, run an SEO pass, roll back a checkpoint, or trigger a cloud job, all in the same conversation.
- **System C — the SEO & Growth Studio** (§6) is a second, fully independent multi-agent pipeline that takes the localized output from System A and makes it competitive in local search — something no commercial localization tool (Lokalise, Crowdin, Phrase) touches at all.

All three are reachable identically from the CLI, the TUI, the zero-build web Studio, and (for A and C) the hosted GitHub bot in `langpeanut-cloud` — because they're Go packages imported as libraries, not separate services.

---

## 4. System A — The Localization Engine

The core pipeline (driven by `langPeanut run` / `translate` / the web Studio / the GitHub bot) is a 6-agent DAG coordinated by a Supervisor:

```
                              ┌───────────────────────────────────┐
                              │  Supervisor / Orchestrator Agent  │
                              │ (Session state, checkpoints & DAG)│
                              └─────────────────┬─────────────────┘
                                                │
       ┌──────────────────────┬─────────────────┼──────────────────┬──────────────────────┐
       │                      │                 │                  │                      │
┌──────▼──────┐        ┌──────▼──────┐   ┌──────▼──────┐    ┌──────▼──────┐        ┌──────▼──────┐
│  AST Scout  │        │  Semantic   │   │ AST Range   │    │ Specialized │        │ 4-Tier      │
│  Extractor  │───────►│  Context    │──►│ Patch Engine│───►│ Translator  │───────►│ Verification│
│ (Tree-Sitter│        │  Disambig.  │   │(Deterministic│   │ (ICU / ARB / │        │ & Reflection│
│    Tools)   │        │   Agent     │   │ Refactor)   │    │  Plurals)   │        │   Critic    │
└─────────────┘        └─────────────┘   └─────────────┘    └─────────────┘        └──────┬──────┘
                                                                                          │
                                                                   ┌──────────────────────┴──────────────┐
                                                                   │ Failed Check? Diagnostic Error Feed │
                                                                   │ (Auto-correction retry loop)        │
                                                                   └──────────────────────┬──────────────┘
                                                                                          │ Pass
                                                                                 ┌────────▼────────┐
                                                                                 │Human Checkpoint │
                                                                                 │ (TUI Approval)  │
                                                                                 └─────────────────┘
```

### 4.1 The 6 core agents (`langpeanut_local/pkg/agents/`, `pkg/orchestrator/`)

1. **Supervisor Orchestrator Agent** (`pkg/orchestrator/`, `pkg/agents/supervisor.go`) — runs the execution DAG, packs token budgets, persists resumable session state, and takes an automatic pre-run snapshot before touching any file.
2. **AST Scout Extractor Agent** (`pkg/agents/ast_scout.go`) — real tree-sitter grammars per platform (TSX/JSX, Dart, Swift, Kotlin — see `pkg/platforms/`) isolate UI string literals and auto-skip non-translatable code (`console.log`, routes, URLs, hex colors, regexes) using the actual syntax tree, not text patterns.
3. **Semantic Context & Disambiguation Agent** (`pkg/agents/context_agent.go`) — reads the surrounding component hierarchy and sibling strings to disambiguate polysemous words (e.g. `"Book"` in a travel app → `reserveFlightBtn`, not `readBookBtn`) and synthesizes clean camelCase keys.
4. **Deterministic AST Range Patch Engine** (`pkg/agents/patch_engine.go`) — computes exact byte offsets to refactor source files without rewriting untouched code, injects framework imports/hooks, and validates the in-memory AST before anything is written to disk.
5. **Specialized Cultural Translator Agent** (`pkg/agents/translator.go`) — translates with Translation Memory (TM) reuse across runs, strict ICU/plural/placeholder preservation, dynamic word-budget batching, and parallel per-language workers.
6. **4-Tier Verification Critic Agent** (`pkg/agents/verifier_critic.go`) — validates AST syntax, ICU variable parity, character-expansion/clipping risk, and cross-locale key parity; feeds structured diagnostics back for automated self-correction before anything reaches the human checkpoint.

Two more agents extend the pipeline past the original 6:

- **Autonomous Code Repair Agent** (`pkg/agents/repair.go`) — runs after a refactor: captures a pre-flight compiler baseline, diffs post-refactor diagnostics against it (so it only ever "blames" regressions it introduced), applies deterministic heuristic fixes first (e.g. a missing `useTranslation` import), escalates to a bounded LLM repair loop if needed, and — if it still can't fix something — flags it for human review instead of silently failing or corrupting the file.
- **Directive Agent** (`pkg/agents/directive_agent.go`) — executes free-form natural-language developer instructions after localization (e.g. *"add a language switcher to the navbar"*) using an outline-and-window strategy so it can safely operate on files with 10,000+ lines without reading them whole, running a bounded ReAct loop with Tree-sitter–validated patches and self-healing retries against compiler diagnostics.

### 4.2 Maintenance agents that keep a project healthy over time

Beyond the one-shot pipeline, three standalone agents treat localization as ongoing upkeep, not a single run:

- **Doctor** (`doctor`, `pkg/agents/doctor.go`) — a 0–100 project health score across framework config, i18n dependency completeness, and estimated hardcoded-string debt, with `--fix` to auto-bootstrap what's missing.
- **Pruner** (`prune`, `pkg/agents/pruner.go`) — AST-scans real code usage to find translation keys that exist in locale files but are no longer referenced anywhere, and safely deletes them (`--dry-run` to preview).
- **Persona Scout** (`persona`, `pkg/agents/persona_scout.go`) — zero-config brand-voice discovery: reads README/docs/manifests to infer tone, target audience, and a brand lexicon (terms that shouldn't be casually translated), feeding tone presets used by the translator and the copilot.

### 4.3 Full command surface

```
langPeanut init                        # detect framework, initialize config
langPeanut install                     # install/bootstrap i18n deps (react-i18next, flutter_localizations, ...)
langPeanut audit / scan                # read-only hardcoded-string & coverage report
langPeanut extract / pull              # AI-classified extraction + base locale generation
langPeanut refactor / apply            # deterministic AST byte-range patch
langPeanut translate / i18n            # 4-Tier-Critic-verified multi-locale translation
langPeanut run / all / auto            # 1-click: scan → filter → refactor → translate → repair
langPeanut rollback                    # atomic, byte-for-byte checkpoint restore
langPeanut reset / clean               # restore a project to pristine unlocalized source
langPeanut doctor [--fix]              # 0-100 project health score
langPeanut prune [--dry-run]           # dead translation-key garbage collection
langPeanut persona                     # brand tone/voice/glossary discovery
langPeanut export / import             # TMX 1.4b / XLIFF 1.2 interchange
langPeanut stats / cost                # token usage & USD cost analytics
langPeanut test-model / probe          # live provider/model connectivity test
langPeanut benchmark                   # 10-case adversarial evaluation suite
```

### 4.4 Translation memory & interoperability

- **Translation Memory (TM)** (`pkg/memory/`) — hash-based cache across runs/projects so repeated strings never re-spend tokens.
- **Industry-standard interchange** (`export`, `import`) — full **TMX 1.4b** and **XLIFF 1.2** support, so a project's translations can round-trip with Crowdin, Phrase, Lokalise, or Trados, and imported TM instantly warms the local cache for zero-cost hits.

---

## 5. System B — The Central AI Copilot

`langPeanut chat` (aliases: `copilot`, `ask`, `ai`) is not a bolt-on chatbot — it's a genuine **agentic control plane** (`pkg/chat/`) that can operate every other system in the platform through natural language, plus one no-code path the CLI/TUI don't otherwise expose: triggering and querying **cloud jobs**.

```bash
langPeanut chat                                   # current directory
langPeanut chat ./examples/nextjs-app --tone gen_z
langPeanut chat --provider ollama                 # fully offline, still tool-calling
```

### 5.1 How it works

`pkg/chat/engine.go`'s `Engine.SendMessage` runs an LLM-driven planning step (`planWithLLM`) that decides which tools to call, executes them through a `ToolRegistry`, and synthesizes a response — with a **deterministic keyword-based fallback** (`detectToolCallsFallback`) so the copilot still routes correctly even with no network access. Results render as rich structured cards (diff, critic report, cost breakdown, stepper, checkpoint list, SERP preview) in both the terminal and the web Studio, not just prose.

### 5.2 The 19 tools it can call (`pkg/chat/tools.go`)

| Category | Tools |
|---|---|
| **Localization (System A)** | `scan_repository`, `inspect_string_context`, `find_hardcoded_strings`, `plan_localization`, `execute_localization`, `verify_translations`, `apply_ast_patch`, `update_translation_key` |
| **SEO (System C)** | `seo_analyze_competitor`, `seo_simulate_serp`, `seo_weave_copy` |
| **Platform operations** | `manage_checkpoints`, `manage_config`, `diagnose_system`, `scout_personas`, `prune_dead_keys` |
| **Cloud / GitHub bot** | `trigger_job`, `query_jobs` |
| **Meta** | `explain_tool_or_concept` |

That table is the concrete proof of the architecture claim in §3: one conversational surface, zero duplicated logic, reaching into localization, SEO, safety/checkpoints, and the hosted job queue.

### 5.3 `pkg/genkit/` — internal, not a dependency

`pkg/genkit/` is a from-scratch Go package that wraps `pkg/chat.Engine` behind a Genkit-styled vocabulary (flows, tools, tracing) — used by the web Studio's chat-streaming and `/api/genkit/runtime` introspection endpoints. **It is not an integration with Google's actual Genkit SDK** — there's no such dependency in `go.mod`. It exists so the web Studio can expose flow/tool metadata in a shape a judge or developer familiar with Genkit will recognize, without taking on the real SDK.

---

## 6. System C — The SEO & Growth Studio

`langPeanut seo` (`pkg/seo/`) is a second, fully independent 5-agent pipeline. It picks up where translation stops: a perfectly translated app is still invisible in local search if the copy isn't optimized for how people actually search in that locale. No commercial localization tool (Lokalise, Crowdin, Phrase) does this — they stop at the translation file.

```
StudioOrchestrator.RunStudio(strategy, sourceKeys, baselineMatrix)
   │
   ├─► 1. SERP Scout Agent        — discovers real or AI-inferred competitor pages per locale
   ├─► 2. Keyword Intelligence    — volume/difficulty/intent-scored keyword insights
   ├─► 3. Semantic Copy Weaver    — rewrites locale copy to weave in keywords,
   │                                 preserving every ICU variable, respecting SERP pixel-width limits
   ├─► 4. SERP Simulator          — renders a mock Google result (title, meta description, URL)
   └─► 5. Growth Predictor Critic — scores/projects traffic & CTR uplift, trust signals
```

```bash
langPeanut seo [directory] --locales ja,de,es --goal traffic --scope high_impact [--apply]
```

`--apply` writes the SEO-optimized copy directly back into the locale files produced by System A — the two systems share the exact same locale catalog on disk, so there's no export/import step between them. The studio first runs the Persona Scout (§4.2) to auto-infer the project's category/audience before building its keyword strategy.

---

## 7. Zero-build web Studio & offline/local AI

### 7.1 Zero-build web Studio (`web`/`ui`/`studio`/`serve`)

A single Go binary serves an inline HTML/CSS/JS single-page app (`pkg/web/server.go`) — no Node, no npm install, no build step. It exposes ~40 REST endpoints covering all three systems above: project scan/switch/reset, candidate review, pipeline execution, diff/apply, locale editing, checkpoints/rollback, settings, local-model download/install/test, a repo tree view, a locale coverage matrix, git status, benchmark runs, dependency management, the SEO studio, and SSE-streamed chat.

```bash
langPeanut web              # http://localhost:3000 (auto-opens browser)
langPeanut web --port 4000 --open=false
```

### 7.2 Local, offline, zero-API-key AI (`models`)

langPeanut can run **completely offline with $0 cost**, using a bundled path to Meta's **NLLB-200** (No Language Left Behind) distilled 600M-parameter model in 4-bit GGUF form (~380MB), executed locally via `llama.cpp`, or by talking to a local **Ollama** daemon:

```bash
langPeanut models download        # fetch the ~380MB NLLB-200 GGUF from Hugging Face
langPeanut models install-runner  # installs llama.cpp via Homebrew, no sudo/root required
langPeanut models list            # show local model/runner status
langPeanut models path            # print the local model cache directory
```

If no `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `GEMINI_API_KEY` / `HF_TOKEN` is set, `AutoDetectClient()` silently falls back to this local path — the tool is fully demoable on a judge's laptop with no accounts, no signup, and no internet dependency once the model is cached. This applies to all three systems, including the chat copilot's tool-planning step.

---

## 8. Supported Platforms & Locale Formats

| Platform | Language | AST Parser | Locale Format | Refactor Pattern |
|---|---|---|---|---|
| **React / Next.js** | TS / JS / TSX | `tree-sitter-typescript` | i18next / next-intl JSON | `t('key')` |
| **Flutter** | Dart | `tree-sitter-dart` | ARB (`.arb`) | `AppLocalizations.of(context)!.key` |
| **SwiftUI / iOS** | Swift | `tree-sitter-swift` | `.xcstrings` / `.strings` | `Text("key")` / `Text(.key)` |
| **Jetpack Compose / Android** | Kotlin | `tree-sitter-kotlin` | `strings.xml` | `stringResource(R.string.key)` |
| **Generic fallback** | any | heuristic | JSON | pluggable (`pkg/platforms/generic.go`) |

Four bundled example apps exercise these platforms end-to-end: `langpeanut_local/examples/nextjs-app`, `examples/flutter-app`, `examples/swiftui-app`, `examples/android-app`.

---

## 9. Zero-Cost Offline Mode (no API key required)

Every read-only and refactor step (`audit`, `extract`, `refactor`, `doctor`, `persona`, `prune`) runs with **zero network calls** — pure tree-sitter AST analysis. Translation, the chat copilot, and the SEO studio *can* use a frontier model (Anthropic Claude, OpenAI, Google Gemini, DeepL) if you provide a key in `.env`, but if you don't, they transparently fall back to the local NLLB/llama.cpp or Ollama path described in [§7.2](#72-local-offline-zero-api-key-ai-models) — so the entire platform, end to end, is runnable at **$0.00** with no signup.

---

## 10. Measured Improvement

`./langPeanut benchmark` live-measures two of the three comparison columns on every run — nothing below is a fixed constant unless labeled an estimate. (This benchmark covers System A, the localization engine — see [REPRODUCE.md](langpeanut_local/REPRODUCE.md) for why that's the fairest apples-to-apples comparison point.)

| Metric | Simple Baseline (Zero-Shot LLM) | Naive Regex Tool | `langPeanut` Multi-Agent Workflow |
| :--- | :--- | :--- | :--- |
| **AST Compilation Pass Rate** | 0–42% (live if `GEMINI_API_KEY` set, else historical estimate) | ~20% (live-measured) | **100.0%** (live, all 10 cases) |
| **False-Positive Extraction Rate** | not scored | live-measured against known-bad content | live-measured against the same known-bad-content set |
| **ICU Placeholder Parity** | live-measured, checks every `{var}`/`$var`/`${expr}`/`\(expr)` | not applicable (no locale files generated) | **100%** (4-Tier Critic Tier 2) |
| **Source Code Comment/Format Drift** | not measured | not measured | **0%** — byte-range patches never touch unrelated text |
| **Token Cost per 1k LoC** | full file sent to the LLM every call | 0 tokens (no LLM call) | AST Scout filters non-UI code before any LLM call |

Run `./langPeanut benchmark` yourself — the LLM baseline column is genuinely non-deterministic run to run; treat this table as a snapshot, not a promise.

---

## 11. Hot Takes & Practical Insights

1. **The "Zero-Generation" Principle for Code Refactoring Agents** — never let an LLM rewrite a full code file. Use the LLM only for structured patch decisions (key + translation) and delegate every file mutation to a deterministic AST patch engine computing exact byte offsets. This same principle is why `pkg/github/pr_template.go` in `langpeanut-cloud` builds PR titles/bodies with zero LLM calls too.
2. **Linters and AST matchers beat prompt engineering every time** — hours spent refining "don't translate `{count}`" prompt rules still leaked 15–25% of the time on low-resource languages. A 30-line deterministic AST token matcher comparing placeholder sets, feeding structured errors back into a retry loop, hit 100% reliability instead.
3. **Localization is an AST boundary problem, not a translation problem** — real-world localization fails on component scoping, `BuildContext` injection, and XML/JSX entity escaping, not on linguistic quality. The winning allocation of engineering effort is roughly 80% static AST analysis, 20% translation.
4. **A conversational copilot is only as trustworthy as the tools it calls** — `pkg/chat`'s 19 tools all delegate to the exact same deterministic agents the CLI calls directly; the LLM never gets to invent a file mutation, only to pick which existing, already-verified tool to invoke. The chat layer adds convenience, not a new trust boundary.

---

## 12. `langpeanut-cloud`: the hosted GitHub bot

[`langpeanut-cloud/`](langpeanut-cloud/) embeds this project's `pkg/agents`/`pkg/platforms`/`pkg/llm`/`pkg/seo` **as a Go library** (not a subprocess) to run all three systems above as a self-hosted GitHub App:

- A team installs the GitHub App, picks a repo from the ones the installation can see, and configures locales/tone/provider — the same knobs as the CLI wizard.
- Clicking "Run" (or, in v2, a push webhook) clones the repo into an **ephemeral, sandboxed Docker container** with only that job's scratch volume, LLM key, and a scoped git token — no access to the database, the Docker socket, or the App's private key.
- The pipeline runs, commits, and pushes; the trusted host process then opens a PR with a **deterministically templated** title/body/labels (zero LLM spend on prose) via `pkg/github/pr_template.go`. The PR always opens, success or partial-failure — repair-agent failures become a `needs-manual-review` label and a review comment, never a blocked PR.
- Single-VPS deployment: one `docker compose up`, SQLite (WAL mode) as the only datastore, the `jobs` table doubling as the queue, Caddy for automatic HTTPS.
- The `langpeanut-cloud/web` dashboard (Next.js 15 + Tailwind) adds its own agentic **copilot chat** (multi-turn memory, 19+ platform tools, live model/provider switching), the SEO studio, and settings UI on top of the same engine — this is the same System B copilot described in §5, reachable over HTTP instead of the terminal.

See [`langpeanut-cloud/README.md`](langpeanut-cloud/README.md) and [`langpeanut-cloud/DEPLOYMENT.md`](langpeanut-cloud/DEPLOYMENT.md) for the full architecture and VPS setup.

---

## 13. Repository Documentation

* 📥 [INSTALL.md](INSTALL.md) — full install guide (local binary, Docker/VPS cloud deploy).
* 🏛️ [JUDGES.md](JUDGES.md) — everything judges need in one place: problem, proof, architecture, how to verify.
* 📖 [langpeanut_local/REPRODUCE.md](langpeanut_local/REPRODUCE.md) — step-by-step reproduction instructions for judges.
* 📝 [langpeanut_local/CHANGELOG.md](langpeanut_local/CHANGELOG.md) / [CHANGELOG1.md](langpeanut_local/CHANGELOG1.md) / [CHANGELOG2.md](langpeanut_local/CHANGELOG2.md) — chronological, session-by-session interaction and improvement log (170+ entries).
* 🤖 [langpeanut_local/AGENTS.md](langpeanut_local/AGENTS.md) — agent operating guidelines & protocols for this repo.
* 📋 [langpeanut_local/PLAN.md](langpeanut_local/PLAN.md) — original implementation milestones & technical design.
* ☁️ [langpeanut_local/cloud_plan.md](langpeanut_local/cloud_plan.md) — architecture plan for `langpeanut-cloud`.
* 💡 [langpeanut_local/idea.md](langpeanut_local/idea.md) — product specification & hackathon alignment.
* 🎬 [langpeanut_local/DEMO_SCRIPT.md](langpeanut_local/DEMO_SCRIPT.md) — 5-minute video walkthrough script.
* 📂 [langpeanut_local/trajectories/](langpeanut_local/trajectories/) — exported agent reasoning and tool traces per benchmark case.
