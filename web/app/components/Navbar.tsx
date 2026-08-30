'use client'

import { useState } from 'react'
import useSWR from 'swr'

const fetcher = (url: string) =>
  fetch(url, { credentials: 'include' }).then((r) => {
    if (!r.ok) throw new Error(`${r.status}`)
    return r.json()
  })

interface UserProfile {
  id: number
  team_id: number
  email: string
  name: string
  github_login: string
  avatar_url: string
}

interface Team {
  id: number
  name: string
}

export default function Navbar() {
  const { data: meData } = useSWR<{
    user: UserProfile
    team: Team
    installations: string[]
    permissions: string[]
  }>('/api/auth/me', fetcher, { shouldRetryOnError: false })

  const [showProfileModal, setShowProfileModal] = useState(false)
  const currentUser = meData?.user ?? null

  async function handleSignOut() {
    await fetch('/api/auth/logout', { method: 'POST', credentials: 'include' })
    setShowProfileModal(false)
    window.location.href = '/login'
  }

  return (
    <>
      <header className="sticky top-0 z-40 border-b border-white/[0.08] bg-[#030712]/85 backdrop-blur-2xl">
        <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between gap-4">
          <div className="flex items-center gap-8">
            <a href="/" className="flex items-center gap-2.5 group">
              <div className="w-9 h-9 rounded-xl bg-slate-900 border border-sky-500/30 flex items-center justify-center shadow-md shadow-sky-950/40 text-sky-400 group-hover:border-sky-400 transition-colors">
                <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="9" className="stroke-sky-500/40" />
                  <path d="M3.6 9h16.8" />
                  <path d="M3.6 15h16.8" />
                  <path d="M11.5 3a17 17 0 0 0 0 18" />
                  <path d="M12.5 3a17 17 0 0 1 0 18" />
                </svg>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-base font-bold tracking-tight text-white">langPeanut</span>
                <span className="text-[10px] font-mono font-medium px-2 py-0.5 rounded-full bg-sky-500/10 text-sky-400 border border-sky-500/20 uppercase tracking-wider">
                  Cloud
                </span>
              </div>
            </a>

              <nav className="hidden md:flex items-center gap-1 text-xs font-medium text-slate-400">
                <a href="/dashboard" className="px-3 py-1.5 rounded-lg hover:text-white hover:bg-white/[0.04] transition-colors">
                  Console
                </a>
                <a href="/#workflows" className="px-3 py-1.5 rounded-lg hover:text-white hover:bg-white/[0.04] transition-colors">
                  Workflows
                </a>
                <a href="/#analytics" className="px-3 py-1.5 rounded-lg hover:text-white hover:bg-white/[0.04] transition-colors">
                  Analytics
                </a>
                <a href="/#playground" className="px-3 py-1.5 rounded-lg hover:text-white hover:bg-white/[0.04] transition-colors">
                  Playground
                </a>
                <a href="/#architecture" className="px-3 py-1.5 rounded-lg hover:text-white hover:bg-white/[0.04] transition-colors">
                  Architecture
                </a>
                <a href="/#benchmark" className="px-3 py-1.5 rounded-lg hover:text-white hover:bg-white/[0.04] transition-colors">
                  Benchmark
                </a>
                <a href="/#frameworks" className="px-3 py-1.5 rounded-lg hover:text-white hover:bg-white/[0.04] transition-colors">
                  Frameworks
                </a>
              </nav>
          </div>

          <div className="flex items-center gap-3.5">
            <div className="hidden sm:flex items-center gap-2 text-[11px] font-mono text-emerald-400 bg-emerald-950/40 border border-emerald-800/40 px-2.5 py-1 rounded-full">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
              <span>6 Agents Active</span>
            </div>

            {currentUser ? (
              <button
                type="button"
                onClick={() => setShowProfileModal(true)}
                className="flex items-center gap-2.5 rounded-xl border border-white/10 bg-white/[0.03] hover:bg-white/[0.08] px-3 py-1.5 transition-all cursor-pointer"
              >
                <div className="w-6 h-6 rounded-full bg-blue-600 flex items-center justify-center text-xs font-bold text-white overflow-hidden border border-white/20">
                  {currentUser.avatar_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={currentUser.avatar_url} alt={currentUser.name} className="w-full h-full object-cover" />
                  ) : (
                    currentUser.name.charAt(0)
                  )}
                </div>
                <div className="text-left hidden sm:block">
                  <p className="text-xs font-semibold text-white leading-none">
                    @{currentUser.github_login || 'dev'}
                  </p>
                  <p className="text-[10px] text-slate-400 leading-tight">
                    {meData?.team?.name || 'Engineering Core'}
                  </p>
                </div>
              </button>
            ) : (
              <a
                href="/login"
                className="rounded-lg bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold px-4 py-2 shadow-lg shadow-blue-600/25 transition-all cursor-pointer flex items-center gap-2"
              >
                <svg className="w-3.5 h-3.5 fill-current" viewBox="0 0 24 24">
                  <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                </svg>
                <span>Sign In</span>
              </a>
            )}
          </div>
        </div>
      </header>

      {/* User Profile & Permissions Modal */}
      {showProfileModal && currentUser && (
        <div className="fixed inset-0 z-50 bg-black/80 backdrop-blur-md flex items-center justify-center p-4">
          <div className="glass-panel bg-[#090d16] border border-white/10 rounded-2xl w-full max-w-lg shadow-2xl p-6 space-y-5">
            <div className="flex items-center justify-between border-b border-white/[0.08] pb-4">
              <div className="flex items-center gap-3">
                <div className="w-11 h-11 rounded-full bg-blue-600 flex items-center justify-center text-sm font-bold text-white overflow-hidden border-2 border-sky-400/50 shadow-md">
                  {currentUser.avatar_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={currentUser.avatar_url} alt={currentUser.name} className="w-full h-full object-cover" />
                  ) : (
                    currentUser.name.charAt(0)
                  )}
                </div>
                <div>
                  <h3 className="font-bold text-base text-white">{currentUser.name}</h3>
                  <p className="text-xs font-mono text-sky-400">@{currentUser.github_login}</p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setShowProfileModal(false)}
                className="text-slate-500 hover:text-slate-300 text-base cursor-pointer"
              >
                ✕
              </button>
            </div>

            {/* Profile & Account Details */}
            <div className="space-y-3.5 text-xs">
              <div className="grid grid-cols-2 gap-3 bg-white/[0.02] border border-white/[0.06] rounded-xl p-3.5">
                <div>
                  <span className="text-[10px] text-slate-500 uppercase tracking-wider block font-semibold">
                    Email Address
                  </span>
                  <span className="text-slate-200 font-medium">{currentUser.email}</span>
                </div>
                <div>
                  <span className="text-[10px] text-slate-500 uppercase tracking-wider block font-semibold">
                    Team Organization
                  </span>
                  <span className="text-slate-200 font-medium">{meData?.team?.name || 'Engineering Core'}</span>
                </div>
              </div>

              {/* Active Repository Permissions */}
              <div className="rounded-xl border border-white/[0.06] bg-white/[0.02] p-3.5 space-y-2">
                <div className="flex items-center justify-between text-[10px] font-semibold text-slate-400 uppercase tracking-wider">
                  <span>Authorized Repository Scopes</span>
                  <span className="text-emerald-400 font-mono">Granted</span>
                </div>
                <div className="grid grid-cols-2 gap-2 text-[11px] text-slate-300">
                  <div className="flex items-center gap-1.5">
                    <span className="text-emerald-400">✓</span>
                    <span>Contents: Read & Write</span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <span className="text-emerald-400">✓</span>
                    <span>Pull Requests: Read & Write</span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <span className="text-emerald-400">✓</span>
                    <span>Metadata: Read Only</span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <span className="text-emerald-400">✓</span>
                    <span>AES-256 Vault Isolated</span>
                  </div>
                </div>
              </div>

              {/* Connected GitHub Installations */}
              <div className="rounded-xl border border-white/[0.06] bg-white/[0.02] p-3.5 space-y-1.5">
                <span className="text-[10px] text-slate-500 uppercase tracking-wider block font-semibold">
                  Connected GitHub Organizations / Accounts
                </span>
                <div className="flex flex-wrap gap-1.5">
                  {meData?.installations && meData.installations.length > 0 ? (
                    meData.installations.map((acc, idx) => (
                      <span
                        key={idx}
                        className="font-mono text-[11px] px-2 py-0.5 rounded bg-sky-500/10 border border-sky-500/20 text-sky-300 font-medium"
                      >
                        @{acc}
                      </span>
                    ))
                  ) : (
                    <span className="text-slate-400 text-[11px]">Default organization active</span>
                  )}
                </div>
              </div>
            </div>

            {/* Actions */}
            <div className="flex items-center justify-between pt-3 border-t border-white/[0.08]">
              <button
                type="button"
                onClick={handleSignOut}
                className="text-xs text-rose-400 hover:text-rose-300 font-semibold cursor-pointer transition-colors flex items-center gap-1.5"
              >
                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
                  <polyline points="16 17 21 12 16 7" />
                  <line x1="21" y1="12" x2="9" y2="12" />
                </svg>
                <span>Sign Out</span>
              </button>
              <button
                type="button"
                onClick={() => setShowProfileModal(false)}
                className="rounded-lg bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium px-4 py-2 cursor-pointer"
              >
                Done
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
