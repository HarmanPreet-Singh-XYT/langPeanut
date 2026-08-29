# 🥜 langPeanut — Universal Multi-Agent Localization Workflow

[![micro1 Hackathon](https://img.shields.io/badge/micro1-Agentic%20Workflows%20Hackathon-purple.svg)](https://micro1.ai)
[![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Benchmark](https://img.shields.io/badge/10--Case%20Benchmark-100%25%20Pass-brightgreen.svg)](benchmark/)

> **A universal, multi-agent AI system that automates end-to-end software localization across any mobile, web, or backend framework with deterministic AST precision, self-correcting reflection loops, and zero-defect code refactoring.**

---

## 1. The 4 Core Questions

| # | Question | Answer |
|---|---|---|
| **01** | **Who has this problem?** | Mobile, web, and backend software developers, product teams, and open-source maintainers worldwide who need to internationalize their apps. |
| **02** | **What bottleneck makes it worth solving?** | Retrofitting localization onto an existing codebase is notoriously tedious and manual. Developers must manually find hardcoded strings across thousands of lines of code, write boilerplate locale files (`.arb`, `.json`, `.xcstrings`, `strings.xml`), and replace raw strings with framework calls. Naive regex and raw LLMs hallucinate syntax, delete comments, mangle nested JSX/widget trees, and corrupt ICU variable placeholders (`{userName}`). |
| **03** | **Does the agent solve it well?** | **Yes.** `langPeanut` achieves a **100% compilation pass rate** and **0.0% formatting drift** across our 10-case adversarial benchmark suite by pairing real tree-sitter AST parsers (TSX, Dart, Swift, Kotlin) with a 4-tier verification critic and deterministic byte-range patching. |
| **04** | **Can another person reproduce the result?** | **Yes.** With a single command (`./langPeanut benchmark`), reviewers can run the complete 10-case evaluation harness from a clean environment in a few seconds at $\$0.00$ cost offline (add a `GEMINI_API_KEY` to `.env` to also live-measure the zero-shot LLM baseline). See [REPRODUCE.md](file:///Users/harmanpreetsingh/Public/Code/langTranslate/REPRODUCE.md). |

---

## 2. Multi-Agent System Architecture

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

### The 6 Specialized Agents:
1. **Supervisor Orchestrator Agent**: Manages the execution DAG, token budget packing, session resumption, and automatic pre-run snapshot checkpoints.
2. **AST Scout Extractor Agent (Tool-Use)**: Uses real tree-sitter grammars per platform (TSX/JSX for React, Dart for Flutter, Swift, Kotlin for Jetpack Compose — see [pkg/platforms](pkg/platforms/)) to isolate UI string literals while auto-skipping non-translatable code (logging, routes, URLs, hex colors, regexes) using the actual syntax tree rather than text patterns.
3. **Semantic Context & Disambiguation Agent**: Ingests surrounding component hierarchy and sibling strings to disambiguate polysemous words (e.g. `"Book"` in travel $\rightarrow$ `reserveFlightBtn`).
4. **Deterministic AST Range Patch Engine**: Computes exact byte offsets to refactor source files cleanly without rewriting untouched code, preserving 100% of comments and formatting.
5. **Specialized Cultural Translator Agent**: Translates strings across locales with Translation Memory (TM) while strictly preserving ICU syntax (`{name}`, `{count, plural, ...}`).
6. **4-Tier Verification Critic Agent (Reflection Loop)**: Validates AST syntax, ICU variable parity, character expansion clipping risks, and cross-locale key parity. Feeds structured diagnostics back to the agents for automated self-correction before human review.

---

## 3. Measured Improvement (micro1 Rubric — 15 Points)

`./langPeanut benchmark` live-measures two of the three comparison columns on every run — nothing below is a fixed constant unless labeled as an estimate:

- **Naive Regex Tool**: a real regex-only extractor (every quoted string, zero context-awareness) is run, patched, and re-parsed with the same tree-sitter grammars as the real pipeline to get a genuine pass/fail signal.
- **Simple Baseline (Zero-Shot LLM)**: when a `GEMINI_API_KEY` is configured, a real single-prompt call is made per case and the response is validated the same way. Without a key, the column falls back to a clearly labeled historical estimate from earlier manual runs instead of a live call.

| Metric | Simple Baseline (Single-Prompt Zero-Shot) | Naive Regex Tool | Final Multi-Agent Workflow (`langPeanut`) |
| :--- | :--- | :--- | :--- |
| **AST Compilation Pass Rate** | *0–42%* (live-measured when API key present; historical estimate otherwise) | *~20%* (live-measured — only the simplest single-string-literal cases survive) | **100.0%** (live-measured, all 10 cases) |
| **False-Positive Extraction Rate** | not scored (LLM output isn't candidate-tagged) | live-measured against known-bad content (URLs, SQL, hex, regex) per run | live-measured against the same known-bad-content check |
| **ICU Placeholder Parity** | live-measured: checks every source `{var}`/`$var`/`${expr}`/`\(expr)` placeholder survives byte-for-byte in the LLM's rewrite | not applicable (no locale files generated) | **100%** (4-Tier Critic Tier 2 check) |
| **Source Code Comment/Format Drift** | not measured | not measured | **0%** — byte-range patches never touch unrelated text |
| **Token Cost per 1k LoC** | full file sent to the LLM every call | 0 tokens (no LLM call) | AST Scout filters non-UI code before any LLM call is made |

Run `./langPeanut benchmark` yourself to see the live numbers for your environment — they vary run to run for the LLM column (real API non-determinism) and are worth re-checking rather than treating this table as final.

---

## 4. Hot Take & Practical Insights (micro1 Rubric — 5 Points)

1. **The "Zero-Generation" Principle for Code Refactoring Agents**: Never let an LLM rewrite full code files. Use the LLM only for structured patch decisions and delegate all file mutations to a deterministic AST patch engine that computes exact byte offsets.
2. **Linters and AST Matchers Beat Prompt Engineering Every Time**: Don't fight model stochasticity with 500-token prompt rules. Build lightweight programmatic AST critics that compare variable token sets and trigger automated self-correction loops.
3. **Localization is an AST Boundary Problem, Not a Translation Problem**: Real-world localization fails because of component scoping, `BuildContext` injection, and XML/JSX entity escaping. Localization tools must invest $80\%$ of engineering effort into static AST analysis and $20\%$ into translation.

---

## 5. Quickstart & Benchmark Execution

```bash
# 1. Build the binary
go build -o langPeanut ./cmd/langPeanut

# 2. Run the 10-case benchmark
./langPeanut benchmark

# 3. Audit any project directory
./langPeanut audit --dir /path/to/project

# 4. Translate codebase into target locales
./langPeanut translate --dir /path/to/project --locales fr,es,de,ja
```

---

## 6. Repository Documentation
* 📖 [REPRODUCE.md](file:///Users/harmanpreetsingh/Public/Code/langTranslate/REPRODUCE.md) — Step-by-step reproduction instructions.
* 📝 [CHANGELOG.md](file:///Users/harmanpreetsingh/Public/Code/langTranslate/CHANGELOG.md) — Live interaction history & improvement progression.
* 🤖 [AGENTS.md](file:///Users/harmanpreetsingh/Public/Code/langTranslate/AGENTS.md) — Agent operating guidelines & protocols.
* 📋 [PLAN.md](file:///Users/harmanpreetsingh/Public/Code/langTranslate/PLAN.md) — Implementation milestones & technical design.
* 💡 [idea.md](file:///Users/harmanpreetsingh/Public/Code/langTranslate/idea.md) — Product specification & Hackathon alignment.
* 📂 [trajectories/](file:///Users/harmanpreetsingh/Public/Code/langTranslate/trajectories/) — Exported agent reasoning and tool traces.
