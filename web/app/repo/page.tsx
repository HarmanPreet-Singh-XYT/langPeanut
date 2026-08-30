'use client'

import useSWR from 'swr'
import { useState, useEffect, Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'

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
  CustomPrompt?: string
  GlossaryTerms?: string[]
  KeyConvention?: string
  CustomInstallCmd?: string
  CustomBuildCmd?: string
  RootDir?: string
  ExistingTranslationsMode?: string
  existing_translations_mode?: string
  has_api_key_override?: boolean
}

interface ProviderCredential {
  provider: string
  configured: boolean
}

interface Job {
  ID: number
  RepoID: number
  TriggerType: string
  Status: string
  Branch: string
  HeadCommitSHA: string
  PRURL: string
  ErrorMessage: string
  CreatedAt: string
  StartedAt: string | null
  FinishedAt: string | null
}

const AVAILABLE_LANGUAGES = [
  { code: 'es', label: 'Spanish', native: 'Español', tag: 'ES', region: 'eu' },
  { code: 'fr', label: 'French', native: 'Français', tag: 'FR', region: 'eu' },
  { code: 'de', label: 'German', native: 'Deutsch', tag: 'DE', region: 'eu' },
  { code: 'ja', label: 'Japanese', native: '日本語', tag: 'JA', region: 'apac' },
  { code: 'zh', label: 'Chinese (Simplified)', native: '简体中文', tag: 'ZH', region: 'apac' },
  { code: 'zh-TW', label: 'Chinese (Traditional)', native: '繁體中文', tag: 'TW', region: 'apac' },
  { code: 'ko', label: 'Korean', native: '한국어', tag: 'KO', region: 'apac' },
  { code: 'pt', label: 'Portuguese (PT)', native: 'Português', tag: 'PT', region: 'eu' },
  { code: 'pt-BR', label: 'Portuguese (BR)', native: 'Português do Brasil', tag: 'BR', region: 'americas' },
  { code: 'it', label: 'Italian', native: 'Italiano', tag: 'IT', region: 'eu' },
  { code: 'nl', label: 'Dutch', native: 'Nederlands', tag: 'NL', region: 'eu' },
  { code: 'ru', label: 'Russian', native: 'Русский', tag: 'RU', region: 'eu' },
  { code: 'ar', label: 'Arabic', native: 'العربية', tag: 'AR', region: 'me' },
  { code: 'hi', label: 'Hindi', native: 'हिन्दी', tag: 'HI', region: 'apac' },
  { code: 'tr', label: 'Turkish', native: 'Türkçe', tag: 'TR', region: 'eu' },
  { code: 'pl', label: 'Polish', native: 'Polski', tag: 'PL', region: 'eu' },
  { code: 'sv', label: 'Swedish', native: 'Svenska', tag: 'SV', region: 'nordics' },
  { code: 'da', label: 'Danish', native: 'Dansk', tag: 'DA', region: 'nordics' },
  { code: 'fi', label: 'Finnish', native: 'Suomi', tag: 'FI', region: 'nordics' },
  { code: 'no', label: 'Norwegian', native: 'Norsk', tag: 'NO', region: 'nordics' },
  { code: 'uk', label: 'Ukrainian', native: 'Українська', tag: 'UK', region: 'eu' },
  { code: 'vi', label: 'Vietnamese', native: 'Tiếng Việt', tag: 'VI', region: 'apac' },
  { code: 'th', label: 'Thai', native: 'ไทย', tag: 'TH', region: 'apac' },
  { code: 'id', label: 'Indonesian', native: 'Bahasa Indonesia', tag: 'ID', region: 'apac' },
  { code: 'ms', label: 'Malay', native: 'Bahasa Melayu', tag: 'MS', region: 'apac' },
  { code: 'fil', label: 'Filipino', native: 'Filipino', tag: 'PH', region: 'apac' },
  { code: 'he', label: 'Hebrew', native: 'עברית', tag: 'HE', region: 'me' },
  { code: 'el', label: 'Greek', native: 'Ελληνικά', tag: 'EL', region: 'eu' },
  { code: 'cs', label: 'Czech', native: 'Čeština', tag: 'CS', region: 'eu' },
  { code: 'ro', label: 'Romanian', native: 'Română', tag: 'RO', region: 'eu' },
  { code: 'hu', label: 'Hungarian', native: 'Magyar', tag: 'HU', region: 'eu' },
]

const PROVIDER_MODELS: Record<string, { label: string; tag: string; models: string[] }> = {
  openai: {
    label: 'OpenAI',
    tag: 'OAI',
    models: ['gpt-4o-mini', 'gpt-4o', 'o3-mini', 'gpt-4-turbo'],
  },
  claude: {
    label: 'Anthropic Claude',
    tag: 'CLD',
    models: ['claude-3-7-sonnet-20250219', 'claude-3-5-haiku-20241022'],
  },
  gemini: {
    label: 'Google Gemini',
    tag: 'GEM',
    models: ['gemini-3.5-flash', 'gemini-2.5-pro'],
  },
  deepl: {
    label: 'DeepL Translate',
    tag: 'DPL',
    models: ['deepl-default'],
  },
  custom: {
    label: 'Custom / Local Ollama',
    tag: 'LOC',
    models: ['qwen2.5:32b', 'llama3.3:70b', 'deepseek-r1:32b', 'mistral-large'],
  },
}

const TONE_PRESETS = [
  { id: 'neutral', name: 'Neutral & Standard', desc: 'Accurate, natural terminology suitable for software apps & websites.' },
  { id: 'casual', name: 'Friendly & Casual', desc: 'Warm, engaging, colloquial tone ideal for consumer apps and social tools.' },
  { id: 'corporate', name: 'Formal & Enterprise', desc: 'Authoritative, precise, enterprise-grade business terminology.' },
  { id: 'genz', name: 'Gen-Z & Playful', desc: 'Slang, expressive vibes, modern idioms for gaming and youth audiences.' },
  { id: 'pirate', name: 'Pirate & RPG Theme', desc: 'Playful nautical phrasing for games, easter eggs, and seasonal campaigns.' },
  { id: 'technical', name: 'Developer & Technical', desc: 'Maintains exact engineering idioms, API terms, and CLI syntax.' },
]

const INITIAL_SAMPLE_MATRIX = [
  { key: 'header.welcome', en: 'Welcome back, {userName}!', es: '¡Bienvenido de nuevo, {userName}!', fr: 'Bon retour, {userName}!', de: 'Willkommen zurück, {userName}!', ja: 'おかえりなさい、{userName}さん！' },
  { key: 'checkout.completeBtn', en: 'Complete Purchase (${total})', es: 'Completar Compra (${total})', fr: 'Finaliser l’achat (${total})', de: 'Kauf abschließen (${total})', ja: '購入を完了する (${total})' },
  { key: 'billing.upgradePlan', en: 'Upgrade to Pro Plan', es: 'Mejorar al Plan Pro', fr: 'Passer au forfait Pro', de: 'Auf Pro-Plan upgraden', ja: 'Proプランにアップグレード' },
  { key: 'messages.unreadCount', en: 'You have {count} unread messages.', es: 'Tienes {count} mensajes no leídos.', fr: 'Vous avez {count} messages non lus.', de: 'Sie haben {count} ungelesene Nachrichten.', ja: '{count}件の未読メッセージがあります。' },
  { key: 'settings.saveChanges', en: 'Save changes', es: 'Guardar cambios', fr: 'Enregistrer les modifications', de: 'Änderungen speichern', ja: '変更を保存' },
]

const STATUS_BADGES: Record<string, { label: string; bg: string; border: string; text: string }> = {
  pending: { label: 'Pending', bg: 'bg-yellow-500/10', border: 'border-yellow-500/30', text: 'text-yellow-400' },
  running: { label: 'In Progress', bg: 'bg-blue-500/10', border: 'border-blue-500/30', text: 'text-blue-400' },
  succeeded: { label: 'Succeeded', bg: 'bg-emerald-500/10', border: 'border-emerald-500/30', text: 'text-emerald-400' },
  needs_review: { label: 'Needs Review', bg: 'bg-amber-500/10', border: 'border-amber-500/30', text: 'text-amber-400' },
  failed: { label: 'Failed', bg: 'bg-rose-500/10', border: 'border-rose-500/30', text: 'text-rose-400' },
  skipped_no_changes: { label: 'Up to Date', bg: 'bg-slate-500/10', border: 'border-slate-500/30', text: 'text-slate-400' },
}

function RepoDetailsContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const repoIdParam = searchParams.get('id')
  const initialTab = (searchParams.get('tab') as 'overview' | 'settings' | 'matrix' | 'runs' | 'bot') || 'overview'

  const [activeTab, setActiveTab] = useState<'overview' | 'settings' | 'matrix' | 'runs' | 'bot'>(initialTab)
  const [authed, setAuthed] = useState(false)
  const [authChecked, setAuthChecked] = useState(false)

  // Auth check
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

  // Sync tab with URL
  const setTab = (t: 'overview' | 'settings' | 'matrix' | 'runs' | 'bot') => {
    setActiveTab(t)
    const newUrl = `/repo?id=${repoIdParam}&tab=${t}`
    window.history.replaceState(null, '', newUrl)
  }

  // Load Repos & Credentials
  const { data: repos, isLoading: reposLoading, mutate: mutateRepos } = useSWR<Repo[]>(
    authed ? '/api/repos' : null,
    fetcher
  )
  const { data: credentials, mutate: mutateCreds } = useSWR<ProviderCredential[]>(
    authed ? '/api/credentials' : null,
    fetcher
  )

  const repo = repos?.find((r) => String(r.ID) === repoIdParam) || null

  // Job History
  const { data: jobsData, mutate: mutateJobs } = useSWR<Job[]>(
    authed && repo ? `/api/repos/${repo.ID}/jobs` : null,
    fetcher,
    { refreshInterval: 4000 }
  )

  // Settings State
  const [selectedLocales, setSelectedLocales] = useState<string[]>(['es', 'fr', 'de'])
  const [localeSearch, setLocaleSearch] = useState<string>('')
  const [customLocaleInput, setCustomLocaleInput] = useState<string>('')
  const [selectedTone, setSelectedTone] = useState<string>('neutral')
  const [selectedProvider, setSelectedProvider] = useState<string>('openai')
  const [selectedModel, setSelectedModel] = useState<string>('gpt-4o-mini')
  const [customPrompt, setCustomPrompt] = useState<string>('')
  const [customInstallCmd, setCustomInstallCmd] = useState<string>('')
  const [customBuildCmd, setCustomBuildCmd] = useState<string>('')
  const [rootDirInput, setRootDirInput] = useState<string>('')
  const [existingMode, setExistingMode] = useState<'skip' | 'replace' | 'prompt'>('skip')
  const [chunkWordBudget, setChunkWordBudget] = useState<number>(50000)
  const [chunkKeyCeiling, setChunkKeyCeiling] = useState<number>(1500)
  const [glossaryInput, setGlossaryInput] = useState<string>('langPeanut, Superwall, Workspace')
  const [keyConvention, setKeyConvention] = useState<string>('camelCase')
  const [userDirective, setUserDirective] = useState<string>('')
  const [apiKeyInput, setApiKeyInput] = useState<string>('')
  const [savingSettings, setSavingSettings] = useState(false)
  const [settingsFeedback, setSettingsFeedback] = useState<string>('')

  // Translation Matrix State (Real from DB / Repo)
  const { data: rawMatrix, mutate: mutateMatrix } = useSWR<Record<string, Record<string, string>>>(
    authed && repo ? `/api/repos/${repo.ID}/matrix` : null,
    fetcher
  )
  const [matrixSearch, setMatrixSearch] = useState('')
  const [editingCell, setEditingCell] = useState<{ rowKey: string; colKey: string } | null>(null)
  const [cellValue, setCellValue] = useState('')
  const [savingCell, setSavingCell] = useState(false)

  // AI Translation Copilot State
  const [copilotState, setCopilotState] = useState<{
    isOpen: boolean
    key: string
    sourceLocale: string
    sourceText: string
    targetLocale: string
    currentTranslation: string
    instruction: string
    loading: boolean
    result: {
      translated_text: string
      explanation: string
      icu_variables_ok: boolean
      length_reduction?: string
    } | null
    error: string | null
  }>({
    isOpen: false,
    key: '',
    sourceLocale: 'en',
    sourceText: '',
    targetLocale: '',
    currentTranslation: '',
    instruction: 'shorter',
    loading: false,
    result: null,
    error: null,
  })

  // Trigger & Live Real Runner Terminal State
  const [selectedJobId, setSelectedJobId] = useState<number | null>(null)
  const activeJobId = selectedJobId || (jobsData && jobsData.length > 0 ? jobsData[0].ID : null)
  const { data: rawJobLogs } = useSWR<any>(
    authed && repo && activeJobId ? `/api/repos/${repo.ID}/jobs/${activeJobId}/logs` : null,
    fetcher,
    { refreshInterval: jobsData?.some((j) => j.Status === 'running' || j.Status === 'pending') ? 2000 : 0 }
  )

  const [triggering, setTriggering] = useState(false)
  const [toastMsg, setToastMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [copiedConfig, setCopiedConfig] = useState(false)
  const [copiedYAML, setCopiedYAML] = useState(false)

  // Autonomous Agentic Feature States
  const [discoveringPersona, setDiscoveringPersona] = useState(false)
  const [runningDoctor, setRunningDoctor] = useState(false)
  const [doctorReport, setDoctorReport] = useState<any>(null)
  const [pruningKeys, setPruningKeys] = useState(false)

  // Target Branch Selector State & Remote Branches SWR
  const [selectedBranch, setSelectedBranch] = useState<string>('')
  const [isBranchDropdownOpen, setIsBranchDropdownOpen] = useState(false)
  const [customBranchInput, setCustomBranchInput] = useState('')
  const { data: branchesData } = useSWR<
    Array<{ name: string; is_default: boolean; protected: boolean }>
  >(
    authed && repo ? `/api/repos/${repo.ID}/branches` : null,
    fetcher
  )

  // Populate settings when repo is loaded
  useEffect(() => {
    if (repo?.settings) {
      setSelectedLocales(repo.settings.Locales || ['es', 'fr', 'de'])
      setSelectedTone(repo.settings.TonePreset || 'neutral')
      setSelectedProvider(repo.settings.Provider || 'openai')
      setSelectedModel(repo.settings.Model || 'gpt-4o-mini')
      setCustomPrompt(repo.settings.CustomPrompt || '')
      setCustomInstallCmd(repo.settings.CustomInstallCmd || '')
      setCustomBuildCmd(repo.settings.CustomBuildCmd || '')
      setRootDirInput(repo.settings.RootDir || '')
      setUserDirective(
        ((repo.settings as any).user_directive ||
          (repo.settings as any).UserDirective ||
          '') as string
      )
      const isFrontier = ['openai', 'claude', 'gemini'].includes(repo.settings.Provider || 'openai')
      setChunkWordBudget(repo.settings.ChunkWordBudget || (isFrontier ? 50000 : 4000))
      setChunkKeyCeiling(repo.settings.ChunkKeyCeiling || (isFrontier ? 1500 : 100))
      setExistingMode(
        ((repo.settings as any).existing_translations_mode ||
          (repo.settings as any).ExistingTranslationsMode ||
          'skip') as 'skip' | 'replace' | 'prompt'
      )
      setGlossaryInput(repo.settings.GlossaryTerms?.join(', ') || 'langPeanut, Superwall, Workspace')
      setKeyConvention(repo.settings.KeyConvention || 'camelCase')
    }
  }, [repo])

  function showToast(text: string, type: 'success' | 'error' = 'success') {
    setToastMsg({ text, type })
    setTimeout(() => setToastMsg(null), 5000)
  }

  function isProviderConfigured(p: string): boolean {
    return credentials?.some((c) => c.provider === p && c.configured) ?? false
  }

  async function saveSettings() {
    if (!repo) return
    if (selectedLocales.length === 0) {
      setSettingsFeedback('Please select at least one target language.')
      return
    }

    setSavingSettings(true)
    setSettingsFeedback('')

    try {
      const payload: any = {
        locales: selectedLocales,
        tone_preset: selectedTone,
        provider: selectedProvider,
        model: selectedModel,
        safety_mode: true,
        chunk_word_budget: chunkWordBudget,
        chunk_key_ceiling: chunkKeyCeiling,
        custom_install_cmd: customInstallCmd.trim(),
        custom_build_cmd: customBuildCmd.trim(),
        root_dir: rootDirInput.trim(),
        existing_translations_mode: existingMode,
        user_directive: userDirective.trim(),
      }
      if (apiKeyInput.trim()) {
        payload.api_key_override = apiKeyInput.trim()
      }

      // 1. Save Repo Settings
      const settingsRes = await fetch(`/api/repos/${repo.ID}/settings`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      if (!settingsRes.ok) {
        const err = await settingsRes.json()
        throw new Error(err.error || 'Failed to save settings')
      }

      mutateRepos()
      setApiKeyInput('')
      showToast(`Settings & Localization strategy saved for ${repo.Owner}/${repo.Name}`)
    } catch (e: unknown) {
      setSettingsFeedback(e instanceof Error ? e.message : 'Error saving settings')
    } finally {
      setSavingSettings(false)
    }
  }

  async function clearRepoKeyOverride() {
    if (!repo) return
    setSavingSettings(true)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/settings`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          locales: selectedLocales,
          tone_preset: selectedTone,
          provider: selectedProvider,
          model: selectedModel,
          safety_mode: true,
          chunk_word_budget: chunkWordBudget,
          chunk_key_ceiling: chunkKeyCeiling,
          custom_install_cmd: customInstallCmd.trim(),
          custom_build_cmd: customBuildCmd.trim(),
          root_dir: rootDirInput.trim(),
          existing_translations_mode: existingMode,
          user_directive: userDirective.trim(),
          api_key_override: '__CLEAR__',
        }),
      })
      if (res.ok) {
        mutateRepos()
        setApiKeyInput('')
        showToast(`Reverted ${repo.Owner}/${repo.Name} to Global Vault Key`)
      }
    } catch (e: unknown) {
      showToast(e instanceof Error ? e.message : 'Failed to clear override', 'error')
    } finally {
      setSavingSettings(false)
    }
  }

  async function triggerJob() {
    if (!repo) return
    setTriggering(true)
    try {
      const targetBranch = selectedBranch || repo.DefaultBranch || 'main'
      const res = await fetch(`/api/repos/${repo.ID}/jobs`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          branch: targetBranch,
          user_directive: userDirective.trim(),
        }),
      })
      const body = await res.json()
      if (res.ok) {
        showToast(`Job #${body.ID} queued for ${repo.Owner}/${repo.Name} on branch '${targetBranch}'`)
        setSelectedJobId(body.ID)
        mutateJobs()
        mutateMatrix()
        setTab('runs')
      } else {
        showToast(body.error || 'Failed to trigger job', 'error')
      }
    } catch (e: unknown) {
      showToast(e instanceof Error ? e.message : 'Network error', 'error')
    } finally {
      setTriggering(false)
    }
  }

  async function saveMatrixCell(rowKey: string, colKey: string, customVal?: string) {
    if (!repo) return
    setSavingCell(true)
    try {
      const val = customVal !== undefined ? customVal : cellValue
      const res = await fetch(`/api/repos/${repo.ID}/matrix`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          locale: colKey,
          key: rowKey,
          value: val,
        }),
      })
      if (res.ok) {
        mutateMatrix()
        setEditingCell(null)
        showToast(`Saved [${colKey}] "${rowKey}"`)
      }
    } catch {
      showToast('Failed to save translation cell', 'error')
    } finally {
      setSavingCell(false)
    }
  }

  function openCopilot(key: string, targetLocale: string, currentVal: string) {
    const srcText = rawMatrix?.['en']?.[key] || key
    setCopilotState({
      isOpen: true,
      key,
      sourceLocale: 'en',
      sourceText: srcText,
      targetLocale,
      currentTranslation: currentVal,
      instruction: 'shorter',
      loading: false,
      result: null,
      error: null,
    })
  }

  async function generateWithCopilot(customInstruction?: string) {
    if (!repo) return
    const instr = customInstruction !== undefined ? customInstruction : copilotState.instruction
    setCopilotState((prev) => ({ ...prev, loading: true, error: null, instruction: instr }))
    try {
      const res = await fetch(`/api/repos/${repo.ID}/matrix/copilot`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          key: copilotState.key,
          source_locale: copilotState.sourceLocale,
          source_text: copilotState.sourceText,
          target_locale: copilotState.targetLocale,
          current_translation: copilotState.currentTranslation,
          instruction: instr,
          apply_directly: false,
        }),
      })
      const data = await res.json()
      if (res.ok) {
        setCopilotState((prev) => ({
          ...prev,
          loading: false,
          result: data,
          error: null,
        }))
      } else {
        setCopilotState((prev) => ({
          ...prev,
          loading: false,
          error: data.error || 'AI generation failed',
        }))
      }
    } catch (e: any) {
      setCopilotState((prev) => ({
        ...prev,
        loading: false,
        error: e?.message || 'Network error during AI Copilot call',
      }))
    }
  }

  async function applyCopilotResult() {
    if (!repo || !copilotState.result) return
    const key = copilotState.key
    const loc = copilotState.targetLocale
    const val = copilotState.result.translated_text

    await saveMatrixCell(key, loc, val)
    setCopilotState((prev) => ({ ...prev, isOpen: false }))
  }

  async function discoverPersona() {
    if (!repo) return
    setDiscoveringPersona(true)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/discover-persona`, {
        method: 'POST',
        credentials: 'include',
      })
      const data = await res.json()
      if (res.ok) {
        if (data.recommended_tone) {
          setSelectedTone(data.recommended_tone)
        }
        if (data.brand_lexicon && data.brand_lexicon.length > 0) {
          setGlossaryInput(data.brand_lexicon.join(', '))
        }
        if (data.locales_suggested && data.locales_suggested.length > 0) {
          setSelectedLocales(Array.from(new Set([...selectedLocales, ...data.locales_suggested])))
        }
        showToast(`✨ Persona discovered: Tone '${data.recommended_tone}', ${data.brand_lexicon?.length || 0} brand terms locked`)
      } else {
        showToast(data.error || 'Failed to discover persona', 'error')
      }
    } catch {
      showToast('Network error during persona discovery', 'error')
    } finally {
      setDiscoveringPersona(false)
    }
  }

  async function runDoctorCheck() {
    if (!repo) return
    setRunningDoctor(true)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/doctor`, {
        credentials: 'include',
      })
      const data = await res.json()
      if (res.ok) {
        setDoctorReport(data)
        showToast(`🩺 Doctor audit: Health score ${data.health_score}/100 (${data.status})`)
      } else {
        showToast(data.error || 'Failed to run doctor audit', 'error')
      }
    } catch {
      showToast('Network error during doctor audit', 'error')
    } finally {
      setRunningDoctor(false)
    }
  }

  async function pruneDeadKeys() {
    if (!repo) return
    setPruningKeys(true)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/prune-keys`, {
        method: 'POST',
        credentials: 'include',
      })
      const data = await res.json()
      if (res.ok) {
        mutateMatrix()
        if (data.total_dead_keys > 0) {
          showToast(`🧹 Pruned ${data.total_dead_keys} stale keys across ${data.pruned_locales?.join(', ') || 'locale files'}`)
        } else {
          showToast('✓ 100% Clean! No orphaned translation keys found.')
        }
      } else {
        showToast(data.error || 'Failed to prune dead keys', 'error')
      }
    } catch {
      showToast('Network error during dead key pruning', 'error')
    } finally {
      setPruningKeys(false)
    }
  }

  function exportRepoConfig() {
    const configData = {
      $schema: 'https://langpeanut.ai/schema.json',
      locales: selectedLocales,
      provider: selectedProvider,
      model: selectedModel,
      tone: selectedTone,
      key_convention: keyConvention,
      root_dir: rootDirInput.trim() || undefined,
      existing_translations_mode: existingMode,
      chunk_word_budget: chunkWordBudget,
      chunk_key_ceiling: chunkKeyCeiling,
      custom_prompt: customPrompt || undefined,
      custom_install_cmd: customInstallCmd || undefined,
      custom_build_cmd: customBuildCmd || undefined,
      glossary: glossaryInput
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
    }
    navigator.clipboard.writeText(JSON.stringify(configData, null, 2))
    setCopiedConfig(true)
    setTimeout(() => setCopiedConfig(false), 3000)
    showToast('.langpeanut.json config copied to clipboard!')
  }

  function copyGitHubActionsYAML() {
    const yaml = `name: langPeanut i18n Autopilot
on:
  push:
    branches: [${repo?.DefaultBranch || 'main'}]
  pull_request:
    types: [opened, synchronize]

jobs:
  localize:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Run langPeanut AST Localization Scout
        uses: langPeanut/action@v2
        with:
          github_token: \${{ secrets.GITHUB_TOKEN }}
          locales: '${selectedLocales.join(',')}'
          openai_api_key: \${{ secrets.OPENAI_API_KEY }}
`
    navigator.clipboard.writeText(yaml)
    setCopiedYAML(true)
    setTimeout(() => setCopiedYAML(false), 3000)
    showToast('GitHub Actions YAML workflow copied to clipboard!')
  }

  if (!authChecked || reposLoading) {
    return (
      <div className="py-24 text-center text-slate-500 text-xs font-mono animate-pulse">
        Loading repository environment…
      </div>
    )
  }

  if (!repo) {
    return (
      <div className="max-w-4xl mx-auto py-16 text-center space-y-4">
        <div className="w-12 h-12 rounded-2xl bg-rose-500/10 border border-rose-500/20 text-rose-400 flex items-center justify-center mx-auto text-lg font-bold">
          !
        </div>
        <h2 className="text-lg font-bold text-white">Repository Not Found</h2>
        <p className="text-xs text-slate-400">
          This repository may not have been imported yet or you do not have permission to view it.
        </p>
        <a
          href="/dashboard"
          className="inline-block rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold px-4 py-2"
        >
          Return to Console
        </a>
      </div>
    )
  }

  const latestJob = jobsData && jobsData.length > 0 ? jobsData[0] : null
  const latestStatus = latestJob ? STATUS_BADGES[latestJob.Status] : null

  return (
    <div className="space-y-6 max-w-7xl mx-auto pb-16">
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

      {/* Top Breadcrumb & Repository Header */}
      <div className="border-b border-white/[0.08] pb-5 space-y-4">
        <div className="flex items-center gap-2 text-xs text-slate-400">
          <a href="/dashboard" className="hover:text-white transition-colors">
            Repositories
          </a>
          <span>/</span>
          <span className="text-slate-200 font-semibold">{repo.Owner}</span>
          <span>/</span>
          <span className="text-sky-400 font-bold">{repo.Name}</span>
        </div>

        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 rounded-2xl bg-slate-900 border border-white/10 flex items-center justify-center text-sky-400 shadow-xl">
              <svg className="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4" />
                <path d="M9 18c-4.51 2-5-2-7-2" />
              </svg>
            </div>
            <div>
              <div className="flex items-center gap-2.5">
                <h1 className="text-2xl font-extrabold text-white tracking-tight">
                  {repo.Owner} / {repo.Name}
                </h1>
                
                {/* Interactive Target Branch Selector */}
                <div className="relative inline-block">
                  <button
                    type="button"
                    onClick={() => setIsBranchDropdownOpen(!isBranchDropdownOpen)}
                    className="text-[11px] font-mono px-2.5 py-0.5 rounded-full bg-slate-800 hover:bg-slate-700 border border-white/15 text-sky-300 flex items-center gap-1.5 cursor-pointer shadow-sm transition-all"
                    title="Click to switch target branch"
                  >
                    <svg className="w-3 h-3 text-sky-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                      <line x1="6" y1="3" x2="6" y2="15" />
                      <circle cx="18" cy="6" r="3" />
                      <circle cx="6" cy="18" r="3" />
                      <path d="M18 9a9 9 0 0 1-9 9" />
                    </svg>
                    <span>branch: <strong className="text-white">{selectedBranch || repo.DefaultBranch || 'main'}</strong></span>
                    <svg className="w-2.5 h-2.5 text-slate-400 ml-0.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="6 9 12 15 18 9" />
                    </svg>
                  </button>

                  {isBranchDropdownOpen && (
                    <div className="absolute left-0 mt-2 w-72 rounded-2xl bg-slate-950 border border-white/15 shadow-2xl p-3 z-50 space-y-2.5 backdrop-blur-xl">
                      <div className="flex items-center justify-between px-1">
                        <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">
                          Select Target Branch
                        </span>
                        <span className="text-[10px] text-sky-400 font-mono">
                          {branchesData?.length || 1} available
                        </span>
                      </div>

                      {/* Custom branch input */}
                      <div>
                        <input
                          type="text"
                          value={customBranchInput}
                          onChange={(e) => setCustomBranchInput(e.target.value)}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter' && customBranchInput.trim()) {
                              setSelectedBranch(customBranchInput.trim())
                              setIsBranchDropdownOpen(false)
                              setCustomBranchInput('')
                              showToast(`Target branch set to: ${customBranchInput.trim()}`)
                            }
                          }}
                          placeholder="Type branch name & press Enter…"
                          className="w-full rounded-xl bg-slate-900 border border-white/10 px-3 py-1.5 text-xs text-white placeholder:text-slate-600 focus:outline-none focus:border-sky-400 font-mono"
                        />
                      </div>

                      <div className="max-h-52 overflow-y-auto space-y-1">
                        {branchesData && branchesData.length > 0 ? (
                          branchesData.map((b) => {
                            const isCur = (selectedBranch || repo.DefaultBranch || 'main') === b.name
                            return (
                              <button
                                key={b.name}
                                type="button"
                                onClick={() => {
                                  setSelectedBranch(b.name)
                                  setIsBranchDropdownOpen(false)
                                  showToast(`Target branch set to: ${b.name}`)
                                }}
                                className={`w-full text-left px-3 py-2 rounded-xl text-xs font-mono flex items-center justify-between cursor-pointer transition-colors ${
                                  isCur
                                    ? 'bg-sky-500/15 text-sky-300 font-bold border border-sky-500/30'
                                    : 'text-slate-300 hover:bg-white/5 hover:text-white'
                                }`}
                              >
                                <div className="flex items-center gap-2 truncate">
                                  <svg className="w-3 h-3 text-slate-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                    <line x1="6" y1="3" x2="6" y2="15" />
                                    <circle cx="18" cy="6" r="3" />
                                    <circle cx="6" cy="18" r="3" />
                                    <path d="M18 9a9 9 0 0 1-9 9" />
                                  </svg>
                                  <span className="truncate">{b.name}</span>
                                </div>
                                <div className="flex items-center gap-1.5 shrink-0">
                                  {b.is_default && (
                                    <span className="text-[9px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 border border-white/5">
                                      default
                                    </span>
                                  )}
                                  {isCur && <span className="text-sky-400 font-bold text-xs">✓</span>}
                                </div>
                              </button>
                            )
                          })
                        ) : (
                          <button
                            type="button"
                            onClick={() => {
                              setSelectedBranch(repo.DefaultBranch || 'main')
                              setIsBranchDropdownOpen(false)
                            }}
                            className="w-full text-left px-3 py-2 rounded-xl text-xs font-mono text-sky-300 bg-sky-500/15 cursor-pointer"
                          >
                            {repo.DefaultBranch || 'main'} (default)
                          </button>
                        )}
                      </div>
                    </div>
                  )}
                </div>

                {latestStatus && (
                  <span className={`text-[11px] font-mono px-2 py-0.5 rounded-full border ${latestStatus.bg} ${latestStatus.border} ${latestStatus.text}`}>
                    {latestStatus.label}
                  </span>
                )}
              </div>
              <p className="text-xs text-slate-400 mt-1">
                Universal Multi-Agent Localization Pipeline • Connected to GitHub App
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2.5">
            <a
              href={`https://github.com/${repo.Owner}/${repo.Name}`}
              target="_blank"
              rel="noopener noreferrer"
              className="rounded-xl bg-white/[0.04] hover:bg-white/[0.08] border border-white/10 text-slate-300 font-medium px-3.5 py-2 text-xs transition-all flex items-center gap-1.5"
            >
              <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
              </svg>
              <span>GitHub</span>
            </a>

            <button
              onClick={triggerJob}
              disabled={triggering}
              className="rounded-xl bg-blue-600 hover:bg-blue-500 disabled:bg-blue-900 text-white font-semibold px-4 py-2 text-xs shadow-lg shadow-blue-600/30 transition-all cursor-pointer flex items-center gap-2"
            >
              <svg className={`w-3.5 h-3.5 ${triggering ? 'animate-spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <polygon points="5 3 19 12 5 21 5 3" />
              </svg>
              <span>{triggering ? 'Starting Pipeline…' : 'Run Localization'}</span>
            </button>
          </div>
        </div>

        {/* Tab Navigation */}
        <div className="flex items-center gap-1.5 pt-2 border-t border-white/[0.05]">
          {[
            { id: 'overview', label: 'Overview' },
            { id: 'settings', label: 'Settings & Strategy' },
            { id: 'matrix', label: 'Translation Matrix' },
            { id: 'runs', label: 'Runs & Logs' },
            { id: 'bot', label: 'PR Bot & Webhooks' },
          ].map((t) => (
            <button
              key={t.id}
              onClick={() => setTab(t.id as any)}
              className={`px-4 py-2 rounded-xl text-xs font-semibold transition-all flex items-center gap-2 cursor-pointer ${
                activeTab === t.id
                  ? 'bg-blue-600/15 border border-blue-500/30 text-sky-300 shadow-md shadow-sky-950/50'
                  : 'text-slate-400 hover:text-white hover:bg-white/[0.03]'
              }`}
            >
              <span>{t.label}</span>
            </button>
          ))}
        </div>
      </div>

      {/* ─── TAB 1: OVERVIEW ─────────────────────────────────────────────────── */}
      {activeTab === 'overview' && (
        <div className="space-y-6">
          {/* Quick Metrics */}
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
            <div className="glass-panel p-4 rounded-2xl space-y-1">
              <span className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">Target Locales</span>
              <p className="text-2xl font-bold text-white">{selectedLocales.length} Languages</p>
              <div className="flex flex-wrap gap-1 pt-1">
                {selectedLocales.map((c) => (
                  <span key={c} className="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-800 text-sky-300 border border-white/5 uppercase">
                    {c}
                  </span>
                ))}
              </div>
            </div>

            <div className="glass-panel p-4 rounded-2xl space-y-1">
              <span className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">Active AI Model</span>
              <p className="text-2xl font-bold text-white capitalize">{selectedProvider}</p>
              <p className="text-xs font-mono text-sky-400">{selectedModel}</p>
            </div>

            <div className="glass-panel p-4 rounded-2xl space-y-1">
              <span className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">Tone Preset</span>
              <p className="text-2xl font-bold text-white capitalize">{selectedTone}</p>
              <p className="text-xs text-slate-400">ICU variable & plural safe</p>
            </div>

            <div className="glass-panel p-4 rounded-2xl space-y-1">
              <span className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">Total Runs</span>
              <p className="text-2xl font-bold text-white">{jobsData?.length || 0} Jobs</p>
              <p className="text-xs text-emerald-400 font-mono">Autopilot Webhooks: Active</p>
            </div>
          </div>

          {/* Quick Trigger Cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
            <div className="glass-panel p-6 rounded-2xl space-y-4">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 flex items-center justify-center font-bold font-mono text-xs">
                  CI
                </div>
                <div>
                  <h3 className="text-sm font-bold text-white">Continuous Push Autopilot</h3>
                  <p className="text-xs text-slate-400">Every commit to {repo.DefaultBranch} triggers AST Scout and opens a clean PR.</p>
                </div>
              </div>
              <div className="rounded-xl bg-slate-900/80 border border-white/10 p-3 text-xs font-mono text-slate-300">
                Webhook: <span className="text-sky-400">POST /api/webhook</span> (Events: push, installation)
              </div>
            </div>

            <div className="glass-panel p-6 rounded-2xl space-y-4">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-sky-500/10 border border-sky-500/20 text-sky-400 flex items-center justify-center font-bold font-mono text-xs">
                  PR
                </div>
                <div>
                  <h3 className="text-sm font-bold text-white">Interactive PR Bot Mentions</h3>
                  <p className="text-xs text-slate-400">Comment on any pull request to localize diffs on demand.</p>
                </div>
              </div>
              <div className="rounded-xl bg-slate-900/80 border border-white/10 p-3 text-xs font-mono text-emerald-400">
                @langpeanut translate --locales es,fr --tone formal
              </div>
            </div>
          </div>

          {/* Autonomous Diagnostic Health Doctor Panel */}
          <div className="glass-panel p-6 rounded-2xl space-y-4 border border-sky-500/20 bg-sky-950/10">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-sky-500/20 border border-sky-500/30 text-sky-400 flex items-center justify-center font-bold text-base">
                  🩺
                </div>
                <div>
                  <h3 className="text-sm font-bold text-white flex items-center gap-2">
                    i18n Readiness & Framework Doctor
                    {doctorReport && (
                      <span className={`text-[10px] font-mono px-2 py-0.5 rounded-full font-bold border ${
                        doctorReport.health_score >= 80
                          ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
                          : doctorReport.health_score >= 50
                          ? 'bg-amber-500/10 text-amber-400 border-amber-500/30'
                          : 'bg-rose-500/10 text-rose-400 border-rose-500/30'
                      }`}>
                        {doctorReport.health_score}/100 {doctorReport.status}
                      </span>
                    )}
                  </h3>
                  <p className="text-xs text-slate-400">
                    Autonomous 360° health audit of framework dependencies, locale directories, and hardcoded UI literals.
                  </p>
                </div>
              </div>

              <button
                type="button"
                onClick={runDoctorCheck}
                disabled={runningDoctor}
                className="rounded-xl bg-sky-600 hover:bg-sky-500 disabled:bg-sky-900 text-white text-xs font-semibold px-4 py-2 cursor-pointer shadow-lg shadow-sky-600/30 flex items-center gap-1.5 shrink-0"
              >
                {runningDoctor ? 'Analyzing Codebase…' : 'Run Health Check 🩺'}
              </button>
            </div>

            {doctorReport && (
              <div className="pt-3 border-t border-white/10 space-y-3">
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
                  <div className="p-2.5 rounded-xl bg-slate-900/80 border border-white/5 space-y-0.5">
                    <span className="text-[10px] uppercase font-bold text-slate-400">Framework</span>
                    <p className="text-white font-semibold">{doctorReport.framework_display || doctorReport.framework}</p>
                  </div>
                  <div className="p-2.5 rounded-xl bg-slate-900/80 border border-white/5 space-y-0.5">
                    <span className="text-[10px] uppercase font-bold text-slate-400">Untranslated Strings</span>
                    <p className="text-amber-400 font-semibold font-mono">~{doctorReport.hardcoded_strings_estimated} literals</p>
                  </div>
                  <div className="p-2.5 rounded-xl bg-slate-900/80 border border-white/5 space-y-0.5">
                    <span className="text-[10px] uppercase font-bold text-slate-400">Configured Dictionaries</span>
                    <p className="text-sky-400 font-semibold font-mono">[{doctorReport.configured_locales?.join(', ') || 'none'}]</p>
                  </div>
                  <div className="p-2.5 rounded-xl bg-slate-900/80 border border-white/5 space-y-0.5">
                    <span className="text-[10px] uppercase font-bold text-slate-400">Auto-Fixable Issues</span>
                    <p className="text-emerald-400 font-semibold">{doctorReport.auto_fixable_count} of {doctorReport.issues?.length || 0}</p>
                  </div>
                </div>

                {doctorReport.issues && doctorReport.issues.length > 0 && (
                  <div className="space-y-1.5 pt-1">
                    {doctorReport.issues.map((iss: any, idx: number) => (
                      <div key={idx} className="p-2.5 rounded-xl bg-slate-900/60 border border-white/5 flex items-start gap-2.5 text-xs">
                        <span className="text-sm">
                          {iss.severity === 'ERROR' ? '❌' : iss.severity === 'WARNING' ? '⚠️' : 'ℹ️'}
                        </span>
                        <div className="flex-1 min-w-0">
                          <div className="font-semibold text-white">{iss.title}</div>
                          <div className="text-slate-400 text-[11px] mt-0.5">{iss.description}</div>
                          {iss.auto_fix_hint && (
                            <div className="text-sky-300 text-[11px] mt-1 font-mono">💡 {iss.auto_fix_hint}</div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {/* ─── TAB 2: SETTINGS & STRATEGY (FULL PAGE, NO DIALOG) ───────────────── */}
      {activeTab === 'settings' && (
        <div className="space-y-8">
          {/* Header */}
          <div className="flex items-center justify-between border-b border-white/[0.08] pb-4">
            <div>
              <h2 className="text-base font-bold text-white">Localization Strategy & Architecture</h2>
              <p className="text-xs text-slate-400 mt-0.5">
                Configure languages, tone presets, LLM provider keys, monorepo paths, and compilation rules.
              </p>
            </div>
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={exportRepoConfig}
                className="rounded-xl bg-white/[0.05] hover:bg-white/[0.1] border border-white/10 text-slate-300 text-xs font-medium px-3.5 py-2 transition-all cursor-pointer"
              >
                {copiedConfig ? '✓ Config Copied' : 'Export .langpeanut.json'}
              </button>
              <button
                type="button"
                onClick={saveSettings}
                disabled={savingSettings}
                className="rounded-xl bg-blue-600 hover:bg-blue-500 disabled:bg-blue-900 text-white text-xs font-semibold px-5 py-2 shadow-lg shadow-blue-600/30 transition-all cursor-pointer"
              >
                {savingSettings ? 'Saving Strategy…' : 'Save Changes'}
              </button>
            </div>
          </div>

          {settingsFeedback && (
            <div className="rounded-xl bg-rose-950/60 border border-rose-800 text-rose-200 px-4 py-3 text-xs">
              {settingsFeedback}
            </div>
          )}

          {/* Section 1: Target Languages */}
          <div className="glass-panel p-6 rounded-2xl space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <h3 className="text-sm font-bold text-white flex items-center gap-2">
                  <span>1. Target Languages</span>
                  <span className="text-xs font-mono text-sky-400 bg-sky-500/10 px-2 py-0.5 rounded-full border border-sky-500/20">
                    {selectedLocales.length} selected
                  </span>
                </h3>
                <p className="text-xs text-slate-400">Select all target languages to synthesize with ICU parity.</p>
              </div>

              <input
                type="text"
                value={localeSearch}
                onChange={(e) => setLocaleSearch(e.target.value)}
                placeholder="Search languages by name, code or region…"
                className="w-full sm:w-64 rounded-xl bg-slate-900/90 border border-white/10 px-3.5 py-1.5 text-xs text-white placeholder:text-slate-500 focus:outline-none focus:border-sky-400"
              />
            </div>

            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-2 max-h-56 overflow-y-auto pr-1">
              {AVAILABLE_LANGUAGES.filter(
                (l) =>
                  l.label.toLowerCase().includes(localeSearch.toLowerCase()) ||
                  l.native.toLowerCase().includes(localeSearch.toLowerCase()) ||
                  l.code.toLowerCase().includes(localeSearch.toLowerCase())
              ).map((l) => {
                const isSelected = selectedLocales.includes(l.code)
                return (
                  <button
                    key={l.code}
                    type="button"
                    onClick={() => {
                      if (isSelected) {
                        setSelectedLocales(selectedLocales.filter((c) => c !== l.code))
                      } else {
                        setSelectedLocales([...selectedLocales, l.code])
                      }
                    }}
                    className={`rounded-xl border p-2.5 text-left transition-all cursor-pointer flex items-center justify-between gap-2 ${
                      isSelected
                        ? 'border-sky-500 bg-sky-500/15 text-white shadow-md shadow-sky-950/40'
                        : 'border-white/[0.06] bg-slate-900/40 text-slate-400 hover:border-white/20 hover:text-slate-200'
                    }`}
                  >
                    <div className="truncate">
                      <span className="font-mono font-bold text-[10px] text-slate-400 mr-1.5">[{l.tag}]</span>
                      <span className="font-semibold text-xs text-slate-200">{l.label}</span>
                      <p className="text-[10px] text-slate-500 font-mono">{l.code}</p>
                    </div>
                    {isSelected && <span className="text-sky-400 font-bold text-xs">✓</span>}
                  </button>
                )
              })}
            </div>

            {/* Custom locale input */}
            <div className="flex items-center gap-2 pt-2 border-t border-white/[0.05]">
              <input
                type="text"
                value={customLocaleInput}
                onChange={(e) => setCustomLocaleInput(e.target.value)}
                placeholder="Add custom BCP-47 locale (e.g. pt-PT, zh-Hant, es-MX)..."
                className="w-72 rounded-xl bg-slate-900/80 border border-white/10 px-3.5 py-1.5 text-xs text-white placeholder:text-slate-500"
              />
              <button
                type="button"
                onClick={() => {
                  const clean = customLocaleInput.trim().toLowerCase()
                  if (clean && !selectedLocales.includes(clean)) {
                    setSelectedLocales([...selectedLocales, clean])
                    setCustomLocaleInput('')
                  }
                }}
                className="rounded-xl bg-white/10 hover:bg-white/15 text-white text-xs font-semibold px-3 py-1.5 cursor-pointer"
              >
                + Add Code
              </button>
            </div>
          </div>

          {/* Section 2: Tone & Brand Persona */}
          <div className="glass-panel p-6 rounded-2xl space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <h3 className="text-sm font-bold text-white">2. Cultural Tone & Brand Persona</h3>
                <p className="text-xs text-slate-400">Sets the translation style memory and vocabulary constraints.</p>
              </div>
              <button
                type="button"
                onClick={discoverPersona}
                disabled={discoveringPersona}
                className="rounded-xl bg-purple-600/20 hover:bg-purple-600/30 border border-purple-500/40 text-purple-300 text-xs font-semibold px-3.5 py-1.5 flex items-center gap-1.5 cursor-pointer transition-all shrink-0"
              >
                {discoveringPersona ? 'Mining Assets…' : '✨ Auto-Discover Persona & Tone'}
              </button>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {TONE_PRESETS.map((t) => {
                const isSelected = selectedTone === t.id
                return (
                  <button
                    key={t.id}
                    type="button"
                    onClick={() => setSelectedTone(t.id)}
                    className={`rounded-2xl border p-4 text-left transition-all cursor-pointer space-y-1 ${
                      isSelected
                        ? 'border-sky-500 bg-sky-500/10 text-white shadow-lg shadow-sky-950/40'
                        : 'border-white/[0.06] bg-slate-900/40 text-slate-400 hover:border-white/20'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-bold text-xs text-slate-200">{t.name}</span>
                      {isSelected && <span className="text-sky-400 font-bold text-xs">✓ Active</span>}
                    </div>
                    <p className="text-[11px] text-slate-400 leading-relaxed">{t.desc}</p>
                  </button>
                )
              })}
            </div>
          </div>

          {/* Section 3: AI Provider, Model & BYO Credentials */}
          <div className="glass-panel p-6 rounded-2xl space-y-5">
            <div>
              <h3 className="text-sm font-bold text-white">3. AI Provider & Model Configuration</h3>
              <p className="text-xs text-slate-400">Select model family and configure execution credentials.</p>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="text-[11px] font-semibold text-slate-300 block mb-1">Provider</label>
                <select
                  value={selectedProvider}
                  onChange={(e) => {
                    const p = e.target.value
                    setSelectedProvider(p)
                    setSelectedModel(PROVIDER_MODELS[p]?.models[0] || '')
                  }}
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white focus:outline-none focus:border-sky-400"
                >
                  {Object.entries(PROVIDER_MODELS).map(([k, p]) => (
                    <option key={k} value={k}>
                      {p.label}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="text-[11px] font-semibold text-slate-300 block mb-1">Model</label>
                <select
                  value={selectedModel}
                  onChange={(e) => setSelectedModel(e.target.value)}
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white focus:outline-none focus:border-sky-400"
                >
                  {PROVIDER_MODELS[selectedProvider]?.models.map((m) => (
                    <option key={m} value={m}>
                      {m}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            {/* Global Key Status Banner & Repo Override */}
            <div className="rounded-xl border border-white/[0.08] bg-slate-900/60 p-4 space-y-3">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-white/[0.06] pb-3">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-bold text-white">Global Team Key Status</span>
                    {isProviderConfigured(selectedProvider) ? (
                      <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                        Active in Global Vault
                      </span>
                    ) : (
                      <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-rose-500/10 text-rose-400 border border-rose-500/20">
                        No Global Key Configured
                      </span>
                    )}
                  </div>
                  <p className="text-[11px] text-slate-400 mt-0.5">
                    By default, this repository inherits your account's Global Vault key for {PROVIDER_MODELS[selectedProvider]?.label || selectedProvider}.
                  </p>
                </div>
                {!isProviderConfigured(selectedProvider) && (
                  <a
                    href="/dashboard"
                    className="text-xs font-semibold text-sky-400 hover:text-sky-300 transition-colors cursor-pointer shrink-0"
                  >
                    Configure in Global Vault ↗
                  </a>
                )}
              </div>

              {/* Repo-Specific Override (Optional) */}
              <div className="space-y-2 pt-1">
                <div className="flex items-center justify-between">
                  <label className="text-[11px] font-semibold text-slate-300 flex items-center gap-2">
                    <span>Optional Per-Repo Key Override</span>
                    {repo.settings?.has_api_key_override && (
                      <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20 font-bold">
                        Repo Override Active
                      </span>
                    )}
                  </label>

                  {repo.settings?.has_api_key_override && (
                    <button
                      type="button"
                      onClick={clearRepoKeyOverride}
                      disabled={savingSettings}
                      className="text-[10px] text-rose-400 hover:text-rose-300 font-semibold cursor-pointer"
                    >
                      Revert to Global Key ✕
                    </button>
                  )}
                </div>

                <input
                  type="password"
                  value={apiKeyInput}
                  onChange={(e) => setApiKeyInput(e.target.value)}
                  placeholder={
                    repo.settings?.has_api_key_override
                      ? '•••••••••••••••• (Repo-specific key active — enter new to change)'
                      : 'Leave empty to use Global Key, or enter repo-specific key (sk-...)'
                  }
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white placeholder:text-slate-600 focus:outline-none focus:border-sky-400 font-mono"
                />
                <p className="text-[10px] text-slate-500">
                  Only enter a key here if you need to isolate billing or use a dedicated client sub-account for this repository.
                </p>
              </div>
            </div>
          </div>

          {/* Section 4: Advanced Engine Knobs & Monorepo */}
          <div className="glass-panel p-6 rounded-2xl space-y-5">
            <div>
              <h3 className="text-sm font-bold text-white">4. Monorepo, Build Toolchain & Extraction Guard</h3>
              <p className="text-xs text-slate-400">Settings for monorepos, subdirectories, and compilation repair agents.</p>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="text-[11px] font-semibold text-slate-300 block mb-1">
                  Monorepo Subdirectory (Relative Root)
                </label>
                <input
                  type="text"
                  value={rootDirInput}
                  onChange={(e) => setRootDirInput(e.target.value)}
                  placeholder="e.g. apps/web, packages/mobile, frontend (empty for root)"
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white font-mono placeholder:text-slate-600"
                />
              </div>

              <div>
                <label className="text-[11px] font-semibold text-slate-300 block mb-1">
                  Existing Translations Handling
                </label>
                <select
                  value={existingMode}
                  onChange={(e) => setExistingMode(e.target.value as any)}
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white"
                >
                  <option value="skip">Skip Already Translated Keys (Preserves Manual Edits)</option>
                  <option value="replace">Overwrite & Force-Resynthesize All Keys</option>
                  <option value="prompt">Prompt on Key Collisions</option>
                </select>
              </div>

              <div>
                <label className="text-[11px] font-semibold text-slate-300 block mb-1">
                  Custom Install Command (Tier-5 Repair Agent)
                </label>
                <input
                  type="text"
                  value={customInstallCmd}
                  onChange={(e) => setCustomInstallCmd(e.target.value)}
                  placeholder="e.g. pnpm install, npm ci, flutter pub get"
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white font-mono placeholder:text-slate-600"
                />
              </div>

              <div>
                <label className="text-[11px] font-semibold text-slate-300 block mb-1">
                  Custom Build / Typecheck Command
                </label>
                <input
                  type="text"
                  value={customBuildCmd}
                  onChange={(e) => setCustomBuildCmd(e.target.value)}
                  placeholder="e.g. pnpm typecheck, tsc --noEmit, flutter analyze"
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white font-mono placeholder:text-slate-600"
                />
              </div>

              <div className="sm:col-span-2">
                <div className="flex items-center justify-between mb-1">
                  <label className="text-[11px] font-semibold text-slate-300 block">
                    Brand Glossary & Do-Not-Translate Lexicon
                  </label>
                  <button
                    type="button"
                    onClick={discoverPersona}
                    disabled={discoveringPersona}
                    className="text-[11px] text-purple-400 hover:text-purple-300 font-semibold cursor-pointer"
                  >
                    {discoveringPersona ? 'Mining…' : '✨ Auto-Detect Terms'}
                  </button>
                </div>
                <input
                  type="text"
                  value={glossaryInput}
                  onChange={(e) => setGlossaryInput(e.target.value)}
                  placeholder="Comma separated brand tokens: langPeanut, Superwall, Workspace"
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white placeholder:text-slate-600"
                />
              </div>
            </div>

            {/* ── Section 5: UI Integration Directive (UI Switcher Agent) ── */}
            <div className="pt-5 border-t border-white/[0.08] space-y-3">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-xs font-bold text-white uppercase tracking-wider flex items-center gap-1.5">
                    <span className="text-sky-400">Section 5</span> — UI Integration Directive (UI Switcher Agent)
                  </h3>
                  <p className="text-[11px] text-slate-400 mt-0.5">
                    Autonomous post-localization UI generation. Instruct the agent to build and auto-link a language switcher component into your UI layout.
                  </p>
                </div>
                <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-sky-500/10 border border-sky-500/30 text-sky-400 font-semibold">
                  DirectiveAgent
                </span>
              </div>

              {/* Quick Directive Preset Chips */}
              <div className="flex flex-wrap gap-2 pt-1">
                {[
                  { label: 'Navbar / Header Switcher', prompt: 'Add a language switcher dropdown in Navbar / Header' },
                  { label: 'Settings Screen Picker', prompt: 'Add a language picker screen and preference selector in Settings' },
                  { label: 'Floating Toggle Widget', prompt: 'Add a floating language toggle button in bottom-right corner' },
                  { label: 'Clear Directive', prompt: '' },
                ].map((preset) => (
                  <button
                    key={preset.label}
                    type="button"
                    onClick={() => setUserDirective(preset.prompt)}
                    className={`text-[11px] px-2.5 py-1 rounded-lg border font-medium transition-all ${
                      userDirective === preset.prompt
                        ? 'bg-sky-500/20 border-sky-500 text-sky-300 font-bold'
                        : 'bg-white/[0.03] border-white/10 text-slate-400 hover:text-white hover:bg-white/[0.06]'
                    }`}
                  >
                    {preset.label}
                  </button>
                ))}
              </div>

              <div>
                <textarea
                  value={userDirective}
                  onChange={(e) => setUserDirective(e.target.value)}
                  rows={2}
                  placeholder="e.g. Add a language switcher dropdown in the navbar right corner next to user profile and persist choice in localStorage"
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white placeholder:text-slate-600 focus:border-sky-400 focus:outline-none"
                />
                <p className="text-[10px] text-slate-500 mt-1">
                  Leave empty if you only want strings and locale JSON/ARB files translated without modifying UI component files.
                </p>
              </div>
            </div>

            <div className="flex justify-end pt-4 border-t border-white/[0.05]">
              <button
                type="button"
                onClick={saveSettings}
                disabled={savingSettings}
                className="rounded-xl bg-blue-600 hover:bg-blue-500 disabled:bg-blue-900 text-white text-xs font-semibold px-6 py-2.5 shadow-lg shadow-blue-600/30 transition-all cursor-pointer"
              >
                {savingSettings ? 'Saving Settings…' : 'Save Localization Strategy'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ─── TAB 3: TRANSLATION MATRIX & SPREADSHEET ─────────────────────────── */}
      {activeTab === 'matrix' && (
        <div className="space-y-5">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-white/[0.08] pb-4">
            <div>
              <h2 className="text-base font-bold text-white">Live Translation Matrix</h2>
              <p className="text-xs text-slate-400 mt-0.5">
                Collaborative multi-lingual matrix. Click any cell to edit translations inline and save to Git.
              </p>
            </div>

            <div className="flex items-center gap-3">
              <input
                type="text"
                value={matrixSearch}
                onChange={(e) => setMatrixSearch(e.target.value)}
                placeholder="Search keys or translations..."
                className="w-64 rounded-xl bg-slate-900 border border-white/10 px-3.5 py-1.5 text-xs text-white placeholder:text-slate-500"
              />
              <button
                type="button"
                onClick={pruneDeadKeys}
                disabled={pruningKeys}
                className="rounded-xl bg-amber-500/10 hover:bg-amber-500/20 border border-amber-500/30 text-amber-300 text-xs font-medium px-3.5 py-1.5 cursor-pointer flex items-center gap-1.5 transition-all"
              >
                {pruningKeys ? 'Pruning…' : '🧹 Prune Dead Keys'}
              </button>
              <button
                type="button"
                onClick={copyGitHubActionsYAML}
                className="rounded-xl bg-white/[0.05] hover:bg-white/[0.1] border border-white/10 text-slate-300 text-xs font-medium px-3.5 py-1.5 cursor-pointer"
              >
                {copiedYAML ? '✓ Copied' : 'Export Actions YAML'}
              </button>
            </div>
          </div>

          <div className="glass-panel rounded-2xl overflow-hidden border border-white/10">
            {(() => {
              const activeLocales = rawMatrix && Object.keys(rawMatrix).length > 0 ? Object.keys(rawMatrix) : selectedLocales
              const allKeys = rawMatrix ? Array.from(new Set(Object.values(rawMatrix).flatMap((m) => Object.keys(m || {})))).sort() : []

              if (allKeys.length === 0) {
                return (
                  <div className="p-12 text-center space-y-3">
                    <div className="text-xs text-slate-400 font-mono">
                      No translation keys extracted in the cloud matrix yet.
                    </div>
                    <p className="text-[11px] text-slate-500 max-w-md mx-auto">
                      Run your first localization job to extract strings from your codebase and populate this collaborative multi-lingual matrix.
                    </p>
                    <button
                      type="button"
                      onClick={triggerJob}
                      disabled={triggering}
                      className="rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold px-4 py-2"
                    >
                      + Run Localization Pipeline
                    </button>
                  </div>
                )
              }

              const filteredKeys = allKeys.filter((k) => {
                if (!matrixSearch) return true
                const s = matrixSearch.toLowerCase()
                if (k.toLowerCase().includes(s)) return true
                return activeLocales.some((loc) => rawMatrix?.[loc]?.[k]?.toLowerCase().includes(s))
              })

              return (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs border-collapse">
                    <thead>
                      <tr className="bg-slate-900/90 border-b border-white/10 text-slate-400 font-semibold uppercase text-[10px] tracking-wider">
                        <th className="p-3.5 w-48">Key Identifier</th>
                        {activeLocales.map((loc) => (
                          <th key={loc} className="p-3.5 w-64 uppercase">
                            {loc} {loc === 'en' ? '(Source)' : ''}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-white/[0.06]">
                      {filteredKeys.map((keyStr) => (
                        <tr key={keyStr} className="hover:bg-white/[0.02] transition-colors">
                          <td className="p-3.5 font-mono text-[11px] text-sky-400 font-semibold">{keyStr}</td>
                          {activeLocales.map((loc) => {
                            const isEditing = editingCell?.rowKey === keyStr && editingCell?.colKey === loc
                            const val = rawMatrix?.[loc]?.[keyStr] || ''

                            return (
                              <td key={loc} className="p-3.5 group relative">
                                {isEditing ? (
                                  <div className="flex items-center gap-1.5">
                                    <input
                                      type="text"
                                      value={cellValue}
                                      onChange={(e) => setCellValue(e.target.value)}
                                      autoFocus
                                      className="w-full rounded bg-slate-900 border border-sky-400 px-2 py-1 text-xs text-white"
                                    />
                                    <button
                                      type="button"
                                      onClick={() => saveMatrixCell(keyStr, loc)}
                                      disabled={savingCell}
                                      className="bg-blue-600 hover:bg-blue-500 text-white px-2 py-1 rounded text-[10px] font-bold"
                                    >
                                      Save
                                    </button>
                                    <button
                                      type="button"
                                      onClick={() => openCopilot(keyStr, loc, cellValue || val)}
                                      title="Open AI Copilot"
                                      className="bg-purple-600/80 hover:bg-purple-500 text-white px-2 py-1 rounded text-[10px] font-bold flex items-center gap-1 cursor-pointer"
                                    >
                                      ✨
                                    </button>
                                  </div>
                                ) : (
                                  <div className="flex items-center justify-between gap-2">
                                    <div
                                      onClick={() => {
                                        setEditingCell({ rowKey: keyStr, colKey: loc })
                                        setCellValue(val)
                                      }}
                                      className="cursor-pointer hover:text-sky-300 text-slate-300 p-1 rounded hover:bg-white/5 transition-all flex-1 min-w-0 truncate"
                                    >
                                      {val || <span className="text-slate-600 italic text-[11px]">untranslated</span>}
                                    </div>
                                    <button
                                      type="button"
                                      onClick={() => openCopilot(keyStr, loc, val)}
                                      title="AI Translation Copilot"
                                      className="opacity-0 group-hover:opacity-100 transition-opacity p-1 rounded hover:bg-purple-500/20 text-purple-400 hover:text-purple-300 text-[10px] font-mono shrink-0 flex items-center gap-0.5 border border-purple-500/30 cursor-pointer"
                                    >
                                      ✨ AI
                                    </button>
                                  </div>
                                )}
                              </td>
                            )
                          })}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )
            })()}
          </div>

          {/* ─── AI TRANSLATION COPILOT MODAL (HUMAN CHECKPOINT) ───────────────── */}
          {copilotState.isOpen && (
            <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4">
              <div className="glass-panel w-full max-w-xl rounded-2xl border border-purple-500/30 bg-slate-950 p-6 shadow-2xl space-y-5 animate-in fade-in zoom-in-95 duration-150">
                {/* Header */}
                <div className="flex items-center justify-between border-b border-white/10 pb-3.5">
                  <div className="flex items-center gap-2.5">
                    <div className="w-8 h-8 rounded-xl bg-purple-500/20 border border-purple-500/40 text-purple-300 flex items-center justify-center font-bold text-sm shadow-inner">
                      ✨
                    </div>
                    <div>
                      <h3 className="text-sm font-bold text-white flex items-center gap-2">
                        AI Translation Copilot
                        <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-purple-500/20 text-purple-300 border border-purple-500/30">
                          Human Checkpoint
                        </span>
                      </h3>
                      <p className="text-[11px] text-slate-400 font-mono">
                        Key: <span className="text-sky-300 font-semibold">{copilotState.key}</span> • Target: <span className="uppercase text-purple-300 font-bold">[{copilotState.targetLocale}]</span>
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => setCopilotState((prev) => ({ ...prev, isOpen: false }))}
                    className="text-slate-400 hover:text-white p-1 text-lg font-bold cursor-pointer"
                  >
                    ✕
                  </button>
                </div>

                {/* Source and Current comparison */}
                <div className="grid grid-cols-2 gap-3 text-xs">
                  <div className="p-3 rounded-xl bg-white/[0.03] border border-white/10 space-y-1">
                    <div className="text-[10px] uppercase font-bold text-slate-400">English (Source)</div>
                    <div className="text-slate-200 font-medium">{copilotState.sourceText || '—'}</div>
                  </div>
                  <div className="p-3 rounded-xl bg-white/[0.03] border border-white/10 space-y-1">
                    <div className="text-[10px] uppercase font-bold text-slate-400">Current [{copilotState.targetLocale}]</div>
                    <div className="text-slate-300 italic">{copilotState.currentTranslation || '(untranslated)'}</div>
                  </div>
                </div>

                {/* Directive / Quick Actions */}
                <div className="space-y-2">
                  <label className="text-[11px] font-bold text-slate-300 block">
                    Agent Directive / Optimization Goal
                  </label>
                  <div className="flex flex-wrap gap-2">
                    {[
                      { id: 'shorter', label: '⚡ Make Shorter (-30%)', hint: 'Compact synonym for mobile buttons' },
                      { id: 'casual', label: '😊 Casual & Friendly', hint: 'Warm colloquial phrasing' },
                      { id: 'formal', label: '👔 Formal & Enterprise', hint: 'B2B professional phrasing' },
                      { id: 'brand_safe', label: '🛡️ Brand Safe', hint: 'Keep technical names untouched' },
                    ].map((preset) => (
                      <button
                        key={preset.id}
                        type="button"
                        onClick={() => {
                          setCopilotState((prev) => ({ ...prev, instruction: preset.id }))
                          generateWithCopilot(preset.id)
                        }}
                        className={`text-xs px-3 py-1.5 rounded-xl border font-medium transition-all cursor-pointer ${
                          copilotState.instruction === preset.id
                            ? 'bg-purple-600/30 border-purple-400 text-purple-200 font-bold shadow-md shadow-purple-900/40'
                            : 'bg-white/[0.03] border-white/10 text-slate-300 hover:text-white hover:bg-white/[0.08]'
                        }`}
                      >
                        {preset.label}
                      </button>
                    ))}
                  </div>

                  {/* Custom prompt input */}
                  <div className="pt-2">
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={
                          ['shorter', 'casual', 'formal', 'brand_safe'].includes(copilotState.instruction)
                            ? ''
                            : copilotState.instruction
                        }
                        onChange={(e) => setCopilotState((prev) => ({ ...prev, instruction: e.target.value }))}
                        placeholder="Or enter custom instruction (e.g. 'Use Latin American Spanish' or 'Sound like a gamer')..."
                        className="flex-1 rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white placeholder:text-slate-500 focus:border-purple-400 focus:outline-none"
                      />
                      <button
                        type="button"
                        onClick={() => generateWithCopilot()}
                        disabled={copilotState.loading}
                        className="rounded-xl bg-purple-600 hover:bg-purple-500 disabled:bg-purple-900 text-white text-xs font-semibold px-4 py-2 flex items-center gap-1.5 cursor-pointer shadow-lg shadow-purple-600/30"
                      >
                        {copilotState.loading ? 'Generating…' : 'Regenerate ✨'}
                      </button>
                    </div>
                  </div>
                </div>

                {/* Error state */}
                {copilotState.error && (
                  <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs font-mono">
                    {copilotState.error}
                  </div>
                )}

                {/* Result Card */}
                {copilotState.result && (
                  <div className="p-4 rounded-xl bg-purple-950/30 border border-purple-500/40 space-y-3 animate-in fade-in duration-200">
                    <div className="flex items-center justify-between">
                      <span className="text-[10px] uppercase font-bold text-purple-300 tracking-wider">
                        AI Suggested Translation
                      </span>
                      <div className="flex items-center gap-1.5">
                        {copilotState.result.icu_variables_ok && (
                          <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 font-semibold">
                            ✓ ICU Matched
                          </span>
                        )}
                        {copilotState.result.length_reduction && (
                          <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-sky-500/10 border border-sky-500/30 text-sky-400 font-semibold">
                            {copilotState.result.length_reduction}
                          </span>
                        )}
                      </div>
                    </div>

                    <div className="text-sm font-semibold text-white bg-slate-900/80 p-3 rounded-lg border border-white/10 font-mono">
                      {copilotState.result.translated_text}
                    </div>

                    {copilotState.result.explanation && (
                      <p className="text-[11px] text-slate-400 italic">
                        Decision Note: {copilotState.result.explanation}
                      </p>
                    )}
                  </div>
                )}

                {/* Modal Actions */}
                <div className="flex items-center justify-end gap-3 pt-3 border-t border-white/10">
                  <button
                    type="button"
                    onClick={() => setCopilotState((prev) => ({ ...prev, isOpen: false }))}
                    className="rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 text-xs font-semibold px-4 py-2 cursor-pointer"
                  >
                    Close
                  </button>
                  <button
                    type="button"
                    onClick={applyCopilotResult}
                    disabled={!copilotState.result || copilotState.loading}
                    className="rounded-xl bg-emerald-600 hover:bg-emerald-500 disabled:bg-slate-800 disabled:text-slate-600 text-white text-xs font-semibold px-5 py-2 shadow-lg shadow-emerald-600/30 flex items-center gap-1.5 cursor-pointer"
                  >
                    ✓ Apply & Save to Matrix
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* ─── TAB 4: RUNS & EXECUTION LOGS ────────────────────────────────────── */}
      {activeTab === 'runs' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between border-b border-white/[0.08] pb-4">
            <div>
              <h2 className="text-base font-bold text-white">Execution History & Runner Logs</h2>
              <p className="text-xs text-slate-400 mt-0.5">
                Audit trail of localization pipelines, sandbox containers, and PR branches.
              </p>
            </div>
            <button
              onClick={triggerJob}
              disabled={triggering}
              className="rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold px-4 py-2"
            >
              + Trigger New Job
            </button>
          </div>

          {/* Real Runner Terminal */}
          {(() => {
            const currentJob = jobsData?.find((j) => j.ID === activeJobId) || jobsData?.[0]
            const logsList: any[] = Array.isArray(rawJobLogs) ? rawJobLogs : []

            return (
              <div className="glass-panel rounded-2xl overflow-hidden border border-slate-700/60 shadow-2xl bg-black/90 font-mono text-xs">
                <div className="bg-slate-900 px-4 py-2.5 flex items-center justify-between border-b border-white/10">
                  <div className="flex items-center gap-2">
                    <span className="w-2.5 h-2.5 rounded-full bg-rose-500" />
                    <span className="w-2.5 h-2.5 rounded-full bg-yellow-500" />
                    <span className="w-2.5 h-2.5 rounded-full bg-emerald-500" />
                    <span className="text-[11px] text-slate-400 ml-2 font-bold">
                      Job #{currentJob?.ID || '—'} Runner Log Stream {currentJob ? `(${currentJob.Branch || 'pending'})` : ''}
                    </span>
                  </div>
                  {currentJob?.Status === 'running' && (
                    <span className="text-emerald-400 text-[10px] animate-pulse">● Live Execution Active</span>
                  )}
                  {currentJob?.Status === 'succeeded' && (
                    <span className="text-emerald-400 text-[10px]">✓ Run Completed</span>
                  )}
                  {currentJob?.Status === 'failed' && (
                    <span className="text-rose-400 text-[10px]">✗ Run Failed</span>
                  )}
                </div>

                <div className="p-4 space-y-1.5 max-h-80 overflow-y-auto text-slate-300">
                  {logsList.length === 0 ? (
                    <div className="text-slate-500 italic py-2">
                      {currentJob?.Status === 'running'
                        ? 'Job running in sandbox container... executing AST Scout, Context Disambiguation & Translation...'
                        : currentJob?.ErrorMessage
                        ? `Failure Diagnostic: ${currentJob.ErrorMessage}`
                        : 'Select a job below to view its execution trajectory.'}
                    </div>
                  ) : (
                    logsList.map((log: any, idx: number) => {
                      const timeStr = log.timestamp ? new Date(log.timestamp).toLocaleTimeString() : `[step ${idx + 1}]`
                      const agent = log.agent || log.Step || 'Agent'
                      const desc = log.description || log.Description || log.msg || JSON.stringify(log)
                      const isErr = log.status === 'Failed' || log.level === 'ERROR'
                      return (
                        <div key={idx} className="leading-relaxed flex items-start gap-2">
                          <span className="text-slate-500 shrink-0">[{timeStr}]</span>
                          <span className="text-sky-400 font-semibold shrink-0">[{agent}]</span>
                          <span className={isErr ? 'text-rose-300' : 'text-slate-200'}>{desc}</span>
                        </div>
                      )
                    })
                  )}
                  {currentJob?.ErrorMessage && (
                    <div className="pt-2 text-rose-400 font-semibold border-t border-rose-500/20">
                      Error: {currentJob.ErrorMessage}
                    </div>
                  )}
                </div>
              </div>
            )
          })()}

          {/* Jobs Table */}
          <div className="glass-panel rounded-2xl overflow-hidden border border-white/10">
            {!jobsData || jobsData.length === 0 ? (
              <div className="p-12 text-center text-xs text-slate-500 font-mono">
                No jobs executed yet. Trigger a manual run or push a commit to {repo.DefaultBranch}.
              </div>
            ) : (
              <table className="w-full text-left text-xs border-collapse">
                <thead>
                  <tr className="bg-slate-900/90 border-b border-white/10 text-slate-400 font-semibold uppercase text-[10px] tracking-wider">
                    <th className="p-3.5">Job ID</th>
                    <th className="p-3.5">Trigger</th>
                    <th className="p-3.5">Status</th>
                    <th className="p-3.5">Commit SHA</th>
                    <th className="p-3.5">Branch</th>
                    <th className="p-3.5">PR Link</th>
                    <th className="p-3.5">Created At</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-white/[0.06]">
                  {jobsData.map((job) => {
                    const badge = STATUS_BADGES[job.Status] || STATUS_BADGES.pending
                    const isSelected = (selectedJobId || jobsData[0].ID) === job.ID

                    return (
                      <tr
                        key={job.ID}
                        onClick={() => setSelectedJobId(job.ID)}
                        className={`transition-colors cursor-pointer ${
                          isSelected ? 'bg-sky-500/10 border-l-2 border-l-sky-400' : 'hover:bg-white/[0.02]'
                        }`}
                      >
                        <td className="p-3.5 font-mono font-bold text-white">#{job.ID}</td>
                        <td className="p-3.5 capitalize text-slate-300">{job.TriggerType.replace('_', ' ')}</td>
                        <td className="p-3.5">
                          <span className={`px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold border ${badge.bg} ${badge.border} ${badge.text}`}>
                            {badge.label}
                          </span>
                        </td>
                        <td className="p-3.5 font-mono text-slate-400 text-[11px]">
                          {job.HeadCommitSHA ? job.HeadCommitSHA.substring(0, 7) : '—'}
                        </td>
                        <td className="p-3.5 font-mono text-sky-400 text-[11px]">{job.Branch || '—'}</td>
                        <td className="p-3.5">
                          {job.PRURL ? (
                            <a
                              href={job.PRURL}
                              target="_blank"
                              rel="noopener noreferrer"
                              onClick={(e) => e.stopPropagation()}
                              className="text-emerald-400 hover:text-emerald-300 font-semibold flex items-center gap-1"
                            >
                              <span>View PR ↗</span>
                            </a>
                          ) : (
                            <span className="text-slate-600">—</span>
                          )}
                        </td>
                        <td className="p-3.5 text-slate-400 font-mono text-[11px]">
                          {new Date(job.CreatedAt).toLocaleString()}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}

      {/* ─── TAB 5: PR BOT & WEBHOOK TRIGGERS ────────────────────────────────── */}
      {activeTab === 'bot' && (
        <div className="space-y-6">
          <div className="border-b border-white/[0.08] pb-4">
            <h2 className="text-base font-bold text-white">GitHub PR Bot & Webhook Automation</h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Interact with @langpeanut in GitHub Pull Requests or set up repository webhooks.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
            <div className="glass-panel p-6 rounded-2xl space-y-4">
              <h3 className="text-sm font-bold text-white">PR Bot Mention Commands</h3>
              <p className="text-xs text-slate-400">
                Mention @langpeanut in any PR comment. The cloud worker will parse the command flags, checkout the PR branch, and commit localized AST files.
              </p>

              <div className="space-y-2 text-xs font-mono">
                <div className="p-3 rounded-xl bg-slate-900 border border-white/10 text-emerald-400">
                  @langpeanut translate --locales es,fr,de --tone formal
                </div>
                <div className="p-3 rounded-xl bg-slate-900 border border-white/10 text-sky-400">
                  @langpeanut review
                </div>
                <div className="p-3 rounded-xl bg-slate-900 border border-white/10 text-slate-300">
                  @langpeanut audit
                </div>
              </div>
            </div>

            <div className="glass-panel p-6 rounded-2xl space-y-4">
              <h3 className="text-sm font-bold text-white">GitHub Webhook Configuration</h3>
              <p className="text-xs text-slate-400">
                Webhooks are automatically routed when the GitHub App is installed.
              </p>

              <div className="space-y-2 text-xs font-mono">
                <div className="p-3 rounded-xl bg-slate-900 border border-white/10 text-slate-300">
                  Payload URL: <span className="text-sky-400">/api/webhook</span>
                </div>
                <div className="p-3 rounded-xl bg-slate-900 border border-white/10 text-slate-300">
                  Content type: <span className="text-amber-400">application/json</span>
                </div>
                <div className="p-3 rounded-xl bg-slate-900 border border-white/10 text-slate-300">
                  Subscribed Events: <span className="text-emerald-400">Push, Issue comments, Installation</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default function RepoPage() {
  return (
    <Suspense fallback={<div className="py-24 text-center text-slate-500 text-xs font-mono animate-pulse">Loading repository details…</div>}>
      <RepoDetailsContent />
    </Suspense>
  )
}
