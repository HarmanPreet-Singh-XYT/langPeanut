
as you know flutter supports localization basically different languages into one app, and etc now its pretty facisnating tbh like u can create one app under different languages, and those all languages r great, but nowadays we have AI tools what if we could make like an AI LLM tool like cli or something that goes through code and goes through code replacing strings with variables, generating different languages, so auto generation of languages, u can like so we'll langchain to have multiple LLM support, with agent, tooling support and etc. this is my plan

Jun 25
This is a solid idea. The pain point is real — Flutter's localization setup (flutter_localizations, ARB files, AppLocalizations.of(context).someKey) is genuinely tedious to retrofit onto an existing codebase. You're basically proposing an automated localization agent that does the grunt work.

Let me think through the architecture with you:

What the agent needs to do

String extraction — scan Dart files, find hardcoded strings (tricky: distinguish UI strings from log messages, enum values, route names, etc.)
Refactoring — replace "Hello World" with AppLocalizations.of(context).helloWorld and generate the corresponding ARB key
ARB generation — write/update app_en.arb and then auto-translate into target locales
Validation — make sure no strings were missed, no keys collide, plurals/interpolations are handled correctly ({name}, {count})
Where it gets hard

Strings with variables: "Hello $name" → needs {name} placeholder in ARB
Plurals: "$count items" → ARB has a specific @ metadata format for pluralization
Context-aware exclusion: you don't want to localize "POST", "api/v1/users", or debugPrint("error: $e")
The refactor needs to inject context into widgets that didn't previously need it
Proposed stack

CLI (Python, Click or Typer)
  └── LangChain / LangGraph agent
        ├── Tool: FileScanner    → reads Dart files, extracts string literals
        ├── Tool: StringClassifier → LLM decides: localizable vs not
        ├── Tool: ARBWriter      → creates/updates .arb files
        ├── Tool: CodeRefactorer → rewrites Dart files with l10n references
        └── Tool: Translator     → generates app_fr.arb, app_es.arb, etc.

Multi-LLM via LangChain: swap between GPT-4o / Claude / Gemini
The smart part is the StringClassifier tool — this is where LLM judgment actually earns its place. Rule-based regex will catch strings but can't reliably decide intent. An LLM with a good prompt ("is this a user-facing string or a code/system string?") handles the ambiguous cases.

ARB format awareness is also something you'd want to encode in the prompt — the model needs to know about @key metadata blocks for plurals and placeholders.

A few questions to sharpen the plan:

Do you want this to work on existing codebases (retrofit) or also scaffold localization from scratch on new projects?
Are you thinking destructive refactor (modifies files in place) or diff/patch mode (shows what it would change, user approves)?
For translation, are you fine with LLM-generated translations or do you want to also support plugging in DeepL/Google Translate for higher quality at scale?
This fits cleanly in the same space as CodiLay — a developer CLI tool that uses LLMs to automate something tedious. You already have the pattern down.

what other features can it have, what's would be workflow, UX and how would users use it, making it toward developers

Jun 25
Great direction. Let me map this out fully — features, workflow, and the developer UX end to end.

Core Feature Set
1. Localization Audit
Before touching anything, the tool scans the codebase and gives a report:

How many hardcoded strings found
Which files have the most violations
Estimated ARB keys that would be generated
Strings it's unsure about (flagged for human review)
This gives developers confidence before running a destructive operation.

2. Smart String Extraction

Detects string literals in Dart files
Classifies each as: localizable, skip, or review
Understands template strings with $variable and converts to ARB {placeholder} syntax
Handles plurals, gender variants
Lets you define exclusion patterns in config (routes, enums, log strings)
3. Code Refactoring

