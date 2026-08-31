'use client'

import useSWR from 'swr'
import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { cn, decodeHtmlEntities } from '@/lib/utils'
import { Lock, Plus, RefreshCw, Play, Trash2, HelpCircle, Code, Shield, Cpu, CheckCheck } from 'lucide-react'

const fetcher = (url: string) =>
  fetch(url, { credentials: 'include' }).then((r) => {
    if (!r.ok) throw new Error(`${r.status}`)
    return r.json()
  })

interface Repo {
  ID: number
  InstallationID: number
  Owner: string
  Name: string
  DefaultBranch: string
  HasSettings: boolean
  settings?: RepoSettings
}

interface RepoSettings {
  RepoID: number
  Locales: string[]
  TonePreset: string
  Provider: string
  Model: string
  SafetyMode: boolean
  ChunkWordBudget: number
  ChunkKeyCeiling: number
  RootDir?: string
  has_api_key_override?: boolean
}

interface AvailableRepo {
  installation_id: number
  account_login: string
  repo_id: number
  owner: string
  name: string
  default_branch: string
  private: boolean
  is_imported: boolean
}

interface ProviderCredential {
  provider: string
  configured: boolean
}

const PROVIDER_MODELS: Record<string, { label: string; tag: string; placeholder: string; envVar: string }> = {
  openai: { label: 'OpenAI', tag: 'OAI', placeholder: 'sk-proj-...', envVar: 'OPENAI_API_KEY' },
  claude: { label: 'Anthropic Claude', tag: 'CLD', placeholder: 'sk-ant-api03-...', envVar: 'ANTHROPIC_API_KEY' },
  gemini: { label: 'Google Gemini', tag: 'GEM', placeholder: 'AIzaSy...', envVar: 'GEMINI_API_KEY' },
  deepl: { label: 'DeepL Translate', tag: 'DPL', placeholder: 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:fx', envVar: 'DEEPL_AUTH_KEY' },
  custom: { label: 'Custom / Local Ollama', tag: 'LOC', placeholder: 'http://localhost:11434/v1 or custom key', envVar: 'CUSTOM_API_KEY' },
}

const ONBOARDING_STEPS = [
  {
    title: 'Universal Codebase AST Extraction',
    subtitle: 'Zero Regex Drift with Tree-sitter Grammars',
    icon: Code,
    bullets: [
      'Direct AST parsing for TypeScript/React, Flutter/Dart, SwiftUI, Kotlin, Vue, HTML, and Go.',
      'Extracts hardcoded UI strings, JSX children, attribute props, and template literals with line-exact byte coordinates.',
      'Automatically skips technical identifiers, regex patterns, import statements, and code logic.',
    ],
  },
  {
    title: 'ICU Syntax & Interpolation Safety',
    subtitle: 'Deterministic Token & Pluralization Protection',
    icon: Shield,
    bullets: [
      'Preserves template variables ({count}, {name}, %d, $price) and complex ICU plural formatting.',
      'Eliminates broken runtime templates and variable distortion across multi-language catalogs.',
      'Protects brand names, punctuation formatting, and glossary keywords.',
    ],
  },
  {
    title: 'Autonomous Multi-Agent Translation',
    subtitle: 'Frontier LLMs or 100% Offline Local Neural Engine',
    icon: Cpu,
    bullets: [
      'Translate via Google Gemini 3.7 Flash, Claude Sonnet 3.7, GPT-4.5, or run 100% offline via local Meta NLLB-200 / Ollama.',
      'Smart token budgeting packs up to 50,000 tokens per request to minimize API rounds and latency.',
      'Maintains cultural idiom fidelity and tone personas (Professional, Casual, Corporate Formal, Gen-Z).',
    ],
  },
  {
    title: '4-Tier Critic Verification & Surgical Patching',
    subtitle: 'Continuous Quality Assurance & Code Integration',
    icon: CheckCheck,
    bullets: [
      'Every translation is audited across syntax, ICU placeholder matching, UI expansion budget, and key parity.',
      'Applies byte-range surgical code diffs directly to source files without full-file hallucination.',
      'Automatically opens GitHub Pull Requests with verified translation catalogs on push triggers.',
    ],
  },
]

export default function DashboardPage() {
  const router = useRouter()
  const [authChecked, setAuthChecked] = useState(false)
  const [authed, setAuthed] = useState(false)

  useEffect(() => {
    let cancelled = false
    fetch('/api/auth/me', { credentials: 'include' })
      .then((r) => {
        if (cancelled) return
        if (!r.ok) {
          router.replace('/login')
          return
        }
        setAuthed(true)
        setAuthChecked(true)
        if (typeof window !== 'undefined' && !localStorage.getItem('langpeanut_onboarding_completed')) {
          setShowOnboardingModal(true)
        }
      })
      .catch(() => {
        if (!cancelled) router.replace('/login')
      })
    return () => {
      cancelled = true
    }
  }, [router])

  const { data: repos, isLoading: reposLoading, mutate: mutateRepos } = useSWR<Repo[]>(
    authed ? '/api/repos' : null,
    fetcher
  )
  const { data: credentials, mutate: mutateCreds } = useSWR<ProviderCredential[]>(
    authed ? '/api/credentials' : null,
    fetcher
  )

  const [showImportModal, setShowImportModal] = useState(false)
  const [showVaultModal, setShowVaultModal] = useState(false)
  const [showOnboardingModal, setShowOnboardingModal] = useState(false)
  const [onboardingStep, setOnboardingStep] = useState(0)
  const [vaultProvider, setVaultProvider] = useState<string>('gemini')
  const [vaultKeyInput, setVaultKeyInput] = useState<string>('')
  const [savingVaultKey, setSavingVaultKey] = useState(false)
  const [vaultFeedback, setVaultFeedback] = useState<string>('')

  const [repoSearch, setRepoSearch] = useState('')
  const [triggeringId, setTriggeringId] = useState<number | null>(null)
  const [deletingRepoId, setDeletingRepoId] = useState<number | null>(null)
  const [toastMsg, setToastMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  // Available GitHub Repos for import
  const { data: availableRepos, isLoading: loadingAvailable, mutate: mutateAvailable } = useSWR<AvailableRepo[]>(
    authed && showImportModal ? '/api/github/available-repos' : null,
    fetcher
  )
  const [importingKey, setImportingKey] = useState<string | null>(null)

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    setToastMsg({ text, type })
    setTimeout(() => setToastMsg(null), 5000)
  }

  async function importRepo(r: AvailableRepo) {
    const key = `${r.owner}/${r.name}`
    setImportingKey(key)
    try {
      const res = await fetch('/api/repos', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          installation_id: r.installation_id,
          owner: r.owner,
          name: r.name,
          default_branch: r.default_branch || 'main',
        }),
      })
      if (!res.ok) {
        const err = await res.json()
        throw new Error(err.error || 'Import failed')
      }
      const imported: Repo = await res.json()
      mutateRepos()
      mutateAvailable()
      showToast(`Imported ${r.owner}/${r.name} successfully`)
      setShowImportModal(false)
      router.push(`/repo?id=${imported.ID}&tab=settings`)
    } catch (e: unknown) {
      showToast(e instanceof Error ? e.message : 'Import failed', 'error')
    } finally {
      setImportingKey(null)
    }
  }

  async function triggerJob(repo: Repo, e: React.MouseEvent) {
    e.stopPropagation()
    setTriggeringId(repo.ID)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/jobs`, {
        method: 'POST',
        credentials: 'include',
      })
      const body = await res.json()
      if (res.ok) {
        showToast(`Job #${body.ID} queued for ${repo.Owner}/${repo.Name}`)
        router.push(`/repo?id=${repo.ID}&tab=runs`)
      } else {
        showToast(body.error || 'Failed to trigger job', 'error')
      }
    } catch (e: unknown) {
      showToast(e instanceof Error ? e.message : 'Network error', 'error')
    } finally {
      setTriggeringId(null)
    }
  }

  function isProviderConfigured(p: string): boolean {
    return credentials?.some((c) => c.provider === p && c.configured) ?? false
  }

  async function saveGlobalVaultKey() {
    if (!vaultKeyInput.trim()) {
      setVaultFeedback('Please enter a valid API key.')
      return
    }
    setSavingVaultKey(true)
    setVaultFeedback('')

    try {
      const res = await fetch(`/api/credentials/${vaultProvider}`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ api_key: vaultKeyInput.trim() }),
      })
      if (!res.ok) {
        const err = await res.json()
        throw new Error(err.error || 'Failed to save credential')
      }
      mutateCreds()
      setVaultKeyInput('')
      showToast(`Global ${PROVIDER_MODELS[vaultProvider]?.label} key saved successfully!`)
      setShowVaultModal(false)
    } catch (e: unknown) {
      setVaultFeedback(e instanceof Error ? e.message : 'Failed to save key')
    } finally {
      setSavingVaultKey(false)
    }
  }

  async function deleteRepo(r: Repo, e: React.MouseEvent) {
    e.stopPropagation()
    const confirmed = window.confirm(
      `Are you sure you want to permanently delete repository ${r.Owner}/${r.Name}?\n\nThis will remove the repository connection, all stored translation data, and cached git mirrors.`
    )
    if (!confirmed) return

    setDeletingRepoId(r.ID)
    try {
      const res = await fetch(`/api/repos/${r.ID}`, { method: 'DELETE', credentials: 'include' })
      const data = await res.json()
      if (res.ok) {
        showToast(data.message || 'Repository deleted successfully.')
        mutateRepos()
      } else {
        showToast(data.error || 'Failed to delete repository', 'error')
      }
    } catch {
      showToast('Network error while deleting repository', 'error')
    } finally {
      setDeletingRepoId(null)
    }
  }

  if (!authChecked) {
    return (
      <div className="space-y-6 max-w-7xl mx-auto pb-16 pt-4">
        <div className="flex justify-between items-center">
          <Skeleton className="h-8 w-64 bg-slate-800" />
          <div className="flex gap-3">
            <Skeleton className="h-9 w-36 bg-slate-800" />
            <Skeleton className="h-9 w-48 bg-slate-800" />
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-48 w-full rounded-2xl bg-slate-800" />
          ))}
        </div>
      </div>
    )
  }

  const filteredRepos = repos?.filter(
    (r) =>
      r.Name.toLowerCase().includes(repoSearch.toLowerCase()) ||
      r.Owner.toLowerCase().includes(repoSearch.toLowerCase())
  )

  return (
    <div className="space-y-8 max-w-7xl mx-auto pb-16">
      {/* Toast Notification */}
      {toastMsg && (
        <div
          className={`fixed top-5 right-5 z-50 rounded-xl px-5 py-3.5 text-xs font-semibold shadow-2xl border transition-all ${
            toastMsg.type === 'error'
              ? 'bg-rose-950/90 border-rose-700 text-rose-200'
              : 'bg-emerald-950/90 border-emerald-700 text-emerald-200'
          }`}
        >
          {toastMsg.text}
        </div>
      )}

      {/* Main Console Header */}
      <section className="space-y-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-white/[0.08] pb-4">
          <div>
            <h1 className="text-2xl font-extrabold text-white flex items-center gap-2.5 tracking-tight">
              <span>Connected Repositories</span>
              <Badge variant="outline" className="text-xs font-mono text-sky-400 bg-sky-500/10 border-sky-500/20 rounded-full">
                {repos?.length || 0} active
              </Badge>
            </h1>
            <p className="text-xs text-slate-400 mt-1">
              Select any project to view its dedicated localization strategy, live translation matrix, and PR automation logs.
            </p>
          </div>

          <div className="flex items-center gap-2.5">
            <Button
              variant="outline"
              size="sm"
              onClick={() => { setOnboardingStep(0); setShowOnboardingModal(true) }}
              className="bg-white/[0.05] hover:bg-white/[0.1] border-white/10 text-slate-200 gap-1.5"
              title="Studio Overview & Quick Start Guide"
            >
              <HelpCircle className="w-3.5 h-3.5 text-sky-400" />
              <span>Guide</span>
            </Button>

            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowVaultModal(true)}
              className="bg-white/[0.05] hover:bg-white/[0.1] border-white/10 text-slate-200 gap-2"
            >
              <Lock className="w-4 h-4 text-sky-400" />
              <span>Global AI Keys Vault</span>
            </Button>

            <Button
              size="sm"
              onClick={() => setShowImportModal(true)}
              className="bg-blue-600 hover:bg-blue-500 shadow-lg shadow-blue-600/30 gap-2"
            >
              <Plus className="w-4 h-4" />
              <span>Connect & Import Repositories</span>
            </Button>
          </div>
        </div>

        {/* Global BYO Credentials Bar (Clickable to edit vault) */}
        <div className="glass-panel rounded-2xl p-4 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-2">
              <span>Global AI Keys Vault</span>
              <span className="text-[10px] font-normal text-slate-500 font-sans">(Applies automatically across all your repositories)</span>
            </span>
            <button
              onClick={() => setShowVaultModal(true)}
              className="text-[11px] text-sky-400 hover:text-sky-300 font-medium cursor-pointer"
            >
              + Configure Keys ↗
            </button>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-2.5">
            {Object.entries(PROVIDER_MODELS).map(([k, p]) => {
              const configured = isProviderConfigured(k)
              return (
                <div
                  key={k}
                  onClick={() => {
                    setVaultProvider(k)
                    setShowVaultModal(true)
                  }}
                  className={`rounded-xl border px-3 py-2 text-xs transition-all cursor-pointer flex items-center justify-between gap-2 hover:border-sky-500/50 ${
                    configured
                      ? 'border-emerald-800/60 bg-emerald-950/20 text-emerald-300'
                      : 'border-white/[0.06] bg-slate-900/50 text-slate-400'
                  }`}
                >
                  <div className="flex items-center gap-1.5 truncate">
                    <span className="font-mono text-[10px] font-bold px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 border border-white/10">
                      {p.tag}
                    </span>
                    <span className="font-medium truncate">{p.label.split(' ')[0]}</span>
                  </div>
                  <span
                    className={`text-[10px] font-mono shrink-0 px-1.5 py-0.5 rounded ${
                      configured ? 'bg-emerald-900/50 text-emerald-300 font-semibold' : 'bg-slate-800 text-slate-500'
                    }`}
                  >
                    {configured ? 'Vault Active' : '+ Add Key'}
                  </span>
                </div>
              )
            })}
          </div>
        </div>

        {/* Repositories Search Filter */}
        <div className="flex items-center justify-between gap-4">
          <Input
            type="text"
            value={repoSearch}
            onChange={(e) => setRepoSearch(e.target.value)}
            placeholder="Search connected repositories by name or owner..."
            className="w-full sm:w-80 bg-slate-900/80 border-white/10 text-white placeholder:text-slate-500 focus-visible:ring-sky-400/50 focus-visible:border-sky-400"
          />
        </div>

        {/* Repositories Grid */}
        <div className="space-y-3">
          {reposLoading && (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {[1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-52 w-full rounded-2xl bg-slate-800" />
              ))}
            </div>
          )}

          {!reposLoading && (!filteredRepos || filteredRepos.length === 0) && (
            <Card className="glass-panel border-dashed border-white/10">
              <CardContent className="p-12 text-center space-y-4">
                <div className="w-12 h-12 rounded-2xl bg-slate-900 border border-white/10 flex items-center justify-center mx-auto text-slate-400">
                  <svg className="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z" />
                  </svg>
                </div>
                <div>
                  <h3 className="font-bold text-sm text-white">No Repositories Connected Yet</h3>
                  <p className="text-xs text-slate-400 mt-1 max-w-md mx-auto">
                    Connect your GitHub repositories to enable automated multi-agent localization on pushes and pull requests.
                  </p>
                </div>
                <Button
                  size="sm"
                  onClick={() => setShowImportModal(true)}
                  className="bg-blue-600 hover:bg-blue-500 shadow-lg shadow-blue-600/30 gap-1.5"
                >
                  <Plus className="w-3.5 h-3.5" />
                  Import First Repository
                </Button>
              </CardContent>
            </Card>
          )}

          {!reposLoading && filteredRepos && filteredRepos.length > 0 && (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {filteredRepos.map((r) => {
                const isTriggering = triggeringId === r.ID
                const locales = r.settings?.Locales || []

                return (
                  <Card
                    key={r.ID}
                    onClick={() => router.push(`/repo?id=${r.ID}`)}
                    className="glass-panel border-white/10 hover:border-sky-500/40 hover:bg-white/[0.03] transition-all cursor-pointer group flex flex-col justify-between"
                  >
                    <CardHeader className="pb-2 space-y-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2 truncate">
                          <svg className="w-4 h-4 text-sky-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4" />
                            <path d="M9 18c-4.51 2-5-2-7-2" />
                          </svg>
                          <span className="font-bold text-sm text-white truncate group-hover:text-sky-300 transition-colors">
                            {r.Owner}/{r.Name}
                          </span>
                        </div>
                        <Badge variant="outline" className="text-[10px] font-mono bg-slate-800 text-slate-400 border-white/5">
                          {r.DefaultBranch}
                        </Badge>
                      </div>
                    </CardHeader>

                    <CardContent className="pb-3 space-y-3">
                      {/* Locales & Strategy badges */}
                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between text-[11px] text-slate-400">
                          <span>Target Locales:</span>
                          <span className="font-semibold text-slate-200">
                            {locales.length > 0 ? `${locales.length} languages` : 'Not configured'}
                          </span>
                        </div>
                        <div className="flex flex-wrap gap-1">
                          {locales.slice(0, 5).map((loc) => (
                            <Badge key={loc} variant="outline" className="text-[10px] font-mono bg-slate-900 text-sky-400 border-white/5 uppercase px-2 py-0.5">
                              {loc}
                            </Badge>
                          ))}
                          {locales.length > 5 && (
                            <Badge variant="outline" className="text-[10px] font-mono bg-slate-900 text-slate-400 border-white/5">
                              +{locales.length - 5}
                            </Badge>
                          )}
                        </div>
                      </div>

                      {r.settings?.Provider && (
                        <div className="flex items-center justify-between text-[11px] text-slate-400">
                          <div className="flex items-center gap-1.5">
                            <span className="capitalize">{r.settings.Provider}</span>
                            <span>•</span>
                            <span className="font-mono text-slate-300">{r.settings.Model}</span>
                          </div>
                          {r.settings?.has_api_key_override ? (
                            <Badge variant="outline" className="text-[10px] font-mono text-amber-400 bg-amber-500/10 border-amber-500/20">
                              Repo Key
                            </Badge>
                          ) : (
                            <Badge variant="outline" className="text-[10px] font-mono text-emerald-400 bg-emerald-500/10 border-emerald-500/20">
                              Global Key
                            </Badge>
                          )}
                        </div>
                      )}
                    </CardContent>

                    {/* Card Footer Actions */}
                    <CardFooter className="pt-3 border-t border-white/[0.06] gap-2 justify-between">
                      <div className="flex items-center gap-1.5">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={(e) => {
                            e.stopPropagation()
                            router.push(`/repo?id=${r.ID}&tab=settings`)
                          }}
                          className="text-slate-300 hover:text-white hover:bg-white/10 text-xs h-7 px-3"
                        >
                          Strategy
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={(e) => deleteRepo(r, e)}
                          disabled={deletingRepoId === r.ID}
                          title="Delete repository connection"
                          className="text-rose-400 hover:text-rose-300 hover:bg-rose-500/10 h-7 w-7 p-0"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </Button>
                      </div>

                      <Button
                        size="sm"
                        onClick={(e) => triggerJob(r, e)}
                        disabled={isTriggering}
                        className="bg-blue-600/80 hover:bg-blue-600 disabled:bg-blue-900 text-white font-semibold h-7 px-3 gap-1.5"
                      >
                        <Play className="w-3 h-3" />
                        {isTriggering ? 'Running…' : 'Run'}
                      </Button>
                    </CardFooter>
                  </Card>
                )
              })}
            </div>
          )}
        </div>
      </section>

      {/* ─── Dialog: Global AI Keys Vault ──────────────────────────────────────── */}
      <Dialog open={showVaultModal} onOpenChange={(open) => {
        if (!open) { setVaultFeedback(''); setVaultKeyInput('') }
        setShowVaultModal(open)
      }}>
        <DialogContent className="bg-[#090d16] border-white/10 text-white max-w-lg">
          <DialogHeader>
            <DialogTitle className="font-bold text-base text-white">Global AI Keys Vault</DialogTitle>
            <DialogDescription className="text-xs text-slate-400">
              Keys saved here apply to all connected repositories across your account automatically.
            </DialogDescription>
          </DialogHeader>

          {vaultFeedback && (
            <div className="rounded-xl bg-rose-950/60 border border-rose-800 text-rose-200 px-4 py-2.5 text-xs">
              {vaultFeedback}
            </div>
          )}

          <div className="space-y-4">
            <div>
              <label className="text-[11px] font-semibold text-slate-300 block mb-1">Select AI Provider</label>
              <div className="grid grid-cols-3 sm:grid-cols-5 gap-1.5">
                {Object.entries(PROVIDER_MODELS).map(([k, p]) => (
                  <button
                    key={k}
                    type="button"
                    onClick={() => setVaultProvider(k)}
                    className={cn(
                      'p-2 rounded-xl text-xs font-semibold text-center border transition-all cursor-pointer',
                      vaultProvider === k
                        ? 'border-sky-500 bg-sky-500/15 text-white'
                        : 'border-white/[0.06] bg-slate-900/50 text-slate-400 hover:text-slate-200'
                    )}
                  >
                    <div className="font-mono text-[10px] text-slate-400">[{p.tag}]</div>
                    <div className="truncate text-[11px]">{p.label.split(' ')[0]}</div>
                  </button>
                ))}
              </div>
            </div>

            <div>
              <div className="flex items-center justify-between mb-1">
                <label className="text-[11px] font-semibold text-slate-300">
                  {PROVIDER_MODELS[vaultProvider]?.label} API Key
                </label>
                <span className="text-[10px] font-mono text-emerald-400">
                  {isProviderConfigured(vaultProvider) ? '✓ Currently Configured in Vault' : 'No Key Configured'}
                </span>
              </div>
              <Input
                type="password"
                value={vaultKeyInput}
                onChange={(e) => setVaultKeyInput(e.target.value)}
                placeholder={
                  isProviderConfigured(vaultProvider)
                    ? '•••••••••••••••••••• (Enter new key to update)'
                    : `Enter ${PROVIDER_MODELS[vaultProvider]?.placeholder || 'API Key'}`
                }
                className="bg-slate-900 border-white/10 text-white placeholder:text-slate-600 focus-visible:border-sky-400 font-mono"
              />
              <p className="text-[10px] text-slate-500 mt-1">
                Stored AES-256-GCM encrypted in SQLite WAL vault. Never exposed in responses.
              </p>
            </div>

            <div className="flex items-center justify-between pt-3 border-t border-white/[0.08]">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => { setShowVaultModal(false); setVaultFeedback(''); setVaultKeyInput('') }}
                className="text-slate-400 hover:text-slate-300"
              >
                Cancel
              </Button>

              <Button
                type="button"
                size="sm"
                onClick={saveGlobalVaultKey}
                disabled={savingVaultKey}
                className="bg-blue-600 hover:bg-blue-500 disabled:bg-blue-900 shadow-lg shadow-blue-600/30"
              >
                {savingVaultKey ? 'Saving to Vault…' : 'Save Global Key'}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* ─── Dialog: Connect & Import Repositories ─────────────────────────────── */}
      <Dialog open={showImportModal} onOpenChange={setShowImportModal}>
        <DialogContent className="bg-[#090d16] border-white/10 text-white max-w-xl">
          <DialogHeader>
            <DialogTitle className="font-bold text-base text-white">Import from GitHub App</DialogTitle>
            <DialogDescription className="text-xs text-slate-400">
              Select any repository granted to your installed GitHub App or install on new accounts.
            </DialogDescription>
          </DialogHeader>

          {/* GitHub App Installation Action Banner */}
          <div className="p-3 rounded-xl bg-sky-500/10 border border-sky-500/20 flex items-center justify-between gap-3 text-xs">
            <div className="space-y-0.5">
              <span className="font-semibold text-sky-300">Grant Access to Repositories</span>
              <p className="text-[11px] text-slate-400">Install the GitHub App on your personal account or organization.</p>
            </div>
            <Button asChild size="sm" variant="outline" className="text-xs h-7 border-sky-500/40 text-sky-300 hover:bg-sky-500/20 shrink-0">
              <a href="https://github.com/apps/langpeanut/installations/new" target="_blank" rel="noopener noreferrer">
                Install App ↗
              </a>
            </Button>
          </div>

          {loadingAvailable ? (
            <div className="space-y-2 py-4">
              {[1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-14 w-full rounded-xl bg-slate-800" />
              ))}
            </div>
          ) : !availableRepos || availableRepos.length === 0 ? (
            <div className="p-8 text-center text-xs text-slate-300 space-y-4">
              <div className="w-12 h-12 rounded-2xl bg-sky-500/10 border border-sky-500/20 text-sky-400 flex items-center justify-center mx-auto text-xl font-bold font-mono">
                APP
              </div>
              <div className="space-y-1">
                <p className="font-bold text-white text-sm">No Repositories Found</p>
                <p className="text-slate-400 text-xs max-w-md mx-auto">
                  To grant access, install the <strong className="text-slate-200">langPeanut GitHub App</strong> on your personal account or organization, then click Refresh.
                </p>
              </div>
              <div className="flex items-center justify-center gap-3 pt-2">
                <Button asChild size="sm" className="bg-blue-600 hover:bg-blue-500 shadow-lg shadow-blue-600/30">
                  <a href="https://github.com/apps/langpeanut/installations/new" target="_blank" rel="noopener noreferrer">
                    Install GitHub App ↗
                  </a>
                </Button>
                <Button variant="outline" size="sm" onClick={() => mutateAvailable()} className="gap-2 border-white/10 text-white bg-white/10 hover:bg-white/15">
                  <RefreshCw className="w-3.5 h-3.5" />
                  Refresh List
                </Button>
              </div>
            </div>
          ) : (
            <div className="max-h-80 overflow-y-auto space-y-2 pr-1 custom-scrollbar">
              {availableRepos.map((r) => {
                const key = `${r.owner}/${r.name}`
                const isImporting = importingKey === key

                return (
                  <div
                    key={key}
                    className="rounded-xl border border-white/[0.06] bg-white/[0.02] p-3.5 flex items-center justify-between gap-3 text-xs"
                  >
                    <div>
                      <p className="font-semibold text-white">
                        {decodeHtmlEntities(r.owner)}/{decodeHtmlEntities(r.name)}
                      </p>
                      <p className="text-[11px] text-slate-500 font-mono">
                        branch: {r.default_branch} • {r.private ? 'Private' : 'Public'}
                      </p>
                    </div>

                    <Button
                      size="sm"
                      disabled={r.is_imported || isImporting}
                      onClick={() => importRepo(r)}
                      className={cn(
                        'h-7 px-3.5 text-xs',
                        r.is_imported
                          ? 'bg-slate-800 text-slate-500 cursor-default hover:bg-slate-800'
                          : isImporting
                          ? 'bg-blue-900 text-blue-200 cursor-wait hover:bg-blue-900'
                          : 'bg-blue-600 hover:bg-blue-500 text-white shadow-md shadow-blue-600/30'
                      )}
                    >
                      {r.is_imported ? '✓ Imported' : isImporting ? 'Importing…' : 'Import'}
                    </Button>
                  </div>
                )
              })}
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* ─── Dialog: Product Walkthrough & Onboarding ───────────────────────────── */}
      <Dialog open={showOnboardingModal} onOpenChange={(open) => {
        if (!open) localStorage.setItem('langpeanut_onboarding_completed', 'true')
        setShowOnboardingModal(open)
      }}>
        <DialogContent className="bg-[#0b0e14] border-white/10 text-white max-w-xl space-y-5 p-6">
          <DialogHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-sky-400" />
                <span className="text-xs font-mono font-bold tracking-wider text-sky-400 uppercase">
                  langPeanut Platform Guide
                </span>
                <span className="text-[10px] font-mono text-slate-500 ml-2">
                  Step {onboardingStep + 1} of {ONBOARDING_STEPS.length}
                </span>
              </div>
            </div>
          </DialogHeader>

          {(() => {
            const step = ONBOARDING_STEPS[onboardingStep]
            const Icon = step.icon
            return (
              <div className="space-y-4 min-h-[220px]">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl bg-sky-500/10 border border-sky-500/20 text-sky-400 flex items-center justify-center text-base shrink-0">
                    <Icon className="w-5 h-5" />
                  </div>
                  <div>
                    <h3 className="text-sm font-bold text-white">{step.title}</h3>
                    <p className="text-xs text-sky-400 font-mono">{step.subtitle}</p>
                  </div>
                </div>

                <div className="p-4 rounded-xl bg-[#07090e] border border-white/[0.08] space-y-2 text-xs text-slate-300">
                  {step.bullets.map((b, idx) => (
                    <div key={idx} className="flex items-start gap-2 leading-relaxed">
                      <span className="text-sky-400 font-mono font-bold mt-0.5">•</span>
                      <span>{b}</span>
                    </div>
                  ))}
                </div>
              </div>
            )
          })()}

          <div className="flex items-center justify-between pt-3 border-t border-white/[0.08]">
            <button
              type="button"
              onClick={() => {
                localStorage.setItem('langpeanut_onboarding_completed', 'true')
                setShowOnboardingModal(false)
              }}
              className="text-xs text-slate-500 hover:text-slate-300 font-medium cursor-pointer"
            >
              Skip Walkthrough
            </button>

            <div className="flex items-center gap-2">
              {onboardingStep > 0 && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setOnboardingStep((s) => Math.max(0, s - 1))}
                  className="border-white/10 text-slate-300 hover:text-white"
                >
                  Previous
                </Button>
              )}
              <Button
                type="button"
                size="sm"
                onClick={() => {
                  if (onboardingStep < ONBOARDING_STEPS.length - 1) {
                    setOnboardingStep((s) => s + 1)
                  } else {
                    localStorage.setItem('langpeanut_onboarding_completed', 'true')
                    setShowOnboardingModal(false)
                    showToast('Welcome to langPeanut Cloud!')
                  }
                }}
                className="bg-blue-600 hover:bg-blue-500 text-white shadow-md shadow-blue-600/30"
              >
                {onboardingStep === ONBOARDING_STEPS.length - 1 ? 'Get Started' : 'Next'}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
