# CHANGELOG1.md — Improvement & Interaction Continuation Log

> **micro1 Agentic Workflows Hackathon Record (Part 2)**  
> This file is the direct continuation of [`CHANGELOG.md`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/CHANGELOG.md) (which contains the formal Hackathon Improvement Progression, Measured Improvements, Hot Takes, and Session Entries 1 through 97).  
> All new session entries continue chronologically in this file starting from Session Entry 98.

---

## Interactive Development & User Directives Log (Continued)

### Session Entry 98: Model-Aware 50k Token Limits, Custom Batch Chunking & Parallel Concurrency Tunables

* **User Directives**:
  1. *"can we have customization like token limit adjustment, like we have this chunking thing in our codebase which is basically dependent upon the words max per call which depends on token limit of the model and i want to have individual limit, being able to modify how many parallel calls we want, how many approx tokens each would have, and obviously this is by default but if only 1 call is needed as context window is large then we just call 1 time"*
  2. *"u can do 50k token as default for frontier models"*
  3. *"changelog1 purpose was to continue from entry 97 not have duplicate of changelog"*

* **Architecture & Enhancements**:
  1. **Dynamic Chunk & Word/Token Budget Tunables (`pkg/memory/config.go`, `pkg/agents/translator.go`)**:
     - Added `ChunkWordBudget` (words/tokens per batch call) and `ChunkKeyCeiling` (max keys per batch call) to `AppConfig` and `TranslatorAgent`.
     - Added `Concurrency` tunable (max concurrent LLM worker goroutines) across chunk dispatch and multi-language supervisor pools.
  2. **Single-Call Fast-Path Optimization**:
     - When the total missing translation keys fit comfortably within the target model's word/token budget and key ceiling, `chunkMapByWordBudget` produces exactly **1 single batch**, eliminating unnecessary chunk splitting, HTTP roundtrip overhead, and reassembly.
  3. **Frontier Model 50k Token Default**:
     - Configured `getEffectiveChunkSettings()` with model-aware dynamic defaults:
       - **Frontier / Large Context Models** (`Claude Sonnet 5`, `GPT-4o`, `Gemini 2.5 Pro/Flash`): Default **50,000 token budget (~38,000 words) & 1,500 keys** with 5 concurrent workers — executes entire application codebases in **1 single API call**.
       - **Local Ollama Models** (`Gemma 3`, `Qwen 2.5 7B`, `Llama 3.2`): Default **3,000 words (~4,000 tokens) & 100 keys**.
       - **Compact Offline Models** (`NLLB-200`): Default **400 words (~500 tokens) & 50 keys**.
  4. **Multi-Layer Configuration Support**:
     - **CLI Flags**: `--concurrency` / `-c`, `--chunk-words`, `--chunk-keys` on both `langPeanut translate` and `langPeanut run`.
     - **Environment Variables**: `LANGPEANUT_CONCURRENCY`, `LANGPEANUT_CHUNK_WORDS`, `LANGPEANUT_CHUNK_KEYS`.
     - **Persistent Config**: `~/.langPeanut/config.json` (global) and `.langPeanut/config.json` (project-level).
     - **Web Studio & Settings API**: Exposed in `/api/settings` and `/api/settings/save`.
  5. **Continuation Log Architecture**:
     - Initialized `CHANGELOG1.md` as the clean continuation log starting from Session Entry 98 onwards.

* **Verification**:
  - Added unit tests in `pkg/agents/agents_test.go`:
    - `TestTranslator_SingleCallWhenUnderBudget`
    - `TestTranslator_EffectiveChunkSettingsModelAware` (asserting 38k words / 1500 keys / 5 concurrency for Claude)
  - `go test ./...` passed 100% across all 15 packages.
  - Rebuilt binary `langPeanut` and updated system binaries.

### Session Entry 99: TUI vs Web UI Synchronization Audit, Fast AST Semantic Parity & Root Path Alignment

* **User Directives**:
  1. *"is web ui out of sync from the tui"*

* **Audit & Synchronization Findings**:
  1. **Core Architectural Alignment**:
     - Both TUI ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go)) and Web UI ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go)) share 100% of the underlying Go multi-agent engine:
       - 6-agent supervisor pipeline DAG (`SupervisorAgent`, `ASTScoutAgent`, `ContextAgent`, `PatchEngine`, `TranslatorAgent`, `VerifierCriticAgent`).
       - 8 AI translation providers (Anthropic Claude, OpenAI, Gemini, Ollama, Meta NLLB-200 Cloud, DeepL, Custom, Local).
       - 5 Tone style presets (Standard Native, Gen-Z Slang, Casual Friendly, Corporate Formal, Pirate / Gamer).
       - 4-Tier verification critic scorecard (AST Syntax, ICU & Variable Parity, UI Layout Expansion, Locale Key Parity).
       - Live token analytics & cost tracking (`llm.GlobalTracker`).
       - 10-Case Adversarial Benchmark evaluation harness (`benchmark.RunBenchmark`).
       - Checkpoints & 1-click atomic rollback (`orchestrator.CheckpointManager`).
  2. **Discrepancies Identified & Resolved**:
     - **AST Scan Context Enhancement Parity**: In `pkg/web/server.go`, `performScan()` previously returned raw AST candidates without calling `Context.EnhanceFast()`. Updated `performScan()` to run `contextAgent.EnhanceFast()` so that semantic disambiguation (e.g. "Book" -> travel vs library), noise filtering, and component name prefixing match the TUI review queue immediately upon scanning.
     - **Repo Root Resolution on Reset**: Updated `handleResetExamples()` in `pkg/web/server.go` to use `findRepoRoot()` so that restoring demo examples cleans directories and executes `git checkout` relative to the repository root even when attached to a subproject (e.g., `examples/nextjs-app`).
     - **Smart Default Target Alignment**: Aligned `NewStudioServer()` with `NewApp()` so launching the Web UI from repository root defaults to `examples/nextjs-app` for instant interactive demo onboarding.

* **Verification**:
  - `go test ./...` passed across all packages (`pkg/web`, `pkg/tui`, `pkg/agents`, `pkg/platforms`, `pkg/llm`).

### Session Entry 100: Advanced Expansion Guard, TMX/XLIFF TMS Interchange, PR Bot Commands & Cloud Matrix Sync

* **User Directives**:
  1. *"is there any suggestion that could be added"*
  2. *"can you implement these"*

