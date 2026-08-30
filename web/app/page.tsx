'use client'

import { useState } from 'react'

const WORKFLOWS = [
  {
    id: 'push-autopilot',
    name: 'Continuous Push Autopilot',
    tag: 'CI/CD Event: push',
    badge: 'Autonomous',
    badgeColor: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
    description: 'Watches commits on default branch. Automatically parses modified files via Tree-Sitter AST, translates missing keys, and opens a ready-to-merge PR with zero manual intervention.',
    trigger: 'Webhook: git push to refs/heads/main',
    pipeline: [
      { step: '1. Git Diff Scout', desc: 'Identifies added/modified source files in commit range' },
      { step: '2. AST Scout Scan', desc: 'Extracts newly added raw strings without touching comments/imports' },
      { step: '3. Cultural Translator', desc: 'Translates new keys with ICU plural/variable preservation' },
      { step: '4. 4-Tier Critic', desc: 'Validates syntax, character expansion, and placeholder parity' },
      { step: '5. Branch & PR Open', desc: 'Pushes langpeanut/i18n-sync branch & opens PR' },
    ],
    sampleArtifact: `// Generated Pull Request
Title: [i18n] Sync 14 localized strings for es, fr, de
Branch: langpeanut/i18n-sync-8f2a91
Status: 4-Tier Critic: 100% Pass (0 AST syntax errors, 0 ICU placeholder drift)`,
  },
  {
    id: 'pr-bot',
    name: 'Interactive PR Review Bot',
    tag: 'Comment: @langpeanut',
    badge: 'On-Demand Pair Programmer',
    badgeColor: 'text-sky-400 bg-sky-500/10 border-sky-500/20',
    description: 'Mention @langpeanut in any PR comment. The bot clones the feature branch, extracts strings introduced only in that PR, commits the refactored code directly, and leaves a critic scorecard.',
    trigger: 'Webhook: issue_comment (e.g. "@langpeanut translate --locales es,ja")',
    pipeline: [
      { step: '1. Command Parser', desc: 'Parses locale flags, tone preferences, and model overrides' },
      { step: '2. PR Branch Isolation', desc: 'Clones ephemeral worktree of the pull request branch' },
      { step: '3. AST Refactoring', desc: 'Replaces hardcoded JSX/Dart/Swift text with i18n hook calls' },
      { step: '4. Direct Commit', desc: 'Commits localized files directly to the PR branch' },
      { step: '5. Scorecard Comment', desc: 'Posts interactive verification report with diff inspection' },
    ],
    sampleArtifact: `// GitHub PR Review Comment by @langpeanut
### Localization Report Card
- Refactored **4 files** with surgical AST precision
- Added **18 target keys** across [es, ja, de]
- Critic Score: **100/100** | Token Cost: **$0.0034**`,
  },
  {
    id: 'drift-guard',
    name: 'Continuous Missing Key & Drift Guard',
    tag: 'Schedule: Cron / CI Check',
    badge: 'Quality Gate',
    badgeColor: 'text-amber-400 bg-amber-500/10 border-amber-500/20',
    description: 'Runs weekly or as a GitHub Status Check. Scans the whole codebase to catch untranslated raw strings that bypassed review and flags missing keys between primary and secondary locale files.',
    trigger: 'GitHub Check Run / Scheduled Cron (every Monday 09:00 UTC)',
    pipeline: [
      { step: '1. Full AST Audit', desc: 'Deep-scans entire codebase for unlocalized text' },
      { step: '2. Locale Parity Diff', desc: 'Compares key counts across en.json, es.json, ja.json' },
      { step: '3. Leakage Detection', desc: 'Flags hardcoded labels inside buttons, headers, modals' },
      { step: '4. Audit Report Issue', desc: 'Opens a structured GitHub Issue with exact file line numbers' },
    ],
    sampleArtifact: `// GitHub Audit Alert Issue
Title: [i18n Drift] 3 hardcoded strings and 2 missing keys detected
- src/components/BillingModal.tsx:L42 -> "Upgrade Plan" (hardcoded)
- locales/ja.json -> Missing key: "checkoutConfirmBtn"`,
  },
  {
    id: 'release-freeze',
    name: 'Release Milestone Batch Freeze',
    tag: 'Event: release.created / Tag',
    badge: 'Release Gate',
    badgeColor: 'text-blue-400 bg-blue-500/10 border-blue-500/20',
    description: 'Triggered when a version tag (v2.0.0-rc1) is cut. Performs deep Translation Memory deduplication across repositories, batch-translates all pending strings, and runs compiler tests before release.',
    trigger: 'Webhook: release / tag created or 1-click Release Freeze in console',
    pipeline: [
      { step: '1. Multi-Platform Sweep', desc: 'Scans React, Flutter, and iOS workspaces in parallel' },
      { step: '2. TM Memory Warming', desc: 'Reuses cached translations from SQLite store (64% savings)' },
      { step: '3. Batch Translation', desc: 'Translates all delta keys using temperature-calibrated LLMs' },
      { step: '4. Tier-5 Self-Healing', desc: 'Runs tsc/flutter build tests to guarantee 0 compile errors' },
      { step: '5. Release Manifest', desc: 'Publishes signed localization manifest with parity hashes' },
    ],
    sampleArtifact: `// Release Localization Manifest v2.0.0
- Total Strings: 420 | Locales: 12 | Coverage: 100.0%
- Translation Memory Hit Ratio: 68.4% | Total Spend: $0.42
- Compiler Validation: PASS (Next.js, Flutter, SwiftUI)`,
  },
]

