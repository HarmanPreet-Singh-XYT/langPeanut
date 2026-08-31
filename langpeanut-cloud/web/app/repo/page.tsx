'use client'

import useSWR from 'swr'
import { useState, useEffect, useRef, Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription, CardFooter } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { toast } from 'sonner'
import {
  Tool,
  PromptInput,
  PromptInputTextarea,
  PromptInputActions,
  PromptInputAction,
  PromptSuggestion,
  Reasoning,
} from '@/app/components/prompt-kit'

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
  user_directive?: string
  UserDirective?: string
  webhook_push_enabled?: boolean
  WebhookPushEnabled?: boolean
  webhook_branch_filter?: string
  WebhookBranchFilter?: string
  webhook_custom_branches?: string
  WebhookCustomBranches?: string
  webhook_action?: string
  WebhookAction?: string
  webhook_pr_comments_enabled?: boolean
  WebhookPRCommentsEnabled?: boolean
  webhook_custom_branch_prefix?: string
  WebhookCustomBranchPrefix?: string
  webhook_path_filter?: string
  WebhookPathFilter?: string
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
  { code: 'en', label: 'English', native: 'Global / US / UK', tag: 'EN', region: 'americas' },
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

export interface ModelMetadata {
  id: string
  name: string
  contextWindow: string
  maxOutput: string
  inputPrice: string
  outputPrice: string
  desc?: string
}