* **Actions Taken & Features Implemented**:
  1. **Enhanced Tier 3 UI Layout & Character Expansion Critic ([`pkg/agents/verifier_critic.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/verifier_critic.go))**:
     - Implemented language-specific character expansion modeling (e.g. German compound words up to 2.8x, Slavic inflections up to 2.6x, East Asian CJK compactness up to 2.2x).
     - Added short-string UI button/header overflow heuristics (< 20 chars source expanding > 2.0x, e.g. "Save" -> "Jetzt verbindlich buchen") to alert developers about fixed-width mobile container clipping.
  2. **Industry-Standard Translation Memory Interchange (TMX 1.4b & XLIFF 1.2) ([`pkg/memory/tmx.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/memory/tmx.go), [`cmd/langPeanut/tmx.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/cmd/langPeanut/tmx.go))**:
     - Created `ExportTMX`, `ImportTMX`, `ExportXLIFF`, and `ImportXLIFF` in `pkg/memory/` for seamless migration with enterprise localization platforms (Crowdin, Phrase, Lokalise, Trados).
     - Added `langPeanut export` and `langPeanut import` CLI commands with automatic Translation Memory cache warming.
     - Added full unit test coverage in [`pkg/memory/tmx_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/memory/tmx_test.go).
  3. **Interactive PR Bot Mention Commands & Webhook Engine ([`pkg/github/webhook.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/github/webhook.go), `langpeanut-cloud/internal/api/handlers.go`)**:
     - Added `EventIssueComment` webhook parsing and `ParseBotCommand()` supporting `@langpeanut translate --locales es,ja --tone formal`, `@langpeanut audit`, `@langpeanut review`.
     - Wired PR comment webhook dispatch in `langpeanut-cloud` to queue on-demand sandboxed runner jobs with PR numbers.
  4. **Collaborative Web Matrix Git Sync Endpoint (`langpeanut-cloud/internal/api/handlers.go`)**:
     - Implemented `PUT /api/repos/{repoID}/matrix` allowing non-technical team members to update translation cells directly from the cloud web interface.

* **Verification**:
  - `go test ./...` in `langpeanut_local`: 100% pass across all packages (`pkg/memory`, `pkg/github`, `pkg/agents`, `pkg/web`, `pkg/tui`, `pkg/llm`, `pkg/platforms`).
  - `go test ./...` in `langpeanut-cloud`: 100% pass across `internal/api`, `internal/auth`, `internal/db`.
  - Rebuilt binaries `bin/langPeanut`, `langpeanut-cloud/server`, and `langpeanut-cloud/runner`.

### Session Entry 101: Automated .gitignore Management & Ephemeral Scratch Isolation

* **User Directives**:
  1. *"also .langPeanut doesn't get automatically added into .gitignore when initialized into any project and that is created or any other unneccessary files like trajectories which shouldn't be in actual repo, because as we finish localization and is ready either cloud version automatically needs commit in new branch then create pr with that or in local we ourself need to commit and in both cases those two folders r basically useless for other person i feel, obviously we can have setting if the user want it or not to have this in effect for both cloud and local"*

* **Actions Taken & Architecture Upgrades**:
  1. **Centralized `.gitignore` Sanitizer ([`pkg/memory/config.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/memory/config.go))**:
     - Added `EnsureGitignore(projectRoot string) error` helper ensuring `.langPeanut/`, `.langpeanut/`, `trajectories/`, `.langPeanut-snapshots/`, and `.langpeanut.lock` are automatically added with structured header comments without clobbering existing developer rules.
     - Added `AutoGitignore *bool` in `AppConfig` (defaults to `true`), with `ShouldAutoGitignore()` toggle support in configuration files.
  2. **Automated Invocation Across All Pipeline Lifecycle Stages**:
     - Wired `memory.EnsureGitignore` into:
       - `NewSupervisorAgent()` ([`pkg/agents/supervisor.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/supervisor.go))
       - `NewStudioServer()` ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go))
       - `NewApp()` ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go))
       - `langPeanut init` ([`cmd/langPeanut/init.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/cmd/langPeanut/init.go))
       - `CloneForJob()` ([`pkg/github/repo_client.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/github/repo_client.go))
  3. **Cloud Sandboxed Runner Commit Cleanliness (`langpeanut-cloud/cmd/runner/main.go`)**:
     - Pre-commit cleanup in sandboxed runner destroys ephemeral `trajectories/` and `.langPeanut/` scratch folders before `git add -A` and `git commit` to ensure zero telemetry or internal state leaks into GitHub Pull Requests.
  4. **Test Suite**:
     - Added comprehensive unit tests in [`pkg/memory/config_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/memory/config_test.go) covering creation, appending, idempotency, existing file preservation, and toggle disabling.

* **Verification**:
  - `go test ./...` in `langpeanut_local`: 100% pass across all packages.
  - `go test ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Rebuilt binaries `bin/langPeanut`, `langpeanut-cloud/server`, and `langpeanut-cloud/runner`.

### Session Entry 102: Autonomous Post-Localization App Integration Agent (Claude Code-style ReAct Harness with Large-File Windowing)

* **User Directives**:
  1. *"ok go ahead and implement it and make sure we handle all the edge cases coz file could be like 10000 lines but our LLM won't have that much context window u know what am saying so"*

* **Actions Taken & Architecture Upgrades**:
  1. **Autonomous `DirectiveAgent` ([`pkg/agents/directive_agent.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/directive_agent.go))**:
     - Built a native ReAct / tool-calling coding agent harness in Go that executes developer post-localization UI directives (e.g. *"Add a language switcher dropdown in Navbar.tsx"*).
     - **Large-File Context Budget Guard (10,000+ Lines)**:
       - `scan_file_outline`: Extracts compact Tree-Sitter AST skeletons (top imports, component signatures, JSX return trees) in `< 60` lines instead of dumping full files.
       - `read_code_window`: Reads targeted line slices (max 120 lines) around specific insertion anchors.
       - `write_component`: Synthesizes clean standalone UI components (e.g. `src/components/LanguagePicker.tsx`).
       - `apply_surgical_patch`: Executes deterministic byte-range search/replace patches with in-memory Tree-Sitter syntax assertions.
       - `run_diagnostics`: Runs compiler diagnostics (`tsc`, `flutter analyze`) and feeds line-specific errors back to the agent for self-healing reflection.
  2. **Integrated as Step 7 in Supervisor Orchestrator DAG ([`pkg/agents/supervisor.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/supervisor.go))**:
     - Added `DirectiveAgent` and `UserDirective` execution to `SupervisorAgent.RunEndToEnd()`.
     - Integrated `DirectiveResult` into `types.PipelineResult` and checkpoint manifests.
  3. **CLI & Cloud Integration ([`cmd/langPeanut/run.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/cmd/langPeanut/run.go), `langpeanut-cloud/cmd/runner/main.go`)**:
     - Added `--directive` / `--instruction` CLI flag to `langPeanut run`.
     - Wired cloud runner to receive `USER_DIRECTIVE` environment variables from repo settings.
  4. **Comprehensive Unit Testing ([`pkg/agents/directive_agent_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/directive_agent_test.go))**:
     - Tested AST skeleton outline generation on multi-hundred line files, bounded window reading, surgical patching with AST validation, and markdown JSON action parsing.

* **Verification**:
  - `go test ./...` in `langpeanut_local`: 100% pass across all packages (`pkg/agents`, `pkg/memory`, `pkg/github`, `pkg/web`, `pkg/tui`, `pkg/llm`, `pkg/platforms`).
  - `go test ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Rebuilt binaries `bin/langPeanut`, `langpeanut-cloud/server`, and `langpeanut-cloud/runner`.

### Session Entry 103: End-to-End Workflow Integration (Web Studio, TUI, and Cloud Dashboard) for App Integration Agent

* **User Directives**:
  1. *"do we have this in workflow like web,tui,cloud web"*

* **Actions Taken & Architecture Upgrades**:
  1. **Local Web Studio ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go))**:
     - Added `Directive string` field to `RunPipelineRequest`.
     - Added an interactive **"App Integration Directive (Optional)"** prompt card to the Pipeline Runner tab in the Web Studio HTML UI (`runnerDirectiveInput`).
     - Wired `executeLocalization()` in JavaScript to serialize and post the directive to `/api/run`.
     - Wired `handleRunPipeline` in Go to assign `supervisor.UserDirective` and log execution in the live terminal stream.
  2. **Interactive TUI ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go))**:
     - Added `directiveInput string` to `tui.Model` state.
     - Wired `startFullLocalization` to assign `sup.UserDirective = m.directiveInput` prior to pipeline launch.
  3. **Cloud Web Dashboard & Sandboxed Runner ([`langpeanut-cloud/web/app/page.tsx`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/web/app/page.tsx), [`langpeanut-cloud/internal/worker/worker.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/internal/worker/worker.go))**:
     - Added `userDirective` state and input field in the repository settings modal of the Next.js 15 cloud dashboard (`🪄 Post-Localization App Directive (UI Switcher Agent)`).
     - Wired `launchSandbox` in the cloud worker to pass `-e USER_DIRECTIVE` into the isolated Docker runner container.
     - Runner receives `USER_DIRECTIVE`, executes `DirectiveAgent`, and includes synthesized widgets in the automated GitHub Pull Request.

* **Verification**:
  - `go test ./...` in `langpeanut_local`: 100% pass across all packages.
  - `go test ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Rebuilt all static binaries: `bin/langPeanut`, `langpeanut-cloud/server`, and `langpeanut-cloud/runner`.

### Session Entry 104: 38+ World Language Catalog, Regional Presets & Custom BCP-47 Code Adder in Web UIs

* **User Directives**:
  1. *"also in web ui, i dont have many target languages options like customizibility is very less"*

* **Actions Taken & Architecture Upgrades**:
  1. **Local Web Studio Target Languages Hub ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go))**:
     - Expanded from 6 hardcoded checkboxes to a full catalog of **38 world languages** with flags, English names, and native scripts (Spanish, French, German, Japanese, Chinese Simplified & Traditional, Korean, Portuguese PT & BR, Italian, Dutch, Russian, Arabic, Hindi, Turkish, Polish, Swedish, Danish, Finnish, Norwegian, Ukrainian, Vietnamese, Thai, Indonesian, Malay, Filipino, Hebrew, Greek, Czech, Romanian, Hungarian, Slovak, Bulgarian, Croatian, Lithuanian, Latvian, Estonian, Slovenian, Catalan).
     - Added **Quick-Select Regional Presets Bar**:
       - `⭐ Top 5` (`es`, `fr`, `de`, `ja`, `zh-CN`)
       - `🇪🇺 EU Tier 1` (`es`, `fr`, `de`, `it`, `pt`, `nl`, `pl`)
       - `🌏 Asia-Pacific` (`ja`, `zh-CN`, `zh-TW`, `ko`, `vi`, `th`, `id`, `hi`)
       - `🌎 Americas` (`es`, `pt-BR`, `fr`, `ca`)
       - `❄️ Nordics` (`sv`, `da`, `fi`, `no`)
       - `✓ All 38` / `✗ Clear`
     - Added **Live Search & Filter** (`filterCatalogLanguages()`) to instantly search by language name, native script, or code.
     - Added **Custom BCP-47 Locale Adder** (`+ Custom: e.g. pt-BR, fil, es-419, de-AT, en-GB`) allowing developers to add any arbitrary dialect or regional locale dynamically.
     - Dynamic selected language count badge (`runnerSelectedCount`).
  2. **Cloud Web Dashboard Repository Settings ([`langpeanut-cloud/web/app/page.tsx`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/web/app/page.tsx))**:
     - Expanded `AVAILABLE_LANGUAGES` to 38 languages.
     - Added matching preset buttons, search filtering, and custom BCP-47 tag input in the repo customization modal.
  3. **Installed CLI Binary**:
     - Recompiled and installed to `/Users/harmanpreetsingh/.local/bin/langPeanut` and `$(go env GOPATH)/bin/langPeanut`.

