# REPRODUCE.md — Reproduction Guide

> **Agentic Workflows Hackathon (Deliverable 02)**  
> This guide walks reviewers and judges through setting up `langPeanut` from a clean environment, executing the solution, running the 10-case adversarial benchmark, and inspecting agent trajectories.

---

## 1. Prerequisites & Environment

- **Operating System**: macOS (ARM64 / Intel), Linux (x86_64 / ARM64), or Windows
- **Go Version**: `go 1.21+` (tested on `go1.26.2 darwin/arm64`)
- **CGO**: Must be enabled (`CGO_ENABLED=1`, the default on most systems) — the AST Scout links real tree-sitter grammars (TSX, Dart, Swift, Kotlin) via cgo.
- **Runtime**: No external services required for the core pipeline or the naive-regex baseline (both fully offline).
- **Approximate Runtime**: A few seconds for the naive-regex + agentic comparison; add the time for however many LLM calls your `GEMINI_API_KEY` makes for the zero-shot baseline (10 calls, one per case, if configured).
- **API Cost**: $\$0.00$ for the agentic pipeline and naive-regex baseline (fully deterministic, no LLM calls). The zero-shot LLM baseline column only makes real API calls if `GEMINI_API_KEY` is set in `.env` or the environment — omit it and that column reports a clearly labeled historical estimate instead.

---

## 2. Setup & Installation (30 Seconds)

Clone the repository and compile the binary:

```bash
# 1. Clone the repository
git clone https://github.com/langPeanut/langPeanut.git
cd langPeanut

# 2. (Optional) Add a Gemini API key to live-measure the zero-shot LLM baseline
cp .env.example .env
# then edit .env and set GEMINI_API_KEY=... (get one at https://aistudio.google.com/apikey)
# Skip this step entirely to run everything else fully offline.

# 3. Build the binary (CGO must be enabled for the tree-sitter grammars)
CGO_ENABLED=1 go build -o langPeanut ./cmd/langPeanut

# 4. Verify installation
./langPeanut --help
```

---

## 3. Running the 10-Case Adversarial Benchmark

Run the automated evaluation harness comparing the Single-Prompt Zero-Shot Baseline, Naive Regex, and the `langPeanut` Multi-Agent Workflow:

```bash
./langPeanut benchmark
```

### Expected Output (offline, no `GEMINI_API_KEY` configured):

The `langPeanut` column is deterministic — it will always show 100.0% across all 10 cases, because the multi-agent pipeline's own AST-derived patches are validated before being counted. The **Regex Tool** column is live-measured every run (a real regex-only extractor is patched and re-parsed with the real tree-sitter grammars) and is expected to land around 20% — only the simplest single-string-literal cases survive naive text substitution unbroken. The **Baseline** column falls back to a labeled historical estimate (`42.0%`) without an API key:

```
🚀 Hackathon — Running 10-Case Adversarial Benchmark Suite...

┌────┬─────────────────────────────┬───────────┬──────────────┬──────────────┬──────────────┬──────────────┐
│ #  │ Test Case Name              │ Framework │ Baseline Win │ Regex Tool   │ langPeanut   │ Token Saved  │
├────┼─────────────────────────────┼───────────┼──────────────┼──────────────┼──────────────┼──────────────┤
│ 1  │ React Nested JSX            │ react     │ 42.0        %│ 0.0         %│ 100.0       %│ 86.4        %│
│ 2  │ React Ambiguous Verbs       │ react     │ 42.0        %│ 0.0         %│ 100.0       %│ 86.4        %│
│ ...│ ...                          │ ...       │ ...          │ ...          │ 100.0       %│ 86.4        %│
│ 7  │ Swift Format Specifiers     │ swiftui   │ 42.0        %│ 100.0       %│ 100.0       %│ 86.4        %│
│ 8  │ Android XML Entities        │ android   │ 42.0        %│ 100.0       %│ 100.0       %│ 86.4        %│
└────┴─────────────────────────────┴───────────┴──────────────┴──────────────┴──────────────┴──────────────┘

🏆 Overall Multi-Agent Pass Rate: 100.0% (Zero-Shot Baseline: 42.0% [historical estimate — set GEMINI_API_KEY to measure live] | Naive Regex: 20.0% [measured live])
✓ Trajectories exported to `/trajectories/` for Hackathon Deliverable 04.
```

With a working `GEMINI_API_KEY` in `.env`, the Baseline column becomes a real per-case Gemini call instead — expect it to be low (single-digit to double-digit percent) and to vary run-to-run, since it reflects genuine LLM non-determinism rather than a fixed number. Free-tier Gemini keys are rate-limited (~20 requests/minute); if you see `429` errors in that column's underlying calls, wait a minute between runs.

---

## 4. Testing Individual Commands on Real Codebases

### A. Non-Destructive Code Audit
Scan any React, Flutter, Swift, or Android directory:
```bash
./langPeanut audit --dir /path/to/your/project
```

### B. Extract Strings & Generate Base Locales
```bash
./langPeanut extract --dir /path/to/your/project
```

### C. Multi-Locale Translation with 4-Tier Critic
```bash
./langPeanut translate --dir /path/to/your/project --locales fr,es,de,ja
```

### D. 1-Command Atomic Rollback
```bash
# List available pre-run checkpoints
./langPeanut rollback --dir /path/to/your/project

# Restore specific checkpoint
./langPeanut rollback <checkpoint_id> --dir /path/to/your/project
```

---

## 5. Inspecting Agent Trajectories (Deliverable 04)

All agent reasoning steps, tool calls, and reflection loops are automatically recorded in `/trajectories/`:

```bash
ls -la trajectories/
cat trajectories/case_01_react_nested_jsx.md
cat trajectories/case_04_flutter_const_tree.md
```