const PROVIDER_MODELS: Record<string, { label: string; tag: string; models: string[]; details: Record<string, ModelMetadata> }> = {
  gemini: {
    label: 'Google Gemini',
    tag: 'GEM',
    models: [
      'gemini-3.7-flash',
      'gemini-3.5-flash',
      'gemini-3.6-flash',
      'gemini-3.5-flash-lite',
      'gemini-3.1-pro-preview',
      'gemini-3.1-flash-live-preview',
      'gemini-2.5-flash',
    ],
    details: {
      'gemini-3.5-flash': { id: 'gemini-3.5-flash', name: 'Gemini 3.5 Flash', contextWindow: '1,000,000', maxOutput: '8,192', inputPrice: '$1.50', outputPrice: '$9.00', desc: 'Optimized for fast, long-horizon agentic workflows and autonomous loops' },
      'gemini-3.7-flash': { id: 'gemini-3.7-flash', name: 'Gemini 3.7 Flash', contextWindow: '1,000,000', maxOutput: '8,192', inputPrice: '$0.75', outputPrice: '$3.75', desc: 'High-intelligence workhorse model for coding and agentic tasks' },
      'gemini-3.6-flash': { id: 'gemini-3.6-flash', name: 'Gemini 3.6 Flash', contextWindow: '1,000,000', maxOutput: '8,192', inputPrice: '$0.75', outputPrice: '$3.75', desc: 'Fast multimodal agentic execution' },
      'gemini-3.5-flash-lite': { id: 'gemini-3.5-flash-lite', name: 'Gemini 3.5 Flash-Lite', contextWindow: '1,000,000', maxOutput: '8,192', inputPrice: '$0.30', outputPrice: '$2.50', desc: 'High-volume, cost-sensitive workhorse model' },
      'gemini-3.1-pro-preview': { id: 'gemini-3.1-pro-preview', name: 'Gemini 3.1 Pro', contextWindow: '1,000,000', maxOutput: '65,536', inputPrice: '$2.00', outputPrice: '$12.00', desc: 'Advanced reasoning and complex coding model' },
      'gemini-3.1-flash-live-preview': { id: 'gemini-3.1-flash-live-preview', name: 'Gemini 3.1 Flash Live', contextWindow: '1,000,000', maxOutput: '8,192', inputPrice: '$0.75', outputPrice: '$3.75', desc: 'Low-latency audio-to-audio dialogue model' },
      'gemini-2.5-flash': { id: 'gemini-2.5-flash', name: 'Gemini 2.5 Flash', contextWindow: '1,000,000', maxOutput: '8,192', inputPrice: '$0.10', outputPrice: '$0.40', desc: 'Ultra cost-efficient high-speed model' },
    },
  },
  claude: {
    label: 'Anthropic Claude',
    tag: 'CLD',
    models: [
      'claude-sonnet-5',
      'claude-fable-5',
      'claude-opus-5',
      'claude-opus-4.8',
      'claude-opus-4.7',
      'claude-opus-4.6',
      'claude-sonnet-4.6',
      'claude-sonnet-4.5',
      'claude-haiku-4.5',
    ],
    details: {
      'claude-sonnet-5': { id: 'claude-sonnet-5', name: 'Claude Sonnet 5', contextWindow: '1,000,000', maxOutput: '128,000', inputPrice: '$2.00', outputPrice: '$10.00', desc: 'Optimal balance of frontier intelligence and speed' },
      'claude-fable-5': { id: 'claude-fable-5', name: 'Claude Fable 5', contextWindow: '1,000,000', maxOutput: '128,000', inputPrice: '$10.00', outputPrice: '$50.00', desc: 'Frontier narrative and hyper-contextual reasoning' },
      'claude-opus-5': { id: 'claude-opus-5', name: 'Claude Opus 5', contextWindow: '1,000,000', maxOutput: '128,000', inputPrice: '$5.00', outputPrice: '$25.00', desc: 'Flagship intelligence for deep code refactoring' },
      'claude-opus-4.8': { id: 'claude-opus-4.8', name: 'Claude Opus 4.8', contextWindow: '1,000,000', maxOutput: '128,000', inputPrice: '$5.00', outputPrice: '$25.00', desc: 'Enterprise complex reasoning model' },
      'claude-opus-4.7': { id: 'claude-opus-4.7', name: 'Claude Opus 4.7', contextWindow: '1,000,000', maxOutput: '128,000', inputPrice: '$5.00', outputPrice: '$25.00', desc: 'Advanced code synthesis and architectural modeling' },
      'claude-opus-4.6': { id: 'claude-opus-4.6', name: 'Claude Opus 4.6', contextWindow: '1,000,000', maxOutput: '128,000', inputPrice: '$5.00', outputPrice: '$25.00', desc: 'High-precision multi-file AST transform engine' },
      'claude-sonnet-4.6': { id: 'claude-sonnet-4.6', name: 'Claude Sonnet 4.6', contextWindow: '1,000,000', maxOutput: '128,000', inputPrice: '$3.00', outputPrice: '$15.00', desc: 'High-accuracy AST localization and grammar fidelity' },
      'claude-sonnet-4.5': { id: 'claude-sonnet-4.5', name: 'Claude Sonnet 4.5', contextWindow: '200,000', maxOutput: '8,192', inputPrice: '$3.00', outputPrice: '$15.00', desc: 'Reliable production workhorse' },
      'claude-haiku-4.5': { id: 'claude-haiku-4.5', name: 'Claude Haiku 4.5', contextWindow: '200,000', maxOutput: '64,000', inputPrice: '$1.00', outputPrice: '$5.00', desc: 'Ultra-fast translation and key validation' },
    },
  },
  openai: {
    label: 'OpenAI',
    tag: 'OAI',
    models: [
      'gpt-5.4-mini',
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.6-luna',
      'gpt-5.5',
      'gpt-5.5-pro',
      'gpt-5.4',
      'gpt-5.4-pro',
    ],
    details: {
      'gpt-5.4-mini': { id: 'gpt-5.4-mini', name: 'GPT-5.4 Mini', contextWindow: '400,000', maxOutput: '128,000', inputPrice: '$0.75', outputPrice: '$4.50', desc: 'Fast, efficient 400K context model' },
      'gpt-5.6-sol': { id: 'gpt-5.6-sol', name: 'GPT-5.6 Sol (Flagship)', contextWindow: '1,050,000', maxOutput: '128,000', inputPrice: '$4.00', outputPrice: '$20.00', desc: 'Flagship generation with 1.05M context window' },
      'gpt-5.6-terra': { id: 'gpt-5.6-terra', name: 'GPT-5.6 Terra', contextWindow: '1,050,000', maxOutput: '128,000', inputPrice: '$2.00', outputPrice: '$12.00', desc: 'Balanced flagship model for large-scale codebases' },
      'gpt-5.6-luna': { id: 'gpt-5.6-luna', name: 'GPT-5.6 Luna', contextWindow: '1,050,000', maxOutput: '128,000', inputPrice: '$0.20', outputPrice: '$1.20', desc: 'High-speed, ultra-low-cost tier with 1.05M context' },
      'gpt-5.5': { id: 'gpt-5.5', name: 'GPT-5.5 Standard', contextWindow: '500,000', maxOutput: '128,000', inputPrice: '$5.00', outputPrice: '$25.00', desc: 'Previous-generation standard architecture' },
      'gpt-5.5-pro': { id: 'gpt-5.5-pro', name: 'GPT-5.5 Pro', contextWindow: '500,000', maxOutput: '128,000', inputPrice: '$30.00', outputPrice: '$180.00', desc: 'Maintained for intensive reasoning capabilities' },
      'gpt-5.4': { id: 'gpt-5.4', name: 'GPT-5.4 Standard', contextWindow: '500,000', maxOutput: '128,000', inputPrice: '$2.50', outputPrice: '$15.00', desc: 'Standard production text & localization' },
      'gpt-5.4-pro': { id: 'gpt-5.4-pro', name: 'GPT-5.4 Pro', contextWindow: '500,000', maxOutput: '128,000', inputPrice: '$30.00', outputPrice: '$180.00', desc: 'Intensive reasoning & complex code refactoring' },
    },
  },
  deepl: {
    label: 'DeepL Translate',
    tag: 'DPL',
    models: ['deepl-default'],
    details: {
      'deepl-default': { id: 'deepl-default', name: 'DeepL Default', contextWindow: '128,000', maxOutput: '32,000', inputPrice: '$20.00', outputPrice: '$20.00', desc: 'Specialized neural translation engine' },
    },
  },
  custom: {
    label: 'Custom / Local Ollama',
    tag: 'LOC',
    models: ['qwen2.5:32b', 'llama3.3:70b', 'deepseek-r1:32b', 'mistral-large'],
    details: {
      'qwen2.5:32b': { id: 'qwen2.5:32b', name: 'Qwen 2.5 32B', contextWindow: '128,000', maxOutput: '8,192', inputPrice: '$0.00', outputPrice: '$0.00', desc: 'Local open-weights model' },
      'llama3.3:70b': { id: 'llama3.3:70b', name: 'LLaMA 3.3 70B', contextWindow: '128,000', maxOutput: '8,192', inputPrice: '$0.00', outputPrice: '$0.00', desc: 'Local open-weights model' },
      'deepseek-r1:32b': { id: 'deepseek-r1:32b', name: 'DeepSeek R1 32B', contextWindow: '128,000', maxOutput: '8,192', inputPrice: '$0.00', outputPrice: '$0.00', desc: 'Local reasoning model' },
      'mistral-large': { id: 'mistral-large', name: 'Mistral Large', contextWindow: '128,000', maxOutput: '8,192', inputPrice: '$0.00', outputPrice: '$0.00', desc: 'Local open-weights model' },
    },
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
  const initialTab = (searchParams.get('tab') as 'copilot' | 'overview' | 'settings' | 'matrix' | 'seo' | 'runs' | 'bot') || 'copilot'

  const [activeTab, setActiveTab] = useState<'copilot' | 'overview' | 'settings' | 'matrix' | 'seo' | 'runs' | 'bot'>(initialTab)
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
  const setTab = (t: 'copilot' | 'overview' | 'settings' | 'matrix' | 'seo' | 'runs' | 'bot') => {
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
  const [selectedProvider, setSelectedProvider] = useState<string>('gemini')
  const [selectedModel, setSelectedModel] = useState<string>('gemini-3.7-flash')
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

  // Webhook Autopilot & PR Bot Settings State
  const [webhookPushEnabled, setWebhookPushEnabled] = useState<boolean>(true)
  const [webhookBranchFilter, setWebhookBranchFilter] = useState<'default_branch' | 'all' | 'custom'>('default_branch')
  const [webhookCustomBranches, setWebhookCustomBranches] = useState<string>('')
  const [webhookAction, setWebhookAction] = useState<'auto_pr' | 'direct_commit' | 'draft_pr'>('auto_pr')
  const [webhookPRCommentsEnabled, setWebhookPRCommentsEnabled] = useState<boolean>(true)
  const [webhookCustomBranchPrefix, setWebhookCustomBranchPrefix] = useState<string>('langpeanut/i18n-')
  const [webhookPathFilter, setWebhookPathFilter] = useState<string>('')

  // Webhook Simulator & Testing State (Tab 5)
  const [simulatingPush, setSimulatingPush] = useState(false)
  const [pushSimResult, setPushSimResult] = useState<any>(null)
  const [simulatingBot, setSimulatingBot] = useState(false)
  const [botSimInput, setBotSimInput] = useState<string>('@langpeanut translate --locales es,ja --tone formal')
  const [botSimResult, setBotSimResult] = useState<any>(null)
  const [copiedWebhookURL, setCopiedWebhookURL] = useState(false)

  // Translation Matrix State (Real from DB / Repo)
  const { data: rawMatrix, mutate: mutateMatrix } = useSWR<Record<string, Record<string, string>>>(
    authed && repo ? `/api/repos/${repo.ID}/matrix` : null,
    fetcher
  )
  const [matrixSearch, setMatrixSearch] = useState('')
  const [editingCell, setEditingCell] = useState<{ rowKey: string; colKey: string } | null>(null)
  const [cellValue, setCellValue] = useState('')
  const [savingCell, setSavingCell] = useState(false)

  // SEO & Market Growth Studio State
  const { data: seoData, mutate: mutateSEO } = useSWR<any>(
    authed && repo ? `/api/repos/${repo.ID}/seo` : null,
    fetcher
  )
  const [seoLocale, setSeoLocale] = useState<string>('en')
  const [seoCategory, setSeoCategory] = useState<string>('')
  const [seoDescription, setSeoDescription] = useState<string>('')
  const [seoGoal, setSeoGoal] = useState<string>('traffic')
  const [seoScope, setSeoScope] = useState<string>('high_impact')
  const [seoCompetitorInput, setSeoCompetitorInput] = useState<string>('')
  const [seoSimView, setSeoSimView] = useState<'desktop' | 'mobile' | 'social'>('desktop')
  const [scoutingSEO, setScoutingSEO] = useState(false)
  const [optimizingSEO, setOptimizingSEO] = useState(false)
  const [applyingSEO, setApplyingSEO] = useState(false)
  const [analyzingDomain, setAnalyzingDomain] = useState(false)
  const [isModelDropdownOpen, setIsModelDropdownOpen] = useState(false)
  const [resettingData, setResettingData] = useState(false)
  const [deletingRepo, setDeletingRepo] = useState(false)

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

  // Central Agent Copilot State (Powered by Google Genkit Go)
  const [centralCanvasTab, setCentralCanvasTab] = useState<'matrix' | 'diff' | 'critic' | 'serp' | 'cost'>('matrix')
  const [lastCopilotCards, setLastCopilotCards] = useState<any[]>([])
  const [centralCopilotMessages, setCentralCopilotMessages] = useState<
    Array<{ role: 'user' | 'assistant'; content: string; reasoning?: string; tool_calls?: any[]; cards?: any[] }>
  >([
    {
      role: 'assistant',
      content: 'I am your langPeanut Copilot. I can inspect your AST for hardcoded UI strings, translate missing keys into target locales, run 4-tier ICU verification critics, simulate Google SERP previews, and modify repository settings. How can I help you today?',
    },
  ])
  const [centralCopilotInput, setCentralCopilotInput] = useState('')
  const [centralCopilotThinking, setCentralCopilotThinking] = useState(false)
  const [showBrowsePromptsModal, setShowBrowsePromptsModal] = useState(false)
  const [showDirectiveModal, setShowDirectiveModal] = useState(false)
  const [showAttachModal, setShowAttachModal] = useState(false)
  const [customDirectiveText, setCustomDirectiveText] = useState('')
  const chatContainerRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (chatContainerRef.current) {
      chatContainerRef.current.scrollTo({
        top: chatContainerRef.current.scrollHeight,
        behavior: 'smooth',
      })
    }
  }, [centralCopilotMessages, centralCopilotThinking])

  // Restore persisted chat messages for this repository
  useEffect(() => {
    if (!repo?.ID) return
    try {
      const savedMessages = localStorage.getItem(`langpeanut_copilot_messages_${repo.ID}`)
      if (savedMessages) {
        const parsed = JSON.parse(savedMessages)
        if (Array.isArray(parsed) && parsed.length > 0) {
          setCentralCopilotMessages(parsed)
        }
      }
      const savedCards = localStorage.getItem(`langpeanut_copilot_cards_${repo.ID}`)
      if (savedCards) {
        const parsedCards = JSON.parse(savedCards)
        if (Array.isArray(parsedCards) && parsedCards.length > 0) {
          setLastCopilotCards(parsedCards)
        }
      }
      const savedCanvas = localStorage.getItem(`langpeanut_copilot_canvas_${repo.ID}`)
      if (savedCanvas && ['matrix', 'diff', 'critic', 'serp', 'cost'].includes(savedCanvas)) {
        setCentralCanvasTab(savedCanvas as any)
      }
    } catch (e) {
      console.error('Failed to load saved chat history:', e)
    }
  }, [repo?.ID])

  // Persist messages whenever they change
  useEffect(() => {
    if (!repo?.ID || centralCopilotThinking) return
    if (centralCopilotMessages.length > 1 || (centralCopilotMessages.length === 1 && centralCopilotMessages[0].role === 'user')) {
      try {
        localStorage.setItem(`langpeanut_copilot_messages_${repo.ID}`, JSON.stringify(centralCopilotMessages))
      } catch (e) {}
    }
  }, [centralCopilotMessages, repo?.ID, centralCopilotThinking])

  // Persist cards whenever they change
  useEffect(() => {
    if (!repo?.ID || lastCopilotCards.length === 0) return
    try {
      localStorage.setItem(`langpeanut_copilot_cards_${repo.ID}`, JSON.stringify(lastCopilotCards))
    } catch (e) {}
  }, [lastCopilotCards, repo?.ID])

  // Persist canvas tab whenever it changes
  useEffect(() => {
    if (!repo?.ID) return
    try {
      localStorage.setItem(`langpeanut_copilot_canvas_${repo.ID}`, centralCanvasTab)
    } catch (e) {}
  }, [centralCanvasTab, repo?.ID])

  const sendCentralCopilotMessage = async (promptText: string) => {
    if (!promptText.trim() || !repo || centralCopilotThinking) return
    const text = promptText.trim()
    setCentralCopilotInput('')
    setCentralCopilotThinking(true)

    setCentralCopilotMessages((prev) => [
      ...prev,
      { role: 'user', content: text },
      { role: 'assistant', content: '', reasoning: '', tool_calls: [], cards: [] },
    ])

    const previousHistory = centralCopilotMessages
      .filter((m) => m.content && m.content.trim())
      .map((m) => ({ role: m.role, content: m.content }))

    try {
      const res = await fetch(`/api/repos/${repo.ID}/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          message: text,
          provider: selectedProvider,
          model: selectedModel,
          history: previousHistory,
        }),
      })

      if (!res.ok) throw new Error(`HTTP ${res.status}`)

      const reader = res.body?.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let assistantMsg: { role: 'assistant'; content: string; reasoning?: string; tool_calls: any[]; cards: any[] } = {
        role: 'assistant',
        content: '',
        reasoning: '',
        tool_calls: [],
        cards: [],
      }

      while (reader) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n\n')
        buffer = lines.pop() || ''

        let hasNewData = false
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const ev = JSON.parse(line.slice(6))
              if (ev.type === 'thought' || ev.type === 'reasoning') {
                assistantMsg.reasoning = (assistantMsg.reasoning ? assistantMsg.reasoning + '\n' : '') + (ev.reasoning || ev.content)
                hasNewData = true
              } else if (ev.type === 'tool_start' && ev.tool_call) {
                assistantMsg.tool_calls.push(ev.tool_call)
                hasNewData = true
              } else if (ev.type === 'tool_end' && ev.tool_call) {
                const existing = assistantMsg.tool_calls.find((tc: any) => tc.id === ev.tool_call.id)
                if (existing) {
                  existing.result = ev.tool_result?.output
                  existing.error = ev.tool_result?.error
                }
                hasNewData = true
              } else if (ev.type === 'card' && ev.card) {
                assistantMsg.cards.push(ev.card)
                setLastCopilotCards((prev) => [...prev, ev.card])
                if (ev.card.type === 'matrix') setCentralCanvasTab('matrix')
                if (ev.card.type === 'diff') setCentralCanvasTab('diff')
                if (ev.card.type === 'critic') setCentralCanvasTab('critic')
                if (ev.card.type === 'serp') setCentralCanvasTab('serp')
                if (ev.card.type === 'cost') setCentralCanvasTab('cost')
                hasNewData = true
              } else if (ev.type === 'chunk' && ev.content) {
                assistantMsg.content += ev.content
                hasNewData = true
              } else if (ev.type === 'done' && ev.content) {
                assistantMsg.content = ev.content
                hasNewData = true
              }
            } catch (e) {}
          }
        }

        if (hasNewData) {
          setCentralCopilotMessages((prev) => {
            const next = [...prev]
            if (next.length > 0 && next[next.length - 1].role === 'assistant') {
              next[next.length - 1] = {
                ...assistantMsg,
                tool_calls: [...assistantMsg.tool_calls],
                cards: [...assistantMsg.cards],
              }
            }
            return next
          })
        }
      }
    } catch (err: any) {
      setCentralCopilotMessages((prev) => {
        const next = [...prev]
        if (next.length > 0 && next[next.length - 1].role === 'assistant') {
          next[next.length - 1] = {
            role: 'assistant',
            content: `Error communicating with Genkit orchestrator: ${err.message}`,
            tool_calls: [],
            cards: [],
          }
        } else {
          next.push({
            role: 'assistant',
            content: `Error communicating with Genkit orchestrator: ${err.message}`,
            tool_calls: [],
            cards: [],
          })
        }
        return next
      })
    } finally {
      setCentralCopilotThinking(false)
    }
  }

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
      setSelectedProvider(repo.settings.Provider || 'gemini')
      setSelectedModel(repo.settings.Model || 'gemini-3.7-flash')
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

      const s = repo.settings as any
      setWebhookPushEnabled(
        s.webhook_push_enabled !== undefined
          ? s.webhook_push_enabled
          : s.WebhookPushEnabled !== undefined
          ? s.WebhookPushEnabled
          : true
      )
      setWebhookBranchFilter(
        (s.webhook_branch_filter || s.WebhookBranchFilter || 'default_branch') as any
      )
      setWebhookCustomBranches(
        s.webhook_custom_branches || s.WebhookCustomBranches || ''
      )
      setWebhookAction(
        (s.webhook_action || s.WebhookAction || 'auto_pr') as any
      )
      setWebhookPRCommentsEnabled(
        s.webhook_pr_comments_enabled !== undefined
          ? s.webhook_pr_comments_enabled
          : s.WebhookPRCommentsEnabled !== undefined
          ? s.WebhookPRCommentsEnabled
          : true
      )
      setWebhookCustomBranchPrefix(
        s.webhook_custom_branch_prefix ||
          s.WebhookCustomBranchPrefix ||
          'langpeanut/i18n-'
      )
      setWebhookPathFilter(
        s.webhook_path_filter || s.WebhookPathFilter || ''
      )
    }
  }, [repo])

  // Sync SEO Strategy & Locales when seoData loads
  useEffect(() => {
    if (seoData?.strategy) {
      if (seoData.strategy.category) setSeoCategory(seoData.strategy.category)
      if (seoData.strategy.product_description) setSeoDescription(seoData.strategy.product_description)
      if (seoData.strategy.goal) setSeoGoal(seoData.strategy.goal)
      if (seoData.strategy.scope_tier) setSeoScope(seoData.strategy.scope_tier)
      if (seoData.strategy.competitor_urls && seoData.strategy.competitor_urls.length > 0) {
        setSeoCompetitorInput(seoData.strategy.competitor_urls.join(', '))
      }
      const validLocales = Array.from(new Set(['en', ...(seoData.strategy.target_locales || []), ...selectedLocales]))
      if (!validLocales.includes(seoLocale)) {
        setSeoLocale('en')
      }
    }
  }, [seoData])

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
        webhook_push_enabled: webhookPushEnabled,
        webhook_branch_filter: webhookBranchFilter,
        webhook_custom_branches: webhookCustomBranches.trim(),
        webhook_action: webhookAction,
        webhook_pr_comments_enabled: webhookPRCommentsEnabled,
        webhook_custom_branch_prefix: webhookCustomBranchPrefix.trim(),
        webhook_path_filter: webhookPathFilter.trim(),
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

  async function simulateWebhookPush(dryRun = true) {
    if (!repo) return
    setSimulatingPush(true)
    setPushSimResult(null)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/webhook/test-push`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          branch: selectedBranch || repo.DefaultBranch || 'main',
          dry_run: dryRun,
        }),
      })
      const data = await res.json()
      setPushSimResult(data)
      if (res.ok && data.matched) {
        if (dryRun) {
          toast.success(data.message || 'Push webhook simulation matched criteria!')
        } else {
          toast.success(data.message || 'Real push webhook job queued!')
          mutateJobs()
        }
      } else if (res.ok) {
        toast.warning(data.message || 'Push webhook skipped based on rules.')
      } else {
        toast.error(data.error || 'Webhook test failed')
      }
    } catch (e: any) {
      setPushSimResult({ error: e.message })
      toast.error(`Error simulating webhook: ${e.message}`)
    } finally {
      setSimulatingPush(false)
    }
  }

  async function simulateBotCommand() {
    if (!repo) return
    if (!botSimInput.trim()) return
    setSimulatingBot(true)
    setBotSimResult(null)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/webhook/test-bot`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ command: botSimInput.trim() }),
      })
      const data = await res.json()
      setBotSimResult(data)
      if (data.valid) {
        toast.success(data.message || 'Bot command parsed successfully!')
      } else {
        toast.error(data.message || 'Invalid bot command')
      }
    } catch (e: any) {
      setBotSimResult({ error: e.message })
      toast.error(`Error testing bot command: ${e.message}`)
    } finally {
      setSimulatingBot(false)
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

  async function handleQuickSwitchModel(newProvider: string, newModel: string) {
    if (!repo) return
    setSelectedProvider(newProvider)
    setSelectedModel(newModel)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/model`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ provider: newProvider, model: newModel }),
      })
      if (res.ok) {
        const provLabel = PROVIDER_MODELS[newProvider]?.label || newProvider
        showToast(`✓ Active model switched to ${newModel} (${provLabel})`)
        mutateRepos()
      } else {
        showToast('Failed to update active model on server', 'error')
      }
    } catch {
      showToast('Network error updating active model', 'error')
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
        showToast(`Persona discovered: Tone '${data.recommended_tone}', ${data.brand_lexicon?.length || 0} brand terms locked`)
      } else {
        showToast(data.error || 'Failed to discover persona', 'error')
      }
    } catch {
      showToast('Network error during persona discovery', 'error')
    } finally {
      setDiscoveringPersona(false)
    }
  }

  async function handleSaveSEOStrategy() {
    if (!repo) return
    const comps = seoCompetitorInput
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/seo/strategy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          project_name: repo.Name,
          category: seoCategory,
          product_description: seoDescription,
          goal: seoGoal,
          scope_tier: seoScope,
          target_locales: selectedLocales,
          competitor_urls: comps,
        }),
      })
      if (res.ok) {
        showToast('SEO product domain & market settings saved')
        mutateSEO()
      } else {
        showToast('Failed to save SEO strategy', 'error')
      }
    } catch {
      showToast('Network error saving SEO strategy', 'error')
    }
  }

  async function handleTriggerAnalyzeDomain() {
    if (!repo) return
    setAnalyzingDomain(true)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/seo/analyze-domain`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
      })
      const data = await res.json()
      if (res.ok) {
        setSeoCategory(data.category)
        setSeoDescription(data.product_description)
        showToast(`AI analyzed ${data.extracted_keys_count} strings: Domain set to "${data.category}"`)
        mutateSEO()
      } else {
        showToast(data.error || 'AI domain analysis failed', 'error')
      }
    } catch {
      showToast('Network error during AI domain analysis', 'error')
    } finally {
      setAnalyzingDomain(false)
    }
  }

  async function handleTriggerSEOScout() {
    if (!repo) return
    setScoutingSEO(true)
    try {
      const comps = seoCompetitorInput
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
      await fetch(`/api/repos/${repo.ID}/seo/strategy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          project_name: repo.Name,
          category: seoCategory,
          product_description: seoDescription,
          goal: seoGoal,
          scope_tier: seoScope,
          target_locales: selectedLocales,
          competitor_urls: comps,
        }),
      })
      const res = await fetch(`/api/repos/${repo.ID}/seo/scout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ locale: seoLocale }),
      })
      const data = await res.json()
      if (res.ok) {
        showToast(`Scouted ${data.competitors?.length || 0} competitors & discovered ${data.keywords?.length || 0} keywords for [${seoLocale.toUpperCase()}]`)
        mutateSEO()
      } else {
        showToast(data.error || 'Scout failed', 'error')
      }
    } catch {
      showToast('Network error during competitor scouting', 'error')
    } finally {
      setScoutingSEO(false)
    }
  }

  async function handleTriggerSEOOptimize() {
    if (!repo) return
    setOptimizingSEO(true)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/seo/optimize`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ locale: seoLocale }),
      })
      const data = await res.json()
      if (res.ok) {
        showToast(`SEO-optimized ${data.optimizations?.length || 0} keys for [${seoLocale.toUpperCase()}]`)
        mutateSEO()
      } else {
        showToast(data.error || 'SEO optimization failed', 'error')
      }
    } catch {
      showToast('Network error during SEO optimization', 'error')
    } finally {
      setOptimizingSEO(false)
    }
  }

  async function handleApplySEOToMatrix() {
    if (!repo) return
    setApplyingSEO(true)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/seo/apply`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ locale: seoLocale }),
      })
      const data = await res.json()
      if (res.ok) {
        showToast(`Successfully applied ${data.applied_count || 0} SEO translations to Live Matrix`)
        mutateSEO()
        mutateMatrix()
      } else {
        showToast(data.error || 'Failed to apply SEO keys', 'error')
      }
    } catch {
      showToast('Network error applying SEO translations', 'error')
    } finally {
      setApplyingSEO(false)
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
        showToast(`Doctor audit: Health score ${data.health_score}/100 (${data.status})`)
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
          showToast(`Pruned ${data.total_dead_keys} stale keys across ${data.pruned_locales?.join(', ') || 'locale files'}`)
        } else {
          showToast('Clean. No orphaned translation keys found.')
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

  async function handleResetRepoData() {
    if (!repo) return
    const confirmed = window.confirm(
      `Are you sure you want to reset all stored localization data for ${repo.Owner}/${repo.Name}?\n\nThis will permanently clear all translation matrix keys, jobs, execution logs, and SEO intelligence so you can start completely fresh from the beginning.`
    )
    if (!confirmed) return

    setResettingData(true)
    try {
      const res = await fetch(`/api/repos/${repo.ID}/reset`, {
        method: 'POST',
        credentials: 'include',
      })
      const data = await res.json()
      if (res.ok) {
        showToast(data.message || 'Repository data has been reset to baseline.')
        mutateRepos()
        mutateMatrix()
        mutateJobs()
        mutateSEO()
      } else {
        showToast(data.error || 'Failed to reset repository data', 'error')
      }
    } catch {
      showToast('Network error while resetting repository data', 'error')
    } finally {
      setResettingData(false)
    }
  }

  async function handleDeleteRepo() {
    if (!repo) return
    const confirmed = window.confirm(
      `Are you sure you want to permanently delete repository ${repo.Owner}/${repo.Name}?\n\nThis will remove the repository connection, all translation matrices, and cached git mirrors. This action cannot be undone.`
    )
    if (!confirmed) return

    setDeletingRepo(true)
    try {
      const res = await fetch(`/api/repos/${repo.ID}`, {
        method: 'DELETE',
        credentials: 'include',
      })
      const data = await res.json()
      if (res.ok) {
        showToast(data.message || 'Repository deleted successfully.')
        router.push('/dashboard')
      } else {
        showToast(data.error || 'Failed to delete repository', 'error')
        setDeletingRepo(false)
      }
    } catch {
      showToast('Network error while deleting repository', 'error')
      setDeletingRepo(false)
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
                  <Badge variant="outline" className={`text-[11px] font-mono px-2 py-0.5 rounded-full ${latestStatus.bg} ${latestStatus.border} ${latestStatus.text}`}>
                    {latestStatus.label}
                  </Badge>
                )}
              </div>
              <p className="text-xs text-slate-400 mt-1">
                Universal Multi-Agent Localization Pipeline • Connected to GitHub App
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2.5">
            <Button
              asChild
              variant="outline"
              size="sm"
              className="rounded-xl bg-white/[0.04] hover:bg-white/[0.08] border-white/10 text-slate-300 font-medium px-3.5 py-2 text-xs h-auto gap-1.5"
            >
              <a
                href={`https://github.com/${repo.Owner}/${repo.Name}`}
                target="_blank"
                rel="noopener noreferrer"
              >
                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                </svg>
                <span>GitHub</span>
              </a>
            </Button>

            <Button
              onClick={triggerJob}
              disabled={triggering}
              size="sm"
              className="rounded-xl bg-blue-600 hover:bg-blue-500 disabled:bg-blue-900 text-white font-semibold px-4 py-2 text-xs shadow-lg shadow-blue-600/30 h-auto gap-2"
            >
              <svg className={`w-3.5 h-3.5 ${triggering ? 'animate-spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <polygon points="5 3 19 12 5 21 5 3" />
              </svg>
              <span>{triggering ? 'Starting Pipeline…' : 'Run Localization'}</span>
            </Button>
          </div>
        </div>

        {/* Tab Navigation */}
        <div className="flex items-center gap-1.5 pt-2 border-t border-white/[0.05] overflow-x-auto whitespace-nowrap pb-1">
          {[
            { id: 'copilot', label: 'Autonomous Copilot', badge: 'CORE' },
            { id: 'overview', label: 'Overview' },
            { id: 'settings', label: 'Settings & Strategy' },
            { id: 'matrix', label: 'Translation Matrix' },
            { id: 'seo', label: 'SEO & Growth Studio' },
            { id: 'runs', label: 'Runs & Logs' },
            { id: 'bot', label: 'PR Bot & Webhooks' },
          ].map((t) => (
            <Button
              key={t.id}
              variant="ghost"
              size="sm"
              onClick={() => setTab(t.id as any)}
              className={`px-4 py-2 rounded-xl text-xs font-semibold h-auto shrink-0 flex items-center gap-2 ${
                activeTab === t.id
                  ? 'bg-blue-600/15 border border-blue-500/30 text-sky-300 shadow-md shadow-sky-950/50 hover:bg-blue-600/20 hover:text-sky-200'
                  : 'text-slate-400 hover:text-white hover:bg-white/[0.03]'
              }`}
            >
              <span>{t.label}</span>
              {t.badge && (
                <Badge variant="outline" className="text-[9px] px-1.5 py-0.2 rounded bg-sky-500/10 text-sky-400 border-sky-500/20 font-mono">
                  {t.badge}
                </Badge>
              )}
            </Button>
          ))}
        </div>
      </div>

      {/* ─── TAB 0: AUTONOMOUS COPILOT WORKSPACE (DEDICATED CHAT PAGE) ───────────────────────────── */}
      {/* ─── TAB 0: AUTONOMOUS COPILOT WORKSPACE (MODERN AI CHAT INTERFACE) ───────────────────────────── */}
      {activeTab === 'copilot' && (
        <div className="w-full flex flex-col lg:flex-row gap-6 h-[calc(100vh-190px)] min-h-[640px]">
          {/* Main Chat Canvas (Spacious Center Column) */}
          <div className="flex-1 glass-panel rounded-2xl flex flex-col overflow-hidden border border-white/10 bg-[#090c13] shadow-2xl h-full">
            {/* Top Bar matching reference */}
            <div className="px-6 py-4 border-b border-white/[0.08] bg-[#0c1018] flex items-center justify-between shrink-0">
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-sky-500/20 to-indigo-500/20 border border-sky-500/30 flex items-center justify-center text-sky-400 font-bold text-xs">
                  <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 2a10 10 0 0 1 10 10c0 5.523-4.477 10-10 10S2 17.523 2 12 6.477 2 12 2z"/><path d="M8 14s1.5 2 4 2 4-2 4-2"/><path d="M9 9h.01"/><path d="M15 9h.01"/></svg>
                </div>
                <div>
                  <h2 className="text-sm font-bold text-white tracking-tight">AI Chat</h2>
                  <p className="text-[11px] text-zinc-400 font-mono">
                    Autonomous Multi-Agent Localization Copilot
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setActiveTab('settings')}
                  className="px-3 py-1.5 rounded-lg bg-gradient-to-r from-amber-500/10 to-orange-500/10 border border-amber-500/30 hover:border-amber-500/50 text-amber-400 text-xs font-medium flex items-center gap-1.5 transition-all cursor-pointer shadow-xs"
                >
                  <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>
                  <span>Vault Keys</span>
                </button>

                {/* Interactive Model & Provider Switcher Popover */}
                <div className="relative">
                  <button
                    type="button"
                    onClick={() => setIsModelDropdownOpen(!isModelDropdownOpen)}
                    className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-zinc-900/90 hover:bg-zinc-800 border border-sky-500/30 hover:border-sky-500/60 text-zinc-200 text-xs font-mono transition-all cursor-pointer shadow-sm"
                    title="Click to switch active AI model & provider"
                  >
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
                    <span className="font-semibold text-sky-300">{selectedModel}</span>
                    <span className="text-[10px] text-zinc-400">({PROVIDER_MODELS[selectedProvider]?.label || selectedProvider})</span>
                    <svg className="w-3 h-3 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                      <polyline points="6 9 12 15 18 9" />
                    </svg>
                  </button>

                  {isModelDropdownOpen && (
                    <div className="absolute right-0 mt-2 w-80 rounded-2xl bg-zinc-950/95 border border-white/15 shadow-2xl p-3.5 z-50 space-y-3 backdrop-blur-xl">
                      <div className="flex items-center justify-between pb-2 border-b border-white/10">
                        <span className="text-[11px] font-bold uppercase tracking-wider text-slate-300">
                          Active Model & Provider
                        </span>
                        <span className="text-[10px] font-mono text-emerald-400">
                          Instant Switch
                        </span>
                      </div>

                      <div className="space-y-3 max-h-80 overflow-y-auto custom-scrollbar pr-1">
                        {Object.entries(PROVIDER_MODELS).map(([provKey, prov]) => {
                          const isCurProv = selectedProvider === provKey
                          return (
                            <div key={provKey} className="space-y-1.5">
                              <div className="flex items-center justify-between text-[10px] font-bold uppercase tracking-wider text-zinc-400 px-1">
                                <span>{prov.label}</span>
                                <span className="font-mono text-zinc-500">[{prov.tag}]</span>
                              </div>
                              <div className="grid grid-cols-1 gap-1">
                                {prov.models.map((m) => {
                                  const isCurModel = isCurProv && selectedModel === m
                                  const d = prov.details?.[m]
                                  return (
                                    <button
                                      key={m}
                                      type="button"
                                      onClick={() => {
                                        handleQuickSwitchModel(provKey, m)
                                        setIsModelDropdownOpen(false)
                                      }}
                                      className={`w-full text-left px-2.5 py-1.5 rounded-xl text-xs flex items-center justify-between cursor-pointer transition-all ${
                                        isCurModel
                                          ? 'bg-sky-500/20 text-sky-300 font-semibold border border-sky-500/40'
                                          : 'text-zinc-300 hover:bg-white/5 hover:text-white border border-transparent'
                                      }`}
                                    >
                                      <div className="min-w-0 pr-2">
                                        <div className="font-mono text-[11px] truncate flex items-center gap-1.5">
                                          {isCurModel && <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 shrink-0" />}
                                          <span>{d?.name || m}</span>
                                        </div>
                                        {d && (
                                          <div className="text-[10px] text-zinc-500 truncate">
                                            {d.inputPrice} in • {d.outputPrice} out
                                          </div>
                                        )}
                                      </div>
                                      {isCurModel && <span className="text-sky-400 font-bold text-xs shrink-0">✓</span>}
                                    </button>
                                  )
                                })}
                              </div>
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  )}
                </div>

                <button
                  onClick={() => {
                    const defaultMsg = [
                      {
                        role: 'assistant' as const,
                        content: 'I am your langPeanut Copilot. I can inspect your AST for hardcoded UI strings, translate missing keys into target locales, run 4-tier ICU verification critics, simulate Google SERP previews, and modify repository settings. How can I help you today?',
                      },
                    ]
                    setCentralCopilotMessages(defaultMsg)
                    setLastCopilotCards([])
                    if (repo?.ID) {
                      localStorage.removeItem(`langpeanut_copilot_messages_${repo.ID}`)
                      localStorage.removeItem(`langpeanut_copilot_cards_${repo.ID}`)
                      localStorage.removeItem(`langpeanut_copilot_canvas_${repo.ID}`)
                    }
                  }}
                  title="Reset conversation"
                  className="p-2 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors text-xs cursor-pointer border border-transparent hover:border-white/10"
                >
                  <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" /><path d="M3 3v5h5" /></svg>
                </button>
              </div>
            </div>

            {/* API Key Missing Alert Banner */}
            {!isProviderConfigured(selectedProvider) && !repo.settings?.has_api_key_override && selectedProvider !== 'custom' && (
              <div className="mx-6 mt-4 p-3 rounded-xl bg-amber-500/10 border border-amber-500/25 flex items-center justify-between gap-3 shrink-0">
                <div className="flex items-center gap-2.5">
                  <div className="w-7 h-7 rounded-lg bg-amber-500/20 text-amber-400 flex items-center justify-center text-xs shrink-0">
                    <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="m21 2-2 2m-6 6 2 2m-2-2-4 4-2-2-4 4 2 2-4 4 4 4 4-4 2 2 4-4-2-2 4-4Z"/></svg>
                  </div>
                  <div>
                    <div className="text-xs font-semibold text-amber-300">
                      No API Key Configured for {PROVIDER_MODELS[selectedProvider]?.label || selectedProvider}
                    </div>
                    <div className="text-[11px] text-zinc-400">
                      Configure your API key in Vault Keys to enable live {selectedModel} completions.
                    </div>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => setActiveTab('settings')}
                  className="px-3 py-1.5 rounded-lg bg-amber-500 hover:bg-amber-400 text-zinc-950 text-xs font-bold transition-all cursor-pointer shrink-0 shadow-sm"
                >
                  Configure Key
                </button>
              </div>
            )}

            {/* Conversation Messages with Modern Styled Cards */}
            <div ref={chatContainerRef} className="flex-1 p-6 overflow-y-auto space-y-6 text-sm custom-scrollbar">
              {centralCopilotMessages.map((msg, idx) => (
                <div key={idx} className="space-y-3">
                  {msg.role === 'user' ? (
                    <div className="flex items-start gap-3 justify-end">
                      <div className="max-w-[78%] bg-zinc-900 border border-white/10 text-zinc-100 rounded-2xl rounded-tr-xs px-4.5 py-3 text-sm shadow-sm leading-relaxed">
                        {msg.content}
                      </div>
                      <div className="w-8 h-8 rounded-full bg-gradient-to-tr from-sky-500 to-indigo-600 flex items-center justify-center text-white text-xs font-bold shrink-0 shadow-sm">
                        U
                      </div>
                    </div>
                  ) : (
                    <div className="flex items-start gap-3 justify-start">
                      <div className="w-8 h-8 rounded-full bg-zinc-800 border border-white/10 flex items-center justify-center text-sky-400 font-bold text-xs shrink-0 mt-0.5 shadow-sm">
                        <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><path d="M12 7v4"/><line x1="8" y1="16" x2="8" y2="16"/><line x1="16" y1="16" x2="16" y2="16"/></svg>
                      </div>
                      <div className="flex-1 min-w-0 max-w-[92%] space-y-3">
                        {msg.reasoning && (
                          <Reasoning defaultOpen={false}>
                            <div className="font-mono text-xs text-zinc-400 whitespace-pre-wrap">{msg.reasoning}</div>
                          </Reasoning>
                        )}

                        {msg.tool_calls && msg.tool_calls.length > 0 && (
                          <div className="space-y-2">
                            {msg.tool_calls.map((tc: any, tIdx: number) => (
                              <Tool
                                key={tIdx}
                                toolPart={{
                                  type: tc.name || 'tool_invocation',
                                  state: tc.error ? 'output-error' : tc.result ? 'output-available' : 'output-available',
                                  input: tc.args,
                                  output: tc.result,
                                  toolCallId: tc.id,
                                  errorText: tc.error,
                                }}
                              />
                            ))}
                          </div>
                        )}

                        {msg.cards && msg.cards.length > 0 && (
                          <div className="space-y-2">
                            {msg.cards.map((c: any, cIdx: number) => (
                              <div key={cIdx} className="rounded-xl border border-white/10 bg-[#06080d] p-3 text-xs font-mono text-zinc-300">
                                {c.rendered_text && (
                                  <pre className="whitespace-pre overflow-x-auto custom-scrollbar">{c.rendered_text}</pre>
                                )}
                              </div>
                            ))}
                          </div>
                        )}

                        {/* Assistant Message Bubble */}
                        <div className="bg-[#10141d]/90 border border-white/[0.08] rounded-2xl rounded-tl-xs p-5 text-sm text-zinc-200 shadow-md space-y-3 font-sans leading-relaxed">
                          <ReactMarkdown
                            remarkPlugins={[remarkGfm]}
                            components={{
                              code({ className, children, ...props }) {
                                const match = /language-(\w+)/.exec(className || '')
                                const isInline = !match && !String(children).includes('\n')
                                return isInline ? (
                                  <code className="rounded bg-zinc-800/90 px-1.5 py-0.5 font-mono text-xs text-sky-300 border border-white/5" {...props}>
                                    {children}
                                  </code>
                                ) : (
                                  <pre className="rounded-xl bg-zinc-950 p-3.5 overflow-x-auto border border-white/10 font-mono text-xs custom-scrollbar my-2 text-zinc-200">
                                    <code className={className} {...props}>
                                      {children}
                                    </code>
                                  </pre>
                                )
                              },
                              ul: ({ children }) => <ul className="list-disc pl-5 space-y-1.5 my-2">{children}</ul>,
                              ol: ({ children }) => <ol className="list-decimal pl-5 space-y-1.5 my-2">{children}</ol>,
                              li: ({ children }) => <li className="leading-relaxed">{children}</li>,
                              p: ({ children }) => <p className="mb-2.5 last:mb-0 leading-relaxed">{children}</p>,
                              strong: ({ children }) => <strong className="font-semibold text-white">{children}</strong>,
                              h1: ({ children }) => <h1 className="text-base font-bold text-white mt-3 mb-1.5">{children}</h1>,
                              h2: ({ children }) => <h2 className="text-sm font-bold text-white mt-2.5 mb-1">{children}</h2>,
                              h3: ({ children }) => <h3 className="text-xs font-semibold text-sky-400 mt-2 mb-1">{children}</h3>,
                            }}
                          >
                            {msg.content}
                          </ReactMarkdown>

                          {/* Message Actions (Like / Copy / Share) */}
                          <div className="pt-2 border-t border-white/5 flex items-center justify-between text-xs text-zinc-400">
                            <div className="flex items-center gap-2">
                              <button
                                onClick={() => toast.success('Feedback recorded')}
                                className="p-1 rounded hover:bg-zinc-800 hover:text-zinc-200 transition-colors cursor-pointer"
                                title="Good response"
                              >
                                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M7 10v12"/><path d="M15 5.88 14 10h5.83a2 2 0 0 1 1.92 2.56l-2.33 8A2 2 0 0 1 17.5 22H4a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2h3Z"/></svg>
                              </button>
                              <button
                                onClick={() => toast.info('Feedback noted')}
                                className="p-1 rounded hover:bg-zinc-800 hover:text-zinc-200 transition-colors cursor-pointer"
                                title="Needs improvement"
                              >
                                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M17 14V2"/><path d="M9 18.12 10 14H4.17a2 2 0 0 1-1.92-2.56l2.33-8A2 2 0 0 1 6.5 2H20a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-3Z"/></svg>
                              </button>
                            </div>
                            <div className="flex items-center gap-3">
                              <button
                                onClick={() => {
                                  navigator.clipboard.writeText(msg.content)
                                  toast.success('Response copied to clipboard')
                                }}
                                className="flex items-center gap-1.5 hover:text-white transition-colors cursor-pointer"
                              >
                                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
                                <span>Copy</span>
                              </button>
                              <button
                                onClick={() => {
                                  navigator.clipboard.writeText(window.location.href)
                                  toast.success('Workspace link copied')
                                }}
                                className="flex items-center gap-1.5 hover:text-white transition-colors cursor-pointer"
                              >
                                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8"/><polyline points="16 6 12 2 8 6"/><line x1="12" y1="2" x2="12" y2="15"/></svg>
                                <span>Share</span>
                              </button>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              ))}

              {centralCopilotThinking && (
                <div className="flex items-start gap-3">
                  <div className="w-8 h-8 rounded-full bg-zinc-800 border border-white/10 flex items-center justify-center text-sky-400 font-bold text-xs shrink-0 mt-0.5">
                    <svg className="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="12" cy="12" r="10" strokeDasharray="32" strokeDashoffset="12"/></svg>
                  </div>
                  <div className="bg-[#10141d] border border-white/[0.08] rounded-2xl px-4 py-3 text-xs text-zinc-400 flex items-center gap-2 font-mono">
                    <span className="w-1.5 h-1.5 rounded-full bg-sky-400 animate-pulse" />
                    <span className="w-1.5 h-1.5 rounded-full bg-sky-400 animate-pulse delay-100" />
                    <span className="w-1.5 h-1.5 rounded-full bg-sky-400 animate-pulse delay-200" />
                    <span className="ml-1">Executing autonomous pipeline...</span>
                  </div>
                </div>
              )}
            </div>

            {/* Floating Regenerate Button */}
            {centralCopilotMessages.length > 1 && !centralCopilotThinking && (
              <div className="flex justify-center -mb-3 z-10">
                <button
                  onClick={() => {
                    const lastUserMsg = [...centralCopilotMessages].reverse().find((m) => m.role === 'user')
                    if (lastUserMsg) {
                      sendCentralCopilotMessage(lastUserMsg.content)
                    }
                  }}
                  className="px-3.5 py-1 rounded-full bg-zinc-900 border border-white/15 hover:bg-zinc-800 text-zinc-300 hover:text-white text-xs flex items-center gap-1.5 shadow-lg transition-all cursor-pointer font-medium"
                >
                  <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" /><path d="M3 3v5h5" /></svg>
                  <span>Regenerate</span>
                </button>
              </div>
            )}

            {/* Bottom Floating Input Card matching reference */}
            <div className="p-4 border-t border-white/10 bg-[#0a0d14]">
              <div className="rounded-2xl border border-white/10 bg-[#0e121b] p-3 shadow-xl space-y-2.5">
                <div className="flex items-center gap-2">
                  <textarea
                    value={centralCopilotInput}
                    onChange={(e) => setCentralCopilotInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault()
                        sendCentralCopilotMessage(centralCopilotInput)
                      }
                    }}
                    placeholder="Send a message..."
                    rows={1}
                    disabled={centralCopilotThinking}
                    className="flex-1 bg-transparent text-sm text-zinc-100 placeholder:text-zinc-500 focus:outline-none resize-none min-h-[36px] max-h-32 leading-relaxed"
                  />
                  <button
                    type="button"
                    onClick={() => sendCentralCopilotMessage(centralCopilotInput)}
                    disabled={centralCopilotThinking || !centralCopilotInput.trim()}
                    className="w-8 h-8 rounded-xl bg-sky-500 hover:bg-sky-400 disabled:opacity-40 text-black flex items-center justify-center transition-all cursor-pointer shrink-0 shadow-sm"
                  >
                    <svg className="w-4 h-4 transform rotate-45" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
                  </button>
                </div>

                {/* Action Pills & Counter */}
                <div className="pt-2 border-t border-white/5 flex items-center justify-between text-xs relative">
                  <div className="flex items-center gap-1.5 flex-wrap">
                    {/* Attach Popover Button */}
                    <div className="relative">
                      <button
                        type="button"
                        onClick={() => {
                          setShowAttachModal(!showAttachModal)
                          setShowDirectiveModal(false)
                          setShowBrowsePromptsModal(false)
                        }}
                        className={`px-2.5 py-1 rounded-lg text-xs flex items-center gap-1.5 border transition-colors cursor-pointer ${
                          showAttachModal
                            ? 'bg-sky-500/20 text-sky-300 border-sky-500/40'
                            : 'bg-zinc-800/70 hover:bg-zinc-700/80 text-zinc-300 border-white/5'
                        }`}
                      >
                        <svg className="w-3.5 h-3.5 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg>
                        <span>Attach Context</span>
                      </button>

                      {/* Attach Dropdown Menu */}
                      {showAttachModal && (
                        <div className="absolute bottom-full left-0 mb-2 w-72 bg-[#121622] border border-white/15 rounded-xl shadow-2xl p-3 z-50 space-y-2 animate-in fade-in slide-in-from-bottom-2 duration-150">
                          <div className="flex items-center justify-between border-b border-white/10 pb-2">
                            <span className="text-xs font-semibold text-white">Attach Workspace Context</span>
                            <button
                              onClick={() => setShowAttachModal(false)}
                              className="text-zinc-400 hover:text-white text-xs cursor-pointer"
                            >
                              ✕
                            </button>
                          </div>
                          <div className="space-y-1">
                            {[
                              { label: 'Scan Codebase AST', text: 'Scan repository AST and audit hardcoded UI strings', desc: 'Runs Tree-sitter scout on project' },
                              { label: 'Active Translation Matrix', text: 'Inspect translation matrix status and missing keys', desc: 'Checks database key-value catalogs' },
                              { label: 'Repository Setup & Gaps', text: 'What settings and configuration items are missing for this repo?', desc: 'Audits API keys and locales' },
                              { label: 'Recent Job Logs & PR', text: 'Show recent platform jobs and check GitHub PR status', desc: 'Queries execution telemetry' },
                            ].map((item, idx) => (
                              <button
                                key={idx}
                                type="button"
                                onClick={() => {
                                  setCentralCopilotInput((prev) => prev ? `${prev} - ${item.text}` : item.text)
                                  setShowAttachModal(false)
                                  toast.info(`Attached: ${item.label}`)
                                }}
                                className="w-full text-left p-2 rounded-lg hover:bg-zinc-800/80 transition-colors cursor-pointer group"
                              >
                                <div className="text-xs font-medium text-zinc-200 group-hover:text-sky-300">{item.label}</div>
                                <div className="text-[10px] text-zinc-400 truncate">{item.desc}</div>
                              </button>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>

                    {/* Directive Popover Button */}
                    <div className="relative">
                      <button
                        type="button"
                        onClick={() => {
                          setShowDirectiveModal(!showDirectiveModal)
                          setShowAttachModal(false)
                          setShowBrowsePromptsModal(false)
                        }}
                        className={`px-2.5 py-1 rounded-lg text-xs flex items-center gap-1.5 border transition-colors cursor-pointer ${
                          showDirectiveModal
                            ? 'bg-amber-500/20 text-amber-300 border-amber-500/40'
                            : 'bg-zinc-800/70 hover:bg-zinc-700/80 text-zinc-300 border-white/5'
                        }`}
                      >
                        <svg className="w-3.5 h-3.5 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>
                        <span>Directive</span>
                      </button>

                      {/* Directive Popover Menu */}
                      {showDirectiveModal && (
                        <div className="absolute bottom-full left-0 mb-2 w-80 bg-[#121622] border border-white/15 rounded-xl shadow-2xl p-3 z-50 space-y-3 animate-in fade-in slide-in-from-bottom-2 duration-150">
                          <div className="flex items-center justify-between border-b border-white/10 pb-2">
                            <span className="text-xs font-semibold text-white">Custom Translation Directive</span>
                            <button
                              onClick={() => setShowDirectiveModal(false)}
                              className="text-zinc-400 hover:text-white text-xs cursor-pointer"
                            >
                              ✕
                            </button>
                          </div>
                          <div className="space-y-1.5">
                            <div className="text-[11px] text-zinc-400 font-medium">Quick Style Presets:</div>
                            <div className="flex flex-wrap gap-1">
                              {[
                                'Friendly, casual tone',
                                'Strict formal business tone',
                                'Protect brand names & terms',
                                'Short UI button labels',
                                'Latin American Spanish vocabulary',
                              ].map((preset, idx) => (
                                <button
                                  key={idx}
                                  type="button"
                                  onClick={() => {
                                    setCustomDirectiveText(preset)
                                  }}
                                  className="text-[10px] px-2 py-0.5 rounded-md bg-zinc-800 hover:bg-zinc-700 text-zinc-300 border border-white/5 cursor-pointer"
                                >
                                  {preset}
                                </button>
                              ))}
                            </div>
                          </div>
                          <div className="space-y-1.5">
                            <textarea
                              value={customDirectiveText}
                              onChange={(e) => setCustomDirectiveText(e.target.value)}
                              placeholder="e.g. Always translate with a casual tone, and never translate product name 'Acme'"
                              rows={2}
                              className="w-full bg-black/40 border border-white/10 rounded-lg p-2 text-xs text-zinc-100 placeholder:text-zinc-500 focus:outline-none focus:border-amber-500/50 resize-none"
                            />
                          </div>
                          <div className="flex items-center justify-end gap-2 pt-1">
                            <button
                              type="button"
                              onClick={() => {
                                if (customDirectiveText.trim()) {
                                  setCentralCopilotInput((prev) =>
                                    prev
                                      ? `${prev}\n[Directive: ${customDirectiveText.trim()}]`
                                      : `Translate with directive: ${customDirectiveText.trim()}`
                                  )
                                  setShowDirectiveModal(false)
                                  toast.success('Directive inserted into prompt')
                                }
                              }}
                              className="px-2.5 py-1 rounded-md bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 border border-amber-500/40 text-xs cursor-pointer font-medium"
                            >
                              Insert into Prompt
                            </button>
                          </div>
                        </div>
                      )}
                    </div>

                    {/* Browse Prompts Popover Button */}
                    <div className="relative">
                      <button
                        type="button"
                        onClick={() => {
                          setShowBrowsePromptsModal(!showBrowsePromptsModal)
                          setShowAttachModal(false)
                          setShowDirectiveModal(false)
                        }}
                        className={`px-2.5 py-1 rounded-lg text-xs flex items-center gap-1.5 border transition-colors cursor-pointer ${
                          showBrowsePromptsModal
                            ? 'bg-purple-500/20 text-purple-300 border-purple-500/40'
                            : 'bg-zinc-800/70 hover:bg-zinc-700/80 text-zinc-300 border-white/5'
                        }`}
                      >
                        <svg className="w-3.5 h-3.5 text-zinc-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="m18 15-6-6-6 6"/></svg>
                        <span>Browse Prompts</span>
                      </button>

                      {/* Browse Prompts Library Modal/Popover */}
                      {showBrowsePromptsModal && (
                        <div className="absolute bottom-full left-0 mb-2 w-96 bg-[#121622] border border-white/15 rounded-xl shadow-2xl p-3 z-50 max-h-[380px] overflow-y-auto space-y-3 animate-in fade-in slide-in-from-bottom-2 duration-150 custom-scrollbar">
                          <div className="flex items-center justify-between border-b border-white/10 pb-2 sticky top-0 bg-[#121622] z-10">
                            <div>
                              <span className="text-xs font-semibold text-white">Prompt Template Library</span>
                              <p className="text-[10px] text-zinc-400">Click to insert into message box or run directly</p>
                            </div>
                            <button
                              onClick={() => setShowBrowsePromptsModal(false)}
                              className="text-zinc-400 hover:text-white text-xs cursor-pointer"
                            >
                              ✕
                            </button>
                          </div>

                          {[
                            {
                              category: 'AST & Codebase Inspection',
                              prompts: [
                                { title: 'Scan Repository AST', text: 'Scan repository and audit hardcoded UI strings across all components' },
                                { title: 'Inspect String Safety', text: 'Inspect string context and verify ICU variable placeholder safety' },
                              ],
                            },
                            {
                              category: 'Cultural Translation & Localization',
                              prompts: [
                                { title: 'Translate Missing Keys', text: 'Translate missing keys into Spanish, German and Japanese in a casual tone' },
                                { title: 'Estimate Token & Pricing Plan', text: 'Plan localization token cost and batch allocation for all target locales' },
                              ],
                            },
                            {
                              category: '4-Tier Critic & Quality Gate',
                              prompts: [
                                { title: 'Run 4-Tier Critic Verification', text: 'Run 4-tier verification critic on all locales (AST, ICU, Expansion, Parity)' },
                                { title: 'Prune Dead Keys', text: 'Prune dead translation keys not referenced in source code' },
                                { title: 'Run System Diagnostics', text: 'Run Doctor system diagnostics and verify API credentials' },
                              ],
                            },
                            {
                              category: 'Global SEO & Regional Growth',
                              prompts: [
                                { title: 'Simulate Google SERP Previews', text: 'Simulate Google SERP desktop and mobile search previews for Japanese and Spanish' },
                                { title: 'Weave High-Converting Keywords', text: 'Weave high-converting regional keywords into localized product copy' },
                              ],
                            },
                            {
                              category: 'Platform Execution & Setup',
                              prompts: [
                                { title: 'Audit Setup & Missing Config', text: 'What repository settings, keys, or information are missing?' },
                                { title: 'Trigger Background Localization Job', text: 'Trigger platform localization job on main branch' },
                                { title: 'Query Recent Jobs & PR Status', text: 'Show recent platform jobs and check GitHub PR status' },
                                { title: 'Check Safety Snapshots & Rollback', text: 'Show rollback checkpoints and snapshot history' },
                              ],
                            },
                          ].map((cat, catIdx) => (
                            <div key={catIdx} className="space-y-1.5">
                              <div className="text-[10px] font-mono uppercase text-sky-400 tracking-wider font-semibold">
                                {cat.category}
                              </div>
                              <div className="space-y-1">
                                {cat.prompts.map((p, pIdx) => (
                                  <div
                                    key={pIdx}
                                    className="p-2 rounded-lg bg-zinc-900/60 border border-white/5 hover:border-white/15 transition-all flex items-center justify-between gap-2 group"
                                  >
                                    <div
                                      onClick={() => {
                                        setCentralCopilotInput(p.text)
                                        setShowBrowsePromptsModal(false)
                                        toast.info('Template inserted into message box')
                                      }}
                                      className="flex-1 cursor-pointer"
                                    >
                                      <div className="text-xs font-medium text-zinc-200 group-hover:text-sky-300">{p.title}</div>
                                      <div className="text-[10px] text-zinc-400 line-clamp-1">{p.text}</div>
                                    </div>
                                    <div className="flex items-center gap-1 shrink-0">
                                      <button
                                        type="button"
                                        onClick={() => {
                                          setCentralCopilotInput(p.text)
                                          setShowBrowsePromptsModal(false)
                                          toast.info('Template inserted')
                                        }}
                                        className="px-2 py-0.5 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-[10px] cursor-pointer"
                                        title="Insert into input"
                                      >
                                        Insert
                                      </button>
                                      <button
                                        type="button"
                                        onClick={() => {
                                          setShowBrowsePromptsModal(false)
                                          sendCentralCopilotMessage(p.text)
                                        }}
                                        className="px-2 py-0.5 rounded bg-sky-500/20 hover:bg-sky-500/30 text-sky-300 text-[10px] cursor-pointer font-medium"
                                        title="Run immediately"
                                      >
                                        Run
                                      </button>
                                    </div>
                                  </div>
                                ))}
                              </div>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>
                  <div className="text-[11px] font-mono text-zinc-500">
                    {centralCopilotInput.length} / 3,000
                  </div>
                </div>
              </div>

              <p className="text-[10px] text-center text-zinc-500 mt-2 font-mono">
                langPeanut may generate suggestions requiring developer review. Engine: Google Genkit Go.
              </p>
            </div>
          </div>

          {/* Right Sidebar: Recent Tasks / Quick Actions matching reference */}
          <div className="w-full lg:w-80 glass-panel rounded-2xl flex flex-col border border-white/10 bg-[#090c13] shadow-xl p-4 space-y-4 shrink-0 overflow-y-auto custom-scrollbar">
            <div className="flex items-center justify-between border-b border-white/10 pb-3">
              <h3 className="text-xs font-bold text-white font-mono uppercase tracking-wider">
                Recent Tasks (7)
              </h3>
              <span className="text-[10px] font-mono text-zinc-400">Workflows</span>
            </div>

            <div className="space-y-2">
              {[
                { title: 'Scan Repository AST', desc: 'Audit strings & build coverage matrix', cmd: 'Scan repository and calculate coverage matrix' },
                { title: 'Translate Missing Keys', desc: 'Batch translate with ICU variable safety', cmd: 'Translate missing keys into Spanish, German and Japanese' },
                { title: '4-Tier Critic Verification', desc: 'Check syntax, variables & expansion', cmd: 'Execute 4-tier verification critic on all locales' },
                { title: 'Simulate Google SERP', desc: 'Generate 600px search previews', cmd: 'Simulate Japanese Google SERP preview' },
                { title: 'Scout Brand Persona', desc: 'Infer tone & glossary lexicon', cmd: 'Scout brand persona and recommended voice' },
                { title: 'Prune Dead Keys', desc: 'Clean orphaned dictionary entries', cmd: 'Analyze and prune dead unused keys' },
                { title: 'Manage Checkpoints', desc: 'Rollback snapshots & safety points', cmd: 'List checkpoints or undo last changes' },
              ].map((task, tIdx) => (
                <button
                  key={tIdx}
                  onClick={() => sendCentralCopilotMessage(task.cmd)}
                  className="w-full text-left p-3 rounded-xl bg-zinc-900/60 hover:bg-zinc-800/80 border border-white/5 hover:border-sky-500/30 transition-all group cursor-pointer space-y-1 shadow-xs"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold text-zinc-200 group-hover:text-sky-300 transition-colors">
                      {task.title}
                    </span>
                    <span className="text-[10px] text-zinc-500 font-mono">Run →</span>
                  </div>
                  <p className="text-[11px] text-zinc-400 line-clamp-1">{task.desc}</p>
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

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
                <div className="w-10 h-10 rounded-xl bg-sky-500/20 border border-sky-500/30 text-sky-400 flex items-center justify-center font-bold text-xs font-mono">
                  DOC
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
                {runningDoctor ? 'Analyzing Codebase…' : 'Run Health Check'}
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
                        <span className={`text-[10px] font-mono px-1.5 py-0.5 rounded font-bold ${
                          iss.severity === 'ERROR'
                            ? 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                            : iss.severity === 'WARNING'
                            ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                            : 'bg-sky-500/10 text-sky-400 border border-sky-500/20'
                        }`}>
                          {iss.severity}
                        </span>
                        <div className="flex-1 min-w-0">
                          <div className="font-semibold text-white">{iss.title}</div>
                          <div className="text-slate-400 text-[11px] mt-0.5">{iss.description}</div>
                          {iss.auto_fix_hint && (
                            <div className="text-sky-300 text-[11px] mt-1 font-mono">Fix: {iss.auto_fix_hint}</div>
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
                {discoveringPersona ? 'Mining Assets…' : 'Auto-Discover Persona & Tone'}
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
                    const nextModel = PROVIDER_MODELS[p]?.models[0] || ''
                    setSelectedProvider(p)
                    setSelectedModel(nextModel)
                    handleQuickSwitchModel(p, nextModel)
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
                  onChange={(e) => {
                    const nextModel = e.target.value
                    setSelectedModel(nextModel)
                    handleQuickSwitchModel(selectedProvider, nextModel)
                  }}
                  className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white focus:outline-none focus:border-sky-400"
                >
                  {PROVIDER_MODELS[selectedProvider]?.models.map((m) => {
                    const d = PROVIDER_MODELS[selectedProvider]?.details?.[m]
                    return (
                      <option key={m} value={m}>
                        {d ? `${d.name} (${d.inputPrice} in / ${d.outputPrice} out)` : m}
                      </option>
                    )
                  })}
                </select>
              </div>
            </div>

            {/* Model Architecture & Pricing Specs Badge */}
            {(() => {
              const currentDetail = PROVIDER_MODELS[selectedProvider]?.details?.[selectedModel]
              if (!currentDetail) return null
              return (
                <div className="p-3.5 rounded-xl bg-slate-900/90 border border-sky-500/20 grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
                  <div>
                    <span className="text-[10px] uppercase font-bold text-slate-500 block">Context Limit</span>
                    <span className="font-mono text-sky-300 font-bold">{currentDetail.contextWindow} tokens</span>
                  </div>
                  <div>
                    <span className="text-[10px] uppercase font-bold text-slate-500 block">Max Output</span>
                    <span className="font-mono text-slate-200 font-bold">{currentDetail.maxOutput} tokens</span>
                  </div>
                  <div>
                    <span className="text-[10px] uppercase font-bold text-slate-500 block">Input / 1M Tok</span>
                    <span className="font-mono text-emerald-400 font-bold">{currentDetail.inputPrice}</span>
                  </div>
                  <div>
                    <span className="text-[10px] uppercase font-bold text-slate-500 block">Output / 1M Tok</span>
                    <span className="font-mono text-emerald-400 font-bold">{currentDetail.outputPrice}</span>
                  </div>
                  {currentDetail.desc && (
                    <div className="col-span-2 sm:col-span-4 pt-1 text-[11px] text-slate-400 border-t border-white/[0.04]">
                      {currentDetail.desc}
                    </div>
                  )}
                </div>
              )
            })()}

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
                    {repo.settings?.has_api_key_override ? (
                      <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20 font-bold">
                        Repo Override Active
                      </span>
                    ) : (
                      <span className={cn(
                        "text-[10px] font-mono px-2 py-0.5 rounded border font-semibold",
                        isProviderConfigured(selectedProvider)
                          ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                          : "bg-amber-500/10 text-amber-400 border-amber-500/20"
                      )}>
                        {isProviderConfigured(selectedProvider)
                          ? `✓ Inheriting Global Vault Key (${PROVIDER_MODELS[selectedProvider]?.label || selectedProvider})`
                          : `⚠ No Key in Global Vault for ${PROVIDER_MODELS[selectedProvider]?.label || selectedProvider}`}
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
                    className="rounded-xl bg-purple-600/20 hover:bg-purple-600/30 border border-purple-500/40 text-purple-300 text-xs font-semibold px-3.5 py-1.5 flex items-center gap-1.5 cursor-pointer transition-all shrink-0"
                  >
                    {discoveringPersona ? 'Mining Assets…' : 'Auto-Discover Persona & Tone'}
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

            {/* ── Section 5: GitHub Push & Webhook Autopilot Engine ── */}
            <div className="pt-5 border-t border-white/[0.08] space-y-4">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                <div>
                  <h3 className="text-xs font-bold text-white uppercase tracking-wider flex items-center gap-2">
                    <span className="text-emerald-400">Section 5</span> — GitHub Push & Webhook Autopilot Engine
                    <span className={cn(
                      "text-[10px] font-mono px-2 py-0.5 rounded-full border font-semibold",
                      webhookPushEnabled
                        ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/30"
                        : "bg-slate-800 text-slate-400 border-white/10"
                    )}>
                      {webhookPushEnabled ? "● Autopilot Active" : "○ Manual Trigger Only"}
                    </span>
                  </h3>
                  <p className="text-[11px] text-slate-400 mt-0.5">
                    Automatically trigger localization runs when code is pushed to GitHub or when team members mention @langpeanut in PRs.
                  </p>
                </div>

                {/* Autopilot Master Switch */}
                <div className="flex items-center gap-2.5 bg-slate-900/90 border border-white/10 px-3.5 py-1.5 rounded-xl shrink-0">
                  <span className="text-xs font-semibold text-slate-300">
                    {webhookPushEnabled ? 'Push Trigger ON' : 'Push Trigger OFF'}
                  </span>
                  <button
                    type="button"
                    onClick={() => setWebhookPushEnabled(!webhookPushEnabled)}
                    className={cn(
                      "w-11 h-6 flex items-center rounded-full p-1 cursor-pointer transition-colors duration-200",
                      webhookPushEnabled ? "bg-emerald-600 justify-end" : "bg-slate-700 justify-start"
                    )}
                  >
                    <div className="bg-white w-4 h-4 rounded-full shadow-md transform transition-transform" />
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 pt-2">
                {/* 1. Branch Strategy */}
                <div>
                  <label className="text-[11px] font-semibold text-slate-300 block mb-1">
                    Monitored Branch Strategy
                  </label>
                  <select
                    value={webhookBranchFilter}
                    onChange={(e) => setWebhookBranchFilter(e.target.value as any)}
                    className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white focus:outline-none focus:border-sky-400"
                  >
                    <option value="default_branch">Default Branch Only ({repo.DefaultBranch || 'main'})</option>
                    <option value="all">All Branches (Trigger on any branch push)</option>
                    <option value="custom">Custom Branch Filter (Globs / Patterns)</option>
                  </select>
                  <p className="text-[10px] text-slate-500 mt-1">
                    {webhookBranchFilter === 'default_branch' && `Only pushes to ${repo.DefaultBranch || 'main'} will trigger automated localization.`}
                    {webhookBranchFilter === 'all' && 'Every branch push to this repo will trigger an autonomous localization pass.'}
                    {webhookBranchFilter === 'custom' && 'Only branches matching the custom glob patterns below will trigger jobs.'}
                  </p>
                </div>

                {/* 2. Target Action */}
                <div>
                  <label className="text-[11px] font-semibold text-slate-300 block mb-1">
                    Autopilot Trigger Action
                  </label>
                  <select
                    value={webhookAction}
                    onChange={(e) => setWebhookAction(e.target.value as any)}
                    className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white focus:outline-none focus:border-sky-400"
                  >
                    <option value="auto_pr">Open Automated Pull Request (Recommended)</option>
                    <option value="direct_commit">Direct Commit & Push to Pushed Branch</option>
                    <option value="draft_pr">Open Draft Pull Request</option>
                  </select>
                  <p className="text-[10px] text-slate-500 mt-1">
                    {webhookAction === 'auto_pr' && 'Creates a dedicated branch and opens a verified Pull Request for review.'}
                    {webhookAction === 'direct_commit' && 'Commits translation catalogs directly back to the target branch.'}
                    {webhookAction === 'draft_pr' && 'Opens a draft Pull Request without notifying reviewers.'}
                  </p>
                </div>

                {/* 3. PR Bot Comment Commands */}
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <label className="text-[11px] font-semibold text-slate-300 block">
                      @langpeanut PR Bot Commands
                    </label>
                    <button
                      type="button"
                      onClick={() => setWebhookPRCommentsEnabled(!webhookPRCommentsEnabled)}
                      className={cn(
                        "w-8 h-4 flex items-center rounded-full p-0.5 cursor-pointer transition-colors duration-200",
                        webhookPRCommentsEnabled ? "bg-sky-600 justify-end" : "bg-slate-700 justify-start"
                      )}
                    >
                      <div className="bg-white w-3 h-3 rounded-full shadow-sm" />
                    </button>
                  </div>
                  <div className="p-2.5 rounded-xl bg-slate-900/90 border border-white/10 text-xs">
                    <span className="text-[11px] text-slate-300 font-medium block">
                      {webhookPRCommentsEnabled ? '✓ PR Bot Mentions Active' : '✕ PR Bot Mentions Disabled'}
                    </span>
                    <p className="text-[10px] text-slate-500 mt-0.5">
                      Enables commands like <code className="text-sky-400 font-mono">@langpeanut translate</code> in PR comments.
                    </p>
                  </div>
                </div>

                {/* Custom Branch Pattern (Conditional) */}
                {webhookBranchFilter === 'custom' && (
                  <div className="sm:col-span-2 lg:col-span-3">
                    <label className="text-[11px] font-semibold text-slate-300 block mb-1">
                      Custom Monitored Branch Patterns (Comma-separated)
                    </label>
                    <input
                      type="text"
                      value={webhookCustomBranches}
                      onChange={(e) => setWebhookCustomBranches(e.target.value)}
                      placeholder="e.g. main, master, release/*, feat/i18n-*"
                      className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white font-mono placeholder:text-slate-600 focus:border-sky-400 focus:outline-none"
                    />
                    <p className="text-[10px] text-slate-500 mt-1">
                      Supports exact branch names and glob wildcards (e.g. <code className="text-slate-400 font-mono">release/*</code>).
                    </p>
                  </div>
                )}

                {/* Custom Branch Prefix */}
                <div>
                  <label className="text-[11px] font-semibold text-slate-300 block mb-1">
                    Automated PR Branch Prefix
                  </label>
                  <input
                    type="text"
                    value={webhookCustomBranchPrefix}
                    onChange={(e) => setWebhookCustomBranchPrefix(e.target.value)}
                    placeholder="e.g. langpeanut/i18n-, l10n/, i18n/auto-"
                    className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white font-mono placeholder:text-slate-600 focus:border-sky-400 focus:outline-none"
                  />
                  <p className="text-[10px] text-slate-500 mt-1">
                    Branch name format: <code className="text-slate-400 font-mono">{webhookCustomBranchPrefix || 'langpeanut/i18n-'}[timestamp]-[sha]</code>
                  </p>
                </div>

                {/* Ignored / Monitored File Paths */}
                <div className="sm:col-span-2">
                  <label className="text-[11px] font-semibold text-slate-300 block mb-1">
                    Path Filter / Monitored Folders (Optional)
                  </label>
                  <input
                    type="text"
                    value={webhookPathFilter}
                    onChange={(e) => setWebhookPathFilter(e.target.value)}
                    placeholder="e.g. src/**, app/**, lib/** (leave empty to monitor all source files)"
                    className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white font-mono placeholder:text-slate-600 focus:border-sky-400 focus:outline-none"
                  />
                  <p className="text-[10px] text-slate-500 mt-1">
                    Pushes that touch only excluded assets or docs will be intelligently skipped.
                  </p>
                </div>
              </div>
            </div>

            {/* ── Section 6: UI Integration Directive (UI Switcher Agent) ── */}
            <div className="pt-5 border-t border-white/[0.08] space-y-3">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-xs font-bold text-white uppercase tracking-wider flex items-center gap-1.5">
                    <span className="text-sky-400">Section 6</span> — UI Integration Directive (UI Switcher Agent)
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

          {/* ── Section 6: Danger Zone (Reset Data & Delete Repository) ── */}
          <div className="glass-panel p-6 rounded-2xl border border-rose-500/20 bg-rose-950/10 space-y-4">
            <div>
              <h3 className="text-xs font-bold text-rose-300 uppercase tracking-wider flex items-center gap-2">
                <span>Danger Zone</span> — Reset Localization State & Repository Management
              </h3>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Permanently purge derived translation matrices, job run history, and SEO intelligence to start fresh from the beginning, or remove this repository.
              </p>
            </div>

            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 pt-3 border-t border-rose-500/10">
              <div className="space-y-0.5">
                <h4 className="text-xs font-bold text-slate-200">Reset Repository Data (Start Fresh)</h4>
                <p className="text-[11px] text-slate-400">
                  Clears all translation matrix key-values, job logs, and SEO competitor caches. Preserves your settings and API keys.
                </p>
              </div>
              <button
                type="button"
                onClick={handleResetRepoData}
                disabled={resettingData}
                className="rounded-xl bg-rose-600/20 hover:bg-rose-600/30 border border-rose-500/40 text-rose-300 hover:text-rose-200 text-xs font-semibold px-4 py-2 flex items-center gap-1.5 transition-all cursor-pointer shrink-0"
              >
                {resettingData ? 'Resetting Data…' : 'Reset All Localization Data'}
              </button>
            </div>

            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 pt-3 border-t border-rose-500/10">
              <div className="space-y-0.5">
                <h4 className="text-xs font-bold text-slate-200">Delete Repository</h4>
                <p className="text-[11px] text-slate-400">
                  Permanently deletes this repository, its configuration, translation matrices, and cached git mirrors from langPeanut Cloud.
                </p>
              </div>
              <button
                type="button"
                onClick={handleDeleteRepo}
                disabled={deletingRepo}
                className="rounded-xl bg-rose-600 hover:bg-rose-500 disabled:bg-rose-900 text-white text-xs font-semibold px-4 py-2 shadow-lg shadow-rose-900/40 flex items-center gap-1.5 transition-all cursor-pointer shrink-0"
              >
                {deletingRepo ? 'Deleting Repo…' : 'Delete Repository'}
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
                {pruningKeys ? 'Pruning…' : 'Prune Dead Keys'}
              </button>
              <button
                type="button"
                onClick={copyGitHubActionsYAML}
                className="rounded-xl bg-white/[0.05] hover:bg-white/[0.1] border border-white/10 text-slate-300 text-xs font-medium px-3.5 py-1.5 cursor-pointer"
              >
                {copiedYAML ? 'Copied' : 'Export Actions YAML'}
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
                                      AI
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
                                      AI Copilot
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
                    <div className="w-8 h-8 rounded-xl bg-purple-500/20 border border-purple-500/40 text-purple-300 flex items-center justify-center font-bold text-xs font-mono shadow-inner">
                      AI
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
                    className="text-slate-400 hover:text-white p-1 text-sm font-bold cursor-pointer"
                  >
                    Close
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
                      { id: 'shorter', label: 'Make Shorter (-30%)', hint: 'Compact synonym for mobile buttons' },
                      { id: 'casual', label: 'Casual & Friendly', hint: 'Warm colloquial phrasing' },
                      { id: 'formal', label: 'Formal & Enterprise', hint: 'B2B professional phrasing' },
                      { id: 'brand_safe', label: 'Brand Safe', hint: 'Keep technical names untouched' },
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
                        {copilotState.loading ? 'Generating…' : 'Regenerate'}
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

      {/* ─── TAB: SEO & GROWTH STUDIO ────────────────────────────────────────── */}
      {activeTab === 'seo' && (
        <div className="space-y-6">
          {/* Header & Intake Bar */}
          <div className="glass-panel p-6 rounded-3xl border border-sky-500/20 bg-gradient-to-r from-sky-950/20 via-slate-900/40 to-indigo-950/20 space-y-5">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div>
                <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-sky-500/10 border border-sky-500/30 text-sky-300 text-xs font-semibold mb-2">
                  <span>Autonomous Multilingual SEO & Market Growth Engine</span>
                </div>
                <h2 className="text-xl font-bold text-white tracking-tight">Regional SERP & Keyword Optimization Studio</h2>
                <p className="text-xs text-slate-400 mt-1 max-w-2xl leading-relaxed">
                  Transform technical AST localization keys into high-ranking, region-dominating search copy.
                  Scout local competitors, discover intent-rich native keywords, simulate live Google SERPs, and preview growth metrics before deploying.
                </p>
              </div>

              <div className="flex items-center gap-2.5">
                <button
                  type="button"
                  onClick={handleTriggerSEOScout}
                  disabled={scoutingSEO}
                  className="rounded-xl bg-sky-600/20 hover:bg-sky-600/30 border border-sky-500/40 text-sky-200 font-semibold px-4 py-2.5 text-xs transition-all flex items-center gap-2 cursor-pointer shadow-lg shadow-sky-950/40"
                >
                  <svg className={`w-3.5 h-3.5 ${scoutingSEO ? 'animate-spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                    <circle cx="11" cy="11" r="8" />
                    <line x1="21" y1="21" x2="16.65" y2="16.65" />
                  </svg>
                  <span>{scoutingSEO ? 'Scouting SERP…' : 'Scout Competitors & Keywords'}</span>
                </button>

                <button
                  type="button"
                  onClick={handleTriggerSEOOptimize}
                  disabled={optimizingSEO}
                  className="rounded-xl bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 text-white font-semibold px-5 py-2.5 text-xs shadow-lg shadow-blue-600/30 transition-all flex items-center gap-2 cursor-pointer"
                >
                  <svg className={`w-3.5 h-3.5 ${optimizingSEO ? 'animate-spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                    <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" />
                  </svg>
                  <span>{optimizingSEO ? 'Weaving Copy…' : 'Run Semantic Copy Weaver'}</span>
                </button>
              </div>
            </div>

            {/* AST Discovery & Domain Overview Readiness Banner */}
            <div className="pt-2">
              {(seoData?.extracted_keys_count && seoData.extracted_keys_count > 0 && seoCategory) ? (
                <div className="flex items-center justify-between px-3 py-2 rounded-xl bg-sky-500/10 border border-sky-500/20 text-xs text-sky-200">
                  <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse shrink-0" />
                    <span>
                      <strong>AI Domain Grounded</strong>: Analyzed <strong>{seoData.extracted_keys_count}</strong> extracted UI strings. Inferred Domain: <em>"{seoCategory}"</em>
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={handleTriggerAnalyzeDomain}
                      disabled={analyzingDomain}
                      className="text-[11px] font-medium px-2.5 py-1 rounded bg-sky-600/30 hover:bg-sky-600/50 text-sky-200 border border-sky-500/30 flex items-center gap-1.5 cursor-pointer transition-all"
                    >
                      <svg className={`w-3 h-3 ${analyzingDomain ? 'animate-spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                        <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" />
                      </svg>
                      <span>{analyzingDomain ? 'Analyzing...' : 'Re-Analyze with AI'}</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => setActiveTab('matrix')}
                      className="text-[11px] font-mono text-sky-400 hover:text-sky-300 underline cursor-pointer"
                    >
                      View Matrix
                    </button>
                  </div>
                </div>
              ) : (seoData?.extracted_keys_count && seoData.extracted_keys_count > 0) ? (
                <div className="flex items-center justify-between px-3 py-2 rounded-xl bg-indigo-500/10 border border-indigo-500/20 text-xs text-indigo-200">
                  <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-indigo-400 shrink-0" />
                    <span>
                      <strong>Strings Extracted ({seoData.extracted_keys_count} keys)</strong>: Run AI Domain Analysis to let the LLM inspect your UI copy and infer product overview.
                    </span>
                  </div>
                  <button
                    type="button"
                    onClick={handleTriggerAnalyzeDomain}
                    disabled={analyzingDomain}
                    className="text-[11px] font-medium px-3 py-1 rounded bg-indigo-600/40 hover:bg-indigo-600/60 text-indigo-100 border border-indigo-500/40 flex items-center gap-1.5 cursor-pointer transition-all shadow-sm"
                  >
                    <svg className={`w-3 h-3 ${analyzingDomain ? 'animate-spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                      <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" />
                    </svg>
                    <span>{analyzingDomain ? 'Analyzing UI Strings...' : 'Analyze Domain with AI'}</span>
                  </button>
                </div>
              ) : (
                <div className="flex items-center justify-between px-3 py-2 rounded-xl bg-amber-500/10 border border-amber-500/20 text-xs text-amber-200">
                  <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-amber-400 shrink-0" />
                    <span>
                      <strong>No Keys in Matrix Yet</strong>: Extract UI strings from your codebase so the AI Agent can analyze your software domain and value proposition.
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={handleTriggerAnalyzeDomain}
                      disabled={analyzingDomain}
                      className="text-[11px] font-medium px-2.5 py-1 rounded bg-amber-600/30 hover:bg-amber-600/50 text-amber-100 border border-amber-500/40 flex items-center gap-1.5 cursor-pointer transition-all"
                    >
                      <svg className={`w-3 h-3 ${analyzingDomain ? 'animate-spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                        <circle cx="11" cy="11" r="8" />
                        <line x1="21" y1="21" x2="16.65" y2="16.65" />
                      </svg>
                      <span>{analyzingDomain ? 'Extracting...' : 'Extract & Analyze with AI'}</span>
                    </button>
                    <button
                      type="button"
                      onClick={triggerJob}
                      className="text-[11px] font-medium px-2.5 py-1 rounded bg-indigo-600/30 hover:bg-indigo-600/50 text-indigo-200 border border-indigo-500/30 cursor-pointer"
                    >
                      Trigger Full Job
                    </button>
                  </div>
                </div>
              )}
            </div>

            {/* Strategic Controls Bar */}
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-3 pt-3 border-t border-white/[0.06]">
              {/* Target Locale Selector */}
              <div className="space-y-1.5">
                <label className="text-[11px] font-semibold text-slate-300 uppercase tracking-wider">Target Market / Language</label>
                <select
                  value={seoLocale}
                  onChange={(e) => setSeoLocale(e.target.value)}
                  className="w-full rounded-xl bg-slate-900/80 border border-white/10 text-white text-xs px-3 py-2 font-medium focus:border-sky-500 focus:outline-none"
                >
                  {Array.from(new Set(['en', ...selectedLocales])).map((loc) => {
                    const found = AVAILABLE_LANGUAGES.find((l) => l.code === loc)
                    return (
                      <option key={loc} value={loc}>
                        {found ? `${found.label} (${found.native})` : loc.toUpperCase()}
                      </option>
                    )
                  })}
                </select>
              </div>

              {/* Commercial Goal Selector */}
              <div className="space-y-1.5">
                <label className="text-[11px] font-semibold text-slate-300 uppercase tracking-wider">Growth & SEO Goal</label>
                <select
                  value={seoGoal}
                  onChange={(e) => setSeoGoal(e.target.value)}
                  className="w-full rounded-xl bg-slate-900/80 border border-white/10 text-white text-xs px-3 py-2 font-medium focus:border-sky-500 focus:outline-none"
                >
                  <option value="traffic">Top-of-Funnel Search Traffic</option>
                  <option value="conversion">High-Intent Buyer Conversion</option>
                  <option value="trust">Local Trust & Regional Compliance</option>
                </select>
              </div>

              {/* Key Scope Tier */}
              <div className="space-y-1.5">
                <label className="text-[11px] font-semibold text-slate-300 uppercase tracking-wider">Optimization Scope</label>
                <select
                  value={seoScope}
                  onChange={(e) => setSeoScope(e.target.value)}
                  className="w-full rounded-xl bg-slate-900/80 border border-white/10 text-white text-xs px-3 py-2 font-medium focus:border-sky-500 focus:outline-none"
                >
                  <option value="high_impact">High-Impact Keys Only (Hero, Meta, FAQs)</option>
                  <option value="full_site">Full-Site Optimization (All Keys)</option>
                </select>
              </div>

              {/* Competitors Input */}
              <div className="space-y-1.5">
                <label className="text-[11px] font-semibold text-slate-300 uppercase tracking-wider">Competitors (Optional / Auto-Scout)</label>
                <input
                  type="text"
                  placeholder="e.g. competitor.jp, rival.de"
                  value={seoCompetitorInput}
                  onChange={(e) => setSeoCompetitorInput(e.target.value)}
                  onBlur={handleSaveSEOStrategy}
                  className="w-full rounded-xl bg-slate-900/80 border border-white/10 text-white text-xs px-3 py-2 placeholder:text-slate-600 focus:border-sky-500 focus:outline-none"
                />
              </div>
            </div>

            {/* Product Domain & Value Proposition Customizer */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3 pt-3 border-t border-white/[0.06]">
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <label className="text-[11px] font-semibold text-slate-300 uppercase tracking-wider">Product Domain / Category</label>
                  <div className="flex items-center gap-1.5">
                    <button
                      type="button"
                      onClick={handleTriggerAnalyzeDomain}
                      disabled={analyzingDomain}
                      className="text-[10px] px-2 py-0.5 rounded bg-sky-500/20 hover:bg-sky-500/30 text-sky-300 border border-sky-500/30 transition-all flex items-center gap-1 cursor-pointer"
                    >
                      <svg className={`w-2.5 h-2.5 ${analyzingDomain ? 'animate-spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                        <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" />
                      </svg>
                      <span>Analyze with AI</span>
                    </button>
                    {['Localization AI', 'Developer Tool', 'SaaS Platform'].map((preset) => (
                      <button
                        key={preset}
                        type="button"
                        onClick={() => {
                          setSeoCategory(preset)
                          setTimeout(handleSaveSEOStrategy, 50)
                        }}
                        className="text-[10px] px-1.5 py-0.5 rounded bg-white/[0.04] hover:bg-sky-500/20 text-slate-400 hover:text-sky-300 border border-white/[0.06] transition-all cursor-pointer"
                      >
                        {preset}
                      </button>
                    ))}
                  </div>
                </div>
                <input
                  type="text"
                  placeholder="Click 'Analyze with AI' or type category..."
                  value={seoCategory}
                  onChange={(e) => setSeoCategory(e.target.value)}
                  onBlur={handleSaveSEOStrategy}
                  className="w-full rounded-xl bg-slate-900/80 border border-white/10 text-white text-xs px-3 py-2 font-medium focus:border-sky-500 focus:outline-none"
                />
              </div>

              <div className="space-y-1.5">
                <label className="text-[11px] font-semibold text-slate-300 uppercase tracking-wider">Product Overview & Core Value Prop</label>
                <input
                  type="text"
                  placeholder="2-sentence product description inferred from extracted UI copy..."
                  value={seoDescription}
                  onChange={(e) => setSeoDescription(e.target.value)}
                  onBlur={handleSaveSEOStrategy}
                  className="w-full rounded-xl bg-slate-900/80 border border-white/10 text-white text-xs px-3 py-2 focus:border-sky-500 focus:outline-none"
                />
              </div>
            </div>
          </div>

          {/* Predictive Growth & Metrics Scorecard */}
          {(() => {
            const metrics = seoData?.metrics?.[seoLocale]
            return (
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
                <div className="glass-panel p-4 rounded-2xl space-y-1 border border-emerald-500/20 bg-emerald-950/10">
                  <div className="flex items-center justify-between">
                    <span className="text-[11px] font-semibold uppercase tracking-wider text-emerald-400">Target Search Reach</span>
                    {metrics ? (
                      <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300">
                        +{metrics.search_volume_uplift_pct.toFixed(0)}%
                      </span>
                    ) : (
                      <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400">
                        Pending Analysis
                      </span>
                    )}
                  </div>
                  <p className="text-2xl font-bold text-white">
                    {metrics ? (
                      <>
                        {metrics.search_volume_optimized.toLocaleString()}{' '}
                        <span className="text-xs font-normal text-slate-400">searches/mo</span>
                      </>
                    ) : (
                      <span className="text-slate-500 font-normal text-lg">Not analyzed</span>
                    )}
                  </p>
                  <p className="text-[11px] text-slate-400">
                    {metrics
                      ? `Baseline without SEO: ${metrics.search_volume_baseline.toLocaleString()}/mo`
                      : 'Run Semantic Copy Weaver to calculate reach'}
                  </p>
                </div>

                <div className="glass-panel p-4 rounded-2xl space-y-1 border border-sky-500/20 bg-sky-950/10">
                  <div className="flex items-center justify-between">
                    <span className="text-[11px] font-semibold uppercase tracking-wider text-sky-400">Projected SERP CTR</span>
                    {metrics ? (
                      <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-sky-500/20 text-sky-300">
                        +{metrics.projected_ctr_uplift_pct.toFixed(0)}%
                      </span>
                    ) : (
                      <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400">
                        Pending Analysis
                      </span>
                    )}
                  </div>
                  <p className="text-2xl font-bold text-white">
                    {metrics ? (
                      `${metrics.projected_ctr_optimized.toFixed(1)}%`
                    ) : (
                      <span className="text-slate-500 font-normal text-lg">Not modeled</span>
                    )}
                  </p>
                  <p className="text-[11px] text-slate-400">
                    {metrics
                      ? `Baseline un-optimized: ${metrics.projected_ctr_baseline.toFixed(1)}%`
                      : 'Models click-through curve vs. rank position'}
                  </p>
                </div>

                <div className="glass-panel p-4 rounded-2xl space-y-1 border border-purple-500/20 bg-purple-950/10">
                  <div className="flex items-center justify-between">
                    <span className="text-[11px] font-semibold uppercase tracking-wider text-purple-400">Local Trust Factor</span>
                    {metrics ? (
                      <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-300">Native Idioms</span>
                    ) : (
                      <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400">
                        Pending Analysis
                      </span>
                    )}
                  </div>
                  <p className="text-2xl font-bold text-white">
                    {metrics ? (
                      <>
                        {metrics.local_trust_score}<span className="text-xs font-normal text-slate-400">/100</span>
                      </>
                    ) : (
                      <span className="text-slate-500 font-normal text-lg">Not evaluated</span>
                    )}
                  </p>
                  <p className="text-[11px] text-slate-400">
                    {metrics
                      ? `Est. Top 10 Ranking: ~${metrics.estimated_ranking_days} days`
                      : 'Evaluates native phrasing & market authority'}
                  </p>
                </div>

                <div className="glass-panel p-4 rounded-2xl space-y-1 border border-amber-500/20 bg-amber-950/10">
                  <div className="flex items-center justify-between">
                    <span className="text-[11px] font-semibold uppercase tracking-wider text-amber-400">Keyword Density</span>
                    {metrics ? (
                      <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${
                        metrics.density_safe ? 'bg-emerald-500/20 text-emerald-300' : 'bg-rose-500/20 text-rose-300'
                      }`}>
                        {metrics.density_safe ? 'Safe & Natural' : 'High Density'}
                      </span>
                    ) : (
                      <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400">
                        Pending Analysis
                      </span>
                    )}
                  </div>
                  <p className="text-2xl font-bold text-white">
                    {metrics ? (
                      `${metrics.keyword_density_pct.toFixed(1)}%`
                    ) : (
                      <span className="text-slate-500 font-normal text-lg">Not scanned</span>
                    )}
                  </p>
                  <p className="text-[11px] text-slate-400">
                    {metrics
                      ? (metrics.density_safe ? 'Anti-stuffing guard: Clean' : 'Warning: High keyword concentration')
                      : 'Guards against search penalty stuffing'}
                  </p>
                </div>
              </div>
            )
          })()}

          {/* Regional Competitor Intelligence & High-Intent Keyword Cloud */}
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
            {/* Competitors List */}
            <div className="lg:col-span-6 glass-panel p-5 rounded-2xl space-y-4">
              <div className="flex items-center justify-between border-b border-white/[0.06] pb-3">
                <div className="flex items-center gap-2">
                  <h3 className="text-sm font-bold text-white">Regional Competitors in [{seoLocale.toUpperCase()}]</h3>
                </div>
                <span className="text-[11px] text-slate-400">
                  {seoData?.competitors?.[seoLocale]?.length || 0} competitors tracked
                </span>
              </div>

              {(!seoData?.competitors?.[seoLocale] || seoData.competitors[seoLocale].length === 0) ? (
                <div className="py-8 text-center space-y-2">
                  <p className="text-xs text-slate-400">No regional competitor data scouted yet.</p>
                  <button
                    onClick={handleTriggerSEOScout}
                    disabled={scoutingSEO}
                    className="text-xs text-sky-400 hover:text-sky-300 font-semibold underline cursor-pointer"
                  >
                    Click to Scout Market Competitors
                  </button>
                </div>
              ) : (
                <div className="space-y-3 max-h-[300px] overflow-y-auto pr-1">
                  {seoData.competitors[seoLocale].map((comp: any, idx: number) => (
                    <div key={idx} className="p-3 rounded-xl bg-slate-900/60 border border-white/[0.05] space-y-1.5">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-blue-600/30 text-sky-300">
                            #{comp.rank || idx + 1}
                          </span>
                          <span className="text-xs font-bold text-white">{comp.domain}</span>
                        </div>
                        {comp.is_discovered && (
                          <span className="text-[9px] px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-300 font-semibold">
                            AI Discovered
                          </span>
                        )}
                      </div>
                      <p className="text-xs text-slate-300 font-medium line-clamp-1">{comp.title}</p>
                      <p className="text-[11px] text-slate-400 line-clamp-2">{comp.meta_description}</p>
                      {comp.keywords && comp.keywords.length > 0 && (
                        <div className="flex flex-wrap gap-1 pt-1">
                          {comp.keywords.slice(0, 4).map((kw: string, kidx: number) => (
                            <span key={kidx} className="text-[10px] px-1.5 py-0.2 rounded bg-slate-800 text-slate-300 border border-white/5">
                              {kw}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Keyword Intelligence Radar */}
            <div className="lg:col-span-6 glass-panel p-5 rounded-2xl space-y-4">
              <div className="flex items-center justify-between border-b border-white/[0.06] pb-3">
                <div className="flex items-center gap-2">
                  <h3 className="text-sm font-bold text-white">High-Intent Regional Keywords [{seoLocale.toUpperCase()}]</h3>
                </div>
                <span className="text-[11px] text-slate-400">
                  {seoData?.keywords?.[seoLocale]?.length || 0} target queries
                </span>
              </div>

              {(!seoData?.keywords?.[seoLocale] || seoData.keywords[seoLocale].length === 0) ? (
                <div className="py-8 text-center space-y-2">
                  <p className="text-xs text-slate-400">No keyword pool synthesized yet.</p>
                  <button
                    onClick={handleTriggerSEOScout}
                    disabled={scoutingSEO}
                    className="text-xs text-sky-400 hover:text-sky-300 font-semibold underline cursor-pointer"
                  >
                    Scout Regional Search Intelligence
                  </button>
                </div>
              ) : (
                <div className="flex flex-wrap gap-2 max-h-[300px] overflow-y-auto pr-1">
                  {seoData.keywords[seoLocale].map((kw: any, idx: number) => (
                    <div
                      key={idx}
                      className={`p-2.5 rounded-xl border transition-all ${
                        kw.is_primary
                          ? 'bg-sky-950/40 border-sky-500/40 text-sky-200'
                          : 'bg-slate-900/60 border-white/[0.06] text-slate-300'
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-bold text-white">{kw.keyword}</span>
                        {kw.is_primary && <span className="text-[9px] px-1 py-0.5 rounded bg-sky-500/30 text-sky-200 font-bold">Primary</span>}
                      </div>
                      <div className="flex items-center gap-2 mt-1 text-[10px] text-slate-400 font-mono">
                        <span>~{kw.est_monthly_volume?.toLocaleString()} vol/mo</span>
                        <span>•</span>
                        <span>KD: {kw.difficulty}</span>
                        <span>•</span>
                        <span className="capitalize">{kw.intent}</span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Multi-Modal Visual SERP & Social Preview Simulator */}
          <div className="glass-panel p-6 rounded-3xl space-y-4 border border-white/[0.08]">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-white/[0.06] pb-4">
              <div>
                <h3 className="text-base font-bold text-white flex items-center gap-2">
                  <span>Multi-Modal Visual SERP & Share Simulator</span>
                  <span className="text-[10px] uppercase font-mono px-2 py-0.5 rounded bg-slate-800 text-sky-400">
                    [{seoLocale}]
                  </span>
                </h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  Live emulation of Google Search snippets and Social OpenGraph share cards for the target country.
                </p>
              </div>

              <div className="flex items-center p-1 rounded-xl bg-slate-900 border border-white/10">
                <button
                  type="button"
                  onClick={() => setSeoSimView('desktop')}
                  className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer ${
                    seoSimView === 'desktop' ? 'bg-blue-600 text-white shadow' : 'text-slate-400 hover:text-white'
                  }`}
                >
                  Desktop (600px)
                </button>
                <button
                  type="button"
                  onClick={() => setSeoSimView('mobile')}
                  className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer ${
                    seoSimView === 'mobile' ? 'bg-blue-600 text-white shadow' : 'text-slate-400 hover:text-white'
                  }`}
                >
                  Mobile
                </button>
                <button
                  type="button"
                  onClick={() => setSeoSimView('social')}
                  className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer ${
                    seoSimView === 'social' ? 'bg-blue-600 text-white shadow' : 'text-slate-400 hover:text-white'
                  }`}
                >
                  Social OG Card
                </button>
              </div>
            </div>

            {/* Render Selected Visual Mockup */}
            {(() => {
              const opts = seoData?.optimizations?.[seoLocale] || []
              const sim = seoData?.simulations?.[seoLocale]
              const hasData = Boolean(sim || opts.length > 0)

              if (!hasData) {
                return (
                  <div className="py-12 text-center space-y-2 max-w-md mx-auto">
                    <p className="text-xs text-slate-400">No SERP simulation generated yet for [{seoLocale.toUpperCase()}].</p>
                    <button
                      type="button"
                      onClick={handleTriggerSEOOptimize}
                      disabled={optimizingSEO}
                      className="text-xs text-sky-400 hover:text-sky-300 font-semibold underline cursor-pointer"
                    >
                      Run Semantic Copy Weaver to generate live Google Search & Social snippets
                    </button>
                  </div>
                )
              }

              let titleTag = sim?.title_tag || `${repo?.Name || 'Application'} | Optimized Platform`
              let metaDesc = sim?.meta_description || `Discover ${repo?.Name || 'Application'} for fast workflows and high conversion.`
              let displayUrl = sim?.display_url || `https://${repo?.Name?.toLowerCase()}.io/${seoLocale}`
              let isTruncated = sim ? sim.is_title_truncated : false

              if (!sim && opts.length > 0) {
                opts.forEach((o: any) => {
                  const k = o.translation_key.toLowerCase()
                  if (k.includes('title') || k.includes('hero')) {
                    if (o.optimized_translation) {
                      titleTag = o.optimized_translation
                      isTruncated = o.is_title_truncated
                    }
                  } else if (k.includes('desc')) {
                    if (o.optimized_translation) metaDesc = o.optimized_translation
                  }
                })
              }

              if (seoSimView === 'social') {
                return (
                  <div className="max-w-xl mx-auto rounded-2xl border border-white/10 bg-slate-950 overflow-hidden shadow-2xl space-y-3 p-4">
                    <div className="h-44 rounded-xl bg-gradient-to-tr from-sky-900 via-indigo-950 to-slate-900 flex items-center justify-center border border-white/10 relative overflow-hidden">
                      <div className="text-center space-y-1 p-4 z-10">
                        <span className="text-xs font-mono font-bold text-sky-400 uppercase tracking-widest">{repo?.Name}</span>
                        <h4 className="text-lg font-bold text-white">{titleTag}</h4>
                      </div>
                      <div className="absolute inset-0 bg-blue-500/10 blur-xl"></div>
                    </div>
                    <div className="space-y-1">
                      <span className="text-[11px] font-mono text-slate-500 uppercase">{displayUrl.replace('https://', '')}</span>
                      <h4 className="text-sm font-bold text-white leading-snug">{sim?.og_card_title || titleTag}</h4>
                      <p className="text-xs text-slate-400 line-clamp-2">{sim?.og_card_description || metaDesc}</p>
                    </div>
                  </div>
                )
              }

              return (
                <div className={`p-6 rounded-2xl bg-[#202124] border border-white/5 font-sans text-left mx-auto shadow-2xl space-y-2 ${
                  seoSimView === 'mobile' ? 'max-w-md' : 'max-w-2xl'
                }`}>
                  <div className="flex items-center gap-2">
                    <div className="w-6 h-6 rounded-full bg-slate-700 flex items-center justify-center text-[10px] text-white font-bold">
                      {repo?.Name ? repo.Name[0].toUpperCase() : 'A'}
                    </div>
                    <div className="space-y-0.5">
                      <p className="text-xs text-[#dadce0] font-medium">{repo?.Name}</p>
                      <p className="text-[11px] text-[#bdc1c6] font-mono">{displayUrl}</p>
                    </div>
                  </div>

                  <div className="space-y-1">
                    <h3 className="text-lg text-[#8ab4f8] hover:underline cursor-pointer font-medium leading-snug">
                      {titleTag}
                    </h3>
                    <p className="text-sm text-[#bdc1c6] leading-relaxed">
                      {metaDesc}
                    </p>
                  </div>

                  {sim?.rich_snippet_faq && sim.rich_snippet_faq.length > 0 && (
                    <div className="pt-2 border-t border-white/[0.06] space-y-1">
                      {sim.rich_snippet_faq.map((faq: string, fidx: number) => (
                        <div key={fidx} className="text-xs text-[#dadce0] flex items-center gap-1.5">
                          <span className="text-sky-400 font-bold">›</span>
                          <span>{faq}</span>
                        </div>
                      ))}
                    </div>
                  )}

                  <div className="flex items-center justify-between pt-2 text-[11px] font-mono text-slate-500 border-t border-white/[0.04]">
                    <span>Target Market: [{seoLocale.toUpperCase()}]</span>
                    {isTruncated ? (
                      <span className="text-rose-400 font-semibold">Pixel Truncated (&gt; 600px)</span>
                    ) : (
                      <span className="text-emerald-400 font-semibold">Desktop Safe Width (≤ 600px)</span>
                    )}
                  </div>
                </div>
              )
            })()}
          </div>

          {/* Interactive Semantic Copy Diff Matrix */}
          <div className="glass-panel p-6 rounded-3xl space-y-5 border border-white/[0.08]">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-white/[0.06] pb-4">
              <div>
                <h3 className="text-base font-bold text-white flex items-center gap-2">
                  <span>Semantic Copy Diff & Keyword Injection Matrix</span>
                  <span className="text-xs font-mono px-2 py-0.5 rounded bg-slate-800 text-sky-300">
                    [{seoLocale.toUpperCase()}]
                  </span>
                </h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  Side-by-side comparison of baseline literal translation vs. SEO-optimized copy with search intent reasoning.
                </p>
              </div>

              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={handleApplySEOToMatrix}
                  disabled={applyingSEO || !seoData?.optimizations?.[seoLocale]}
                  className="rounded-xl bg-emerald-600 hover:bg-emerald-500 disabled:bg-slate-800 disabled:text-slate-600 text-white text-xs font-semibold px-5 py-2.5 shadow-lg shadow-emerald-600/30 flex items-center gap-2 cursor-pointer transition-all"
                >
                  <svg className={`w-3.5 h-3.5 ${applyingSEO ? 'animate-spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                  <span>{applyingSEO ? 'Applying to Matrix…' : 'Apply SEO Copy to Live Matrix'}</span>
                </button>
              </div>
            </div>

            {(!seoData?.optimizations?.[seoLocale] || seoData.optimizations[seoLocale].length === 0) ? (
              <div className="py-12 text-center space-y-3">
                <h4 className="text-sm font-bold text-white">No SEO Optimizations Generated Yet</h4>
                <p className="text-xs text-slate-400 max-w-md mx-auto">
                  Run the Semantic Copy Weaver to automatically inject regional keywords into your landing page and UI keys.
                </p>
                <button
                  onClick={handleTriggerSEOOptimize}
                  disabled={optimizingSEO}
                  className="rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold px-5 py-2.5 shadow cursor-pointer inline-flex items-center gap-2"
                >
                  <span>Run Semantic Copy Weaver</span>
                </button>
              </div>
            ) : (
              <div className="overflow-x-auto rounded-2xl border border-white/[0.06]">
                <table className="w-full text-left text-xs border-collapse">
                  <thead>
                    <tr className="bg-slate-900/80 text-[11px] uppercase tracking-wider text-slate-400 border-b border-white/[0.08]">
                      <th className="py-3 px-4 font-semibold">Key / Impact</th>
                      <th className="py-3 px-4 font-semibold">Source English (en)</th>
                      <th className="py-3 px-4 font-semibold">Baseline Literal Translation</th>
                      <th className="py-3 px-4 font-semibold text-sky-300">SEO-Optimized Translation</th>
                      <th className="py-3 px-4 font-semibold">Injected Keywords & Rationale</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-white/[0.04] bg-slate-950/40 font-sans">
                    {seoData.optimizations[seoLocale].map((opt: any, idx: number) => (
                      <tr key={idx} className="hover:bg-white/[0.02] transition-colors">
                        <td className="py-3.5 px-4 font-mono font-medium text-slate-300 align-top">
                          <div className="space-y-1">
                            <div>{opt.translation_key}</div>
                            <span className={`text-[10px] font-sans px-2 py-0.5 rounded font-semibold uppercase ${
                              opt.impact_tier === 'high' ? 'bg-sky-500/20 text-sky-300' : 'bg-slate-800 text-slate-400'
                            }`}>
                              {opt.impact_tier === 'high' ? 'High Impact' : 'Standard'}
                            </span>
                          </div>
                        </td>
                        <td className="py-3.5 px-4 text-slate-300 align-top max-w-xs">
                          {opt.source_en || '—'}
                        </td>
                        <td className="py-3.5 px-4 text-slate-400 italic align-top max-w-xs">
                          {opt.baseline_translation || '(untranslated)'}
                        </td>
                        <td className="py-3.5 px-4 text-white font-medium align-top max-w-sm bg-sky-950/10">
                          <div className="space-y-1.5">
                            <div className="text-sky-200 font-semibold">{opt.optimized_translation}</div>
                            {opt.icu_variables_matched && (
                              <span className="text-[10px] font-mono text-emerald-400">ICU Variables Preserved</span>
                            )}
                          </div>
                        </td>
                        <td className="py-3.5 px-4 text-slate-300 align-top max-w-sm">
                          <div className="space-y-1.5">
                            {opt.injected_keywords && opt.injected_keywords.length > 0 && (
                              <div className="flex flex-wrap gap-1">
                                {opt.injected_keywords.map((kw: string, kidx: number) => (
                                  <span key={kidx} className="text-[10px] px-1.5 py-0.5 rounded bg-sky-500/20 text-sky-300 font-semibold">
                                    +{kw}
                                  </span>
                                ))}
                              </div>
                            )}
                            <p className="text-[11px] text-slate-400">{opt.rationale}</p>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
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

      {/* ─── TAB 5: PR BOT & WEBHOOK CONTROL CENTER ────────────────────────── */}
      {activeTab === 'bot' && (
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-white/[0.08] pb-4">
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-base font-bold text-white">GitHub PR Bot & Webhook Control Center</h2>
                <span className={cn(
                  "text-[10px] font-mono px-2 py-0.5 rounded-full border font-semibold",
                  webhookPushEnabled
                    ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/30"
                    : "bg-amber-500/10 text-amber-400 border-amber-500/30"
                )}>
                  {webhookPushEnabled ? "● Autopilot Active" : "○ Manual Trigger Only"}
                </span>
              </div>
              <p className="text-xs text-slate-400 mt-0.5">
                Manage automated push triggers, PR bot mention commands, branch filters, and webhook delivery simulations.
              </p>
            </div>

            <div className="flex items-center gap-2.5">
              <button
                type="button"
                onClick={async () => {
                  setWebhookPushEnabled(!webhookPushEnabled)
                  // Auto-save the toggle
                  if (!repo) return
                  const nextState = !webhookPushEnabled
                  try {
                    await fetch(`/api/repos/${repo.ID}/settings`, {
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
                        webhook_push_enabled: nextState,
                        webhook_branch_filter: webhookBranchFilter,
                        webhook_custom_branches: webhookCustomBranches.trim(),
                        webhook_action: webhookAction,
                        webhook_pr_comments_enabled: webhookPRCommentsEnabled,
                        webhook_custom_branch_prefix: webhookCustomBranchPrefix.trim(),
                        webhook_path_filter: webhookPathFilter.trim(),
                      }),
                    })
                    mutateRepos()
                    toast.success(`Push Autopilot turned ${nextState ? 'ON' : 'OFF'}`)
                  } catch (e: any) {
                    toast.error(`Failed to update toggle: ${e.message}`)
                  }
                }}
                className={cn(
                  "rounded-xl text-xs font-semibold px-4 py-2 flex items-center gap-2 transition-all cursor-pointer border",
                  webhookPushEnabled
                    ? "bg-emerald-600/20 hover:bg-emerald-600/30 border-emerald-500/40 text-emerald-300"
                    : "bg-slate-800 hover:bg-slate-700 border-white/10 text-slate-300"
                )}
              >
                <span>{webhookPushEnabled ? 'Push Autopilot Enabled' : 'Push Autopilot Paused'}</span>
              </button>
            </div>
          </div>

          {/* Autopilot Strategy Summary Cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
            <div className="glass-panel p-4 rounded-xl border border-white/[0.08] space-y-1">
              <span className="text-[10px] font-bold uppercase tracking-wider text-slate-500 block">Monitored Branch</span>
              <span className="text-xs font-mono font-bold text-sky-400 truncate block">
                {webhookBranchFilter === 'default_branch'
                  ? `Default (${repo.DefaultBranch || 'main'})`
                  : webhookBranchFilter === 'all'
                  ? 'All Branches (*)'
                  : webhookCustomBranches || 'Custom'}
              </span>
              <p className="text-[10px] text-slate-500">Filter configured in Settings</p>
            </div>

            <div className="glass-panel p-4 rounded-xl border border-white/[0.08] space-y-1">
              <span className="text-[10px] font-bold uppercase tracking-wider text-slate-500 block">Trigger Action</span>
              <span className="text-xs font-semibold text-emerald-400 truncate block">
                {webhookAction === 'auto_pr'
                  ? 'Open Pull Request'
                  : webhookAction === 'direct_commit'
                  ? 'Direct Commit'
                  : 'Open Draft PR'}
              </span>
              <p className="text-[10px] text-slate-500">
                Prefix: <code className="font-mono text-slate-400">{webhookCustomBranchPrefix || 'langpeanut/i18n-'}</code>
              </p>
            </div>

            <div className="glass-panel p-4 rounded-xl border border-white/[0.08] space-y-1">
              <span className="text-[10px] font-bold uppercase tracking-wider text-slate-500 block">PR Bot Commands</span>
              <span className="text-xs font-semibold text-purple-300 flex items-center gap-1">
                {webhookPRCommentsEnabled ? '✓ Active (@langpeanut)' : '✕ Disabled'}
              </span>
              <p className="text-[10px] text-slate-500">Issue/PR comments</p>
            </div>

            <div className="glass-panel p-4 rounded-xl border border-white/[0.08] space-y-1">
              <span className="text-[10px] font-bold uppercase tracking-wider text-slate-500 block">Target Locales</span>
              <div className="flex flex-wrap gap-1">
                {selectedLocales.slice(0, 4).map((loc) => (
                  <span key={loc} className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-sky-500/10 text-sky-400 font-semibold">
                    {loc}
                  </span>
                ))}
                {selectedLocales.length > 4 && (
                  <span className="text-[10px] text-slate-400 font-mono">+{selectedLocales.length - 4} more</span>
                )}
              </div>
              <p className="text-[10px] text-slate-500">{selectedLocales.length} language catalogs</p>
            </div>
          </div>

          {/* Webhook Endpoint & Setup Card */}
          <div className="glass-panel p-6 rounded-2xl space-y-4 border border-white/10">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <div>
                <h3 className="text-sm font-bold text-white flex items-center gap-2">
                  <span>GitHub Webhook Configuration</span>
                  <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-semibold">
                    POST /api/webhook
                  </span>
                </h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  Webhooks are automatically routed when the GitHub App is installed. You can also configure them manually in GitHub repository settings.
                </p>
              </div>

              <button
                type="button"
                onClick={() => {
                  const url = `${typeof window !== 'undefined' ? window.location.origin : ''}/api/webhook`
                  navigator.clipboard.writeText(url)
                  setCopiedWebhookURL(true)
                  setTimeout(() => setCopiedWebhookURL(false), 3000)
                  toast.success('Webhook Payload URL copied to clipboard')
                }}
                className="rounded-xl bg-white/[0.05] hover:bg-white/[0.1] border border-white/10 text-slate-200 text-xs font-semibold px-3.5 py-2 transition-all cursor-pointer shrink-0"
              >
                {copiedWebhookURL ? '✓ URL Copied' : 'Copy Webhook Payload URL'}
              </button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs font-mono">
              <div className="p-3.5 rounded-xl bg-slate-900 border border-white/10 space-y-1">
                <span className="text-[10px] uppercase font-bold text-slate-500 block">Payload URL</span>
                <span className="text-sky-400 break-all select-all font-semibold">
                  {typeof window !== 'undefined' ? window.location.origin : ''}/api/webhook
                </span>
              </div>

              <div className="p-3.5 rounded-xl bg-slate-900 border border-white/10 space-y-1">
                <span className="text-[10px] uppercase font-bold text-slate-500 block">Content Type</span>
                <span className="text-amber-400 font-semibold">application/json</span>
              </div>

              <div className="p-3.5 rounded-xl bg-slate-900 border border-white/10 space-y-1">
                <span className="text-[10px] uppercase font-bold text-slate-500 block">Security Verification</span>
                <span className="text-emerald-400 font-semibold">HMAC-SHA256 (X-Hub-Signature-256)</span>
              </div>
            </div>

            {/* Subscribed Events Pills */}
            <div className="pt-2 border-t border-white/[0.05] flex flex-wrap items-center gap-2">
              <span className="text-[11px] font-semibold text-slate-400">Subscribed Events:</span>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-lg bg-emerald-500/10 text-emerald-300 border border-emerald-500/20 font-semibold">
                ✓ push (commits & branches)
              </span>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-lg bg-emerald-500/10 text-emerald-300 border border-emerald-500/20 font-semibold">
                ✓ issue_comment (PR bot commands)
              </span>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-lg bg-emerald-500/10 text-emerald-300 border border-emerald-500/20 font-semibold">
                ✓ installation_repositories (repo sync)
              </span>
            </div>
          </div>

          {/* Interactive Webhook Simulator & Testing Card */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
            {/* 1. Simulate Push Webhook */}
            <div className="glass-panel p-6 rounded-2xl space-y-4 border border-white/10">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-white flex items-center gap-2">
                    <span>Simulate Git Push Webhook</span>
                  </h3>
                  <p className="text-xs text-slate-400">Test if a commit push on a branch triggers localization.</p>
                </div>
                <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-sky-500/10 text-sky-400 border border-sky-500/20 font-semibold">
                  Dry-Run Test
                </span>
              </div>

              <div className="space-y-3 text-xs">
                <div>
                  <label className="text-[11px] font-semibold text-slate-300 block mb-1">Target Branch to Simulate</label>
                  <div className="flex items-center gap-2">
                    <input
                      type="text"
                      value={selectedBranch || repo.DefaultBranch || 'main'}
                      onChange={(e) => setSelectedBranch(e.target.value)}
                      placeholder="e.g. main, dev, feature/checkout"
                      className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white font-mono placeholder:text-slate-600 focus:border-sky-400 focus:outline-none"
                    />
                    <button
                      type="button"
                      onClick={() => simulateWebhookPush(true)}
                      disabled={simulatingPush}
                      className="rounded-xl bg-sky-600 hover:bg-sky-500 disabled:bg-sky-900 text-white text-xs font-semibold px-4 py-2 transition-all cursor-pointer shrink-0"
                    >
                      {simulatingPush ? 'Testing…' : 'Test Rule'}
                    </button>
                    <button
                      type="button"
                      onClick={() => simulateWebhookPush(false)}
                      disabled={simulatingPush}
                      className="rounded-xl bg-emerald-600 hover:bg-emerald-500 disabled:bg-emerald-900 text-white text-xs font-semibold px-4 py-2 transition-all cursor-pointer shrink-0 shadow-lg shadow-emerald-900/30"
                    >
                      Trigger Run
                    </button>
                  </div>
                </div>

                {pushSimResult && (
                  <div className={cn(
                    "p-3.5 rounded-xl border font-mono text-xs space-y-1.5",
                    pushSimResult.matched
                      ? "bg-emerald-950/40 border-emerald-500/40 text-emerald-200"
                      : "bg-amber-950/40 border-amber-500/40 text-amber-200"
                  )}>
                    <div className="flex items-center justify-between font-bold">
                      <span>{pushSimResult.matched ? '✓ MATCH SUCCESSFUL' : '⚠ TRIGGER SKIPPED'}</span>
                      <span className="text-[10px] text-slate-400 font-normal">status: {pushSimResult.status}</span>
                    </div>
                    <p className="text-[11px] leading-relaxed">{pushSimResult.message || JSON.stringify(pushSimResult)}</p>
                    {pushSimResult.job_id && (
                      <p className="text-[10px] text-sky-400 font-bold pt-1 border-t border-white/10">
                        Queued Job #{pushSimResult.job_id} — check the Runs tab for progress!
                      </p>
                    )}
                  </div>
                )}
              </div>
            </div>

            {/* 2. Simulate PR Bot Mention */}
            <div className="glass-panel p-6 rounded-2xl space-y-4 border border-white/10">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-white flex items-center gap-2">
                    <span>Simulate @langpeanut Bot Mention</span>
                  </h3>
                  <p className="text-xs text-slate-400">Test parsing PR comment commands with flags and directives.</p>
                </div>
                <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-purple-500/10 text-purple-300 border border-purple-500/20 font-semibold">
                  Parser Test
                </span>
              </div>

              <div className="space-y-3 text-xs">
                <div>
                  <label className="text-[11px] font-semibold text-slate-300 block mb-1">Simulate PR Comment Input</label>
                  <div className="flex items-center gap-2">
                    <input
                      type="text"
                      value={botSimInput}
                      onChange={(e) => setBotSimInput(e.target.value)}
                      placeholder="@langpeanut translate --locales es,ja --tone formal"
                      className="w-full rounded-xl bg-slate-900 border border-white/10 px-3.5 py-2 text-xs text-white font-mono placeholder:text-slate-600 focus:border-purple-400 focus:outline-none"
                    />
                    <button
                      type="button"
                      onClick={simulateBotCommand}
                      disabled={simulatingBot}
                      className="rounded-xl bg-purple-600 hover:bg-purple-500 disabled:bg-purple-900 text-white text-xs font-semibold px-4 py-2 transition-all cursor-pointer shrink-0"
                    >
                      {simulatingBot ? 'Parsing…' : 'Test Parse'}
                    </button>
                  </div>
                </div>

                {/* Quick Command Chips */}
                <div className="flex flex-wrap gap-1.5 pt-1">
                  {[
                    '@langpeanut translate --locales es,ja',
                    '@langpeanut review',
                    '@langpeanut audit',
                    '@langpeanut doctor',
                  ].map((cmd) => (
                    <button
                      key={cmd}
                      type="button"
                      onClick={() => setBotSimInput(cmd)}
                      className="text-[10px] font-mono px-2 py-0.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/10 text-slate-300 transition-colors"
                    >
                      {cmd}
                    </button>
                  ))}
                </div>

                {botSimResult && (
                  <div className={cn(
                    "p-3.5 rounded-xl border font-mono text-xs space-y-1.5",
                    botSimResult.valid
                      ? "bg-purple-950/40 border-purple-500/40 text-purple-200"
                      : "bg-rose-950/40 border-rose-500/40 text-rose-200"
                  )}>
                    <div className="flex items-center justify-between font-bold">
                      <span>{botSimResult.valid ? '✓ COMMAND PARSED' : '✗ INVALID COMMAND'}</span>
                      <span className="text-[10px] text-slate-400 font-normal">action: {botSimResult.action || 'none'}</span>
                    </div>
                    <p className="text-[11px] leading-relaxed">{botSimResult.message}</p>
                    {botSimResult.locales && botSimResult.locales.length > 0 && (
                      <p className="text-[10px] text-sky-400">
                        Locales: [{botSimResult.locales.join(', ')}]
                      </p>
                    )}
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* PR Bot Mention Commands Reference Guide */}
          <div className="glass-panel p-6 rounded-2xl space-y-4 border border-white/10">
            <h3 className="text-sm font-bold text-white">Supported @langpeanut PR Mention Commands</h3>
            <p className="text-xs text-slate-400">
              When opened in any GitHub Pull Request, developers and reviewers can trigger autonomous bot actions by posting a comment:
            </p>

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {[
                {
                  cmd: '@langpeanut translate --locales es,fr,de --tone formal',
                  desc: 'Extracts missing strings from PR diff, translates into specified locales, and pushes commits to the PR branch.',
                  badge: 'Localization',
                },
                {
                  cmd: '@langpeanut review',
                  desc: 'Performs 4-tier ICU critic inspection on all translation files modified in this PR and comments diagnostics.',
                  badge: 'Verification',
                },
                {
                  cmd: '@langpeanut audit',
                  desc: 'Scans all modified files for unextracted hardcoded UI strings, ambiguous verbs, and ICU syntax traps.',
                  badge: 'AST Scout',
                },
                {
                  cmd: '@langpeanut doctor',
                  desc: 'Runs full repository health check (syntax validity, missing locale keys, orphan translation catalogs).',
                  badge: 'Diagnostic',
                },
                {
                  cmd: '@langpeanut prune',
                  desc: 'Scans for unused translation keys in source code and cleans up dead entries across all locale files.',
                  badge: 'Pruner',
                },
                {
                  cmd: '@langpeanut directive "add a language dropdown to navbar"',
                  desc: 'Invokes DirectiveAgent to synthesize a reactive language switcher component and wire it to app layout.',
                  badge: 'UI Directive',
                },
              ].map((item) => (
                <div key={item.cmd} className="p-4 rounded-xl bg-slate-900 border border-white/10 space-y-2 flex flex-col justify-between">
                  <div>
                    <div className="flex items-center justify-between gap-2 mb-1.5">
                      <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-sky-500/10 text-sky-400 border border-sky-500/20 font-bold">
                        {item.badge}
                      </span>
                      <button
                        type="button"
                        onClick={() => {
                          navigator.clipboard.writeText(item.cmd)
                          toast.success(`Copied: ${item.cmd}`)
                        }}
                        className="text-[10px] text-slate-400 hover:text-white transition-colors cursor-pointer"
                      >
                        Copy
                      </button>
                    </div>
                    <code className="text-xs font-mono font-bold text-emerald-400 block break-words">
                      {item.cmd}
                    </code>
                    <p className="text-[11px] text-slate-400 mt-2 leading-relaxed">
                      {item.desc}
                    </p>
                  </div>
                </div>
              ))}
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