* **Verification**:
  - `go test ./...` in `langpeanut_local`: 100% pass across all packages.
  - `go test ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Rebuilt all static binaries: `bin/langPeanut`, `langpeanut-cloud/server`, and `langpeanut-cloud/runner`.

### Session Entry 105: Typography Modernization to Google Roboto & Roboto Mono

* **User Directives**:
  1. *"also change the web fonts to roboto"*

* **Actions Taken & Architecture Upgrades**:
  1. **Local Web Studio ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go))**:
     - Updated CSS imports and styling to Google Fonts **`Roboto`** (`wght@300;400;500;700;900`) and **`Roboto Mono`** (`wght@400;500;600;700`).
     - Applied `font-family: 'Roboto', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif` to all body, button, input, select, and textarea elements.
     - Applied `font-family: 'Roboto Mono', 'JetBrains Mono', monospace` to all pre, code, and `.font-mono` elements.
  2. **Cloud Web Dashboard ([`langpeanut-cloud/web/app/layout.tsx`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/web/app/layout.tsx), [`langpeanut-cloud/web/app/globals.css`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/web/app/globals.css))**:
     - Swapped Next.js Google Fonts loader to `Roboto` (`--font-roboto`) and `Roboto_Mono` (`--font-roboto-mono`).
     - Updated `--font-sans` and `--font-mono` CSS custom properties in `globals.css`.
  3. **Installed CLI Binary**:
     - Recompiled and copied to `/Users/harmanpreetsingh/.local/bin/langPeanut`.

* **Verification**:
  - `go test ./...` in `langpeanut_local`: 100% pass across all packages.
  - `go test ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Rebuilt binaries `bin/langPeanut`, `langpeanut-cloud/server`, and `langpeanut-cloud/runner`.

### Session Entry 106: Secret Credential Isolation & Sanitization of Project-Level Config Files

* **User Directives**:
  1. *"key for openai getting saved in config.json for the project but that is in project directory"*

* **Actions Taken & Architecture Upgrades**:
  1. **Strict Credential Isolation ([`pkg/memory/config.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/memory/config.go))**:
     - Introduced `ProjectConfig` which models sanitized project settings (`active_provider`, `active_model`, `style_preset`, `selected_locales`, `chunk_word_budget`, `chunk_key_ceiling`, `concurrency`, `auto_gitignore`) and completely omits `APIKeys`.
     - Separated `SaveGlobal()` (saves full credentials to user home `~/.langPeanut/config.json` with secure `0600` permissions) from `SaveProject(projectDir)` (writes sanitized `ProjectConfig` to `projectDir/.langPeanut/config.json`).
     - Ensured that any sensitive API keys or `api_keys` JSON keys are **NEVER** written to the project workspace.
  2. **Hermetic Test Isolation ([`pkg/memory/config_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/memory/config_test.go))**:
     - Added `SetGlobalConfigDirOverride()` and unit test `TestAppConfig_ProjectConfigDoesNotContainAPIKeys` verifying that project config files remain 100% free of secrets.
  3. **Installed CLI Binary**:
     - Recompiled and updated in `/Users/harmanpreetsingh/.local/bin/langPeanut`.

