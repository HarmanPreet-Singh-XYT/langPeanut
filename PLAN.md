# langPeanut — Implementation & Execution Plan

> **micro1 Agentic Workflows Hackathon Submission Plan**  
> Target: Production-ready Universal Multi-Agent Localization CLI with 10-Case Adversarial Benchmark Suite, Trajectory Logging, and Self-Correction Reflection Loops.

---

## 1. System Architecture & Module Structure

```
langTranslate/
├── cmd/
│   └── langPeanut/             # Main CLI entry point (Cobra commands)
│       ├── main.go
│       ├── root.go             # Root flags & global configs
│       ├── init.go             # Project detection & setup
│       ├── audit.go            # Scan & health reporting
│       ├── extract.go          # Extraction & Interactive TUI
│       ├── refactor.go         # Deterministic AST Patch execution
│       ├── translate.go        # Translation & Verifier loop
│       ├── rollback.go         # Checkpoint rollback & diff
│       └── benchmark.go        # 10-case evaluation benchmark runner
│
├── pkg/
│   ├── orchestrator/           # Supervisor Agent & execution DAG
│   │   ├── supervisor.go       # DAG coordinator & session state
│   │   ├── checkpoint.go       # Snapshots & diff rollback manager
│   │   └── session.go          # Resumable session state (.langPeanut/session/)
│   │
│   ├── agents/                 # Specialized Agent Implementations
│   │   ├── ast_scout.go        # AST Scout Agent (tree-sitter wrapper)
│   │   ├── context_agent.go    # Semantic Context & Disambiguation Agent
│   │   ├── patch_engine.go     # Deterministic Byte-Range AST Patch Engine
│   │   ├── translator.go       # Cultural & ICU Translator Agent
│   │   └── verifier_critic.go  # 4-Tier Critic & Reflection loop
│   │
│   ├── platforms/              # Framework Platform Plugins
│   │   ├── platform.go         # Platform interface definition
│   │   ├── react_ts.go         # React / Next.js (TypeScript / JSX)
│   │   ├── flutter_dart.go     # Flutter (Dart + ARB)
│   │   ├── swift_ios.go        # SwiftUI / iOS (.xcstrings / .strings)
│   │   ├── kotlin_android.go   # Jetpack Compose / Android (strings.xml)
│   │   └── generic.go          # Generic fallback format
│   │
│   ├── memory/                 # Translation Memory & Permanent Caches
│   │   ├── memory.go           # TM hash lookup and storage
│   │   └── cache.go            # LLM classification cache
│   │
│   ├── tui/                    # Terminal User Interface (Bubble Tea & Lip Gloss)
│   │   ├── review_view.go      # Interactive string candidate reviewer
│   │   ├── progress_view.go    # Multi-locale live progress meter
│   │   └── diff_view.go        # In-terminal colorized diff preview
│   │
│   ├── llm/                    # Multi-LLM Provider Client
│   │   ├── client.go           # Provider interface (OpenAI, Claude, Gemini, DeepL)
│   │   ├── batcher.go          # Token-budget batch manager
│   │   └── ratelimit.go        # Proactive rate-limiter & 429 jitter backoff
│   │
│   └── trajectory/             # Hackathon Trajectory Logger
│       └── logger.go           # Structured JSON/Markdown trajectory exporter
│
├── benchmark/                  # 10-Case Adversarial Benchmark Suite
│   ├── cases/                  # Test input code files
│   │   ├── 01_react_nested_jsx.tsx
│   │   ├── 02_react_ambiguous_verbs.tsx
│   │   ├── 03_react_ternary_plurals.tsx
│   │   ├── 04_flutter_const_tree.dart
│   │   ├── 05_flutter_complex_icu.arb
│   │   ├── 06_flutter_mixed_logging.dart
│   │   ├── 07_swift_format_specifiers.swift
│   │   ├── 08_android_xml_entities.xml
│   │   ├── 09_massive_analytics_dashboard.tsx
│   │   └── 10_adversarial_code_trap.tsx
│   ├── ground_truth/           # Expected perfect outputs
│   └── runner.go               # Automated evaluator comparing Baseline vs langPeanut
│
├── trajectories/               # Saved run trajectories for submission
├── REPRODUCE.md                # Step-by-step reproduction instructions for judges
├── go.mod
└── go.sum
```

