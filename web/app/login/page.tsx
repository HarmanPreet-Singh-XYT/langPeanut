'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'

export default function LoginPage() {
  const router = useRouter()
  const [githubUsername, setGithubUsername] = useState('harmanpreetsingh')
  const [email, setEmail] = useState('')
  const [loading, setLoading] = useState(false)
  const [authMode, setAuthMode] = useState<'github' | 'email'>('github')
  const [error, setError] = useState('')

  async function handleSignIn(payload: { email?: string; github_login?: string; name?: string; avatar_url?: string }) {
    setLoading(true)
    setError('')
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      if (!res.ok) {
        throw new Error('Authentication failed')
      }
      const data = await res.json()
      // Store user session in localStorage
      localStorage.setItem('langpeanut_user', JSON.stringify(data.user))
      localStorage.setItem('langpeanut_team', JSON.stringify(data.team))
      localStorage.setItem('langpeanut_token', data.token)
      router.push('/#dashboard')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  function handleGitHubLogin() {
    const handle = githubUsername.trim().replace(/^@/, '') || 'harmanpreetsingh'
    handleSignIn({
      github_login: handle,
      name: handle,
      email: `${handle}@users.noreply.github.com`,
      avatar_url: `https://github.com/${handle}.png`,
    })
  }

  return (
    <div className="min-h-[80vh] flex flex-col justify-center items-center py-12 px-4">
      {/* Background ambient glow */}
      <div className="fixed top-1/3 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[350px] bg-gradient-to-tr from-indigo-600/20 via-purple-600/10 to-transparent blur-[100px] pointer-events-none -z-10" />

      <div className="w-full max-w-md space-y-6">
        {/* Brand Header */}
        <div className="text-center space-y-2">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-2xl bg-gradient-to-tr from-indigo-600 to-purple-500 shadow-xl shadow-indigo-600/30 text-2xl mb-1">
            🥜
          </div>
          <h1 className="text-2xl font-extrabold tracking-tight text-white">
            Welcome to langPeanut
          </h1>
          <p className="text-xs text-slate-400">
            Sign in to automate localization across your repositories.
          </p>
        </div>

        {/* Auth Card */}
        <div className="glass-panel bg-[#090d16]/90 border border-white/10 rounded-2xl p-6 shadow-2xl space-y-5">
          {error && (
            <div className="rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-xs text-rose-300 font-medium text-center">
              {error}
            </div>
          )}

          {/* Auth Method Tabs */}
          <div className="grid grid-cols-2 gap-1 bg-slate-900/90 p-1 rounded-xl border border-white/[0.06] text-xs font-semibold">
            <button
              type="button"
              onClick={() => setAuthMode('github')}
              className={`py-2 rounded-lg transition-all cursor-pointer ${
                authMode === 'github'
                  ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/30'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              GitHub Account
            </button>
            <button
              type="button"
              onClick={() => setAuthMode('email')}
              className={`py-2 rounded-lg transition-all cursor-pointer ${
                authMode === 'email'
                  ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/30'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              Work Email
            </button>
          </div>

          {authMode === 'github' ? (
            <div className="space-y-4">
              <div className="space-y-1.5">
                <label className="block text-xs font-semibold text-slate-300">
                  GitHub Username / Handle
                </label>
                <div className="relative flex items-center">
                  <div className="absolute left-3 w-5 h-5 rounded-full overflow-hidden bg-indigo-600 flex items-center justify-center text-[10px] font-bold text-white shrink-0">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={`https://github.com/${(githubUsername.trim().replace(/^@/, '') || 'github')}.png`}
                      alt="Avatar"
                      className="w-full h-full object-cover"
                      onError={(e) => {
                        (e.target as HTMLElement).style.display = 'none'
                      }}
                    />
                  </div>
                  <input
                    type="text"
                    placeholder="e.g. harmanpreetsingh"
                    value={githubUsername}
                    onChange={(e) => setGithubUsername(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleGitHubLogin()
                    }}
                    className="w-full rounded-xl border border-white/10 bg-[#030712] text-white text-xs pl-10 pr-3 py-2.5 focus:border-indigo-500 focus:outline-none font-mono"
                  />
                </div>
                <p className="text-[11px] text-slate-500">
                  Signs you in with your GitHub developer identity & team workspace.
                </p>
              </div>

              {/* GitHub Authorization Permissions Preview */}
              <div className="rounded-xl border border-white/[0.06] bg-white/[0.02] p-3.5 space-y-2 text-xs">
                <div className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider flex items-center justify-between">
                  <span>Requested Scopes</span>
                  <span className="text-emerald-400 font-mono text-[10px]">App Authorized</span>
                </div>
                <div className="space-y-1 text-slate-300 text-[11px]">
                  <div className="flex items-center gap-2">
                    <span className="text-emerald-400">✓</span>
                    <span><strong>Repository Contents</strong> (Read & Write)</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-emerald-400">✓</span>
                    <span><strong>Pull Requests</strong> (Automated PR Opening)</span>
                  </div>
                </div>
              </div>

              <button
                type="button"
                disabled={loading}
                onClick={handleGitHubLogin}
                className="w-full rounded-xl bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white font-semibold py-3.5 text-xs shadow-lg shadow-indigo-600/30 transition-all cursor-pointer flex items-center justify-center gap-2.5 disabled:opacity-50"
              >
                <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
                  <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                </svg>
                <span>{loading ? 'Signing in…' : `Continue as @${(githubUsername.trim().replace(/^@/, '') || 'developer')}`}</span>
              </button>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="space-y-1.5">
                <label className="block text-xs font-semibold text-slate-300">
                  Work Email Address
                </label>
                <input
                  type="email"
                  placeholder="engineer@company.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full rounded-xl border border-white/10 bg-[#030712] text-white text-xs px-3 py-2.5 focus:border-indigo-500 focus:outline-none"
                />
              </div>

              <button
                type="button"
                disabled={loading}
                onClick={() =>
                  handleSignIn({
                    email: email || 'engineer@company.com',
                    name: email.split('@')[0] || 'Team Engineer',
                    github_login: 'team-dev',
                  })
                }
                className="w-full rounded-xl bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white font-semibold py-3.5 text-xs shadow-lg shadow-indigo-600/30 transition-all cursor-pointer flex items-center justify-center gap-2 disabled:opacity-50"
              >
                <span>✉️</span>
                <span>{loading ? 'Signing in…' : 'Continue with Email'}</span>
              </button>
            </div>
          )}
        </div>

        {/* Security & Isolation Badges */}
        <div className="text-center text-[11px] text-slate-500 flex items-center justify-center gap-3">
          <span>🔒 AES-256 Encrypted</span>
          <span>•</span>
          <span>🛡️ Docker Sandboxed</span>
          <span>•</span>
          <span>🚀 Zero Host Leakage</span>
        </div>
      </div>
    </div>
  )
}