* **Verification**:
  - `go test ./...` in `langpeanut_local`: 100% pass across all packages.
  - `go test ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Rebuilt binaries `bin/langPeanut`, `langpeanut-cloud/server`, and `langpeanut-cloud/runner`.

### Session Entry 107: TUI Wizard Directive Step, Web Runner Auto-Detection, and AI Repair Cost Circuit Breakers

* **User Directives**:
  1. *"in tui i wasn't asked of directive like if i wanna have anything added, or something like that like navbar choosen went i went with run, and web run doesn't work like its broken, only parses strings into keys, also there should be a limit on how much AI tries to repair and when it gives up coz we dont wanna have cost spikes"*

* **Actions Taken & Architecture Upgrades**:
  1. **TUI 5-Step 1-Click Localization Wizard with UI Directive ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go))**:
     - Added an interactive **Step 3: App Integration Directive (UI Switcher Agent)** directly in the TUI setup wizard with 4 options:
       - `1. Add Language Switcher Dropdown in Navigation Bar` (`Add a language switcher dropdown in Navbar / Header`)
       - `2. Add Language Picker Screen / Setting Option` (`Add a language selector option in Settings screen`)
       - `3. Add Floating Language Toggle Button` (`Add a floating language toggle widget in bottom right corner`)
       - `4. None (Standard Localization Only)`
     - Updated step navigation badges, cursor bounds (`maxCursorForState`), back navigation (`[b]`), and summary display (`directiveStr`).
  2. **Cost Circuit Breakers & Retry Caps ([`pkg/agents/repair.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/repair.go), [`pkg/agents/directive_agent.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/directive_agent.go))**:
     - Hard-capped `CodeRepairAgent` to **max 2 attempts per file**. If compiler errors persist after 2 attempts, the repair loop halts immediately, flags the remaining diagnostics for human developer review, and returns a clear warning explanation rather than continuing unbounded LLM calls.
     - Hard-capped `DirectiveAgent` to **max 3 turns**.
  3. **Web Studio Pipeline Execution Resilience ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go))**:
     - Upgraded `handleRunPipeline` to check for active keys before attempting an unconfigured offline engine, falling back seamlessly to `llm.AutoDetectClient()`.
     - Automatically invokes `s.performScan()` on pipeline completion to immediately refresh extracted candidates, matrix view, diff viewer, and logs in the web interface.
  4. **Installed CLI Binary**:
     - Recompiled and updated in `/Users/harmanpreetsingh/.local/bin/langPeanut`.

* **Verification**:
  - `go test ./...` in `langpeanut_local`: 100% pass across all packages (including `TestTUI_1ClickLocalizationAsyncFlow` and `pkg/memory` credential tests).
  - `go test ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Rebuilt binaries `bin/langPeanut`, `langpeanut-cloud/server`, and `langpeanut-cloud/runner`.

### Session Entry 108: Live Custom Text Input Editor for TUI Wizard Directive Step

* **User Directives**:
  1. *"why is it giving me this option dude - Step 3 of 5: App Integration Directive (UI Switcher Agent), like what the fk are these options ... i want a custom text write option like how i want"*

* **Actions Taken & Architecture Upgrades**:
  1. **Direct Live Text Input in TUI Setup Wizard ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go))**:
     - Converted Step 3 into a **Direct Interactive Text Input Box** (`textinput.Model`) with focus, cursor, full keyboard typing, backspace, and arrow keys.
     - Developers can type any custom instructions directly (e.g. `"Add a language switcher dropdown in Navbar.tsx with a globe icon and Tailwind styling"`).
     - Pressing `[Tab]` cycles through instant suggestion presets if desired.
     - Pressing `[Enter]` with empty input smoothly skips UI synthesis and proceeds to standard code localization.
     - Pressing `[Enter]` with text captures `m.directiveInput` and seamlessly moves to Step 4 (Safety Mode).
  2. **Installed CLI Binary**:
     - Recompiled and updated in `/Users/harmanpreetsingh/.local/bin/langPeanut`.

* **Verification**:
  - `go test ./...` in `langpeanut_local`: 100% pass across all packages.
  - `go test ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Rebuilt binaries `bin/langPeanut`, `langpeanut-cloud/server`, and `langpeanut-cloud/runner`.

### Session Entry 109: Referenced Key Discovery & Target Language File Completeness Audit

* **User Directives**:
  1. *"also it only checks like if we have swapped the text with keys, not that do we have language files, do those language files actually contain text, and if not then that should be generated"*

* **Actions Taken & Architecture Upgrades**:
  1. **Referenced i18n Code Key Scanner ([`pkg/agents/supervisor.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/supervisor.go))**:
     - Added `extractReferencedKeys(projectRoot, extList, skipDirs)` and `humanizeKey()` to scan codebases for existing wrapped translation keys (e.g. `t('flight_details')`, `AppLocalizations.of(context)!.welcome`, `NSLocalizedString(...)`, `stringResource(R.string...)`, `<Trans i18nKey="...">`, etc.).
     - If code is already refactored with keys but the language files do not exist or are missing those keys, `SupervisorAgent` automatically populates the catalog with humanized English base values.
  2. **Comprehensive Target Language File Audit & Generation ([`pkg/agents/supervisor.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/supervisor.go))**:
     - Upgraded the translation pre-flight check in Step 5: for each target locale, it audits whether the file exists on disk and checks each key for presence and non-empty content (`!ok || strings.TrimSpace(val) == ""`).
     - Any missing or blank keys are sent to `TranslatorAgent` to be generated.
     - Formats and writes the complete target language files (e.g. `src/locales/es.json`, `app_es.arb`, `Localizable.xcstrings`, `values-es/strings.xml`) to disk even when zero source code modifications were required.
  3. **Regression Test Suite ([`pkg/agents/agents_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/agents_test.go))**:
     - Added `TestSupervisor_GeneratesMissingLanguageFilesFromReferencedKeys` verifying that pre-wrapped code with missing language files automatically generates the complete target locale catalogs on disk.
  4. **Installed CLI Binary**:
     - Recompiled and updated in `/Users/harmanpreetsingh/.local/bin/langPeanut`.

* **Verification**:
  - `go test ./...` in `langpeanut_local`: 100% pass across all packages.
  - `go test ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Rebuilt binaries `bin/langPeanut`, `langpeanut-cloud/server`, and `langpeanut-cloud/runner`.

### Session Entry 110: Universal Language Dependency Manager & Package Installer

* **User Directives**:
  1. *"after we add the keys to the project and run it, we r not installation like most of the time dependency won't be there, we need to run install on language dependency"*

* **Failure Modes Observed**:
  1. **Missing Runtime Framework Packages**: When refactoring code to use `react-i18next` (`import { useTranslation } from 'react-i18next'`) or Flutter `AppLocalizations`, greenfield or existing projects often lack the required localization libraries in `package.json` (`react-i18next`, `i18next`) or `pubspec.yaml` (`flutter_localizations`, `intl`, `generate: true`, `l10n.yaml`).
  2. **Uncaught Compiler Diagnostic Regressions**: Running post-refactor typechecks (`tsc --noEmit`, `dart analyze`) immediately failed with module not found errors (`TS2307: Cannot find module 'react-i18next'`), erroneously triggering AI code repair attempts on uninstalled dependencies.
  3. **Broken Developer Cold-Start**: After running `langPeanut`, running `npm run dev` or `flutter run` crashed until the developer manually investigated, installed packages, and wrote bootstrap config files.

* **Actions Taken & Architecture Upgrades**:
  1. **Universal Dependency Manager ([`pkg/platforms/dependencies.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/platforms/dependencies.go))**:
     - **React / Next.js (`ReactEnsureDependencies`)**:
       - Auto-detects package manager (`pnpm-lock.yaml` -> `pnpm`, `yarn.lock` -> `yarn`, `bun.lockb`/`bun.lock` -> `bun`, default -> `npm`).
       - Inspects `package.json` and cleanly injects missing `"react-i18next": "^14.1.2"` and `"i18next": "^23.11.5"` into `"dependencies"`.
       - Automatically creates standardized i18n setup bootstrap file (`src/i18n.ts` or `i18n.ts`) configuring `i18n.use(initReactI18next).init(...)` if absent.
       - Runs the detected package manager install command with timeouts and graceful error isolation.
     - **Flutter / Dart (`FlutterEnsureDependencies`)**:
       - Injects `flutter_localizations: sdk: flutter` and `intl: any` into `pubspec.yaml` under `dependencies:`.
       - Injects `generate: true` under `flutter:`.
       - Creates standard `l10n.yaml` (`arb-dir: lib/l10n`, `template-arb-file: app_en.arb`, `output-localization-file: app_localizations.dart`).
       - Executes `flutter pub get` and `flutter gen-l10n`.
     - **Android & Swift (`AndroidEnsureDependencies`, `SwiftEnsureDependencies`)**:
       - Ensures native Android `res/values` structure and Apple Foundation `.xcstrings` catalog directories.
  2. **Platform Interface Extension ([`pkg/platforms/platform.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/platforms/platform.go))**:
     - Added `CheckDependencies(projectRoot string) (*types.DependencyStatus, error)` and `EnsureDependencies(projectRoot string, autoInstall bool) (*types.DependencyStatus, error)` across all 5 framework plugins (`ReactPlatform`, `FlutterPlatform`, `SwiftPlatform`, `AndroidPlatform`, `GenericPlatform`).
  3. **Pipeline Lifecycle Integration ([`pkg/agents/supervisor.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/supervisor.go))**:
     - Integrated `EnsureDependencies` directly into Step 7 prior to running compiler diagnostics, resolving compiler module imports cleanly before verification and healing loops.
     - Added `DependencyStatus` struct to [`pkg/types/types.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/types/types.go) and `PipelineResult`.
  4. **Dedicated CLI Command `langPeanut install` ([`cmd/langPeanut/install.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/cmd/langPeanut/install.go))**:
     - Added `langPeanut install [directory]` (aliases: `deps`, `setup`, `add-deps`, `get`) with `--no-install` flag.
     - Updated `langPeanut run` ([`cmd/langPeanut/run.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/cmd/langPeanut/run.go)) to report dependency installation summary.
  5. **Web Studio & API Integration ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go))**:
     - Added `/api/dependencies` endpoint for on-demand dependency checks/installs.
     - Streamed dependency installation events to the Web Studio terminal log.
  6. **Unit Test Suite ([`pkg/platforms/dependencies_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/platforms/dependencies_test.go))**:
     - Added unit tests covering React package injection, i18n bootstrap creation, Flutter pubspec patching, l10n.yaml generation, and Android/Swift checks.