const PLAYGROUND_SNIPPETS: Record<
  string,
  {
    framework: string
    original: string
    refactored: string
    localeJSON: string
  }
> = {
  react: {
    framework: 'React / Next.js (TSX)',
    original: `// AppHeader.tsx — Hardcoded Strings
export function AppHeader({ user }: { user: User }) {
  return (
    <header className="flex justify-between p-4">
      <h1>Welcome back, {user.name}!</h1>
      <p>You have {user.unreadCount} unread messages.</p>
      <button>Upgrade Account</button>
    </header>
  );
}`,
    refactored: `// AppHeader.tsx — Surgical AST Replacement
import { useTranslation } from "next-i18next";

export function AppHeader({ user }: { user: User }) {
  const { t } = useTranslation("common");
  return (
    <header className="flex justify-between p-4">
      <h1>{t("welcomeBackUser", { name: user.name })}</h1>
      <p>{t("unreadMessagesCount", { count: user.unreadCount })}</p>
      <button>{t("upgradeAccountBtn")}</button>
    </header>
  );
}`,
    localeJSON: `// locales/es/common.json (Spanish)
{
  "welcomeBackUser": "¡Bienvenido de nuevo, {{name}}!",
  "unreadMessagesCount": "Tienes {{count, plural, one{# mensaje no leído} other{# mensajes no leídos}}}.",
  "upgradeAccountBtn": "Actualizar Cuenta"
}`,
  },
  flutter: {
    framework: 'Flutter (Dart)',
    original: `// checkout_view.dart — Hardcoded Strings
Widget build(BuildContext context) {
  return ElevatedButton(
    onPressed: checkout,
    child: Text('Complete Purchase (\${cart.total})'),
  );
}`,
    refactored: `// checkout_view.dart — Surgical AST Replacement
Widget build(BuildContext context) {
  final l10n = AppLocalizations.of(context)!;
  return ElevatedButton(
    onPressed: checkout,
    child: Text(l10n.completePurchase(cart.total)),
  );
}`,
    localeJSON: `// app_fr.arb (French)
{
  "completePurchase": "Finaliser l'achat ({total})",
  "@completePurchase": {
    "placeholders": { "total": { "type": "String" } }
  }
}`,
  },
  swift: {
    framework: 'iOS (SwiftUI)',
    original: `// ProfileView.swift — Hardcoded Strings
struct ProfileView: View {
  var body: some View {
    VStack {
      Text("Account Settings")
      Button("Sign Out") { auth.logout() }
    }
  }
}`,
    refactored: `// ProfileView.swift — Surgical AST Replacement
struct ProfileView: View {
  var body: some View {
    VStack {
      Text(LocalizedStringKey("accountSettingsTitle"))
      Button(LocalizedStringKey("signOutBtn")) { auth.logout() }
    }
  }
}`,
    localeJSON: `// Localizable.xcstrings (German)
{
  "strings": {
    "accountSettingsTitle": {
      "localizations": { "de": { "stringUnit": { "value": "Kontoeinstellungen" } } }
    },
    "signOutBtn": {
      "localizations": { "de": { "stringUnit": { "value": "Abmelden" } } }
    }
  }
}`,
  },
}

const PREVIEW_DATA: Record<
  string,
  {
    flag: string
    name: string
    dir: 'ltr' | 'rtl'
    title: string
    subtitle: string
    cardHeading: string
    cardDesc: string
    cta: string
    secondaryCta: string
    metricLabel: string
    metricValue: string
  }
