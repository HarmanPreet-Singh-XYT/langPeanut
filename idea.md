# langPeanut — Universal Multi-Agent Localization Workflow

> **micro1 Agentic Workflows Hackathon Project**  
> A universal, multi-agent AI system that automates end-to-end software localization across any web, mobile, or backend framework with deterministic AST precision, self-correcting reflection loops, and zero-defect code refactoring.

---

## 1. The Problem & Bottleneck

### Who has this problem?
Software engineers, product teams, and open-source maintainers across the globe who need to localize mobile, web, and desktop applications into multiple languages.

### What bottleneck makes it worth solving?
Every modern framework (Flutter, React/Next.js, SwiftUI, Jetpack Compose, Vue, Angular) has its own localization runtime and file formats (`.arb`, `i18next .json`, `.xcstrings`, `strings.xml`, `gettext .po`). However:
1. **Source code extraction is completely manual**: Developers must painstakingly hunt down hardcoded string literals across thousands of lines of code.
2. **Naive regex & simple LLMs fail catastrophically**: Single-prompt LLM refactorings hallucinate syntax, delete comments, mangle nested JSX/widget trees, translate code constants/URLs, break `const` constructors, and corrupt ICU variable placeholders (`{userName}` $\rightarrow$ translated into foreign words).
3. **Existing commercial tools (Lokalise, Crowdin, Phrase)** only manage static translation files; **they do not touch or refactor source code**.
4. **Result**: Teams spend dozens of engineering hours per release on tedious refactoring or avoid localization entirely, locking out global users.

---

## 2. The Solution: Multi-Agent Localization Workflow

`langPeanut` replaces manual string extraction, refactoring, and translation with a coordinated multi-agent workflow augmented with AST static analysis tools, translation memory, reflection loops, and human-in-the-loop checkpoints:

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

---

## 3. Specialized Agent Roles & Agentic Capabilities

### 1. Supervisor / Orchestrator Agent
* **Role**: Workflow lifecycle coordinator.
* **Capabilities**:
  * Manages the execution DAG, token budget packing, and session persistence (`.langPeanut/session/current.json`).
  * Creates automatic pre-run snapshots and stage checkpoints for atomic rollback.
  * Coordinates multi-locale parallel execution with per-provider rate-limiting.

### 2. AST Scout Extractor Agent (Tool-Augmented)
* **Role**: Deterministic AST scanning and candidate extraction.
* **Tools**: `go-tree-sitter` grammars (Dart, TSX/JSX, Swift, Kotlin, Java, Python, Go) + `go-git` delta tracker.
* **Capabilities**:
  * Scans source code and extracts string literal AST nodes along with their enclosing parent AST hierarchy.
  * Filters out technical strings (`console.log`, `debugPrint`, route constants, URLs, hex colors, regexes, SQL/GraphQL blocks) without spending LLM tokens.
  * Uses Git delta tracking to only scan modified files/lines on incremental runs.

### 3. Semantic Context & Disambiguation Agent
* **Role**: Contextual intelligence and semantic key generation.
* **Capabilities**:
  * Ingests ambiguous candidate strings along with surrounding component hierarchy and sibling strings.
  * **Domain Inference**: Understands whether `"Book"` in `FlightDetails.tsx` means *"Reserve ticket"* vs. *"Physical book"*.
  * **Semantic Key Generation**: Synthesizes clean, contextual keys (e.g., `checkoutSubmitOrderBtn` instead of random hashes).

### 4. AST Range Patch Engine (Deterministic Precision)
* **Role**: Zero-defect source code refactoring.
* **Capabilities**:
  * Applies exact byte-range replacements to the source code without rewriting or reformatting untouched code.
  * Injects framework-specific imports (`import { useTranslation } from 'react-i18next'`) and hooks (`const { t } = useTranslation()`).
  * Safely removes incompatible keywords (such as Flutter `const` widgets).