* **Verification**:
  - `go test -v ./...`: 100% pass across all 15 packages.
  - Tested `langPeanut install ./examples/nextjs-app --no-install` (verified `package.json` check & `src/i18n.ts` creation).
  - Recompiled and updated system binary `/Users/harmanpreetsingh/.local/bin/langPeanut`.

### Session Entry 111: Universal Custom Build & Install Commands (CLI, Web UI, TUI, Cloud Web)

* **User Directives**:
  1. *"allow user to have the command u know like if the user wanna have custom build/install command, then with web ui, tui, cloud web"*

* **Context & Motivation**:
  - In real-world enterprise monorepos, specialized toolchains, private registries, or non-standard project setups (e.g. `pnpm install --filter ...`, `yarn add --immutable`, `flutter pub get --no-example`, `npm run typecheck`, `gradle build`, `cargo check`), automated package manager defaults need to be flexibly overridable by developers.
  - Custom commands must be configurable globally (`~/.langPeanut/config.json`), per-project (`.langPeanut/config.json`), via CLI flags, through the local Web Studio, within the interactive terminal TUI, and across the Cloud Web dashboard and sandboxed runner containers.

* **Actions Taken & Architecture Upgrades**:
  1. **Core Configuration & Memory ([`pkg/memory/config.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/memory/config.go))**:
     - Added `CustomInstallCmd` and `CustomBuildCmd` fields to `AppConfig` and `ProjectConfig`.
     - Saved and loaded seamlessly from project `.langPeanut/config.json` and global preferences.
  2. **Platform Toolchains & Execution ([`pkg/platforms/dependencies.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/platforms/dependencies.go), [`pkg/platforms/typecheck.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/platforms/typecheck.go))**:
     - Created `ExecuteCustomCommand(projectRoot, cmdLine string)` for cross-platform shell execution with timeouts and output capture.
     - Implemented `ReactEnsureDependenciesWithCustom`, `FlutterEnsureDependenciesWithCustom`, and `GenericEnsureDependenciesWithCustom` to prioritize custom install commands when configured.
     - Added `RunDiagnosticsWithCustom(projectRoot string, targetFiles []string, customBuildCmd string)`: runs custom compiler/build/typecheck commands (e.g. `pnpm typecheck`, `npm run build`, `flutter analyze`) and converts output into structured `CompilerDiagnostic` errors for autonomous code repair.
  3. **Multi-Agent Supervisor Pipeline ([`pkg/agents/supervisor.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/supervisor.go))**:
     - Added `CustomInstallCmd` and `CustomBuildCmd` to `SupervisorAgent`.
     - Automatically wired custom build commands into pre-flight baseline typechecking and post-refactor validation.
     - Executed custom install commands during Step 7 Dependency Resolution.
  4. **CLI Framework ([`cmd/langPeanut/run.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/cmd/langPeanut/run.go), [`cmd/langPeanut/install.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/cmd/langPeanut/install.go))**:
     - Added `--install-cmd` (alias `--custom-install-cmd`) and `--build-cmd` (alias `--custom-build-cmd`) flags to `langPeanut run`.
     - Added `--cmd` (alias `--custom-cmd`) to `langPeanut install`.
  5. **Interactive TUI ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go))**:
     - Added `customInstallCmd` and `customBuildCmd` state to `tui.Model`.
     - Wired custom build/install options into `startFullLocalization` and `startRefactor`.
  6. **Local Web Studio ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go))**:
     - Added `CustomInstallCmd` and `CustomBuildCmd` to `RunPipelineRequest` and `SaveSettingsRequest`.
     - Updated `/api/settings` and `/api/settings/save` to get and persist custom toolchain commands.
     - Added **Custom Toolchain Commands** Card in the Web Studio Settings tab with real-time saving and execution feedback.
  7. **Cloud Web Dashboard & Sandboxed Runner (`langpeanut-cloud/`)**:
     - Added migration `003_custom_toolchain_commands.sql` adding `custom_install_cmd` and `custom_build_cmd` to `repo_settings`.
     - Updated `RepoSettings` model, `UpsertRepoSettings`, and `GetRepoSettings` in `internal/db/queries.go`.
     - Updated API handler in `internal/api/handlers.go`.
     - Passed `CUSTOM_INSTALL_CMD` and `CUSTOM_BUILD_CMD` env vars in `internal/worker/worker.go` to ephemeral Docker runner containers.
     - Initialized `supervisor.CustomInstallCmd` and `supervisor.CustomBuildCmd` in `cmd/runner/main.go`.
     - Added Custom Install Command and Custom Build / Diagnostics inputs to Repository Settings modal and `.langpeanut.json` export in `web/app/page.tsx`.