Replaces hardcoded strings with AppLocalizations.of(context).keyName
Auto-generates camelCase key names from the string content
Injects BuildContext where needed
Handles const widget breaking (since l10n calls aren't const)
4. ARB Generation

Creates app_en.arb as the source of truth
Generates @metadata blocks for plurals and placeholders
Merges into existing ARB files without clobbering manual edits
5. Multi-Language Translation

Translates source ARB into any target locales
Multiple backends: Claude, GPT-4o, Gemini, DeepL, Google Translate
Per-key confidence scores, flags low-confidence translations for review
Supports re-translating only new/changed keys (incremental)
6. Interactive Review Mode

Before writing anything, shows a TUI diff of every proposed change
Developer can approve / skip / edit each string inline
Bulk approve by file or confidence threshold
7. Watch Mode

Monitors Dart files for new hardcoded strings as you code
Surfaces warnings in terminal as you save
Optional VS Code extension integration (same pattern as CodiLay)
8. Config File

yaml
# flutterlocal.yaml
source_locale: en
target_locales: [fr, es, de, ja, ar]
llm_provider: claude          # claude | openai | gemini
translation_provider: deepl   # llm | deepl | google
arb_output: lib/l10n/
exclude_patterns:
  - "^api/"
  - "^http"
  - "debug|log|error"
exclude_files:
  - lib/constants/routes.dart
  - lib/models/
key_style: camelCase          # camelCase | snake_case
review_mode: interactive      # interactive | auto | dry-run
confidence_threshold: 0.85
9. Key Management

Detects duplicate strings → reuses existing key
Detects orphaned keys in ARB (translated but no longer in code) → flags for cleanup
Rename a key and it propagates across all ARB files and all Dart references
10. CI/CD Integration

--ci flag: exits with non-zero if new hardcoded strings are detected
GitHub Action that comments on PRs with a localization audit report
JSON/SARIF output format for integration with linters
Workflow End to End
New project                     Existing project
     │                                │
     ▼                                ▼
flutterlocal init           flutterlocal audit
(scaffolds l10n setup)      (non-destructive scan report)
     │                                │
     └──────────────┬─────────────────┘
                    ▼
          flutterlocal extract
      (classifies + extracts strings)
                    │
                    ▼
          Interactive TUI review
       (approve / skip / edit each)
                    │
                    ▼
          flutterlocal refactor
        (rewrites Dart files, generates ARB)
                    │
                    ▼
          flutterlocal translate --locales fr,es,de
        (fills target ARB files, flags low confidence)
                    │
                    ▼
          flutterlocal validate
        (checks missing keys, broken placeholders, orphans)
                    │
                    ▼
                  Done ✓
          (runs flutter gen-l10n automatically)
CLI UX — Commands
bash
# Onboarding
flutterlocal init                          # scaffolds pubspec, l10n.yaml, ARB dirs

# Audit (read-only, always safe to run)
flutterlocal audit                         # full report
flutterlocal audit --file lib/screens/home.dart

# Extraction + Refactor
flutterlocal extract                       # classify strings, enter review TUI
flutterlocal extract --auto                # skip TUI, use LLM confidence scores
flutterlocal extract --dry-run             # show what would change, touch nothing

# Translation
flutterlocal translate                     # translate all missing keys in all locales
flutterlocal translate --locale fr         # single locale
flutterlocal translate --provider deepl    # override provider
flutterlocal translate --incremental       # only new/changed keys

# Key management
flutterlocal keys list                     # all keys + usage locations
flutterlocal keys orphans                  # keys in ARB but not in code
flutterlocal keys rename oldKey newKey     # rename + propagate everywhere

# Watch
flutterlocal watch                         # monitor for new hardcoded strings

# CI
flutterlocal audit --ci --fail-on-new      # exits 1 if violations found
TUI Review Experience
This is the most important UX moment — where the developer stays in control:

┌─ String Review (34 of 112) ──────────────────────────────────┐
│                                                               │
│  File: lib/screens/home_screen.dart:47                       │
│                                                               │
│  Found:    "Welcome back, $name!"                            │
│  Key:      welcomeBackName                                    │
│  ARB:      "Welcome back, {name}!"                           │
│  Confidence: ████████░░ 84%  →  localizable                  │
│                                                               │
│  Refactor preview:                                            │
│  - Text("Welcome back, $name!")                              │
│  + Text(AppLocalizations.of(context).welcomeBackName(name))  │
│                                                               │
│  [A] Approve  [S] Skip  [E] Edit key  [V] View file  [Q] Quit│
└───────────────────────────────────────────────────────────────┘
Keyboard-driven, fast, feels like a git interactive rebase — developers already know this pattern.

What Makes it Stand Out
Audit-first philosophy — never touches code without the developer seeing a report
Confidence scoring — LLM isn't treated as infallible, uncertain strings surface for human judgment
Incremental — works on codebases that are partially localized already, doesn't nuke existing ARB
Multi-LLM — not locked to one provider, developer can use what they already pay for
Fits the existing Flutter mental model — generates standard ARB + flutter gen-l10n, no proprietary format lock-in
This positions it well as a tool developers actually trust to run on production codebases — which is the bar you need to clear for something that rewrites source files.

also keeping track, like we don't want to use LLM everywhere, we want to have normal code logic as much as we can instead of parsing everything with LLM like specifically targeting elements like const Text() and etc u can add, we can get all those like what if something was newly added or anything etc, using git tracking, and etc

Jun 25
Exactly the right instinct — LLM calls cost money and are slow, so you want them only where human-level judgment is actually needed. The rest should be fast, deterministic, static analysis.

The Two-Layer Architecture
Layer 1: Static Analysis (free, fast, deterministic)
  └── AST parsing, regex, git diff, pattern matching
        → handles 80-90% of all strings

Layer 2: LLM (paid, slow, judgment-based)
  └── only called for ambiguous cases Layer 1 can't resolve
        → maybe 10-20% of strings
The goal is to make Layer 2 as small as possible.

Layer 1 — Static Analysis in Detail
AST-based Dart parsing is the core. Instead of regex on raw text, you parse the Dart file into an AST and target specific node types:

Targeted nodes (high confidence = localizable):
  ConstText("...")           → Text widget, almost always UI
  Text("...")
  TextSpan(text: "...")
  TextButton(child: Text(...))
  SnackBar(content: Text(...))
  AppBar(title: Text(...))
  InputDecoration(labelText: "...", hintText: "...", errorText: "...")
  ElevatedButton(child: Text(...))
  Tooltip(message: "...")
  AlertDialog(title: Text(...), content: Text(...))
  DropdownMenuItem(child: Text(...))

Auto-skip nodes (high confidence = not localizable):
  debugPrint("...")
  print("...")
  log("...")
  throw Exception("...")
  assert(condition, "...")
  Uri.parse("...")
  RegExp("...")
  Color(0xFF...)
  named params: key:, route:, tag:, semanticLabel: (configurable)
  strings inside model files, repository files, service files
This alone catches the vast majority without any LLM involvement. You know const Text("Submit") is localizable. You know debugPrint("user fetched") is not. No AI needed.

Pattern-based exclusions on the string content itself:

python
SKIP_PATTERNS = [
    r'^https?://',          # URLs
    r'^/[a-z]',             # routes like /home, /profile
    r'^\d+$',               # pure numbers
    r'^[A-Z_]{2,}$',        # constants like API_KEY, POST
    r'^\s*$',               # whitespace
    r'^#[0-9a-fA-F]{3,8}$', # hex colors
    r'\.(dart|json|png|svg)$', # file paths
]
Git Tracking — The Delta Engine
This is what makes it production-safe and incremental. You never re-scan the whole codebase on every run.

How it works:

bash
# On first run — full scan, write a baseline
flutterlocal extract
# stores: .flutterlocal/baseline.json
# contains: { file: hash, strings: [...], keys: {...} }

# On subsequent runs — git diff only
flutterlocal extract
# git diff HEAD .flutterlocal/baseline_commit → changed files only
# only scans those files, merges results with existing baseline
What gets tracked in .flutterlocal/baseline.json:

json
{
  "commit": "a3f9c12",
  "scanned_files": {
    "lib/screens/home_screen.dart": {
      "hash": "d41d8cd9",
      "strings": [
        {
          "raw": "Welcome back, $name!",
          "key": "welcomeBackName",
          "line": 47,
          "node_type": "Text",
          "status": "extracted",
          "translated": ["fr", "es", "de"]
        }
      ]
    }
  }
}
Git integration flows:

bash
# New file added → full scan of that file
# Modified file → re-scan only changed lines (git diff --unified=0)
# Deleted file → remove its strings from baseline, flag orphaned ARB keys
# Renamed file → carries string status forward, no re-scan needed
Watch mode uses git diff on save:

lib/screens/checkout.dart modified
  → 2 new hardcoded strings detected
  → "Place Order"  (Text widget, line 89)  → auto-extract
  → "order_ref_$id" (suspicious)           → flagged for review
The LLM Escalation Path
Only strings that fall through static analysis go to the LLM:

Static analysis result:
  LOCALIZABLE    → extract immediately, no LLM
  SKIP           → ignore, no LLM
  UNCERTAIN      → queue for LLM batch call

Uncertain triggers:
  - String in a widget but contains code-like patterns
  - String in an ambiguous context (custom widget, unknown parent)
  - String that matches neither skip patterns nor known widget types
  - String inside a ternary or conditional expression
LLM call is batched — you don't call it per-string, you accumulate all uncertain strings and send them in one call:

python
# Instead of: 1 LLM call per uncertain string (expensive)
# Do: 1 LLM call per file, or 1 call per batch of 50 strings

prompt = """
Classify each string as LOCALIZABLE or SKIP.
LOCALIZABLE = shown to users in the UI.
SKIP = internal code, logs, routes, keys, IDs.

Strings:
1. "order_confirmed" (inside CustomStatusWidget)
2. "retry" (inside catch block custom handler)
3. "v2/payments" (inside a Text widget displaying API info to developer)
...

Return JSON array: [{"id": 1, "result": "SKIP"}, ...]
"""
One batched call per uncertain group. Cheap.

String Identity and Deduplication
"Submit"  appears in 14 files
  → one ARB key: submit
  → all 14 references point to same key
  → translated once

"Cancel"  appears in 8 files
  → one ARB key: cancel

"Cancel"  appears in a dialog with different meaning
  → LLM or dev decides: same key or cancelSubscription?
  → configurable: strict_dedup: false allows context-based keys
The Full Decision Tree Per String
String found in AST
        │
        ▼
  Known widget node?  ──Yes──→  LOCALIZABLE ──→ Extract
        │
        No
        ▼
  Matches skip pattern? ──Yes──→ SKIP ──→ Ignore
        │
        No
        ▼
  In excluded file/dir? ──Yes──→ SKIP ──→ Ignore
        │
        No
        ▼
  In baseline already? ──Yes──→ Use cached status
        │
        No
        ▼
  Queue for LLM batch
        │
        ▼
  LLM returns LOCALIZABLE/SKIP
        │
        ▼
  Cache result in baseline ──→ Never ask LLM about this string again
That last point is key — once the LLM classifies a string, that decision is stored. Same string in a different file? Reuse the cached decision. The LLM cost is a one-time spend per unique uncertain string.

What This Looks Like in Practice
On a 50k line Flutter codebase, realistically:

Total strings found:        ~800
Auto-extracted (Layer 1):   ~680  (85%)  → 0 LLM calls
Auto-skipped (Layer 1):      ~90  (11%)  → 0 LLM calls
Sent to LLM (uncertain):     ~30  (4%)   → 1-2 batched calls

On subsequent runs (git delta):
  Changed files: 3
  New strings: 8
  LLM calls needed: 0-1
Fast, cheap, and the developer only sees strings that actually need their attention.

also we need logic that prevents like sending a lot of stuff like what if the content becomes 10k+, that's where we need to send in batches, and also cant send small api calls so we dont get rate limited and automatic rate limit handling, additionally edge cases handling like resume and etc we need to think a lot of about controls, edge case, ux

Jun 25
Great — this is the infrastructure layer that separates a toy CLI from a tool developers actually trust on real codebases. Let me map it all out.

Batching Strategy
The core problem: you have 800 strings, LLM context limits, rate limits, and you need it to feel seamless.

Batch sizing logic:

python
class BatchManager:
    MAX_TOKENS_PER_BATCH = 6000   # leave headroom for response
    MIN_STRINGS_PER_BATCH = 5     # never send tiny calls
    MAX_STRINGS_PER_BATCH = 50    # cap for response reliability
    MIN_DELAY_BETWEEN_CALLS = 1.2 # seconds, prevents burst rate limiting

    def build_batches(self, strings: list[UncertainString]) -> list[Batch]:
        # Token-aware packing, not just count-based
        # Short strings → more per batch
        # Long strings with interpolation → fewer per batch
        # Never split strings from the same file across batches
        # (preserves context for better LLM decisions)
The key insight: batch by token budget, not string count. "OK" and "Are you sure you want to permanently delete your account and all associated data?" are not equivalent batch units.

Rate Limit Handling
Three tiers of protection:

Tier 1: Proactive throttling
  → enforce MIN_DELAY between every call
  → never burst, even if rate limit hasn't been hit

Tier 2: Reactive backoff on 429
  → exponential backoff with jitter
  → 1s → 2s → 4s → 8s → 16s → give up after 5 retries
  → jitter prevents thundering herd if multiple processes run

Tier 3: Provider-aware limits
  → Claude:  ~1000 RPM on Sonnet, track per-minute window
  → OpenAI:  TPM limits vary by tier, track tokens sent
  → Gemini:  different limits per model
  → config lets user declare their tier to set limits correctly
Token budget tracking per provider:

python
class RateLimitTracker:
    def __init__(self, provider: str, tier: str):
        self.window = deque()  # timestamps of calls in last 60s
        self.token_window = deque()  # (timestamp, tokens) in last 60s
        self.limits = PROVIDER_LIMITS[provider][tier]

    def can_send(self, estimated_tokens: int) -> tuple[bool, float]:
        # returns (can_send_now, wait_seconds)
        # checks both RPM and TPM windows
        # returns precise wait time, not a guess
Resume System
This is what makes it production-safe. If anything fails mid-run — network drop, user Ctrl+C, API outage — you pick up exactly where you left off.

Session state stored in .flutterlocal/session/:

.flutterlocal/
  session/
    current.json          # active session (deleted on clean finish)
    last_completed.json   # last successful run (for reference)
  baseline.json
  cache/
    llm_decisions.json    # permanent cache of LLM classifications
    translations.json     # permanent cache of all translations
current.json structure:

json
{
  "session_id": "2024-01-15-143022",
  "stage": "translating",
  "started_at": "2024-01-15T14:30:22Z",
  "config_hash": "a3f9c12",
  "progress": {
    "extract": { "status": "done", "total": 312, "processed": 312 },
    "classify": { "status": "done", "total": 34, "processed": 34 },
    "refactor": { "status": "done", "total": 28, "processed": 28 },
    "translate": {
      "status": "in_progress",
      "locales": {
        "fr": { "status": "done",        "total": 28, "processed": 28 },
        "es": { "status": "in_progress", "total": 28, "processed": 17 },
        "de": { "status": "pending",     "total": 28, "processed": 0  },
        "ja": { "status": "pending",     "total": 28, "processed": 0  }
      }
    }
  },
  "pending_review": [
    { "string": "retry", "file": "lib/widgets/error.dart", "line": 23 }
  ]
}
On startup, always check for incomplete session:

$ flutterlocal translate

⚠  Incomplete session found (started 14 minutes ago)
   Stage: translating — es 17/28 done, de and ja pending

   [R] Resume   [D] Discard and start fresh   [I] Inspect session
Resume is the default — pressing Enter resumes. Developer never loses work.

The Permanent Cache
Separate from session state. Lives forever until explicitly cleared.

python
# LLM classification cache — keyed by (string_content, node_type)
# "Submit" in a Text() → LOCALIZABLE
# Seen again in a different file? Cache hit, no LLM call.

# Translation cache — keyed by (arb_key, source_text, target_locale, provider)
# If the English string hasn't changed, never re-translate it
# Changing provider doesn't invalidate — keeps old translations unless --force

cache_key = hashlib.sha256(
    f"{source_text}:{target_locale}:{provider}".encode()
).hexdigest()
Cache invalidation triggers:

LLM classification cache:
  → never auto-invalidates (deterministic enough)
  → flutterlocal cache clear --classifications

Translation cache:
  → invalidated when source string changes (hash mismatch)
  → NOT invalidated when unrelated strings change
  → flutterlocal cache clear --translations --locale fr
  → flutterlocal cache clear --key welcomeBackName
Edge Cases and Controls
File-level atomicity:

Never partially rewrite a file.
Write to lib/screens/home_screen.dart.tmp first,
validate the output parses as valid Dart,
then rename to lib/screens/home_screen.dart atomically.
If validation fails → keep original, log error, continue to next file.
Git safety checks before any refactor:

$ flutterlocal extract

Checking git status...
⚠  3 files have uncommitted changes:
   lib/screens/home.dart
   lib/screens/profile.dart
   lib/widgets/button.dart

Proceeding will modify these files. Options:
  [S] Stash changes and continue
  [C] Continue anyway (not recommended)
  [A] Abort

Recommendation: commit or stash first so you can diff the refactor cleanly.
Context injection edge cases:

dart
// Case 1: StatelessWidget — needs context from build()
// Handled: inject via BuildContext context parameter

// Case 2: String used outside widget tree
final title = "My App";  // in initState, outside build
// Handled: flag for manual review, don't auto-refactor

// Case 3: const constructor breaking
const MyWidget(label: "Submit")
// Handled: remove const, add comment // l10n: was const

// Case 4: String in a list literal
final items = ["Home", "Profile", "Settings"];
// Handled: extract each, but flag — may need manual restructuring
Conflict detection on ARB merge:

Scenario: developer manually edited app_fr.arb
  → tool detects file hash changed since last run
  → diffs proposed changes against manual edits
  → shows conflict resolution UI, never overwrites silently

Scenario: two keys would generate same name
  "submit" → submit
  "Submit" → submit  (collision)
  → auto-suffix: submitButton vs submitLabel based on parent node type
  → or prompt developer to name one manually
Partial translate failure:

Translating 28 strings to Japanese...
  ████████████░░░░  18/28

API error on string 19: context length exceeded

Handling:
  → save progress (strings 1-18 cached)
  → split remaining strings into smaller batch
  → retry string 19 alone
  → if still fails → mark as NEEDS_MANUAL in ARB with placeholder
     "welcomeBack": "⚠️ NEEDS_TRANSLATION: Welcome back, {name}!"
  → report at end: 1 string needs manual translation in ja
Progress UX During Long Runs
$ flutterlocal translate --locales fr,es,de,ja,ar,zh,ko,pt

Translating 847 strings across 8 locales...

  French       ████████████████████  212/212  ✓ done        (cached: 198, new: 14)
  Spanish      ████████████████░░░░  178/212  translating...
  German       ████░░░░░░░░░░░░░░░░   52/212  queued
  Japanese     ░░░░░░░░░░░░░░░░░░░░    0/212  queued
  Arabic       ░░░░░░░░░░░░░░░░░░░░    0/212  queued
  Chinese      ░░░░░░░░░░░░░░░░░░░░    0/212  queued
  Korean       ░░░░░░░░░░░░░░░░░░░░    0/212  queued
  Portuguese   ░░░░░░░░░░░░░░░░░░░░    0/212  queued

  Rate limit status:  RPM 34/60  │  TPM 41k/90k  │  Next call in 1.2s
  Session:  resumable  │  Cache hits: 198  │  Elapsed: 0:02:14
  Est. remaining: ~8 minutes     [P] Pause   [Q] Quit and save
Key details: cache hits shown prominently (developers want to see they're not being charged for re-work), rate limit meter is live, always pauseable.

Pause/quit is always safe:

[P] pressed

Pausing after current batch completes...
  ✓ Batch 7/24 finished. State saved.

  Resume anytime with: flutterlocal translate --resume
  Session expires: never (saved to .flutterlocal/session/current.json)
The Controls Surface
bash
# Batch controls
flutterlocal translate --batch-size 20        # override auto batch sizing
flutterlocal translate --delay 2.0            # force delay between calls (seconds)
flutterlocal translate --concurrency 1        # default; increase if provider allows

# Rate limit controls
flutterlocal config set rpm-limit 60          # declare your tier's RPM
flutterlocal config set tpm-limit 90000       # declare your tier's TPM
flutterlocal config set provider-tier pro     # auto-set known limits

# Resume controls
flutterlocal resume                           # resume last incomplete session
flutterlocal session list                     # show all sessions
flutterlocal session inspect                  # show current session detail
flutterlocal session discard                  # clear and start fresh

# Cache controls
flutterlocal cache stats                      # show cache size and hit rate
flutterlocal cache clear --translations       # force re-translate everything
flutterlocal cache clear --locale ja          # re-translate one locale only
flutterlocal cache clear --older-than 30d     # prune stale cache entries

# Safety controls
flutterlocal extract --dry-run                # show changes, touch nothing
flutterlocal extract --no-git-check          # skip git status check
flutterlocal extract --backup                 # zip original files before refactor
What Gets Logged
Every run writes to .flutterlocal/logs/YYYY-MM-DD-HHMMSS.log:

[14:30:22] Session started: 2024-01-15-143022
[14:30:22] Git commit: a3f9c12, 3 files changed since baseline
[14:30:23] Scanned 3 files, found 34 strings
[14:30:23] Layer 1: 28 auto-classified (82%), 6 uncertain → LLM queue
[14:30:24] LLM batch 1/1: 6 strings, 1240 tokens, provider: claude
[14:30:26] LLM response: 5 LOCALIZABLE, 1 SKIP (retry in catch block)
[14:30:26] Cache: wrote 6 new classifications
[14:30:27] Refactor: home_screen.dart — 12 strings replaced, validated ✓
[14:30:27] Refactor: profile_screen.dart — 8 strings replaced, validated ✓
[14:30:28] ARB: app_en.arb updated, 20 keys added, 0 conflicts
[14:30:28] Translate: fr batch 1/1 — 20 strings, provider: deepl
[14:31:02] Translate: fr complete — 20/20 done
[14:31:02] Session complete. Wrote last_completed.json
Logs are the safety net. If a developer sees something wrong in their codebase after running the tool, they can trace exactly what happened and why.

The mental model you're building toward is: the tool should feel like a careful, experienced engineer pair-programming with you — it does the mechanical work fast, tells you exactly what it's doing, never surprises you, and always leaves you with a way out.

checkpoints to be able to go at starting when it wasnt started or in between

Jun 25
Exactly — checkpoints are essentially save states. Let me think through this properly.

Checkpoint Mental Model
The key distinction is three different things people mean by "go back":

1. Before anything started     → pre-run snapshot (git stash equivalent)
2. Between pipeline stages     → stage checkpoints  
3. Mid-stage (batch recovery)  → batch-level resume points
All three need different handling.

Checkpoint Types
Type 1: Pre-Run Snapshot

Before the tool touches a single file, it captures everything:

$ flutterlocal extract

Creating checkpoint: pre-run-2024-01-15-143022
  ✓ Snapshotting 47 Dart files
  ✓ Copying existing ARB files
  ✓ Recording git HEAD: a3f9c12
  ✓ Checkpoint saved → .flutterlocal/checkpoints/pre-run-2024-01-15-143022/

Proceeding with extraction...
This is always created automatically, no flags needed. Developer never has to think about it.

Type 2: Stage Checkpoints

After each pipeline stage completes successfully:

.flutterlocal/checkpoints/
  pre-run-2024-01-15-143022/          ← before anything
  post-extract-2024-01-15-143045/     ← strings extracted, nothing refactored yet
  post-refactor-2024-01-15-143201/    ← dart files rewritten, ARB generated
  post-translate-fr-2024-01-15-143512/ ← french done, others pending
  post-translate-es-2024-01-15-143821/
Each checkpoint is lightweight — it only stores diffs from the previous checkpoint, not full file copies. Except the pre-run snapshot which stores full copies.

Type 3: Batch Checkpoints (Mid-Stage)

For long operations like translation across many locales, checkpoints happen within a stage too:

Translating Japanese... (212 strings)

  Batch  1/11 ✓  checkpoint saved
  Batch  2/11 ✓  checkpoint saved
  Batch  3/11 ✗  API error
  
  → Rolling back to batch 2/11 checkpoint
  → Retrying batch 3/11 with smaller batch size...
Checkpoint Storage Structure
.flutterlocal/checkpoints/pre-run-2024-01-15-143022/
  manifest.json          ← what this checkpoint contains and how to restore
  dart_files/            ← full copies of all dart files at this point
    lib/screens/home_screen.dart
    lib/screens/profile_screen.dart
    ...
  arb_files/             ← full copies of all ARB files
    app_en.arb
    app_fr.arb
  state.json             ← pipeline state at this point

.flutterlocal/checkpoints/post-extract-2024-01-15-143045/
  manifest.json
  dart_files/            ← only files that changed since pre-run (diff-based)
  arb_files/             ← only changed ARB files
  state.json
  extraction_results.json ← what was extracted, so restore knows what to undo
manifest.json:

json
{
  "id": "post-extract-2024-01-15-143045",
  "type": "stage",
  "stage": "extract",
  "created_at": "2024-01-15T14:30:45Z",
  "git_commit": "a3f9c12",
  "previous_checkpoint": "pre-run-2024-01-15-143022",
  "files_changed": 12,
  "is_full_snapshot": false,
  "restorable": true,
  "summary": "312 strings extracted, 28 localizable, 0 files refactored yet"
}
Rollback UX
Interactive rollback:

$ flutterlocal rollback

Available checkpoints:

  #  Checkpoint                          Stage          When            Summary
  ─────────────────────────────────────────────────────────────────────────────────
  1  pre-run-2024-01-15-143022          (start)        14 min ago      Before anything ran
  2  post-extract-2024-01-15-143045     extract done   13 min ago      312 strings found
  3  post-refactor-2024-01-15-143201    refactor done  12 min ago      47 files rewritten
  4  post-translate-fr-2024-01-15-1435  fr done        8 min ago       French complete
→ 5  post-translate-es-2024-01-15-1438  es done        5 min ago       Spanish complete
     (current)

Restore to which checkpoint? [1-4] or [Q] quit:
Arrow shows where you currently are. Numbers go backwards. Simple.

After selecting:

Restore to: pre-run-2024-01-15-143022 (before anything ran)?

This will:
  → Restore 47 Dart files to original state
  → Restore app_en.arb, app_fr.arb, app_es.arb
  → Clear current session state
  → NOT clear LLM classification cache (those decisions were correct)
  → NOT clear translation cache (reusable if you run again)

Current checkpoint will be saved as:
  abandoned-2024-01-15-144501 (recoverable for 7 days)

[Y] Confirm   [N] Cancel
Two important points surfaced here:

LLM cache is preserved on rollback — you already paid for those classifications
The checkpoint you're rolling back from is kept as abandoned, not deleted — you can un-rollback
Un-Rollback (Forward Again)
$ flutterlocal rollback

Available checkpoints:

  1  pre-run-2024-01-15-143022     (current — restored 2 min ago)
  2  post-extract-2024-01-15-143045
  3  post-refactor-2024-01-15-143201
  4  post-translate-fr-2024-01-15-1435
  5  post-translate-es-2024-01-15-1438  ← abandoned, recoverable
Abandoned checkpoints are visually distinct but still selectable. Developer changed their mind? One command to go back forward.

Checkpoint Lifecycle
Created:    automatically at each stage boundary
            automatically every N batches during long stages
            manually: flutterlocal checkpoint save "before trying new config"

Kept:       last 10 checkpoints always kept
            pre-run checkpoints kept for 30 days
            manual checkpoints kept until explicitly deleted

Pruned:     flutterlocal checkpoint prune        → removes old auto checkpoints
            flutterlocal checkpoint prune --all  → everything except current
            auto-pruned if .flutterlocal/ exceeds 500MB
Manual Checkpoints
For when the developer wants explicit control:

bash
# Save a named checkpoint at any point
flutterlocal checkpoint save "before switching to deepl"
flutterlocal checkpoint save "post-review approved strings"

# List all checkpoints
flutterlocal checkpoint list

# Restore specific one by name or id
flutterlocal checkpoint restore "before switching to deepl"
flutterlocal checkpoint restore 3

# Compare two checkpoints
flutterlocal checkpoint diff 2 4
# shows: which files changed, which ARB keys added, which strings refactored

# Delete a specific checkpoint
flutterlocal checkpoint delete 2
The Diff View
When comparing checkpoints, developers need to see what actually changed — not just that files changed:

$ flutterlocal checkpoint diff 1 3

Checkpoint diff: pre-run → post-refactor

Dart files changed: 12
  lib/screens/home_screen.dart
    line 47:  - Text("Welcome back, $name!")
              + Text(AppLocalizations.of(context).welcomeBackName(name))
    line 89:  - Text("Sign out")
              + Text(AppLocalizations.of(context).signOut)
    + 8 more changes

ARB files created/modified:
  app_en.arb  → 28 keys added
    + "welcomeBackName": "Welcome back, {name}!"
    + "signOut": "Sign out"
    + 26 more keys

  app_fr.arb  → not yet created at this checkpoint

LLM calls made between checkpoints:
  6 strings classified  (1 batch call, ~1200 tokens)

[V] View full diff   [R] Restore to checkpoint 1   [Q] Back
How It All Fits Together
Start
  │
  ▼
checkpoint: pre-run ◄─────────────────────────────────────────┐
  │                                                            │
  ▼                                                       rollback
Extract strings                                               │
  │                                                            │
  ▼                                                            │
checkpoint: post-extract ◄──────────────────────────────┐     │
  │                                                      │     │
  ▼                                                 rollback   │
Interactive review TUI                                    │     │
  │                                                      │     │
  ▼                                                      │     │
checkpoint: post-review ─────────────────────────────────┘     │
  │                                                            │
  ▼                                                            │
Refactor dart files                                            │
  │                                                            │
  ▼                                                            │
checkpoint: post-refactor ─────────────────────────────────────┘
  │
  ▼
Translate [batch 1] → checkpoint
Translate [batch 2] → checkpoint
Translate [batch 3] → checkpoint  ← error → auto-rollback to batch 2
  │
  ▼
checkpoint: post-translate-{locale}
  │
  ▼
Done ✓  →  session complete, checkpoints kept for 30 days
The developer can jump to any node in this graph at any time. That's the guarantee.

ability to add different languages which user specified or exclude those ones and etc

Jun 25
Good addition. Language management needs to be a first-class feature, not an afterthought in a config file.

Language Management as a First-Class Feature
Three distinct concerns:

1. Discovery    → what locales exist, what's their status
2. Targeting    → which locales to include/exclude and why
3. Lifecycle    → adding new ones, removing old ones, partial states
The Locale Registry
Every locale has a tracked state, not just present/absent:

$ flutterlocal locales list

Configured locales (source: flutterlocal.yaml + overrides)

  Locale   Name              Status         Coverage    Last Updated    Provider
  ───────────────────────────────────────────────────────────────────────────────
  en       English           source         100%        —               —
  fr       French            complete       100%        2 days ago      deepl
  es       Spanish           complete       100%        2 days ago      deepl
  de       German            stale          94%         8 days ago      claude
  ja       Japanese          in_progress    61%         today           claude
  ar       Arabic            pending        0%          —               —
  zh-CN    Chinese (Simp.)   excluded       —           —               (excluded: beta only)
  ko       Korean            needs_review   100%        5 days ago      gpt-4o
  pt-BR    Portuguese        error          43%         today           deepl  ← 3 strings failed

Status legend:
  source       → this is the base language, never translated
  complete     → all keys translated, up to date
  stale        → translated but source strings changed since
  in_progress  → translation currently running or paused
  pending      → configured but not started yet
  excluded     → explicitly excluded, won't be touched
  needs_review → translated but flagged strings awaiting human review
  error        → last run had failures, partially translated
This gives developers an instant health dashboard.

Adding Locales
Via CLI:

bash
# Add single locale
flutterlocal locales add pt-BR

# Add multiple at once
flutterlocal locales add zh-CN zh-TW ko pl nl

# Add with specific provider override
flutterlocal locales add ja --provider deepl

# Add but don't translate yet — just register intent
flutterlocal locales add ar --pending

# Add with a label/note (shows in list)
flutterlocal locales add zh-CN --note "beta markets only, Q3 launch"
What happens on add:

$ flutterlocal locales add pt-BR

Adding locale: pt-BR (Portuguese - Brazil)

  Checking existing ARB files...  none found
  Keys to translate: 212 (from app_en.arb)
  Estimated cost: ~$0.04 (claude-sonnet, 212 strings)
  Estimated time: ~3 minutes

  Translate now or queue for next run?
  [T] Translate now   [Q] Queue (translate on next flutterlocal translate)   [C] Cancel
Developer chooses — adding a locale doesn't force an immediate LLM call.

Excluding Locales
Exclusion is explicit and documented, not just absence from a list:

bash
# Exclude a locale with a reason
flutterlocal locales exclude zh-CN --reason "not in scope for v1"
flutterlocal locales exclude ar --reason "RTL support not implemented yet"

# Exclude temporarily
flutterlocal locales exclude de --until 2024-03-01 --reason "translator review pending"

# Exclude from specific operations only
flutterlocal locales exclude ko --operations translate
# ko still shows in audit, still validates, just won't be auto-translated

# Re-include
flutterlocal locales include zh-CN
Excluded locales are visible but clearly marked:

  zh-CN    Chinese (Simp.)   excluded  —  —  (not in scope for v1)
  ar       Arabic            excluded  —  —  (RTL support not implemented yet)
  de       German            excluded  —  —  (until 2024-03-01: translator review)
Exclusions are never silent. Anyone running locales list sees them and why.

Removing Locales
Removal is a heavier operation — it touches ARB files and potentially pubspec:

$ flutterlocal locales remove ko

Removing locale: ko (Korean)

  This will:
  → Delete app_ko.arb (247 translated strings)
  → Remove ko from l10n.yaml supported locales
  → Remove ko from pubspec.yaml flutter.generate config
  → NOT affect any Dart files (no code references locale directly)

  Checkpoint will be created before removal.

  Are you sure? [yes/N]: yes

  ✓ Checkpoint saved: pre-remove-ko-2024-01-15-150233
  ✓ app_ko.arb deleted
  ✓ l10n.yaml updated
  ✓ Removal complete

  Undo anytime: flutterlocal checkpoint restore pre-remove-ko-2024-01-15-150233
Always checkpointed before destructive operations.

Locale Groups
For larger apps with many locales, grouping makes management easier:

yaml
# flutterlocal.yaml
locale_groups:
  launch:     [en, fr, es, de]        # v1 launch markets
  expansion:  [ja, ko, zh-CN, zh-TW]  # Q3 expansion
  beta:       [ar, he, fa]            # RTL beta
  internal:   [en-DEBUG]              # shows keys instead of strings, dev only
bash
# Target operations by group
flutterlocal translate --group launch
flutterlocal translate --group expansion --pending-only
flutterlocal audit --group beta

# Add entire group at once
flutterlocal locales add --group expansion

# Exclude entire group
flutterlocal locales exclude --group beta --reason "RTL milestone not ready"
Per-Locale Provider Overrides
Different locales may need different translation providers based on quality:

yaml
# flutterlocal.yaml
default_provider: claude

locale_providers:
  ja: deepl          # DeepL has better Japanese quality
  zh-CN: deepl
  zh-TW: deepl
  ar: gpt-4o         # GPT-4o better for Arabic nuance
  en-DEBUG: none     # outputs keys directly, no translation needed
bash
# Override per-run
flutterlocal translate --locale ja --provider gpt-4o

# Compare provider output side by side
flutterlocal translate --locale fr --provider deepl --compare claude
# shows both translations side by side, lets developer pick per-string
Stale Detection
When source strings change, affected locales are automatically marked stale:

$ flutterlocal audit

Stale translations detected:

  de  German   →  8 strings stale
    "welcomeBackName"   EN changed: "Welcome back, {name}!" → "Hey {name}, welcome back!"
    "checkoutTotal"     EN changed: "Total: ${amount}" → "Order total: ${amount}"
    + 6 more

  ja  Japanese →  8 strings stale (same keys)
  ko  Korean   →  8 strings stale (same keys)

  Options:
  [R] Re-translate stale strings only    (recommended)
  [V] View all stale strings
  [S] Mark as reviewed (if changes were minor and translation still valid)
  [I] Ignore for now
Stale granularity:

bash
# Re-translate only stale strings across all locales
flutterlocal translate --stale-only

# Re-translate stale strings for specific locale
flutterlocal translate --locale de --stale-only

# Mark stale strings as still-valid (minor source edit, translation holds)
flutterlocal locales mark-valid de --keys welcomeBackName,checkoutTotal

# Force re-translate everything regardless of staleness
flutterlocal translate --locale fr --force
Adding a New Locale Mid-Project (Full Flow)
The realistic scenario — app already has fr, es, de, now adding Japanese six months later:

$ flutterlocal locales add ja

Adding ja (Japanese) to existing project

  Existing locales: en (source), fr, es, de
  Total ARB keys in source: 847
  Keys already in app_ja.arb: 0

  Analyzing what needs translation...
  ✓ 847 strings to translate
  ✓ 0 strings in cache (first time for ja)
  
  Estimated:
    Cost:   ~$0.18  (claude-sonnet)
    Time:   ~12 minutes
    Batches: 17

  Provider for ja? [claude] [deepl] [gpt-4o] (default: claude from config)
  > deepl

  Creating checkpoint: pre-add-ja-2024-01-15-151200
  Starting translation...

  Japanese  ████████░░░░░░░░░░░░  398/847  translating...
            Batch 8/17  │  Rate: 48 RPM  │  Est. 6 min remaining
            [P] Pause   [Q] Quit and save progress
Resumable, checkpointed, provider-aware. Picks up mid-batch if interrupted.

The en-DEBUG Pseudo-Locale
Useful enough to be built-in:

bash
flutterlocal locales add en-DEBUG
Instead of translating, generates strings that make localization issues immediately visible during development:

"Submit"              → "[[ Submit ]]"
"Welcome back, {name}!" → "[[ Welcome back, ~~name~~ !! ]]"
"OK"                  → "[[ OK ]]"
Developers run the app in en-DEBUG locale to instantly spot any hardcoded strings that weren't extracted — they show up as raw English while everything else shows brackets.

Config Representation
Everything is reflected in flutterlocal.yaml and editable there too — CLI is just a friendlier interface to the same config:

yaml
locales:
  source: en
  
  targets:
    fr:
      status: complete
      provider: deepl
    de:
      status: stale
      provider: claude
    ja:
      status: in_progress
      provider: deepl
    ar:
      excluded: true
      reason: "RTL support not implemented yet"
    zh-CN:
      excluded: true
      reason: "not in scope for v1"
      until: "2024-06-01"

  groups:
    launch: [fr, es, de]
    expansion: [ja, ko, zh-CN]

  provider_overrides:
    ja: deepl
    ar: gpt-4o
YAML is the source of truth. CLI commands modify it. Developers can edit it directly and the CLI respects it — same as how tools like eslint or prettier work.

should I go with go

Jun 25
For this specific tool, yes — Go is a strong fit. Let me break down why and where the tradeoffs are.

Why Go Works Well Here
CLI performance

The first thing a developer notices is startup time. Python adds 200-400ms just to import a typical CLI stack (Click, LangChain, rich). Go binaries start in under 10ms. For a tool that runs on every save in watch mode, that difference is felt constantly.

Distribution

This is the biggest practical win. Go compiles to a single static binary:

flutterlocal-darwin-arm64    ~12MB
flutterlocal-darwin-amd64   ~12MB
flutterlocal-linux-amd64    ~12MB
flutterlocal-windows-amd64  ~13MB
Developer downloads one file, puts it in PATH, done. No pip install, no virtualenv, no Python version conflicts, no node_modules. Flutter developers especially will appreciate this — they're used to installing CLI tools like fvm, melos, dart itself, all single binaries.

Concurrency

Your batching and rate limit system maps perfectly onto goroutines:

go
// Translate multiple locales concurrently, each respecting its own rate limiter
for _, locale := range targetLocales {
    go func(l Locale) {
        limiter := rateLimiters[l.Provider]
        for _, batch := range batches {
            limiter.Wait()
            translateBatch(batch, l)
            saveCheckpoint()
        }
    }(locale)
}
Python can do this with asyncio but Go's goroutine model is more natural for this pattern and easier to reason about when you add rate limiters, retry logic, and checkpoint saving on top.

File I/O at scale

Scanning 50k lines across hundreds of Dart files, computing hashes, writing checkpoint diffs — Go handles this without any ceremony.

Where Go Has Real Friction
Be honest with yourself on these:

Dart AST parsing

There's no mature Go library for parsing Dart. Your options:

Option A: Shell out to dart analyze
  → reliable, uses official parser
  → requires Dart SDK installed (it will be, it's a Flutter project)
  → adds subprocess overhead
  → output parsing is brittle

Option B: Tree-sitter Go bindings (go-tree-sitter)
  → tree-sitter has a Dart grammar
  → mature, fast, used in editors
  → recommended path

Option C: Regex + heuristics
  → works for 80% of cases
  → breaks on edge cases (multiline strings, nested quotes)
  → not recommended as primary strategy
Tree-sitter is the right call. go-tree-sitter with the Dart grammar gives you a real AST without needing the Dart toolchain as a dependency.

LangChain

LangChain doesn't have a mature Go port. But honestly for what you're building you don't need LangChain — you need:

go
// This is all LangChain is doing for you here:
type LLMProvider interface {
    Complete(ctx context.Context, prompt string) (string, error)
    Name() string
}

type ClaudeProvider struct { ... }
type OpenAIProvider struct { ... }
type GeminiProvider struct { ... }
A clean interface with three implementations. LangChain's agent/tool abstractions add complexity you don't need — your "tools" are deterministic Go functions, not LLM-called tools. The LLM is only doing classification and translation, both of which are simple prompt → response calls.

TUI

Go actually has better TUI libraries than Python for this use case:

Bubble Tea (Charm)   → component-based TUI, excellent ecosystem
  └── Bubbles        → pre-built components (progress bars, tables, spinners)
  └── Lip Gloss      → styling
Charm's ecosystem is genuinely best-in-class for CLI tools right now. The interactive review TUI you designed maps cleanly onto Bubble Tea's model.

The Stack
CLI framework:      Cobra          (standard for Go CLIs, used by kubectl, hugo)
TUI:                Bubble Tea     (Charm)
TUI components:     Bubbles        (Charm) — progress bars, tables, lists
TUI styling:        Lip Gloss      (Charm)
Dart parsing:       go-tree-sitter + tree-sitter-dart grammar
Config:             Viper          (YAML/JSON/env, pairs with Cobra)
HTTP client:        stdlib net/http (sufficient, no need for extra libs)
JSON:               stdlib encoding/json
File hashing:       stdlib crypto/sha256
Git integration:    go-git         (pure Go git implementation, no git CLI dependency)
Checkpoint storage: stdlib (JSON files, nothing fancy needed)
go-git is worth highlighting — it means your git diff tracking, commit hash reading, and file change detection works without shelling out to git. Works even if the user has a non-standard git setup.

Comparison Against Python
                    Go              Python
Startup time        <10ms           200-400ms
Distribution        single binary   pip + deps
Dart AST            tree-sitter     tree-sitter (same)
LangChain           roll your own   native
TUI                 Bubble Tea ✓    Textual (good but heavier)
Concurrency         goroutines ✓    asyncio (works but more complex)
Dev familiarity     moderate        high
Time to first run   longer          faster
If you were building this as a quick internal tool, Python. Since you're building something developer-facing that you want people to actually install and use — Go.

One Honest Caveat
If you haven't shipped a Go project before, budget extra time for the first two weeks. The things that feel fast in Python (iterating on prompts, reshaping data structures, trying things) feel slower in Go until you're fluent. But once the core pipeline is working, Go's compiler becomes your best collaborator — refactoring is safe, performance is free, and the binary just works everywhere.

Given you already know Python well and have CodiLay as a reference point, you could also consider: prototype the core pipeline in Python first to validate the approach, then rewrite the distribution-facing version in Go once the logic is proven. Not always worth it, but worth considering given the Dart AST parsing uncertainty.

I think instead of making it for only flutter, my goal was to aim for all these type of frameworks, libraries and etc like to make localization easy with AI coz if it was only for flutter then a plugin would have been better but goal was to be a solution of all like kotlin, react native, swift and etc

Jun 25
That's a much stronger product position. A Flutter-only tool has a ceiling — a universal localization agent is a real developer infrastructure play.

What This Changes Fundamentally
Flutter-only plugin          Universal CLI tool
─────────────────────────────────────────────────
pub.dev ecosystem            npm, brew, curl install
Dart AST only                Multiple AST parsers
ARB format only              Multiple file formats
Flutter devs only            Any mobile/web dev
Limited TAM                  Massive TAM
Competes with plugins        No direct competitor at this scope
The CLI form factor was already the right call. This just confirms it harder.

Target Platforms
Mobile
  ├── Flutter          Dart        → ARB files
  ├── React Native     JS/TS       → i18next JSON / react-intl
  ├── Swift (iOS)      Swift       → .strings / .stringsdict / xcstrings
  ├── Kotlin (Android) Kotlin/Java → strings.xml
  └── Xamarin          C#          → .resx files

Web
  ├── React            JS/TS       → i18next JSON / react-intl / FormatJS
  ├── Next.js          JS/TS       → next-intl / next-i18next
  ├── Vue              JS/TS       → vue-i18n JSON
  ├── Angular          TS          → XLIFF / JSON (ngx-translate)
  └── Svelte           JS/TS       → svelte-i18n JSON

Desktop
  ├── Electron         JS/TS       → i18next JSON
  ├── Tauri            Rust/JS     → i18next JSON
  └── .NET MAUI        C#          → .resx files

Backend (API strings, emails, notifications)
  ├── Node.js          JS/TS       → i18next JSON
  ├── Python           Python      → .po / .pot (gettext)
  └── Go               Go          → go-i18n TOML/JSON
The Core Abstraction
This is the architectural insight that makes universal support possible. You define clean interfaces that every platform implements:

go
// Everything the tool needs to know about a platform
type Platform interface {
    // Detection
    Detect(projectRoot string) bool        // is this project our platform?
    Confidence(projectRoot string) float64  // how sure are we (0-1)

    // Parsing
    Parser() ASTParser                     // returns platform-specific parser
    StringNodeTypes() []NodeType           // what AST nodes contain UI strings
    SkipPatterns() []SkipRule              // platform-specific auto-skip rules

    // Format
    LocaleFormat() LocaleFormat            // ARB / JSON / strings.xml / etc
    LocaleFilePaths(root string) []string  // where locale files live
    KeyStyle() KeyStyle                    // camelCase / snake_case / dot.notation

    // Refactor
    Refactor(node StringNode) RefactorPlan // how to replace string with l10n call
    ImportStatement() string               // what import to inject
    ContextStrategy() ContextStrategy      // how to get translation at runtime
}
Every platform is a plugin implementing this interface. Core pipeline never changes — only the platform layer does.

go
// Core pipeline is platform-agnostic
func Run(project Project, platform Platform, config Config) error {
    strings  := platform.Parser().Extract(project.Files)
    filtered := classify(strings, platform.SkipPatterns())
    reviewed := reviewTUI(filtered)
    refactor(reviewed, platform.Refactor)
    writeLocaleFiles(reviewed, platform.LocaleFormat())
    translate(reviewed, config.Locales, config.Provider)
}
Platform Auto-Detection
On flutterlocal init (rename incoming — more on that below), the tool detects what it's looking at:

$ langPeanut init         (new name idea, more on this below)

Scanning project...

  ✓ pubspec.yaml found          → Flutter (confidence: 100%)
  ✓ package.json found          → Node.js project
    ✓ react-native in deps      → React Native (confidence: 95%)
  ✓ build.gradle found          → Android / Kotlin (confidence: 88%)
  ✓ Package.swift found         → Swift Package (confidence: 91%)

  Detected: React Native (JS/TS)

  Confirm or select manually:
  [1] React Native    ← detected
  [2] Flutter
  [3] Swift (iOS)
  [4] Kotlin (Android)
  [5] React (Web)
  [6] Next.js
  [7] Vue
  [8] Angular
  [9] Other / Manual config
Multi-platform monorepos (e.g. React Native + shared backend) get detected as multiple targets:

  Detected multiple targets:
  [1] React Native    → apps/mobile/
  [2] Node.js         → packages/api/
  
  Configure both? [Y/n]
Locale File Format Matrix
Each platform has an expected format. The tool handles all of them natively:

Platform          Format           Example
──────────────────────────────────────────────────────────────────────
Flutter           ARB              {"submit": "Submit", "@submit": {}}
React / RN        i18next JSON     {"submit": "Submit"}
Next.js           next-intl JSON   {"submit": "Submit"}
Vue               vue-i18n JSON    {"submit": "Submit"}
Angular           XLIFF            <trans-unit id="submit">...</trans-unit>
iOS Swift         .xcstrings       JSON-based (Xcode 15+)
iOS Swift (old)   .strings         "submit" = "Submit";
Android Kotlin    strings.xml      <string name="submit">Submit</string>
.NET / MAUI       .resx            XML resource format
Python            .po / .pot       gettext format
Go                TOML/JSON        go-i18n format
The key insight: translation output is the same regardless of format — only serialization differs. You translate once, serialize to whatever format the platform expects.

AST Parser Matrix
Each language needs its own parser, but tree-sitter covers most of them:

Language     Parser                    String node targets
────────────────────────────────────────────────────────────────────────────
Dart         tree-sitter-dart          Text(), InputDecoration, AppBar title
JS/TS        tree-sitter-javascript    JSX text nodes, string literals in props
             tree-sitter-typescript
Swift        tree-sitter-swift         Text(), Label(), Button() in SwiftUI
                                       NSLocalizedString() calls (already l10n'd)
Kotlin       tree-sitter-kotlin        Text(), string resources in composables
             + XML parser              strings.xml for non-Compose
Java         tree-sitter-java          setText(), string literals in views
C#           tree-sitter-c-sharp       Text, Label, Button.Content in MAUI
Python       tree-sitter-python        _(), gettext() patterns
All tree-sitter. Same Go bindings, different grammar files. This is why Go was the right call — tree-sitter's Go bindings are solid and you get all grammars from one interface.

Smart String Classification Per Platform
The LLM escalation criteria changes per platform because each has different conventions:

Flutter:
  Auto-localizable:   Text(), TextSpan(), InputDecoration labels
  Auto-skip:          debugPrint, route strings, enum names

React Native:
  Auto-localizable:   <Text> JSX content, placeholder= props, title= props
  Auto-skip:          console.log, testID=, style keys, import paths
  Already-localized:  t('key'), i18n.t('key') → skip, already done

Swift (SwiftUI):
  Auto-localizable:   Text("...") — SwiftUI Text() auto-localizes if string
  Auto-skip:          systemImage names, SF Symbol names, modifier values
  Special case:       Text("submit") might already be a LocalizedStringKey

Kotlin (Compose):
  Auto-localizable:   Text(), Button content, stringResource already done
  Auto-skip:          Log.d(), tag strings, route constants
The platform layer encodes all of this. Core classification logic stays the same.

The Refactor Output Per Platform
Same extracted string, different refactor output:

Input string: "Welcome back, {name}!"

dart
// Flutter
Text(AppLocalizations.of(context).welcomeBackName(name))

// React Native  
import { useTranslation } from 'react-i18next';
const { t } = useTranslation();
<Text>{t('welcomeBackName', { name })}</Text>

// SwiftUI
Text("welcomeBackName \(name)", comment: "Welcome greeting")
// or with Swift 5.9+:
Text(.welcomeBackName(name))

// Jetpack Compose (Kotlin)
Text(stringResource(R.string.welcome_back_name, name))

// Vue
{{ $t('welcomeBackName', { name }) }}

// Angular
{{ 'welcomeBackName' | translate:{ name: name } }}
The refactor engine takes a RefactorPlan from the platform interface and applies it. The developer sees the correct output for their stack automatically.

Tool Rename
flutterlocal no longer makes sense. Some directions:

langPeanut       → localization + AI, short, memorable
loci        → shorthand for localization + AI
polylocal   → multi-platform localization
langshift   → shift your strings to any language
i18nai      → descriptive but ugly
lokalai     → lokal (localization) + ai
stringpull  → what it does mechanically
langPeanut feels the strongest — short, obvious what it does, easy to type, works as a command name.

bash
langPeanut init
langPeanut audit
langPeanut extract
langPeanut translate --locales fr,es,ja
langPeanut locales add ko
langPeanut rollback
What the Expanded Scope Gives You
Productization angle:
  → SaaS dashboard version (teams, shared translation memory, cost tracking)
  → Per-seat or per-string pricing model
  → Translation memory across projects (if "Submit" is translated in project A,
     reuse it in project B for free — cache at account level not project level)

Competitive position:
  → Lokalise, Crowdin, Phrase all require manual export/import workflow
  → None of them do automatic codebase refactoring
  → None of them do AST-aware string extraction
  → You own the "zero manual setup" angle

Open source core, paid cloud:
  → CLI is open source (drives adoption)
  → langPeanut.dev cloud adds: team translation memory, PR comments,
     shared locale management, usage dashboard
  → Same playbook as Prettier, ESLint, Turborepo
The universal scope is what makes this worth building as a standalone product rather than a plugin. A Flutter plugin gets 5 stars on pub.dev. A universal localization agent gets a landing page, a GitHub org, and eventually a pricing page.

maybe with cli, it would be also great to have UI desktop app for easier controls and etc

5:19 PM
Great instinct. The CLI and desktop app aren't competing — they serve different moments for the same developer.

The Two Surfaces, Different Jobs
CLI                              Desktop App
─────────────────────────────────────────────────────────
CI/CD pipelines                  First-time setup
Automation scripts               Visual project overview
Power users                      Team leads, less CLI-heavy devs
Watch mode daemon                Interactive string review
Quick locale add                 Translation management dashboard
Git hooks                        Side-by-side diff review
Neither replaces the other. A developer might use the desktop app to onboard a project, then use the CLI for day-to-day watch mode and CI integration. Same underlying engine, two control surfaces.

Desktop App Tech Stack
Since the core is Go, you have a natural fit:

Wails is the right call here.

Wails = Go backend + Web frontend (React/Vue/Svelte)
  → Go core logic runs natively, no rewrite
  → Frontend is just a UI layer calling Go functions
  → Single binary output, same as CLI
  → Cross-platform: macOS, Windows, Linux
  → Much lighter than Electron (no bundled Chromium)
Alternative considered: Electron
  → Familiar for JS devs
  → Heavy (~200MB)
  → Would need to rewrite Go logic in Node.js or shell out
  → Not worth it given Go core

Alternative considered: Tauri
  → Similar to Wails but Rust backend
  → Go backend doesn't fit cleanly
  → Skip

Alternative considered: Flutter desktop
  → Ironic given the tool's purpose
  → Would work but adds Dart dependency to a Go project
  → Skip
Wails lets you call Go functions directly from the frontend with no HTTP server needed:

go
// Go backend
func (a *App) GetLocaleStatus() []LocaleStatus {
    return a.engine.GetLocaleStatus()
}

func (a *App) AddLocale(code string, provider string) error {
    return a.engine.AddLocale(code, provider)
}

func (a *App) StartTranslation(locales []string) (<-chan Progress, error) {
    return a.engine.Translate(locales)
}
javascript
// React frontend — direct Go function calls
import { GetLocaleStatus, AddLocale, StartTranslation } from '../wailsjs/go/main/App'

const locales = await GetLocaleStatus()
await AddLocale('ja', 'deepl')
Zero HTTP, zero serialization overhead, same business logic as CLI.

What the Desktop App Covers
1. Project Dashboard

The first screen when you open the app:

┌─ langPeanut ──────────────────────────────────────────────────────────────────┐
│                                                                           │
│  Projects                          + New Project                          │
│  ──────────────────────────────────────────────────────────────────────  │
│                                                                           │
│  ● MyShoppingApp          React Native   8 locales   94% coverage        │
│    /Users/harman/projects/shopping                   3 stale              │
│                                                                           │
│  ● AdminDashboard         Next.js        3 locales   100% coverage       │
│    /Users/harman/projects/admin                      up to date           │
│                                                                           │
│  ● GameApp                Flutter        12 locales  67% coverage        │
│    /Users/harman/projects/game                       in progress          │
│                                                                           │
└───────────────────────────────────────────────────────────────────────────┘
Multiple projects managed from one place — something the CLI doesn't give you easily.

2. Locale Health View

┌─ MyShoppingApp — Locales ─────────────────────────────────────────────────┐
│                                                                            │
│  Source: English (en)   247 keys                    + Add Locale          │
│                                                                            │
│  Locale        Coverage    Status       Stale    Provider    Last run      │
│  ──────────────────────────────────────────────────────────────────────── │
│  🇫🇷 French      100%       ✓ Complete    0        DeepL      2 days ago   │
│  🇪🇸 Spanish     100%       ✓ Complete    0        DeepL      2 days ago   │
│  🇩🇪 German       94%       ⚠ Stale       8        Claude     8 days ago   │
│  🇯🇵 Japanese     61%       ⟳ In Progress  —        DeepL      today       │
│  🇸🇦 Arabic        0%       ○ Pending      —        —          —            │
│  🇨🇳 Chinese       —        ✗ Excluded     —        —          not in v1   │
│                                                                            │
│  [Translate Stale]   [Resume Japanese]   [Start Arabic]                   │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
3. Interactive String Review

This is where the desktop app genuinely beats the CLI TUI. Instead of keyboard-only navigation you get a proper review interface:

┌─ String Review — 34 of 112 ───────────────────────────────────────────────┐
│                                                                            │
│  Filter: [All ▾]  [Localizable ▾]  [Uncertain ▾]     Search: ________    │
│                                                                            │
│  ┌──────────────────────────────────────────────────────────────────────┐ │
│  │ ✓  Text("Submit")                    home_screen.dart:47   100%     │ │
│  │ ✓  Text("Cancel")                    home_screen.dart:52   100%     │ │
│  │ ?  Text("v2/confirm")                checkout.dart:89       41%     │ │  ← selected
│  │ ✓  InputDecoration(hint: "Email")    login_screen.dart:23   97%     │ │
│  │ ✗  debugPrint("user loaded")         profile.dart:14         0%     │ │
│  └──────────────────────────────────────────────────────────────────────┘ │
│                                                                            │
│  Selected: Text("v2/confirm")   checkout.dart:89                          │
│  Confidence: 41%  —  uncertain (looks like a route but inside Text())     │
│                                                                            │
│  Proposed key:   v2Confirm          [Edit key name____________]           │
│  ARB value:      "v2/confirm"                                             │
│                                                                            │
│  Code preview:                                                             │
│  - Text("v2/confirm")                                                     │
│  + Text(AppLocalizations.of(context).v2Confirm)                          │
│                                                                            │
│  [✓ Localizable]   [✗ Skip]   [→ Open in Editor]   [⊞ View Context]     │
│                                                                            │
│           [Approve All High Confidence]      [Run Refactor →]             │
└────────────────────────────────────────────────────────────────────────────┘
"Open in Editor" deep-links to VS Code or Cursor at the exact file and line. That alone is worth having the desktop app.

4. Translation Progress View

Live progress with real controls, not a terminal progress bar:

┌─ Translating — MyShoppingApp ─────────────────────────────────────────────┐
│                                                                            │
│  247 strings  ×  6 locales  =  1,482 total translations                  │
│                                                                            │
│  🇫🇷 French     ████████████████████  247/247  ✓ Complete                 │
│  🇪🇸 Spanish    ████████████████████  247/247  ✓ Complete                 │
│  🇩🇪 German     █████████████░░░░░░░  163/247  Translating...             │
│  🇯🇵 Japanese   ░░░░░░░░░░░░░░░░░░░░    0/247  Queued                     │
│  🇰🇷 Korean     ░░░░░░░░░░░░░░░░░░░░    0/247  Queued                     │
│  🇵🇹 Portuguese ░░░░░░░░░░░░░░░░░░░░    0/247  Queued                     │
│                                                                            │
│  Provider: DeepL   Rate: 42 RPM / 60 RPM   Tokens: 34k / 90k TPM        │
│  Cache hits: 198 (saved ~$0.08)   Est. remaining: 6 min                  │
│                                                                            │
│  Checkpoints:  ● pre-run  ● post-extract  ● post-fr  ● post-es  ○ ...   │
│                                                                            │
│         [⏸ Pause]   [⏹ Stop & Save]   [↩ Rollback]                       │
└────────────────────────────────────────────────────────────────────────────┘
Checkpoint dots are clickable — click any past checkpoint to restore to it.

5. Key Manager

┌─ Key Manager ─────────────────────────────────────────────────────────────┐
│                                                                            │
│  Search: [submit_____________]          Filter: [All ▾]  [Orphans ▾]     │
│                                                                            │
│  Key                  EN                     Locales    Used in           │
│  ──────────────────────────────────────────────────────────────────────── │
│  submit               "Submit"               6/6 ✓      4 files          │
│  submitOrder          "Submit Order"         6/6 ✓      1 file           │
│  submitReview         "Submit Review"        4/6 ⚠       2 files          │
│  cancelOrder          "Cancel Order"         6/6 ✓      2 files          │
│  ⚠ oldSubmitBtn       "Submit"               6/6        0 files  ORPHAN  │
│                                                                            │
│  Selected: oldSubmitBtn                                                   │
│  [✎ Rename]  [⊞ Find Usages]  [🗑 Delete Orphan]  [⊕ Merge with submit] │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
6. Settings & Provider Config

┌─ Settings ────────────────────────────────────────────────────────────────┐
│                                                                            │
│  LLM Providers                                                             │
│  ──────────────────────────────────────────────────────────────────────── │
│  Claude     API Key: sk-ant-••••••••••••••••••  [Test] ✓ Connected       │
│  OpenAI     API Key: sk-••••••••••••••••••••••  [Test] ✓ Connected       │
│  DeepL      API Key: ••••••••••••••••••••••••   [Test] ✗ Invalid key     │
│  Gemini     API Key: not configured             [+ Add Key]               │
│                                                                            │
│  Rate Limits                                                               │
│  ──────────────────────────────────────────────────────────────────────── │
│  Claude     Tier: [Pro ▾]    RPM: 1000   TPM: 100k    [Custom]           │
│  OpenAI     Tier: [Tier 2 ▾] RPM: 3500   TPM: 90k     [Custom]          │
│                                                                            │
│  Classification                                                            │
│  ──────────────────────────────────────────────────────────────────────── │
│  Confidence threshold for auto-approve:  [85%  ────────●────── ]         │
│  Batch size:                             [Auto ▾]                         │
│  Min delay between calls:                [1.2s ────●──────────── ]       │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
How CLI and Desktop Stay in Sync
They share the same state files — running one never breaks the other:

.flutterlocal/
  config.yaml          ← both read/write
  session/current.json ← both read/write
  baseline.json        ← both read/write
  checkpoints/         ← both read/write
  cache/               ← both read/write
Scenario: developer starts translation in desktop app,
          closes laptop, SSHes into server, wants to check progress

$ langPeanut session inspect
→ reads same current.json desktop app wrote
→ shows identical progress state
→ can resume from CLI seamlessly
Scenario: CI pipeline runs langPeanut audit --ci
          developer opens desktop app next morning
→ desktop app reads baseline.json CI updated
→ shows audit results from overnight run
→ developer clicks into flagged strings directly
One source of truth, two control surfaces.

Shared Architecture
┌─────────────────────────────────────────────────────┐
│                   langPeanut core (Go)                   │
│  Parser  Classifier  Refactor  Translator  Checkpoint│
└──────────────────┬──────────────────────────────────┘
                   │
        ┌──────────┴──────────┐
        │                     │
┌───────▼──────┐    ┌─────────▼───────┐
│  CLI binary  │    │  Wails desktop  │
│  (Cobra)     │    │  (Go + React)   │
│              │    │                 │
│ langPeanut audit  │    │  Dashboard UI   │
│ langPeanut extract│    │  Review TUI     │
│ langPeanut translate    │  Key Manager   │
└──────────────┘    └─────────────────┘
Core logic written once. CLI and desktop are just two different shells around the same engine. Any bug fixed in core is fixed in both. Any new platform support added works in both immediately.

Distribution
CLI:
  brew install langPeanut          (Homebrew tap)
  curl -fsSL langPeanut.dev/install | sh
  scoop install langPeanut         (Windows)

Desktop:
  langPeanut.dev/download
  → langPeanut-macos.dmg
  → langPeanut-windows.exe
  → langPeanut-linux.AppImage

Both ship from same Go repo, same release pipeline.
GitHub Actions builds all four binaries + installers on tag push.
Phased Build Order
Don't build both at once:

Phase 1:  CLI only
          → core engine, all platforms, full pipeline
          → ship this, get real users, validate the approach

Phase 2:  Desktop shell
          → Wails skeleton wrapping existing Go core
          → Dashboard + locale health view first (highest value)
          → Review TUI second
          → Key manager third

Phase 3:  Cloud (optional SaaS layer)
          → Team translation memory
          → PR comments via GitHub App
          → Shared locale dashboard for teams
The desktop app is maybe 20% additional work once the Go core is solid — because you're not rewriting logic, just building a UI over it. That's the payoff of the Wails architecture.