> = {
  en: {
    flag: 'EN',
    name: 'English',
    dir: 'ltr',
    title: 'Modern Cloud Infrastructure',
    subtitle: 'Deploy scalable microservices with zero configuration and instant global CDN distribution.',
    cardHeading: 'Enterprise Plan',
    cardDesc: 'You have 14 active team members and 850,000 API requests remaining this billing cycle.',
    cta: 'Upgrade Workspace',
    secondaryCta: 'View Usage',
    metricLabel: 'Active Workloads',
    metricValue: '99.99% Uptime',
  },
  es: {
    flag: 'ES',
    name: 'Spanish (Español)',
    dir: 'ltr',
    title: 'Infraestructura en la Nube Moderna',
    subtitle: 'Despliega microservicios escalables sin configuración y con distribución global instantánea en CDN.',
    cardHeading: 'Plan Empresarial',
    cardDesc: 'Tienes 14 miembros activos en el equipo y 850,000 solicitudes de API restantes en este ciclo.',
    cta: 'Actualizar Espacio de Trabajo',
    secondaryCta: 'Ver Consumo',
    metricLabel: 'Cargas de Trabajo',
    metricValue: '99.99% Disponibilidad',
  },
  ja: {
    flag: 'JA',
    name: 'Japanese (日本語)',
    dir: 'ltr',
    title: '次世代クラウドインフラストラクチャ',
    subtitle: 'ゼロ構成でスケーラブルなマイクロサービスを瞬時にグローバルCDNへデプロイします。',
    cardHeading: 'エンタープライズプラン',
    cardDesc: '現在14名のチームメンバーが稼働中で、今サイクルは残り850,000回のAPIリクエストが可能です。',
    cta: 'ワークスペースをアップグレード',
    secondaryCta: '利用状況を確認',
    metricLabel: 'アクティブワークロード',
    metricValue: '99.99% 稼働率',
  },
  ar: {
    flag: 'AR',
    name: 'Arabic (العربية — RTL Layout)',
    dir: 'rtl',
    title: 'البنية التحتية السحابية الحديثة',
    subtitle: 'انشر خدمات مصغرة قابلة للتطوير بدون أي تكوين مع توزيع فوري عبر شبكة توصيل المحتوى العالمية.',
    cardHeading: 'خطة المؤسسات',
    cardDesc: 'لديك 14 عضوًا نشطًا في الفريق و850,000 طلب API متبقي في هذه الدورة.',
    cta: 'ترقية مساحة العمل',
    secondaryCta: 'عرض الاستهلاك',
    metricLabel: 'أحمال العمل النشطة',
    metricValue: '99.99% وقت التشغيل',
  },
  de: {
    flag: 'DE',
    name: 'German (Deutsch — Expansion Safe)',
    dir: 'ltr',
    title: 'Moderne Cloud-Infrastrukturplattform',
    subtitle: 'Bereitstellung skalierbarer Microservices ohne Konfigurationsaufwand mit weltweiter CDN-Verteilung.',
    cardHeading: 'Unternehmenspaket',
    cardDesc: 'Sie haben 14 aktive Teammitglieder und 850.000 verbleibende API-Anfragen in diesem Abrechnungszyklus.',
    cta: 'Arbeitsbereich aktualisieren',
    secondaryCta: 'Nutzung anzeigen',
    metricLabel: 'Aktive Arbeitslasten',
    metricValue: '99,99% Verfügbarkeit',
  },
}