* **Verification**:
  - `go test -v ./...` in `langpeanut_local`: 100% pass across all packages (including new `TestCustomCommands_InstallAndBuild`).
  - `go test -v ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Tested `langPeanut install ./examples/nextjs-app --cmd "echo 'Custom install passed!'"` (verified execution).
  - Recompiled and updated system binary `/Users/harmanpreetsingh/.local/bin/langPeanut`.

### Session Entry 112: Monorepo Root Subdirectory & Per-Repo / Per-Project Scoping

* **User Directives**:
  1. *"for cloud web, it could be per repo, for local it could be per project, and we should be also able to set root folder in cloud web coz its repo so thats also one thing"*

* **Context & Architectural Need**:
  - **Per-Repo vs. Per-Project Scoping**: Cloud Web operates on connected GitHub repositories (each having distinct `repo_settings`), while local CLI and Web Studio operate on local filesystem projects (each saving to `.langPeanut/config.json` inside its project root).
  - **Monorepo Subdirectory Support (`root_dir`)**: Many real-world GitHub repositories store applications in subdirectories (e.g. `apps/web`, `frontend/`, `packages/client`, `mobile/`). When `langpeanut-cloud` clones the repository, the localization supervisor and AST scouts must execute against the specified subdirectory root, while Git commits, branch creation, and PR operations continue operating at the repository root.

* **Actions Taken & Architecture Upgrades**:
  1. **Cloud Database Schema & Migrations (`langpeanut-cloud/`)**:
     - Updated migration `003_custom_toolchain_commands.sql` to add `root_dir TEXT NOT NULL DEFAULT ''` to `repo_settings`.
     - Updated `RepoSettings` model, `UpsertRepoSettings`, and `GetRepoSettings` in [`internal/db/queries.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/internal/db/queries.go).
  2. **Cloud API & Worker Subdirectory Forwarding (`langpeanut-cloud/`)**:
     - Updated `upsertSettingsReq` and `handleUpsertSettings` in [`internal/api/handlers.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/internal/api/handlers.go) to parse, validate, and store `root_dir`.
     - In [`internal/worker/worker.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/internal/worker/worker.go), injected `-e ROOT_DIR=` into the Docker runner container invocation.
  3. **Sandboxed Runner Monorepo Resolution ([`cmd/runner/main.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/cmd/runner/main.go))**:
     - Added `rootDir` to `runnerConfig`.
     - Resolved `projectDir := filepath.Join(cfg.workDir, cfg.rootDir)` for platform detection and AST supervisor execution, while maintaining `cfg.workDir` for Git checkout, branch creation, commit, and push.
  4. **Cloud Web Dashboard UI ([`web/app/page.tsx`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/web/app/page.tsx))**:
     - Added **📁 Project Root Subdirectory (Monorepos)** input field to the Repository Settings Modal (`apps/web`, `frontend`, `packages/app`).
     - Wired into state, `openSettingsModal`, `saveSettings`, and `.langpeanut.json` export.
  5. **Local Project-Scoped Configuration ([`pkg/memory/config.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/memory/config.go))**:
     - Verified that `SaveProject` saves `custom_install_cmd` and `custom_build_cmd` strictly into `<project_dir>/.langPeanut/config.json` without leaking to global configurations.

* **Verification**:
  - `go test -v ./...` in `langpeanut-cloud`: 100% pass (including updated `TestDB_MigrationsAndCRUD` with `RootDir` assertions).
  - `go test -v ./...` in `langpeanut_local`: 100% pass across all 15 packages.
  - Built binaries: `langPeanut` CLI, `langpeanut-cloud/server`, `langpeanut-cloud/runner`.

### Session Entry 113: Dedicated Workflow Completion Summary Screen & Interactive Dependency Install

* **User Directives**:
  1. *[Provided Screenshot]* *"also we need to install where we land after the workflow is complete coz it seems like confusing"*

* **Failure Mode & Confusion Observed**:
  - **Abrupt Fallthrough to Scan Audit Screen**: Previously, after completing 1-Click Localization or AST refactoring in the TUI, the handler called `return m, m.startScan()`, which triggered a fresh AST scan and immediately dumped the user back into `ViewAudit` ("Codebase Hardcoded String Audit Report").
  - Because non-UI tokens or variable snippets remained in the scan table (e.g. `flutter pub getflutter run`), users were disoriented into thinking the workflow hadn't finished or needed re-running.
  - **No Visibility into Dependency Resolution**: Users had no post-flight confirmation showing which packages were installed, which bootstrap setups were created, or an immediate shortcut to trigger/re-run dependency installation.

