'use client'

import useSWR from 'swr'
import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'

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
  const [vaultProvider, setVaultProvider] = useState<string>('openai')
  const [vaultKeyInput, setVaultKeyInput] = useState<string>('')
  const [savingVaultKey, setSavingVaultKey] = useState(false)
  const [vaultFeedback, setVaultFeedback] = useState<string>('')

  const [repoSearch, setRepoSearch] = useState('')
  const [triggeringId, setTriggeringId] = useState<number | null>(null)
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

  if (!authChecked) {
    return (
      <div className="py-24 text-center text-slate-500 text-xs font-mono animate-pulse">
        Checking session…
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
              <span className="text-xs font-mono text-sky-400 bg-sky-500/10 px-2.5 py-0.5 rounded-full border border-sky-500/20">
                {repos?.length || 0} active
              </span>
            </h1>
            <p className="text-xs text-slate-400 mt-1">
              Select any project to view its dedicated localization strategy, live translation matrix, and PR automation logs.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={() => setShowVaultModal(true)}
              className="rounded-xl bg-white/[0.05] hover:bg-white/[0.1] border border-white/10 text-slate-200 font-medium px-4 py-2.5 text-xs transition-all cursor-pointer flex items-center gap-2"
            >
              <svg className="w-4 h-4 text-sky-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
                <path d="M7 11V7a5 5 0 0 1 10 0v4" />
              </svg>
              <span>Global AI Keys Vault</span>
            </button>

            <button
              onClick={() => setShowImportModal(true)}
              className="rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold px-4 py-2.5 text-xs shadow-lg shadow-blue-600/30 transition-all cursor-pointer flex items-center gap-2"
            >
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
              <span>Connect & Import Repositories</span>
            </button>
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
          <input
            type="text"
            value={repoSearch}
            onChange={(e) => setRepoSearch(e.target.value)}
            placeholder="Search connected repositories by name or owner..."
            className="w-full sm:w-80 rounded-xl bg-slate-900/80 border border-white/10 px-3.5 py-2 text-xs text-white placeholder:text-slate-500 focus:outline-none focus:border-sky-400"
          />
        </div>

        {/* Repositories Grid */}
        <div className="space-y-3">
          {reposLoading && (
            <div className="glass-panel rounded-2xl p-12 text-center text-slate-500 text-xs animate-pulse font-mono">
              Loading connected repositories…
            </div>
          )}

          {!reposLoading && (!filteredRepos || filteredRepos.length === 0) && (
            <div className="glass-panel rounded-2xl p-12 text-center border-dashed border-white/10 space-y-4">
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
              <button
                onClick={() => setShowImportModal(true)}
                className="rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold px-4 py-2 text-xs shadow-lg shadow-blue-600/30 transition-all cursor-pointer inline-flex items-center gap-1.5"
              >
                <span>+ Import First Repository</span>
              </button>
            </div>
          )}

          {!reposLoading && filteredRepos && filteredRepos.length > 0 && (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {filteredRepos.map((r) => {
                const isTriggering = triggeringId === r.ID
                const locales = r.settings?.Locales || []

                return (
                  <div
                    key={r.ID}
                    onClick={() => router.push(`/repo?id=${r.ID}`)}
                    className="glass-panel rounded-2xl p-5 border border-white/10 hover:border-sky-500/40 hover:bg-white/[0.03] transition-all cursor-pointer space-y-4 group flex flex-col justify-between"
                  >
                    <div className="space-y-3">
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
                        <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 border border-white/5">
                          {r.DefaultBranch}
                        </span>
                      </div>

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
                            <span key={loc} className="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-900 text-sky-400 border border-white/5 uppercase">
                              {loc}
                            </span>
                          ))}
                          {locales.length > 5 && (
                            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-900 text-slate-400 border border-white/5">
                              +{locales.length - 5}
                            </span>
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
                            <span className="text-[10px] font-mono text-amber-400 bg-amber-500/10 px-1.5 py-0.5 rounded">
                              Repo Key
                            </span>
                          ) : (
                            <span className="text-[10px] font-mono text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded">
                              Global Key
                            </span>
                          )}
                        </div>
                      )}
                    </div>

                    {/* Card Footer Actions */}
                    <div className="flex items-center justify-between pt-3 border-t border-white/[0.06] gap-2">
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          router.push(`/repo?id=${r.ID}&tab=settings`)
                        }}
                        className="rounded-lg bg-white/5 hover:bg-white/10 text-slate-300 text-xs px-3 py-1.5 transition-colors flex items-center gap-1.5"
                      >
                        <span>Strategy</span>
                      </button>

                      <button
                        onClick={(e) => triggerJob(r, e)}
                        disabled={isTriggering}
                        className="rounded-lg bg-blue-600/80 hover:bg-blue-600 disabled:bg-blue-900 text-white text-xs font-semibold px-3 py-1.5 shadow-md shadow-blue-600/20 transition-all flex items-center gap-1.5"
                      >
                        <span>{isTriggering ? 'Running…' : '▶ Run'}</span>
                      </button>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </section>

      {/* ─── Modal: Global AI Keys Vault ──────────────────────────────────────── */}
      {showVaultModal && (
        <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex items-center justify-center p-4">
          <div className="glass-panel bg-[#090d16] border border-white/10 rounded-2xl w-full max-w-lg shadow-2xl p-6 space-y-5">
            <div className="flex items-center justify-between border-b border-white/[0.08] pb-4">
              <div>
                <h3 className="font-bold text-base text-white">Global AI Keys Vault</h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  Keys saved here apply to all connected repositories across your account automatically.
                </p>
              </div>
              <button
                onClick={() => {
                  setShowVaultModal(false)
                  setVaultFeedback('')
                  setVaultKeyInput('')
                }}
                className="text-slate-500 hover:text-slate-300 text-base cursor-pointer"
              >
                ✕
              </button>
            </div>

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
                      className={`p-2 rounded-xl text-xs font-semibold text-center border transition-all cursor-pointer ${
                        vaultProvider === k
                          ? 'border-sky-500 bg-sky-500/15 text-white'
                          : 'border-white/[0.06] bg-slate-900/50 text-slate-400 hover:text-slate-200'
                      }`}
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
                <input
                  type="password"
                  value={vaultKeyInput}
                  onChange={(e) => setVaultKeyInput(e.target.value)}
                  placeholder={
                    isProviderConfigured(vaultProvider)
                      ? '•••••••••••••••••••• (Enter new key to update)'
                      : `Enter ${PROVIDER_MODELS[vaultProvider]?.placeholder || 'API Key'}`
                  }
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2.5 text-xs text-white placeholder:text-slate-600 focus:outline-none focus:border-sky-400 font-mono"
                />
                <p className="text-[10px] text-slate-500 mt-1">
                  Stored AES-256-GCM encrypted in SQLite WAL vault. Never exposed in responses.
                </p>
              </div>

              <div className="flex items-center justify-between pt-3 border-t border-white/[0.08]">
                <button
                  type="button"
                  onClick={() => {
                    setShowVaultModal(false)
                    setVaultFeedback('')
                    setVaultKeyInput('')
                  }}
                  className="rounded-xl bg-white/5 hover:bg-white/10 text-slate-400 text-xs px-4 py-2"
                >
                  Cancel
                </button>

                <button
                  type="button"
                  onClick={saveGlobalVaultKey}
                  disabled={savingVaultKey}
                  className="rounded-xl bg-blue-600 hover:bg-blue-500 disabled:bg-blue-900 text-white font-semibold text-xs px-5 py-2 shadow-lg shadow-blue-600/30 transition-all cursor-pointer"
                >
                  {savingVaultKey ? 'Saving to Vault…' : 'Save Global Key'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ─── Modal: Connect & Import Repositories ─────────────────────────────── */}
      {showImportModal && (
        <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex items-center justify-center p-4">
          <div className="glass-panel bg-[#090d16] border border-white/10 rounded-2xl w-full max-w-xl shadow-2xl p-6 space-y-5">
            <div className="flex items-center justify-between border-b border-white/[0.08] pb-4">
              <div>
                <h3 className="font-bold text-base text-white">Import from GitHub App</h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  Select any repository granted to your installed GitHub App.
                </p>
              </div>
              <button
                onClick={() => setShowImportModal(false)}
                className="text-slate-500 hover:text-slate-300 text-base cursor-pointer"
              >
                ✕
              </button>
            </div>

            {loadingAvailable ? (
              <div className="p-8 text-center text-xs text-slate-400 animate-pulse font-mono">
                Fetching accessible repositories from GitHub App installations…
              </div>
            ) : !availableRepos || availableRepos.length === 0 ? (
              <div className="p-8 text-center text-xs text-slate-300 space-y-4">
                <div className="w-12 h-12 rounded-2xl bg-sky-500/10 border border-sky-500/20 text-sky-400 flex items-center justify-center mx-auto text-xl font-bold font-mono">
                  KEY
                </div>
                <div className="space-y-1">
                  <p className="font-bold text-white text-sm">No Repositories Found</p>
                  <p className="text-slate-400 text-xs max-w-md mx-auto">
                    To grant access to repositories, install the <strong className="text-slate-200">langPeanut Localization Bot</strong> on your personal GitHub account or organization.
                  </p>
                </div>
                <div className="flex items-center justify-center gap-3 pt-2">
                  <a
                    href="https://github.com/settings/apps"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold px-4 py-2 text-xs shadow-lg shadow-blue-600/30 transition-all flex items-center gap-2"
                  >
                    <span>Install GitHub App ↗</span>
                  </a>
                  <button
                    onClick={() => mutateAvailable()}
                    className="rounded-xl bg-white/10 hover:bg-white/15 text-white font-medium px-4 py-2 text-xs transition-all cursor-pointer"
                  >
                    Refresh List
                  </button>
                </div>
              </div>
            ) : (
              <div className="max-h-80 overflow-y-auto space-y-2 pr-1">
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
                          {r.owner}/{r.name}
                        </p>
                        <p className="text-[11px] text-slate-500 font-mono">
                          branch: {r.default_branch} • {r.private ? 'Private' : 'Public'}
                        </p>
                      </div>

                      <button
                        disabled={r.is_imported || isImporting}
                        onClick={() => importRepo(r)}
                        className={`rounded-lg px-3.5 py-1.5 text-xs font-semibold transition-all ${
                          r.is_imported
                            ? 'bg-slate-800 text-slate-500 cursor-default'
                            : isImporting
                            ? 'bg-blue-900 text-blue-200 cursor-wait'
                            : 'bg-blue-600 hover:bg-blue-500 text-white cursor-pointer shadow-md shadow-blue-600/30'
                        }`}
                      >
                        {r.is_imported ? '✓ Imported' : isImporting ? 'Importing…' : 'Import'}
                      </button>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