---

## 2. Implementation Milestones

### Milestone 1: Core Scaffolding & AST Scout Tooling
- [ ] Initialize Go module (`go.mod`) with dependencies (`cobra`, `bubbletea`, `lipgloss`, `go-tree-sitter`, `go-git`).
- [ ] Implement `Platform` plugin interface with support for:
  - React/Next.js (TSX/JSX $\rightarrow$ i18next)
  - Flutter (Dart $\rightarrow$ ARB)
  - SwiftUI (Swift $\rightarrow$ `.xcstrings`)
  - Android (Kotlin $\rightarrow$ `strings.xml`)
- [ ] Implement AST Scout Agent (`ast_scout.go`) targeting UI nodes (`Text()`, JSX text, props) and auto-skipping non-UI nodes (`debugPrint`, URLs, regexes, routes, hex colors).

### Milestone 2: Context Disambiguation & Semantic Key Agent
- [ ] Implement `context_agent.go`:
  - Component hierarchy breadcrumb extractor.
  - Sibling vector clustering for domain inference (e.g. travel booking vs book reading).
  - Semantic key naming algorithm (`flightBookBtn` vs random hash).
- [ ] Implement Token-Budget `BatchManager` and `RateLimitTracker` with exponential jitter backoff.

### Milestone 3: Deterministic AST Range Patch Engine
- [ ] Implement `patch_engine.go`:
  - In-place byte-range replacement engine (preserves comments, untouched formatting).
  - AST import injector (e.g., `import { useTranslation } from 'react-i18next'`).
  - Safe Dart `const` keyword stripper.
  - In-memory AST compiler verification before disk write.

### Milestone 4: Cultural Translator & 4-Tier Critic Reflection Loop
- [ ] Implement `translator.go` with ICU syntax and placeholder preservation.
- [ ] Implement `verifier_critic.go` with 4 verification tiers:
  - Tier 1: Syntax & AST Parse check.
  - Tier 2: ICU & Variable Token Alignment check (`{name}` / `%@` count & name parity).
  - Tier 3: UI Character Expansion / Layout Critic.
  - Tier 4: Cross-Locale Key Parity diff.
- [ ] Implement closed-loop self-correction feedback loop (Critic $\rightarrow$ Translator retry).
- [ ] Implement Translation Memory (`memory/`) cache across runs.

### Milestone 5: Supervisor Orchestrator, Checkpoints & Interactive TUI
- [ ] Implement pre-run snapshots & stage checkpoints (`checkpoint.go`) for 1-command rollback.
- [ ] Build Bubble Tea TUI (`tui/`) for human approval, candidate review, and live translation meters.
- [ ] Implement structured `trajectory/logger.go` saving step-by-step logs for Deliverable 04.

### Milestone 6: 10-Case Adversarial Benchmark & Submission Assets
- [ ] Construct the 10 adversarial test cases under `benchmark/cases/`.
- [ ] Implement `benchmark.go` to execute:
  1. Simple Baseline (Single-Prompt zero-shot LLM).
  2. Naive Regex Baseline.
  3. `langPeanut` Multi-Agent Workflow.
- [ ] Generate comparative metrics table and trajectory dumps (`/trajectories/`).
- [ ] Write `REPRODUCE.md` with exact one-line commands for judges.

---

## 3. Evaluation Metrics & Success Criteria

| Evaluation Criterion | Target Goal |
| :--- | :--- |
| **AST Compilation Pass Rate** | $100\%$ on all 10 benchmark cases |
| **False-Positive Extraction Rate** | $<1.5\%$ on adversarial code files |
| **ICU Placeholder Integrity** | $100\%$ exact variable token matching across all locales |
| **Token Reduction** | $>80\%$ token savings over raw code prompt baseline |
| **Self-Correction Success Rate** | $>90\%$ of critic-flagged issues resolved in $\le 2$ retries |
| **Rollback Reliability** | $100\%$ clean byte-for-byte state restoration |