* **Actions Taken & Architecture Upgrades**:
  1. **New Dedicated Completion View (`ViewSummary`) ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go))**:
     - Added `ViewSummary` to `ViewState`.
     - Stored `lastPipelineResult`, `lastPipelineType`, and `depInstallStatus` on `tui.Model`.
     - Upon `fullLocDoneMsg`, `refactorDoneMsg`, or `translateDoneMsg`, transitioned cleanly to `ViewSummary`.
  2. **Comprehensive Execution Summary Dashboard (`renderSummaryView`) ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go))**:
     - **Project & Framework Header**: Displays target project, active framework, model, and tone preset.
     - **Framework Dependencies & Manifest Box**: Displays manifest status (`package.json`, `pubspec.yaml`), configured packages (`react-i18next`, `i18next`), bootstrap file creation (`src/i18n.ts`), and exact install command executed.
     - **Refactoring & Translation Catalogs Box**: Lists refactored source files, generated locale catalogs (`[en, es, ja, fr, de]`), key count, and compiler repair metrics.
     - **Action Shortcuts Bar**: Prominently shows `[i] Run Dependency Install`, `[w] Open Web Studio`, `[t] Token Stats`, `[a] Audit Codebase Strings`, `[r] Re-run Pipeline`, and `[Enter/Esc] Menu`.
  3. **Instant Dependency Installation Shortcut ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go))**:
     - Added `startInstallDeps()` asynchronous command triggered via `[i]` key on any view.
     - Handled `installDepsDoneMsg` with live feedback and status update.
  4. **Unit Test Suite ([`pkg/tui/tui_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/tui_test.go))**:
     - Added `TestTUI_WorkflowCompletionSummaryAndDependencyInstall` verifying clean transition to `ViewSummary`, all rendered summary boxes, and interactive `[i]` install execution.

* **Verification**:
  - `go test -v ./pkg/tui/...`: 100% pass across all 9 tests.
  - `go test -v ./...`: 100% pass across all 15 packages.
  - Recompiled and updated system binary `/Users/harmanpreetsingh/.local/bin/langPeanut`.

### Session Entry 114: Next.js App Router / React Server Components (RSC) `'use client'` Directive Injection & Healing

* **User Directives**:
  1. *[Provided Screenshot]* *"we also need to handle this with react/next.js i guess"*
  2. *Screenshot Error*: `Runtime TypeError: createContext only works in Client Components. Add the "use client" directive at the top of the file to use it. Read more: https://nextjs.org/docs/messages/context-in-server-component` at `components/layout/Footer.tsx (7:1) @ module evaluation: import { useTranslation } from 'react-i18next';`

* **Root Cause & RSC Analysis**:
  - In Next.js App Router (Next.js 13/14/15/16+), components inside `app/` and imported child components default to **React Server Components (RSC)**.
  - `react-i18next` and the `useTranslation()` hook internally invoke `React.createContext` for translation providers and state subscriptions.
  - When `langPeanut` refactored a component without a pre-existing `'use client'` directive to import `useTranslation`, Next.js evaluated the component on the server, triggering the fatal `createContext only works in Client Components` runtime error.

* **Actions Taken & Architectural Fixes**:
  1. **Deterministic `'use client'` Injection in Patch Engine ([`pkg/agents/patch_engine.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/patch_engine.go))**:
     - Added `EnsureUseClientDirective(src string) string` to insert `'use client';` at the very top of React/Next.js files (`.tsx`, `.jsx`, `.ts`, `.js`) whenever `react-i18next` / `useTranslation` is introduced.
     - Preserves existing file header comments, shebangs, or pre-existing `'use client'` / `"use client"` directives without duplicating.
     - Updated `injectImport` to properly insert imports below any top-level `'use client'` directive.
  2. **Autonomous Code Repair Rule 3 for RSC Diagnostics ([`pkg/agents/repair.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/repair.go))**:
     - Added autonomous heuristic repair for `createContext only works in Client Components` or `Add the "use client" directive` compiler/runtime diagnostics, ensuring any missing directives are automatically healed during self-correction reflection loops.
  3. **Directive Agent Component Synthesis ([`pkg/agents/directive_agent.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/directive_agent.go))**:
     - Updated `writeComponent` and the integration agent prompt to enforce `'use client';` on generated interactive React/Next.js widgets (e.g. `LanguageSwitcher.tsx`).
  4. **Unit & Regression Testing ([`pkg/agents/repair_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/repair_test.go), [`pkg/agents/agents_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/agents_test.go))**:
     - Added `TestEnsureUseClientDirective` (testing directive-less files, files with comments, and files with existing directives).
     - Added `TestCodeRepairAgent_NextJSUseClientRepair` (verifying automated healing of Next.js RSC `createContext` diagnostics).
     - Updated `TestPatchEngine_ApplyRefactorPlan` to assert `'use client'` presence in refactored TSX output.

* **Verification**:
  - `go test -v ./pkg/agents/...`: 100% pass.
  - `go test -v ./...` in `langpeanut_local`: 100% pass across all 15 packages.
  - `go test -v ./...` in `langpeanut-cloud`: 100% pass.
  - Recompiled CLI binary `/Users/harmanpreetsingh/.local/bin/langPeanut`.

---

## Session Entry 115 — Single-Pass Next.js App Router I18nProvider & Autonomous Navbar Directive Wiring

* **User Directives**:
  1. *"facing problem like i dont know but text is looking very with i18n implementation like see in these pics... fix it"*
  2. *"but this exists with single pass too like brand new project, also i added directive to add switch to navbar but only switcher file was created, not linked or added into navbar, and its not about fixing the damn nextjs project, langpeanut should have been able to handle this not u doing this manually"*

* **Root Cause & Architectural Analysis**:
  1. **Raw Key Rendering on Screen**:
     - `ensureReactI18nBootstrap` previously created an empty skeleton `i18n.ts` without importing and binding locale JSON files into `resources`.
     - Because `i18next` had no resources loaded, `t('key')` rendered the raw key name (e.g. `navbarPingroute`, `100LocalProbingMdashNo`) in the browser.
  2. **Next.js App Router Server Component Incompatibility**:
     - In Next.js App Router, `app/layout.tsx` is evaluated on the server as a Server Component.
     - Direct `import '../i18n'` in `layout.tsx` triggered `TypeError: createContext is not a function` during production builds because `initReactI18next` invokes `React.createContext`.
  3. **Directive Agent Missing Navbar Linking**:
     - `discoverRelevantFiles` walked project directories alphabetically and capped results at 20 files, filling the list with `app/.../page.tsx` before reaching `components/layout/Navbar.tsx`.
     - The LLM did not receive `Navbar.tsx` in its discovered files list, and `maxDirectiveTurns` was capped at 3 turns, causing it to create `LanguageSwitcher.tsx` without patching the parent Navbar.

* **Actions Taken & System-Wide Fixes**:
  1. **Dynamic Resource Bootstrap in React Platform ([`pkg/platforms/dependencies.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/platforms/dependencies.go))**:
     - Added `EnsureReactI18nBootstrapWithLocales(projectRoot, locales)` to dynamically import all configured locale JSON files (`en.json`, `es.json`, `fr.json`, `de.json`, `ja.json`) and pass them into `i18n.init({ resources: { ... } })`.
     - Connected directly into Step 7 of `SupervisorAgent.RunEndToEnd` so `i18n.ts` is always synchronized with all written locale catalogs.
  2. **Automated App Router `I18nProvider` Synthesis ([`pkg/platforms/dependencies.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/platforms/dependencies.go))**:
     - Updated `InjectReactI18nRootImport` to detect Next.js App Router (`app/layout.tsx`).
     - Automatically generates `components/I18nProvider.tsx` with `'use client';` and imports `i18n`.
     - Injects `<I18nProvider>{children}</I18nProvider>` in `app/layout.tsx`, keeping `layout.tsx` a pure Server Component while initializing client translation state cleanly.
  3. **Directive Agent 2-Step Protocol & Safety Fallback Linker ([`pkg/agents/directive_agent.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/directive_agent.go))**:
     - Updated `discoverRelevantFiles` to prioritize navigation, header, and layout files (`*nav*`, `*header*`, `*layout*`, `*bar*`, `*main*`).
     - Increased ReAct turns from 3 to 6 and updated system prompt with mandatory 2-step widget synthesis + parent mounting protocol.
     - Added deterministic `autoLinkComponent` safety fallback to guarantee `<LanguageSwitcher />` is rendered inside parent `Navbar.tsx` / `Header.tsx` action bars with Tree-Sitter syntax verification.
  4. **Regression & Unit Tests ([`pkg/agents/directive_agent_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/directive_agent_test.go))**:
     - Added `TestDirectiveAgent_AutoLinkComponent` to verify automatic import and JSX placement inside React navbars.

* **Verification**:
  - `go test -v ./...` in `langpeanut_local`: 100% pass across all test suites.
  - Recompiled binary `/Users/harmanpreetsingh/.local/bin/langPeanut`.
  - Executed end-to-end single-pass run on clean `pingroute-web`: `langPeanut run --source en --locales es,fr,de,ja --directive "Add a language switcher dropdown to Navbar"`.
  - Verified 300 keys extracted, 5 locale catalogs populated, `i18n.ts` created with full `resources`, `I18nProvider.tsx` wired into `app/layout.tsx`, and `LanguageSwitcher.tsx` created and mounted into `components/layout/Navbar.tsx`.

---

## Session Entry 116 — Existing Translations Conflict Strategies & Per-Repo Regeneration Preferences

* **User Directives**:
  > *"also we need to handle like what if user want to regenerate existing entries, selected some new languages and then went with it, we need to have handling on this like ux wise prompting user with what he wants, warn and if they want replace, skip or what for existing, for all the interfaces like web,tui,cloud web, and then also have setting what u want by default in settings if u dont want prompt then also in cloud web, we need to do preference by per repo"*

* **Architectural Scope & Strategy Triad**:
  Designed and implemented 3 distinct conflict resolution modes for handling pre-existing translations:
  1. `skip` (Incremental Delta — Default): Preserves existing translated values and human edits; only scans, extracts, and translates missing or newly introduced keys.
  2. `replace` / `overwrite` (Regenerate All): Translates and overwrites every key in target catalogs using current source strings and style presets.
  3. `prompt` (Interactive Confirmation): Prompts the user interactively in terminal CLI/TUI before taking action whenever existing catalogs are discovered.

* **Multi-Interface Implementations**:
  1. **Core Engine & Supervisor ([`pkg/agents/supervisor.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/supervisor.go), [`pkg/memory/config.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/memory/config.go))**:
     - Added `ExistingMode` property to `SupervisorAgent` and `ExistingTranslationsMode` to `AppConfig` / `ProjectConfig`.
     - In `SupervisorAgent.RunEndToEnd`, dynamically branches translation batches based on `isReplaceMode` vs `isSkipMode`.
     - Added unit test `TestSupervisorAgent_ReplaceExistingTranslations` in `pkg/agents/agents_test.go` (100% pass).
  2. **CLI Engine ([`cmd/langPeanut/run.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/cmd/langPeanut/run.go))**:
     - Added `--existing-mode` (`skip`, `replace`, `prompt`) and `--regenerate` / `--force` flags.
     - Added interactive terminal selector prompt when `--existing-mode=prompt` or when prompt is set in project configuration.
  3. **Local Web Mode Studio ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go))**:
     - Added `Existing Translations Strategy` dropdown in Runner screen toolbar.
     - Added `Default Strategy on Existing Translations` selector in Settings screen, persisting directly to `.langPeanut/config.json`.
     - Added `existing_mode` parameter in `/api/run` and `/api/settings/save`.
  4. **Cloud Web & Database ([`langpeanut-cloud`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud))**:
     - Added SQLite schema migration `004_existing_translations_mode.sql`.
     - Updated `RepoSettings` model, `UpsertRepoSettings`, and `GetRepoSettings` in `internal/db/queries.go`.
     - Updated `handleUpsertSettings` in `internal/api/handlers.go`.
     - Updated `launchSandbox` in `internal/worker/worker.go` and `cmd/runner/main.go` to pass and apply `EXISTING_TRANSLATIONS_MODE`.
     - Added 3-way visual Strategy selector (`⚡ Skip Existing`, `🔄 Regenerate All`, `❓ Prompt Me`) in the Cloud Web repository settings modal and export configuration generator (`web/app/page.tsx`).

