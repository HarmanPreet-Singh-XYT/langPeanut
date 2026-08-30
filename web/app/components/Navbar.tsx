'use client'

import { useState, useEffect } from 'react'
import useSWR from 'swr'

const fetcher = (url: string) =>
  fetch(url, { headers: { 'X-Team-ID': '1' } }).then((r) => r.json())

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
  const { data: meData, mutate } = useSWR<{
    user: UserProfile
    team: Team
    installations: string[]
    permissions: string[]
  }>('/api/auth/me', fetcher)

  const [showProfileModal, setShowProfileModal] = useState(false)
  const [currentUser, setCurrentUser] = useState<UserProfile | null>(null)

  useEffect(() => {
    if (meData?.user) {
      setCurrentUser(meData.user)
    } else {
      const stored = localStorage.getItem('langpeanut_user')
      if (stored) {
        try {
          setCurrentUser(JSON.parse(stored))
        } catch {
          // ignore
        }
      }
    }
  }, [meData])

  function handleSignOut() {
    localStorage.removeItem('langpeanut_user')
    localStorage.removeItem('langpeanut_team')
    localStorage.removeItem('langpeanut_token')
    setCurrentUser(null)
    setShowProfileModal(false)
    window.location.href = '/login'
  }

  return (
    <>
      <header className="sticky top-0 z-40 border-b border-white/[0.08] bg-[#030712]/85 backdrop-blur-2xl">
        <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between gap-4">
          <div className="flex items-center gap-8">
            <a href="/" className="flex items-center gap-2.5 group">
              <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-indigo-600 via-indigo-500 to-purple-500 flex items-center justify-center shadow-md shadow-indigo-500/20 text-lg group-hover:scale-105 transition-transform">
                🥜
              </div>
              <div className="flex items-center gap-2">
                <span className="text-base font-bold tracking-tight text-white">langPeanut</span>
                <span className="text-[10px] font-mono font-medium px-2 py-0.5 rounded-full bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 uppercase tracking-wider">
                  Cloud
                </span>
              </div>
            </a>

              <nav className="hidden md:flex items-center gap-1 text-xs font-medium text-slate-400">
                <a href="/#dashboard" className="px-3 py-1.5 rounded-lg hover:text-white hover:bg-white/[0.04] transition-colors">
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
                <div className="w-6 h-6 rounded-full bg-indigo-600 flex items-center justify-center text-xs font-bold text-white overflow-hidden border border-white/20">
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
                className="rounded-lg bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white text-xs font-semibold px-4 py-2 shadow-lg shadow-indigo-600/25 transition-all cursor-pointer flex items-center gap-1.5"
              >
                <span>🐙</span>
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
                <div className="w-11 h-11 rounded-full bg-indigo-600 flex items-center justify-center text-sm font-bold text-white overflow-hidden border-2 border-indigo-400/50 shadow-md">
                  {currentUser.avatar_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={currentUser.avatar_url} alt={currentUser.name} className="w-full h-full object-cover" />
                  ) : (
                    currentUser.name.charAt(0)
                  )}
                </div>
                <div>
                  <h3 className="font-bold text-base text-white">{currentUser.name}</h3>
                  <p className="text-xs font-mono text-indigo-400">@{currentUser.github_login}</p>
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
                        className="font-mono text-[11px] px-2 py-0.5 rounded bg-indigo-500/10 border border-indigo-500/20 text-indigo-300 font-medium"
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
                <span>🚪</span>
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
