# AGENTS.md — Agent Operating Guidelines & Project Context

> **Operating System & Protocols for AI Agents working on `langPeanut`**  
> This file establishes mandatory instructions, architectural context, and logging rules for all agents operating in this repository.

---

## 1. Project Overview & Scope

**Project Name**: `langPeanut`  
**Purpose**: Universal Multi-Agent Localization Workflow & CLI tool for software developers, built for the **Agentic Workflows Hackathon**.  
**Language & Stack**: 
- **Go** (Single static binary, sub-10ms startup, native goroutines for concurrency).
- **CLI Framework**: Cobra (`cmd/langPeanut`).
- **Interactive TUI**: Bubble Tea & Lip Gloss (`pkg/tui`).
- **AST Parsing**: `go-tree-sitter` (Dart, TypeScript/JSX, Swift, Kotlin, Java, Python, Go).
- **Git Integration**: `go-git/v5`.
- **Target Platforms**: React/Next.js, Flutter, iOS SwiftUI, Android Jetpack Compose, Vue, Angular, .NET MAUI, Python, Go.

---

## 2. Multi-Agent System Architecture

Every agent working on this codebase must adhere to the 6-agent separation of concerns:

```
┌────────────────────────────────────────────────────────────────────────┐
│                      Supervisor Orchestrator Agent                     │
│               (Session DAG, State Machine, Checkpoint Rollback)        │
└──────────────┬──────────────────┬──────────────────┬───────────────────┘
               │                  │                  │
       ┌───────▼────────┐ ┌───────▼────────┐ ┌───────▼────────┐
       │ AST Scout      │ │ Context Agent  │ │ AST Patch      │
       │ (Tree-sitter)  │ │ (Disambiguate) │ │ (Byte Ranges)  │
       └───────┬────────┘ └───────┬────────┘ └───────┬────────┘
               │                  │                  │
               └──────────────────┼──────────────────┘
                                  ▼
                     ┌────────────────────────┐
                     │ Cultural Translator    │
                     │ (ICU / Plural / Memory)│
                     └────────────┬───────────┘
                                  ▼
                     ┌────────────────────────┐
                     │ 4-Tier Verifier Critic │◄─── Self-Correction
                     │ (AST / ICU / Parity)   │     Reflection Loop
                     └────────────┬───────────┘
                                  ▼
                     ┌────────────────────────┐
                     │ Human Checkpoint (TUI) │
                     └────────────────────────┘
```

---

## 3. Mandatory Agent Operating Protocols

### Protocol 1: Continuous Changelog Maintenance (Strictly Enforced)
* Every agent **MUST** update the active log [CHANGELOG2.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/CHANGELOG2.md) (continuation from [CHANGELOG1.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/CHANGELOG1.md) and [CHANGELOG.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/CHANGELOG.md)) on every meaningful interaction, user directive, bug fix, or architectural change.
* When the user gives instructions, redirects architecture, or points out a flaw:
  1. Note the user's directive and why it was given.
  2. Document the action taken and files modified.
  3. Record the failure mode observed (if fixing a bug) and the resolution.
  4. Connect the change to the formal Hackathon Improvement Progression.

### Protocol 2: Context Preservation Across Agents
* When initializing new subagents or beginning a new task, agents must read:
  1. [idea.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/idea.md) — Core problem, solution, and hackathon requirements.
  2. [PLAN.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/PLAN.md) — Current implementation milestones and technical design.
  3. [CHANGELOG2.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/CHANGELOG2.md) & [CHANGELOG1.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/CHANGELOG1.md) — Chronological history of user directives and learnings.

### Protocol 3: Deterministic Verification & No Whole-File Hallucinations
* Agents must **NEVER** ask an LLM to rewrite a full source code file from scratch.
* All code refactorings must use **deterministic byte-range replacements** and pass **in-memory AST validation** before saving to disk.
* All translated strings must undergo **4-Tier verification** (Syntax, ICU variable matching, character expansion estimation, and key parity diffs).

### Protocol 4: Trajectory & Benchmark Logging
* All multi-agent runs, tool calls, critic reflections, and retry attempts must be structured and loggable into `/trajectories/` for the Hackathon Deliverable 04.
* Benchmark cases must be tested against the baseline under `benchmark/`.

---

## 4. Key Reference Files

| File | Purpose |
| :--- | :--- |
| [AGENTS.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/AGENTS.md) | System guidelines and operating protocols for agents. |
| [CHANGELOG2.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/CHANGELOG2.md) | **Active live log** of user directives, AI actions, fixes, and improvement iterations (Session Entries 112+). |
| [CHANGELOG1.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/CHANGELOG1.md) | Historical interaction archive (Session Entries 98–111). |
| [CHANGELOG.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/CHANGELOG.md) | Foundational hackathon archive (Session Entries 1–97, Hot Takes, Improvement Progression). |
| [idea.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/idea.md) | Product specification, multi-agent architecture, and hackathon alignment. |
| [PLAN.md](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/PLAN.md) | Technical implementation milestones and task tracking. |