* **Verification**:
  - `go test -v ./...` in `langpeanut_local`: 100% pass across all 15 packages.
  - `go test -v ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Recompiled CLI binary `/Users/harmanpreetsingh/.local/bin/langPeanut`.

---

## Session Entry 117 — Token Budget, Key Ceilings & Batch Concurrency Settings Integration

* **User Directives**:
  > *"also i had instructed to add the token setting, like being able to set the max token per but am not seeing those settings anywhere"*

* **Root Cause Analysis**:
  - The core engine had implemented dynamic model-aware chunking (`ChunkWordBudget`, `ChunkKeyCeiling`, `Concurrency` in `pkg/memory/config.go` and `pkg/agents/translator.go`), but the UI cards in the Local Web Studio and Cloud Web repository settings were omitted.
  - CLI flags only included `--chunk-words` without developer-intuitive aliases like `--max-tokens` and `--tokens-per-batch`.

* **Actions Taken & System-Wide Integration**:
  1. **Local Web Studio Settings Panel ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go))**:
     - Added a dedicated card: **`Token Budget & Batch Chunking Tunables`**.
     - Added inputs for:
       - **Word/Token Budget per Batch** (`settingsChunkWordBudget`, 0 = auto model-aware).
       - **Key Ceiling per Prompt** (`settingsChunkKeyCeiling`, 0 = auto model-aware).
       - **Parallel Concurrency** (`settingsConcurrency`, 1–50 simultaneous API calls).
     - Bound inputs in `loadSettings()` and persisted via `saveProjectSettings()` to `.langPeanut/config.json`.
  2. **Cloud Web Studio Modal ([`langpeanut-cloud/web/app/page.tsx`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/web/app/page.tsx))**:
     - Added the **`Token Budget & Batch Chunking Tunables`** card to the Repository Settings modal.
     - Added interactive inputs for `Max Word / Token Budget per Batch` and `Max Keys Ceiling per Prompt`.
     - Wired state to `openSettingsModal`, `saveSettings`, and `exportRepoConfig`.
  3. **CLI Flags & Aliases ([`cmd/langPeanut/run.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/cmd/langPeanut/run.go))**:
     - Added `--max-tokens` and `--tokens-per-batch` as direct aliases for `--chunk-words`.
     - Added `--keys-per-batch` as an alias for `--chunk-keys`.
     - Exposed `--concurrency` / `-c` for parallel worker tuning.

* **Verification**:
  - `go test -v ./...` in `langpeanut_local`: 100% pass across all packages.
  - `go test -v ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Recompiled CLI binary `/Users/harmanpreetsingh/.local/bin/langPeanut`.

---

## Session Entry 118 — Exact 50k Frontier vs 4k Standard Model Token Budget Harmonization

* **User Directives**:
  > *"i didn't understand why we have this, like i had instructured to have 50k token max for each call for frontier models and 4k for others"*

* **Clarification & Verification of Engine Logic**:
  - Validated that `TranslatorAgent.getEffectiveChunkSettings` in [`pkg/agents/translator.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/agents/translator.go) enforces the exact **50k vs 4k** split:
    - **Frontier Models (OpenAI, Claude, Gemini)**: Max token budget of **50,000 tokens** (~38,000 words, up to 1,500 keys per call).
    - **Standard / Local Models (Ollama, Custom endpoints)**: Max token budget of **4,000 tokens** (~3,000 words, up to 100 keys per call).
    - **NLLB-200**: 512 token sequence limit (~400 words, 50 keys).
  - Aligned Cloud Web backend handlers ([`internal/api/handlers.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/internal/api/handlers.go)) and Next.js UI state ([`web/app/page.tsx`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/web/app/page.tsx)) so the default budget initializes to **50,000 tokens / 1,500 keys** for Frontier models and **4,000 tokens / 100 keys** for Local/Custom models.

* **Verification**:
  - Unit test `TestTranslator_EffectiveChunkSettingsModelAware` passes with 100% assertion coverage.
  - `go test ./...` passes in both repositories.
  - Recompiled binary `/Users/harmanpreetsingh/.local/bin/langPeanut`.

---

## Session Entry 119 — Google Gemini Model Upgrade to `gemini-3.5-flash`

* **User Directives**:
  > *"swap 2.5-flash gemini with 3.5-flash"*

* **Actions Taken & System-Wide Updates**:
  1. **LLM Client & Cost Tracker ([`pkg/llm/client.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/llm/client.go), [`pkg/llm/tracker.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/llm/tracker.go))**:
     - Updated default Gemini model identifier to `"gemini-3.5-flash"`.
     - Added `"gemini-3.5-flash"` to the cost tracking rate matrix.
  2. **Local Web Studio ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go))**:
     - Updated Gemini option label and default selection to `"gemini-3.5-flash"`.
  3. **Interactive TUI ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go))**:
     - Updated model placeholder, default activation, step 1 onboarding option, and settings provider menu to `"gemini-3.5-flash"`.
  4. **Cloud Web & Tests ([`langpeanut-cloud/web/app/page.tsx`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/web/app/page.tsx), [`internal/api/handlers_test.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut-cloud/internal/api/handlers_test.go))**:
     - Updated `PROVIDER_MODELS.gemini.models` to `['gemini-3.5-flash', 'gemini-2.5-pro']`.
     - Updated API integration tests to use `gemini-3.5-flash`.

* **Verification**:
  - `go test -v ./...` in `langpeanut_local`: 100% pass across all packages.
  - `go test -v ./...` in `langpeanut-cloud`: 100% pass across all packages.
  - Recompiled CLI binary `/Users/harmanpreetsingh/.local/bin/langPeanut`.

---

## Session Entry 120 — Google Gemini 3.5 Flash Pricing & Cost Estimation Calibration

* **User Directives**:
  > *"Gemini 3.5 Flash costs $1.50 per million input tokens and $9.00 per million output tokens on the standard paid tier.Pricing BreakdownInput Price: $1.50 per 1M tokens (text, image, video, audio)Output Price: $9.00 per 1M tokens (including thinking tokens)Context Caching: $0.27 per 1M tokens plus $1.00 per 1M tokens per hour for storageFree Tier: Available with usage limits through Google AI Studio"*

* **Actions Taken & System-Wide Updates**:
  1. **Cost Tracker Matrix ([`pkg/llm/tracker.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/llm/tracker.go))**:
     - Configured exact pricing for `gemini-3.5-flash`: **$1.50 per 1M input tokens** and **$9.00 per 1M output tokens**.
     - Retained `gemini-2.5-flash` / `1.5-flash` for legacy logs ($0.075 / $0.30).
  2. **UI Price Labels & Tooltips**:
     - Updated Local Web Studio selector ([`pkg/web/server.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/web/server.go)) to `Google Gemini (gemini-3.5-flash — $1.50 in / $9.00 out per 1M)`.
     - Updated TUI provider selector ([`pkg/tui/app.go`](file:///Users/harmanpreetsingh/Public/Code/langpeanut_local/pkg/tui/app.go)) to `$1.50 in / $9.00 out per 1M — high efficiency & large batch context`.

* **Verification**:
  - `go test -v ./...` in `langpeanut_local`: 100% pass across all packages.
  - Recompiled CLI binary `/Users/harmanpreetsingh/.local/bin/langPeanut`.




