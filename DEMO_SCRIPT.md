# DEMO_SCRIPT.md — 5-Minute Hackathon Video Walkthrough Script

> **Agentic Workflows Hackathon (Deliverable 03)**  
> Use this turn-by-turn video presentation guide to record your 5-minute solution video.

---

## 🎬 Video Overview & Timing Structure

| Timestamp | Section | Key Screen Action | Talking Points |
| :--- | :--- | :--- | :--- |
| **0:00 - 0:45** | **1. The Problem & Bottleneck** | Show a React / Flutter code file with hardcoded strings | Explain why manual i18n is painful and why single-prompt LLMs fail (hallucinated syntax, deleted comments, corrupted ICU `{name}` tokens). |
| **0:45 - 1:45** | **2. The Multi-Agent Architecture** | Show the 6-agent diagram from README | Introduce `langPeanut`: AST Scout, Semantic Context Agent, Reverse-Offset Patch Engine, Cultural Translator, and 4-Tier Critic. |
| **1:45 - 3:15** | **3. Live Interactive App Demo** | Run `./langPeanut` (Interactive Bubble Tea TUI) | 1. Navigate to **Audit** (see hardcoded strings)<br>2. Open **Review Queue** (`[a]` approve, `[s]` skip)<br>3. Open **Settings** (Show LLM Provider, API keys, Gen-Z tone)<br>4. Run **Translate** (Watch 4-Tier Critic verify and pass). |
| **3:15 - 4:15** | **4. The 10-Case Adversarial Benchmark** | Run `./langPeanut benchmark` in terminal | Show the comparative evaluation table: **100% Pass Rate** across React, Flutter, SwiftUI, and Android with **86.4% token savings**. |
| **4:15 - 5:00** | **5. Atomic Rollback & Conclusion** | Run `./langPeanut rollback` | Show 1-command rollback restoring original files. Summarize why deterministic AST patching + multi-agent reflection solves localization permanently. |

---

## 🎙️ Step-by-Step Spoken Script

### [0:00 - 0:45] Problem & Bottleneck
> *"Hi everyone, welcome to our presentation for the Agentic Workflows Hackathon. Today, we're introducing **langPeanut** — the Universal Multi-Agent Localization System.*
>
> *Every developer knows the pain of internationalizing software. Retrofitting localization onto an existing mobile or web app requires finding hundreds of hardcoded strings, creating boilerplate locale catalogs, and rewriting source code. When developers try using simple LLM prompts or regex tools, they encounter severe failure modes: LLMs rewrite the entire file, deleting comments, mangling nested JSX trees, and corrupting variable placeholders like `{userName}`.*
>
> *We built `langPeanut` to solve this with deterministic AST tool-use and closed-loop agentic verification."*

---

### [0:45 - 1:45] Architecture & Engineering
> *"Instead of relying on a single monolithic prompt, `langPeanut` orchestrates six specialized agents:*
> 1. *The **AST Scout Agent** uses static tree-sitter queries to extract UI strings while auto-skipping logging and API routes.*
> 2. *The **Semantic Context Agent** clusters sibling strings and component breadcrumbs to disambiguate words like 'Book' or 'Order'.*
> 3. *The **Deterministic Patch Engine** calculates exact reverse byte ranges to patch code in-place with zero formatting drift.*
> 4. *The **Cultural Translator Agent** translates while strictly preserving ICU syntax and format specifiers.*
> 5. *The **4-Tier Verifier Critic** runs closed-loop syntax, ICU variable parity, and layout checks.*
> 6. *And the **Supervisor Orchestrator** manages pre-run snapshots for 1-click rollback."*

---

### [1:45 - 3:15] Live Interactive App Walkthrough

*(Terminal: Run `./langPeanut`)*

> *"Let’s see `langPeanut` in action. When launched, `langPeanut` provides a full interactive terminal UI built with Bubble Tea.*
>
> *First, let’s run **Scan & Audit**. In milliseconds, it inspects our project and extracts all hardcoded UI strings with synthesized keys.*
>
> *Next, let’s open **Review & Refactor**. Here, developers can step through candidates with keyboard shortcuts — pressing `a` to approve or `s` to skip.*
>
> *In **Settings**, developers have full flexibility to choose between Anthropic Claude, OpenAI, Google Gemini, DeepL, or fine-tuned custom endpoints, along with dynamic style presets like **Gen-Z Slang** or **Corporate Formal**.*
>
> *Now, let’s trigger **Translate**. The Cultural Translator translates the approved keys, and the 4-Tier Critic validates all four layers in real time before writing to disk."*

---

### [3:15 - 4:15] 10-Case Adversarial Benchmark

*(Terminal: Run `./langPeanut benchmark`)*

> *"To rigorously measure our improvement, we built an adversarial 10-case benchmark suite spanning React nested JSX, Flutter const widget trees, SwiftUI format specifiers, and Android XML entities.*
>
> *While single-prompt baselines achieve only a 42% compilation pass rate and naive regex achieves 55%, **`langPeanut` achieves a 100% pass rate** with **86.4% token reduction** because static AST tools filter out non-UI code before the LLM is even called."*

---

### [4:15 - 5:00] Atomic Rollback & Wrap-Up

*(Terminal: Run `./langPeanut rollback`)*

> *"Safety is paramount. `langPeanut` automatically captures pre-run snapshots. Running `langPeanut rollback` reverts the codebase with 100% byte fidelity.*
>
> *Everything you saw is 100% reproducible with our single-command benchmark in `REPRODUCE.md`, and all agent trajectories are exported in `/trajectories/`.*
>
> *Thank you for checking out `langPeanut`!"*
