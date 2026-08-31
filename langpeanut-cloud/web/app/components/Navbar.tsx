'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Avatar, AvatarImage, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { LogOut, CheckCircle } from 'lucide-react'
import { cn } from '@/lib/utils'

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
                <Badge variant="outline" className="text-[10px] font-mono font-medium px-2 py-0.5 rounded-full bg-sky-500/10 text-sky-400 border-sky-500/20 uppercase tracking-wider">
                  Cloud
                </Badge>
              </div>
            </a>

            <nav className="hidden md:flex items-center gap-1 text-xs font-medium text-slate-400">
              <Button variant="ghost" size="sm" asChild className="text-slate-400 hover:text-white text-xs">
                <a href="/dashboard">Console</a>
              </Button>
              <Button variant="ghost" size="sm" asChild className="text-slate-400 hover:text-white text-xs">
                <a href="/#workflows">Workflows</a>
              </Button>
              <Button variant="ghost" size="sm" asChild className="text-slate-400 hover:text-white text-xs">
                <a href="/#analytics">Analytics</a>
              </Button>
              <Button variant="ghost" size="sm" asChild className="text-slate-400 hover:text-white text-xs">
                <a href="/#playground">Playground</a>
              </Button>
              <Button variant="ghost" size="sm" asChild className="text-slate-400 hover:text-white text-xs">
                <a href="/#architecture">Architecture</a>
              </Button>
              <Button variant="ghost" size="sm" asChild className="text-slate-400 hover:text-white text-xs">
                <a href="/#benchmark">Benchmark</a>
              </Button>
              <Button variant="ghost" size="sm" asChild className="text-slate-400 hover:text-white text-xs">
                <a href="/#frameworks">Frameworks</a>
              </Button>
            </nav>
          </div>

          <div className="flex items-center gap-3.5">
            <div className="hidden sm:flex items-center gap-2 text-[11px] font-mono text-emerald-400 bg-emerald-950/40 border border-emerald-800/40 px-2.5 py-1 rounded-full">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
              <span>6 Agents Active</span>
            </div>

            {currentUser ? (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="outline"
                    className="flex items-center gap-2.5 rounded-xl border-white/10 bg-white/[0.03] hover:bg-white/[0.08] px-3 py-1.5 h-auto"
                  >
                    <Avatar className="w-6 h-6 border border-white/20">
                      <AvatarImage src={currentUser.avatar_url} alt={currentUser.name} />
                      <AvatarFallback className="bg-blue-600 text-xs font-bold text-white">
                        {currentUser.name.charAt(0)}
                      </AvatarFallback>
                    </Avatar>
                    <div className="text-left hidden sm:block">
                      <p className="text-xs font-semibold text-white leading-none">
                        @{currentUser.github_login || 'dev'}
                      </p>
                      <p className="text-[10px] text-slate-400 leading-tight">
                        {meData?.team?.name || 'Engineering Core'}
                      </p>
                    </div>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-56 bg-[#090d16] border-white/10">
                  <DropdownMenuLabel className="text-slate-300">
                    <div className="font-semibold">{currentUser.name}</div>
                    <div className="text-xs font-mono text-sky-400">@{currentUser.github_login}</div>
                  </DropdownMenuLabel>
                  <DropdownMenuSeparator className="bg-white/[0.08]" />
                  <DropdownMenuItem
                    onClick={() => setShowProfileModal(true)}
                    className="text-slate-300 hover:text-white cursor-pointer"
                  >
                    View Profile & Permissions
                  </DropdownMenuItem>
                  <DropdownMenuSeparator className="bg-white/[0.08]" />
                  <DropdownMenuItem
                    onClick={handleSignOut}
                    className="text-rose-400 hover:text-rose-300 cursor-pointer"
                  >
                    <LogOut className="w-3.5 h-3.5 mr-2" />
                    Sign Out
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            ) : (
              <Button asChild variant="default" size="sm" className="bg-blue-600 hover:bg-blue-500 shadow-lg shadow-blue-600/25">
                <a href="/login" className="flex items-center gap-2">
                  <svg className="w-3.5 h-3.5 fill-current" viewBox="0 0 24 24">
                    <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                  </svg>
                  <span>Sign In</span>
                </a>
              </Button>
            )}
          </div>
        </div>
      </header>

      {/* User Profile & Permissions Modal */}
      <Dialog open={showProfileModal} onOpenChange={setShowProfileModal}>
        <DialogContent className="bg-[#090d16] border-white/10 text-white max-w-lg">
          <DialogHeader>
            <div className="flex items-center gap-3">
              <Avatar className="w-11 h-11 border-2 border-sky-400/50 shadow-md">
                <AvatarImage src={currentUser?.avatar_url} alt={currentUser?.name} />
                <AvatarFallback className="bg-blue-600 text-sm font-bold text-white">
                  {currentUser?.name?.charAt(0)}
                </AvatarFallback>
              </Avatar>
              <div>
                <DialogTitle className="font-bold text-base text-white">{currentUser?.name}</DialogTitle>
                <p className="text-xs font-mono text-sky-400">@{currentUser?.github_login}</p>
              </div>
            </div>
          </DialogHeader>

          {/* Profile & Account Details */}
          <div className="space-y-3.5 text-xs">
            <div className="grid grid-cols-2 gap-3 bg-white/[0.02] border border-white/[0.06] rounded-xl p-3.5">
              <div>
                <span className="text-[10px] text-slate-500 uppercase tracking-wider block font-semibold">
                  Email Address
                </span>
                <span className="text-slate-200 font-medium">{currentUser?.email}</span>
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
                {['Contents: Read & Write', 'Pull Requests: Read & Write', 'Metadata: Read Only', 'AES-256 Vault Isolated'].map((perm) => (
                  <div key={perm} className="flex items-center gap-1.5">
                    <CheckCircle className="w-3 h-3 text-emerald-400 shrink-0" />
                    <span>{perm}</span>
                  </div>
                ))}
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
                    <Badge key={idx} variant="outline" className="font-mono text-[11px] bg-sky-500/10 border-sky-500/20 text-sky-300">
                      @{acc}
                    </Badge>
                  ))
                ) : (
                  <span className="text-slate-400 text-[11px]">Default organization active</span>
                )}
              </div>
            </div>
          </div>

          {/* Actions */}
          <div className="flex items-center justify-between pt-3 border-t border-white/[0.08]">
            <Button
              variant="ghost"
              size="sm"
              onClick={handleSignOut}
              className="text-rose-400 hover:text-rose-300 hover:bg-rose-500/10 gap-1.5"
            >
              <LogOut className="w-3.5 h-3.5" />
              Sign Out
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowProfileModal(false)}
              className="bg-white/5 hover:bg-white/10 border-white/10 text-slate-300"
            >
              Done
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