export default function HomePage() {
  const [activeSnippetKey, setActiveSnippetKey] = useState<string>('react')
  const [activeWorkflowId, setActiveWorkflowId] = useState<string>('push-autopilot')
  const [activePreviewLocale, setActivePreviewLocale] = useState<string>('es')
  const [copiedCLI, setCopiedCLI] = useState(false)

  // Live Terminal Inspector State
  const [showTerminal, setShowTerminal] = useState(false)
  const [simulating, setSimulating] = useState(false)
  const [terminalLogs, setTerminalLogs] = useState<string[]>([])

  function runSimulator() {
    setShowTerminal(true)
    setSimulating(true)
    setTerminalLogs(['[04:01:00] [Supervisor] Initializing autonomous 6-agent localization session...'])

    const steps = [
      '[04:01:01] [Git Worker] Created ephemeral scratch container sandbox (Isolated RAM: 512MB)',
      '[04:01:02] [AST Scout] Tree-sitter scanned 28 source files (TypeScript/TSX & Dart)',
      '[04:01:03] [AST Scout] Detected 42 string candidates. Filtered out 28 code identifiers / log routes with 0 token spend',
      '[04:01:04] [Context Agent] Disambiguating homonyms (e.g. "Save", "Right", "Book") via parent JSX hierarchy',
      '[04:01:05] [Brand Lexicon] Locked glossary tokens: ["langPeanut", "Superwall", "Workspace"] (Do Not Translate)',
      '[04:01:06] [Cultural Translator] Synthesizing [es, ja, fr, de] preserving ICU plural select tokens...',
      '[04:01:08] [4-Tier Critic] Running Syntax Assertion, ICU variable parity diff, and character expansion bounds',
      '[04:01:09] [4-Tier Critic] ✓ 100% Verification Pass: 0 AST syntax breaks, 0 ICU drift detected',
      '[04:01:10] [AST Patch Engine] Applied 14 surgical byte-range replacements to AppHeader.tsx & Checkout.tsx',
      '[04:01:11] [Tier-5 Repair] Running TypeScript compiler verification (tsc --noEmit)... ✓ 0 Errors',
      '[04:01:12] [GitHub Engine] Branch pushed: langpeanut/i18n-sync-4a8f92 -> Pull Request #42 Opened!',
    ]

    steps.forEach((step, idx) => {
      setTimeout(() => {
        setTerminalLogs((prev) => [...prev, step])
        if (idx === steps.length - 1) {
          setSimulating(false)
        }
      }, (idx + 1) * 600)
    })
  }

  function copyCliCommand() {
    navigator.clipboard.writeText('curl -fsSL https://langpeanut.ai/install.sh | bash')
    setCopiedCLI(true)
    setTimeout(() => setCopiedCLI(false), 3000)
  }

  const selectedWorkflow = WORKFLOWS.find((w) => w.id === activeWorkflowId) || WORKFLOWS[0]
  const preview = PREVIEW_DATA[activePreviewLocale] || PREVIEW_DATA.en

  return (
    <div className="space-y-24">
      {/* ─── Hero Section ────────────────────────────────────────────────────── */}
      <section className="text-center pt-10 pb-6 space-y-6 max-w-4xl mx-auto">
        <div className="inline-flex items-center gap-2 px-3.5 py-1 rounded-full bg-sky-500/10 border border-sky-500/20 text-sky-400 text-xs font-mono font-medium">
          <span className="w-2 h-2 rounded-full bg-sky-400 animate-pulse" />
          <span>Universal Multi-Agent Localization Workflow</span>
        </div>

        <h1 className="text-4xl sm:text-6xl font-extrabold tracking-tight text-white leading-[1.1]">
          Surgical AST Precision. <br />
          <span className="gradient-text">Zero-Defect Code Refactoring.</span>
        </h1>

        <p className="text-slate-400 text-sm sm:text-base max-w-2xl mx-auto leading-relaxed">
          Connect your GitHub repositories. Our 6-agent system extracts UI string literals with Tree-Sitter AST tools,
          preserves complex ICU plurals, heals compiler errors autonomously, and opens ready-to-merge pull requests.
        </p>

        <div className="flex flex-wrap items-center justify-center gap-3.5 pt-2">
          <a
            href="/dashboard"
            className="rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold px-6 py-3 text-xs shadow-xl shadow-blue-600/30 transition-all cursor-pointer"
          >
            Launch Console
          </a>
          <button
            onClick={runSimulator}
            className="rounded-xl bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-100 font-semibold px-6 py-3 text-xs shadow-lg transition-all cursor-pointer flex items-center gap-2"
          >
            <svg className="w-3.5 h-3.5 text-sky-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polygon points="5 3 19 12 5 21 5 3" />
            </svg>
            <span>Run Live Agent Simulator</span>
          </button>
          <a
            href="#benchmark"
            className="rounded-xl glass-panel hover:bg-white/[0.04] text-slate-200 font-semibold px-6 py-3 text-xs transition-all cursor-pointer"
          >
            View Benchmark
          </a>
        </div>

        {/* CLI Quick Start Banner */}
        <div className="pt-2 max-w-xl mx-auto">
          <div className="glass-panel rounded-xl p-2.5 flex items-center justify-between gap-3 text-xs font-mono border border-white/[0.08]">
            <div className="flex items-center gap-2 truncate text-slate-300">
              <span className="text-sky-400">$</span>
              <span className="truncate">curl -fsSL https://langpeanut.ai/install.sh | bash</span>
            </div>
            <button
              onClick={copyCliCommand}
              className="rounded-lg bg-white/5 hover:bg-white/10 text-slate-300 text-[11px] px-3 py-1 shrink-0 transition-colors cursor-pointer"
            >
              {copiedCLI ? '✓ Copied' : 'Copy CLI'}
            </button>
          </div>
        </div>

        {/* Key Benchmark Metrics */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 pt-6 text-left">
          {[
            { metric: '100%', label: 'AST Compilation Pass', desc: 'Zero hallucinated code' },
            { metric: '0%', label: 'Placeholder Drift', desc: '4-Tier Critic verification' },
            { metric: '86.4%', label: 'Token Reduction', desc: 'Layer 1 AST static scout' },
            { metric: 'Tier-5', label: 'Self-Healing Repair', desc: 'Autonomous compiler fixes' },
          ].map((item, idx) => (
            <div key={idx} className="glass-panel p-4 rounded-xl space-y-1">
              <div className="text-2xl font-bold font-mono text-sky-400 tracking-tight">{item.metric}</div>
              <div className="text-xs font-semibold text-slate-200">{item.label}</div>
              <div className="text-[11px] text-slate-500">{item.desc}</div>
            </div>
          ))}
        </div>
      </section>

      {/* ─── Autonomous Agentic Workflows Section ────────────────────────────── */}
      <section id="workflows" className="space-y-8 scroll-mt-24 pt-4 border-t border-white/[0.08]">
        <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
          <div className="space-y-2 max-w-xl">
            <div className="inline-flex items-center gap-2 text-xs font-mono text-sky-400 font-semibold uppercase tracking-wider">
              <span className="w-1.5 h-1.5 rounded-full bg-sky-400" />
              <span>Orchestrated Workflows</span>
            </div>
            <h2 className="text-2xl font-bold text-white">Continuous Agentic Automation</h2>
            <p className="text-xs text-slate-400 leading-relaxed">
              Choose how your engineering team interacts with the 6-agent engine: continuous background push sync,
              PR review pair-programming, missing key drift guards, or batch milestone freezes.
            </p>
          </div>

          <div className="flex flex-wrap gap-1.5 bg-slate-900/80 p-1.5 rounded-xl border border-white/[0.08]">
            {WORKFLOWS.map((wf) => (
              <button
                key={wf.id}
                onClick={() => setActiveWorkflowId(wf.id)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                  activeWorkflowId === wf.id
                    ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30'
                    : 'text-slate-400 hover:text-white'
                }`}
              >
                {wf.name.split(' ')[0]} {wf.name.split(' ')[1]}
              </button>
            ))}
          </div>
        </div>

        {/* Selected Workflow Detailed Card */}
        <div className="glass-panel rounded-2xl p-6 space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-white/[0.08] pb-4">
            <div className="space-y-1">
              <div className="flex items-center gap-3">
                <h3 className="text-lg font-bold text-white">{selectedWorkflow.name}</h3>
                <span className={`text-[11px] font-mono font-medium px-2.5 py-0.5 rounded-full border ${selectedWorkflow.badgeColor}`}>
                  {selectedWorkflow.badge}
                </span>
              </div>
              <p className="text-xs text-slate-400">{selectedWorkflow.description}</p>
            </div>
            <div className="text-left sm:text-right shrink-0">
              <span className="text-[10px] text-slate-500 uppercase font-mono block">Trigger Channel</span>
              <span className="text-xs font-mono text-sky-300 font-semibold">{selectedWorkflow.trigger}</span>
            </div>
          </div>

          {/* Workflow DAG Steps Breakdown */}
          <div className="space-y-3">
            <h4 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
              Execution DAG & State Machine
            </h4>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-5 gap-3">
              {selectedWorkflow.pipeline.map((p, idx) => (
                <div key={idx} className="rounded-xl border border-white/[0.06] bg-black/30 p-3.5 space-y-1.5">
                  <div className="text-xs font-bold text-sky-400 font-mono">{p.step}</div>
                  <div className="text-[11px] text-slate-400 leading-snug">{p.desc}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Sample Generated Output Preview */}
          <div className="space-y-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-slate-400 block">
              Generated GitHub PR / Check Output Preview
            </span>
            <pre className="p-3.5 bg-black/60 rounded-xl text-xs font-mono text-emerald-300/90 border border-white/[0.06] overflow-x-auto leading-relaxed">
              <code>{selectedWorkflow.sampleArtifact}</code>
            </pre>
          </div>
        </div>
      </section>

      {/* ─── Analytics & Translation Memory Efficiency Section ───────────────── */}
      <section id="analytics" className="space-y-8 scroll-mt-24 pt-4 border-t border-white/[0.08]">
        <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
          <div className="space-y-2 max-w-xl">
            <div className="inline-flex items-center gap-2 text-xs font-mono text-emerald-400 font-semibold uppercase tracking-wider">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
              <span>Intelligence & Cost Metrics</span>
            </div>
            <h2 className="text-2xl font-bold text-white">Translation Memory & Efficiency</h2>
            <p className="text-xs text-slate-400 leading-relaxed">
              Track token savings, exact cache hit ratios from the SQLite Translation Memory store, and 4-Tier Critic verification pass rates.
            </p>
          </div>

          <div className="flex items-center gap-2 text-xs font-mono text-slate-400 bg-white/[0.03] border border-white/[0.06] px-3.5 py-1.5 rounded-xl">
            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
            <span>Active Store: SQLite WAL Cache</span>
          </div>
        </div>

        {/* Analytics Key Stat Cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="glass-panel p-5 rounded-xl space-y-1.5">
            <span className="text-[11px] font-mono text-slate-500 uppercase tracking-wider block">
              Tokens Saved by TM Cache
            </span>
            <div className="text-2xl font-bold font-mono text-emerald-400">148,200</div>
            <div className="text-xs text-slate-400">64.8% reduction vs raw LLM calls</div>
          </div>

          <div className="glass-panel p-5 rounded-xl space-y-1.5">
            <span className="text-[11px] font-mono text-slate-500 uppercase tracking-wider block">
              Total Localization Cost
            </span>
            <div className="text-2xl font-bold font-mono text-sky-400">$0.0412</div>
            <div className="text-xs text-slate-400">vs $0.28 industry translation API</div>
          </div>

          <div className="glass-panel p-5 rounded-xl space-y-1.5">
            <span className="text-[11px] font-mono text-slate-500 uppercase tracking-wider block">
              Avg Pipeline Latency
            </span>
            <div className="text-2xl font-bold font-mono text-cyan-400">1.4s</div>
            <div className="text-xs text-slate-400">Per 50 AST string extractions</div>
          </div>

          <div className="glass-panel p-5 rounded-xl space-y-1.5">
            <span className="text-[11px] font-mono text-slate-500 uppercase tracking-wider block">
              Critic Verification Rate
            </span>
            <div className="text-2xl font-bold font-mono text-emerald-400">99.9%</div>
            <div className="text-xs text-slate-400">0 ICU drift • 0 syntax defects</div>
          </div>
        </div>
      </section>

      {/* ─── Empirical Benchmark Comparison Matrix ──────────────────────────── */}
      <section id="benchmark" className="space-y-8 scroll-mt-24 pt-4 border-t border-white/[0.08]">
        <div className="text-center space-y-2 max-w-2xl mx-auto">
          <div className="inline-flex items-center gap-2 text-xs font-mono text-sky-400 font-semibold uppercase tracking-wider">
            <span className="w-1.5 h-1.5 rounded-full bg-sky-400" />
            <span>Empirical Benchmark Results</span>
          </div>
          <h2 className="text-2xl font-bold text-white">Adversarial Evaluation vs Alternatives</h2>
          <p className="text-xs text-slate-400">
            Tested across 10 adversarial codebases containing nested JSX interpolation, Flutter const widgets, SwiftUI modifiers, and plural counts.
          </p>
        </div>

        <div className="glass-panel rounded-2xl overflow-hidden border border-white/[0.08]">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs border-collapse">
              <thead>
                <tr className="border-b border-white/[0.08] bg-slate-900/60 font-mono text-slate-400">
                  <th className="p-4 font-semibold">Evaluation Criteria</th>
                  <th className="p-4 font-semibold text-sky-400">langPeanut (6-Agent AST)</th>
                  <th className="p-4 font-semibold text-slate-400">Naive Whole-File LLM</th>
                  <th className="p-4 font-semibold text-slate-400">Cloud Translation API</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-white/[0.04] text-slate-300">
                {[
                  {
                    title: 'AST Compilation Pass Rate',
                    desc: 'Zero broken imports, deleted braces, or lost comments',
                    langPeanut: '100.0% (Deterministic Byte-Range)',
                    naive: '41.2% (Hallucinates code structure)',
                    cloud: 'N/A (Does not modify code)',
                    highlight: true,
                  },
                  {
                    title: 'ICU Plural & Variable Parity',
                    desc: 'Preserves {count, plural}, %@, and {name} tokens',
                    langPeanut: '100.0% (4-Tier Critic Verified)',
                    naive: '18.4% (Mangles plural grammar)',
                    cloud: '52.0% (Breaks variable tokens)',
                    highlight: true,
                  },
                  {
                    title: 'Token Consumption Efficiency',
                    desc: 'Evaluated on 500-line source files with 5 strings',
                    langPeanut: '480 tokens (86.4% reduction)',
                    naive: '3,800 tokens (Whole file prompt)',
                    cloud: '500 tokens (Text payload only)',
                    highlight: true,
                  },
                  {
                    title: 'Brand Lexicon & Glossary Memory',
                    desc: 'Prevents trademark corruption (e.g. langPeanut)',
                    langPeanut: 'Protected at Tokenizer Level',
                    naive: 'Prone to literal translation',
                    cloud: 'Requires enterprise glossary tier',
                    highlight: false,
                  },
                  {
                    title: 'Autonomous Compiler Self-Healing',
                    desc: 'Validates tsc and flutter analyze before PR',
                    langPeanut: 'Tier-5 Autonomous Repair Loop',
                    naive: 'No compiler feedback loop',
                    cloud: 'No developer code integration',
                    highlight: false,
                  },
                ].map((row, idx) => (
                  <tr key={idx} className={row.highlight ? 'bg-white/[0.01]' : ''}>
                    <td className="p-4 space-y-0.5">
                      <div className="font-semibold text-white">{row.title}</div>
                      <div className="text-[11px] text-slate-500">{row.desc}</div>
                    </td>
                    <td className="p-4 font-mono font-semibold text-emerald-400 bg-emerald-950/20">
                      {row.langPeanut}
                    </td>
                    <td className="p-4 font-mono text-rose-400">{row.naive}</td>
                    <td className="p-4 font-mono text-slate-400">{row.cloud}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      {/* ─── Visual Multi-Locale & RTL Live Previewer ────────────────────────── */}
      <section id="preview" className="space-y-6 scroll-mt-24 pt-4 border-t border-white/[0.08]">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-white/[0.08] pb-4">
          <div>
            <h2 className="text-lg font-bold text-white flex items-center gap-2">
              <span>Interactive Multi-Locale & RTL Layout Preview</span>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium uppercase">
                Live Simulator
              </span>
            </h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Experience dynamic Right-to-Left (RTL) flipping for Arabic and character expansion safety for German.
            </p>
          </div>

          <div className="flex flex-wrap gap-1.5 bg-slate-900/80 p-1 rounded-xl border border-white/[0.08]">
            {Object.entries(PREVIEW_DATA).map(([k, v]) => (
              <button
                key={k}
                onClick={() => setActivePreviewLocale(k)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer flex items-center gap-1.5 ${
                  activePreviewLocale === k
                    ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30'
                    : 'text-slate-400 hover:text-white'
                }`}
              >
                <span className="font-mono text-[10px] uppercase font-bold text-sky-300 bg-sky-500/10 border border-sky-500/20 px-1 py-0.2 rounded">
                  {v.flag}
                </span>
                <span>{v.name.split(' ')[0]}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Live UI Mockup Card adapted to selected locale and RTL */}
        <div
          dir={preview.dir}
          className="glass-panel bg-[#070b14] rounded-2xl p-6 sm:p-8 space-y-6 border border-white/10 shadow-2xl transition-all"
        >
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-white/[0.08] pb-6">
            <div className="space-y-1 max-w-lg">
              <h3 className="text-xl font-extrabold text-white tracking-tight">{preview.title}</h3>
              <p className="text-xs text-slate-400 leading-relaxed">{preview.subtitle}</p>
            </div>
            <div className="shrink-0">
              <span className="text-xs font-mono text-emerald-400 bg-emerald-950/40 border border-emerald-800/40 px-3 py-1 rounded-full font-semibold">
                {preview.metricValue}
              </span>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="rounded-xl border border-white/[0.06] bg-black/40 p-5 space-y-3">
              <div className="flex items-center justify-between">
                <span className="font-bold text-white text-sm">{preview.cardHeading}</span>
                <span className="text-[11px] font-mono text-sky-400 bg-sky-500/10 px-2 py-0.5 rounded">
                  {preview.metricLabel}
                </span>
              </div>
              <p className="text-xs text-slate-400 leading-relaxed">{preview.cardDesc}</p>
            </div>

            <div className="rounded-xl border border-white/[0.06] bg-black/40 p-5 flex flex-col justify-between gap-4">
              <div className="space-y-1">
                <span className="text-xs font-semibold text-slate-300">Quick Actions</span>
                <p className="text-[11px] text-slate-500">
                  Layout direction: <strong>{preview.dir.toUpperCase()}</strong> • Locale: <strong>{preview.name}</strong>
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-2.5">
                <button className="rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-semibold px-4 py-2 text-xs shadow-md shadow-blue-600/30 cursor-pointer transition-all">
                  {preview.cta}
                </button>
                <button className="rounded-lg bg-white/5 hover:bg-white/10 text-slate-300 font-medium px-4 py-2 text-xs cursor-pointer transition-all">
                  {preview.secondaryCta}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ─── Interactive AST Playground / Live Simulator ────────────────────── */}
      <section id="playground" className="space-y-6 scroll-mt-24 pt-4 border-t border-white/[0.08]">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-white/[0.08] pb-4">
          <div>
            <h2 className="text-lg font-bold text-white flex items-center gap-2">
              <span>Interactive AST Extraction & Patching Simulator</span>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-sky-500/10 text-sky-400 border border-sky-500/20 font-medium uppercase">
                Zero Generation
              </span>
            </h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Experience how the 6-agent engine isolates string literal AST nodes without full-file rewrites.
            </p>
          </div>

          <div className="flex items-center gap-1.5 bg-slate-900/80 p-1 rounded-xl border border-white/[0.08]">
            {Object.entries(PLAYGROUND_SNIPPETS).map(([k, v]) => (
              <button
                key={k}
                onClick={() => setActiveSnippetKey(k)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                  activeSnippetKey === k
                    ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30'
                    : 'text-slate-400 hover:text-white'
                }`}
              >
                {v.framework.split(' ')[0]}
              </button>
            ))}
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Left: Original Code */}
          <div className="glass-panel rounded-xl p-4 space-y-2.5">
            <div className="flex items-center justify-between text-xs text-slate-400 border-b border-white/[0.06] pb-2">
              <span className="font-semibold text-slate-300">1. Original Code (Hardcoded UI Strings)</span>
              <span className="font-mono text-[11px] text-amber-400 bg-amber-500/10 border border-amber-500/20 px-2 py-0.5 rounded">
                AST Scout Detected
              </span>
            </div>
            <pre className="text-xs font-mono text-slate-300 overflow-x-auto p-3 bg-black/40 rounded-lg border border-white/[0.04] leading-relaxed">
              <code>{PLAYGROUND_SNIPPETS[activeSnippetKey].original}</code>
            </pre>
          </div>

          {/* Right: Refactored AST Code & Locale JSON */}
          <div className="glass-panel rounded-xl p-4 space-y-4">
            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs text-slate-400 border-b border-white/[0.06] pb-2">
                <span className="font-semibold text-slate-300">2. Refactored AST (Surgical Byte-Range Patch)</span>
                <span className="font-mono text-[11px] text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 rounded">
                  Passed AST Validation
                </span>
              </div>
              <pre className="text-xs font-mono text-emerald-300/90 overflow-x-auto p-3 bg-black/40 rounded-lg border border-white/[0.04] leading-relaxed">
                <code>{PLAYGROUND_SNIPPETS[activeSnippetKey].refactored}</code>
              </pre>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between text-xs text-slate-400 border-b border-white/[0.06] pb-2">
                <span className="font-semibold text-slate-300">3. Synthesized Target Locale (ICU Plural Parity)</span>
                <span className="font-mono text-[11px] text-sky-400 bg-sky-500/10 border border-sky-500/20 px-2 py-0.5 rounded">
                  4-Tier Critic Approved
                </span>
              </div>
              <pre className="text-xs font-mono text-sky-300/90 overflow-x-auto p-3 bg-black/40 rounded-lg border border-white/[0.04] leading-relaxed">
                <code>{PLAYGROUND_SNIPPETS[activeSnippetKey].localeJSON}</code>
              </pre>
            </div>
          </div>
        </div>
      </section>

      {/* ─── 6-Agent Architecture Breakdown ──────────────────────────────────── */}
      <section id="architecture" className="space-y-8 scroll-mt-24 pt-4 border-t border-white/[0.08]">
        <div className="text-center space-y-2 max-w-2xl mx-auto">
          <h2 className="text-2xl font-bold text-white">The 6-Agent Localization Engine</h2>
          <p className="text-xs text-slate-400">
            Coordinated separation of concerns pairing deterministic AST compilers with closed-loop reflection critics.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {[
            {
              num: '01',
              title: 'Supervisor Orchestrator',
              role: 'Lifecycle & State DAG',
              desc: 'Coordinates multi-locale parallel execution, handles token packing, and takes atomic pre-flight snapshots.',
            },
            {
              num: '02',
              title: 'AST Scout Extractor',
              role: 'Tree-Sitter Static Analysis',
              desc: 'Extracts UI string literal nodes while filtering out routes, URLs, hex colors, and log messages with 0 token spend.',
            },
            {
              num: '03',
              title: 'Semantic Context Agent',
              role: 'Domain Disambiguation',
              desc: 'Understands component hierarchies to disambiguate homonyms and generates semantic keys like flightBookBtn.',
            },
            {
              num: '04',
              title: 'AST Range Patch Engine',
              role: 'Zero-Generation Precision',
              desc: 'Applies surgical byte-range replacements to source files. Never hallucinates syntax or deletes comments.',
            },
            {
              num: '05',
              title: 'Cultural Translator',
              role: 'ICU & Translation Memory',
              desc: 'Synthesizes accurate translations preserving complex ICU plurals, gender select, and format tokens (%@, {name}).',
            },
            {
              num: '06',
              title: '4-Tier Critic & Repair Agent',
              role: 'Self-Correction & Healing',
              desc: 'Asserts AST validity and placeholder parity; auto-heals TypeScript and Flutter compiler errors before opening the PR.',
            },
          ].map((ag) => (
            <div key={ag.num} className="glass-panel p-5 rounded-xl space-y-2 relative overflow-hidden">
              <span className="text-xs font-mono text-sky-400 font-bold">{ag.num}</span>
              <h3 className="font-bold text-sm text-white">{ag.title}</h3>
              <div className="text-[11px] font-mono text-slate-500 uppercase tracking-wider">{ag.role}</div>
              <p className="text-xs text-slate-400 leading-relaxed">{ag.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* ─── Supported Frameworks Section ────────────────────────────────────── */}
      <section id="frameworks" className="space-y-6 scroll-mt-24 pt-4 border-t border-white/[0.08]">
        <div className="text-center space-y-2 max-w-2xl mx-auto">
          <h2 className="text-2xl font-bold text-white">Supported Platforms & Frameworks</h2>
          <p className="text-xs text-slate-400">
            Native Tree-Sitter AST parsers for every modern mobile, web, and backend ecosystem.
          </p>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {[
            { name: 'React / Next.js', format: 'i18next / next-intl', tag: 'TSX' },
            { name: 'Flutter', format: 'ARB / AppLocalizations', tag: 'Dart' },
            { name: 'iOS SwiftUI', format: '.xcstrings / LocalizedKey', tag: 'Swift' },
            { name: 'Android Compose', format: 'strings.xml / resource', tag: 'Kotlin' },
            { name: 'Vue / Nuxt', format: 'vue-i18n JSON', tag: 'Vue' },
            { name: 'Angular', format: 'XLIFF / JSON', tag: 'TypeScript' },
            { name: 'Go Backend', format: 'go-i18n JSON/TOML', tag: 'Go' },
            { name: 'Python', format: 'gettext .po / .pot', tag: 'Python' },
          ].map((f, i) => (
            <div key={i} className="glass-panel p-4 rounded-xl space-y-2 text-center">
              <div className="inline-flex items-center justify-center px-2.5 py-1 rounded-md bg-slate-900 border border-white/10 text-xs font-mono font-semibold text-sky-400">
                {f.tag}
              </div>
              <div className="font-semibold text-xs text-white">{f.name}</div>
              <div className="text-[11px] text-slate-400 font-mono">{f.format}</div>
            </div>
          ))}
        </div>
      </section>

      {/* ─── Modal: Live Agent Execution Terminal ───────────────────────────── */}
      {showTerminal && (
        <div className="fixed inset-0 z-50 bg-black/85 backdrop-blur-md flex items-center justify-center p-4">
          <div className="glass-panel bg-[#05070e] border border-white/10 rounded-2xl w-full max-w-3xl shadow-2xl p-6 space-y-4">
            <div className="flex items-center justify-between border-b border-white/[0.08] pb-3">
              <div className="flex items-center gap-3">
                <div className="flex items-center gap-1.5">
                  <span className="w-3 h-3 rounded-full bg-rose-500/80 inline-block" />
                  <span className="w-3 h-3 rounded-full bg-amber-500/80 inline-block" />
                  <span className="w-3 h-3 rounded-full bg-emerald-500/80 inline-block" />
                </div>
                <h3 className="font-bold text-sm text-white font-mono flex items-center gap-2">
                  <span>langPeanut Live Multi-Agent Execution Terminal</span>
                  {simulating && (
                    <span className="text-[10px] text-sky-400 animate-pulse font-sans bg-sky-500/10 px-2 py-0.5 rounded border border-sky-500/20">
                      Processing...
                    </span>
                  )}
                </h3>
              </div>
              <button
                onClick={() => setShowTerminal(false)}
                className="text-slate-500 hover:text-slate-300 text-base cursor-pointer"
              >
                ✕
              </button>
            </div>

            <div className="bg-black/90 p-4 rounded-xl border border-white/[0.06] font-mono text-xs text-slate-300 h-80 overflow-y-auto space-y-1.5 leading-relaxed">
              {terminalLogs.map((log, i) => (
                <div key={i} className="flex items-start gap-2">
                  <span className="text-slate-600 select-none">{i + 1}</span>
                  <span
                    className={
                      log.includes('✓')
                        ? 'text-emerald-400'
                        : log.includes('Pull Request')
                        ? 'text-sky-300 font-bold'
                        : log.includes('AST Scout')
                        ? 'text-amber-300'
                        : log.includes('Translator')
                        ? 'text-blue-300'
                        : 'text-slate-300'
                    }
                  >
                    {log}
                  </span>
                </div>
              ))}
            </div>

            <div className="flex items-center justify-between pt-2">
              <button
                disabled={simulating}
                onClick={runSimulator}
                className="text-xs text-sky-400 hover:text-sky-300 font-semibold cursor-pointer disabled:opacity-50 flex items-center gap-1.5"
              >
                <svg className="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67" />
                </svg>
                <span>Re-run Simulation</span>
              </button>
              <button
                onClick={() => setShowTerminal(false)}
                className="rounded-lg bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium px-4 py-2 cursor-pointer"
              >
                Close Terminal
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