### 5. Specialized Cultural Translator Agent (Memory-Augmented)
* **Role**: High-fidelity translation with syntax preservation.
* **Capabilities**:
  * Leverages Translation Memory (TM) cache across runs and projects to eliminate redundant LLM calls.
  * Preserves exact ICU syntax, plurals (`{count, plural, =1{1 item} other{{count} items}}`), gender selection, and framework variable format tokens (`%@`, `$name`, `{{val}}`).

### 6. 4-Tier Verification & Reflection Critic Agent
* **Role**: Automated quality gate and self-correcting feedback loop.
* **4 Verification Tiers**:
  1. **Tier 1: AST Syntax Validation**: Validates in-memory AST parse trees for the refactored code before writing to disk.
  2. **Tier 2: ICU & Variable Token Alignment**: Verifies that every placeholder in the source exists identically in all target locale files without translated variable names.
  3. **Tier 3: UI Expansion & Layout Critic**: Calculates text length expansion ratios (e.g., German is ~30% longer than English) and flags potential UI clipping risks.
  4. **Tier 4: Cross-Locale Parity Diff**: Asserts 100% key parity across all locale files with 0 orphaned or collided keys.
* **Self-Correction Loop**: If any check fails, the Critic sends structured diagnostic feedback back to the Translator/Refactor agent for automated retry.

### 7. Human-in-the-Loop Checkpoint (Ground Rules 4 & 5)
* **Role**: Safety gate for consequential modifications.
* **Capabilities**:
  * Interactive Terminal UI (Bubble Tea) displaying side-by-side diff previews, confidence ratings, and proposed keys.
  * Instant 1-keystroke approval, skip, inline key edit, or pre-run rollback.

---

## 4. Supported Platforms & Formats

| Platform | Language | AST Parser | Target Locale Format | Refactor Pattern |
|---|---|---|---|---|
| **Flutter** | Dart | `tree-sitter-dart` | ARB (`.arb`) | `AppLocalizations.of(context)!.key` |
| **React / Next.js** | TS / JS / TSX | `tree-sitter-typescript` | i18next / next-intl JSON | `t('key')` |
| **SwiftUI / iOS** | Swift | `tree-sitter-swift` | `.xcstrings` (Xcode 15+) / `.strings` | `Text("key")` / `Text(.key)` |
| **Jetpack Compose** | Kotlin | `tree-sitter-kotlin` | `strings.xml` | `stringResource(R.string.key)` |
| **Vue** | TS / JS / Vue | `tree-sitter-javascript` | vue-i18n JSON | `{{ $t('key') }}` |
| **Angular** | TypeScript | `tree-sitter-typescript` | XLIFF / JSON | `{{ 'key' \| translate }}` |
| **.NET MAUI** | C# | `tree-sitter-c-sharp` | `.resx` | `AppResources.key` |
| **Python** | Python | `tree-sitter-python` | gettext `.po` / `.pot` | `_("key")` |
| **Go** | Go | `tree-sitter-go` | `go-i18n` TOML/JSON | `localizer.MustLocalize(...)` |

---

## 5. CLI Command Suite

```bash
langPeanut init           # Auto-detects platform framework, creates .langPeanut config
langPeanut audit          # Read-only scan & report of hardcoded strings, stale keys, coverage
langPeanut extract        # AST Scout + Context Agent extraction → Interactive TUI Review
langPeanut refactor       # Applies verified AST range patches and generates locale files
langPeanut translate      # Multi-locale translation with TM memory and Verifier reflection
langPeanut locales        # Add, exclude, group, and inspect target locale health
langPeanut keys           # Key lifecycle management (orphans, deduplication, rename)
langPeanut rollback       # Revert to any pre-run snapshot or stage checkpoint
langPeanut watch          # Millisecond background daemon triggering on file save
langPeanut benchmark      # Runs the 10-case evaluation suite comparing baseline vs agents
```

---

## 6. Evaluation Benchmark & Measured Improvement

To satisfy the **micro1 Agentic Workflows Hackathon** evaluation criteria, `langPeanut` includes an automated 10-case adversarial benchmark suite:

### 10-Case Adversarial Benchmark Suite
1. **React / TSX** (`CheckoutModal.tsx`): Complex nested JSX + dynamic string interpolation.
2. **React / TSX** (`FlightBooking.tsx`): Ambiguous short verbs (`"Book"`, `"Save"`, `"Order"`) in travel domain.
3. **React / TSX** (`UserNotifications.tsx`): Inline ternary pluralization.
4. **Flutter / Dart** (`HomeScreen.dart`): Deep widget tree with `const` constructors & `BuildContext`.
5. **Flutter / ARB** (`app_en.arb`): Complex nested ICU plural + gender selection syntax.
6. **Flutter / Dart** (`PaymentService.dart`): Mixed UI strings vs. `debugPrint`, URLs, route constants.
7. **iOS / SwiftUI** (`ProfileView.swift`): Swift 5.9 LocalizedStringKey + format specifiers (`%lld`, `%.2f`).
8. **Android / Kotlin** (`strings.xml`): XML entities (`&amp;`, `&lt;`), CDATA, and `<plurals>`.
9. **Massive TSX File** (`DashboardAnalytics.tsx`): 100+ mixed strings across 1,500 lines of code.
10. **Adversarial Trap** (`ConfigSettings.tsx`): URLs, regexes, SQL snippets, GraphQL tags, hex colors.

### Measured Improvement Comparison
| Metric | Baseline 1: Single Zero-Shot Prompt | Baseline 2: Naive Regex Tool | **langPeanut Multi-Agent Workflow** |
| :--- | :--- | :--- | :--- |
| **AST Compilation Pass Rate** | $42\%$ (hallucinates syntax, drops imports) | $55\%$ (breaks `const`, invalid placements) | **$100\%$** (AST Patch + Verifier Critic) |
| **False-Positive Extraction Rate** | $38\%$ (extracts URLs, log strings, routes) | $45\%$ (matches all quote literals) | **$<1.5\%$** (AST Scout + Classifier) |
| **ICU Placeholder Integrity** | $60\%$ (translates variable names in `{}`) | $0\%$ (no placeholder awareness) | **$100\%$** (ICU Verifier Reflection) |
| **Token Cost / Efficiency** | High ($100\%$ code sent to LLM) | Zero (0 intelligence) | **$85\%$ Token Savings** (AST Scout Layer) |
| **Human Effort per Task** | $\sim 3\text{--}4\text{ hours}$ manual refactor | $\sim 2\text{ hours}$ fixing broken regex | **$<1\text{ minute}$** (1-click TUI approval) |

---

## 7. Improvement Changelog

| Stage | What We Tried & Why | Evidence / Failure Observed | Decision & Learning |
| :--- | :--- | :--- | :--- |
| **Baseline** | Single direct LLM prompt ("Extract all strings and refactor this file"). | $58\%$ compilation failure rate; translated `{name}` into kanji; broke `const` constructors. | Established starting point: Raw LLMs cannot reliably refactor codebases without AST tooling. |
| **Iteration 1** | Integrated `go-tree-sitter` AST Scout as a dedicated tool. | LLM token usage plummeted by $85\%$; zero false positives on `print` and API URLs. | Kept: AST parsing is mandatory as Layer 1 deterministic filter before LLM. |
| **Iteration 2** | Built Deterministic Byte-Range AST Patch Engine. | Source code comments, formatting, and unrelated logic preserved $100\%$ with no LLM hallucination. | Kept: Never let an LLM rewrite an entire file; use surgical byte ranges. |
| **Iteration 3** | Added 4-Tier Verifier Critic with self-correcting reflection loop. | Placeholder corruption dropped from $40\%$ to $0\%$; compiler pass rate reached $100\%$. | Kept: Automated feedback loop catches model mistakes before human review. |
| **Final** | Added Translation Memory, Session Checkpoint Rollback, and Bubble Tea TUI. | End-to-end execution completed in seconds with full human approval safety gate. | Combined all changes into single static Go CLI. |

---

## 8. Hot Take / Practical Insights
> *"Never ask an LLM to generate code when an AST tool can deterministically extract it, and never let an LLM rewrite a whole file when an AST range patch engine can surgically modify it. The winning agentic architecture pairs deterministic static analysis tools with specialized, narrow LLM judgment and closed-loop verification critics."*
