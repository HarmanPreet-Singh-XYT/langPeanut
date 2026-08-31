'use client'

import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'

export default function LoginPage() {
  return (
    <div className="min-h-[80vh] flex flex-col justify-center items-center py-12 px-4">
      {/* Background ambient glow */}
      <div className="fixed top-1/3 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[350px] bg-gradient-to-tr from-sky-600/15 via-blue-600/10 to-transparent blur-[120px] pointer-events-none -z-10" />

      <div className="w-full max-w-md space-y-6">
        {/* Brand Header */}
        <div className="text-center space-y-2">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-2xl bg-slate-900 border border-sky-500/30 text-sky-400 shadow-xl shadow-sky-950/50 mb-1">
            <svg className="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="9" className="stroke-sky-500/40" />
              <path d="M3.6 9h16.8" />
              <path d="M3.6 15h16.8" />
              <path d="M11.5 3a17 17 0 0 0 0 18" />
              <path d="M12.5 3a17 17 0 0 1 0 18" />
            </svg>
          </div>
          <h1 className="text-2xl font-extrabold tracking-tight text-white">
            Welcome to langPeanut
          </h1>
          <p className="text-xs text-slate-400">
            Sign in with GitHub to automate localization across your repositories.
          </p>
        </div>

        {/* Auth Card */}
        <Card className="glass-panel bg-[#090d16]/90 border-white/10 shadow-2xl">
          <CardContent className="pt-6 space-y-5">
            {/* GitHub Authorization Permissions Preview */}
            <div className="rounded-xl border border-white/[0.06] bg-white/[0.02] p-3.5 space-y-2 text-xs">
              <div className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider flex items-center justify-between">
                <span>What langPeanut will request</span>
              </div>
              <div className="space-y-1 text-slate-300 text-[11px]">
                <div className="flex items-center gap-2">
                  <span className="text-emerald-400">✓</span>
                  <span>Your GitHub identity (username, avatar, email)</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-emerald-400">✓</span>
                  <span>Access to repos you explicitly install the langPeanut App on</span>
                </div>
              </div>
              <p className="text-[10px] text-slate-500 pt-1">
                Repo access is granted separately via the GitHub App installer — signing in does not
                by itself give us access to any code.
              </p>
            </div>

            <Button asChild className="w-full rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold py-3.5 text-xs shadow-lg shadow-blue-600/30 h-auto" size="lg">
              <a href="/api/auth/github/start" className="flex items-center justify-center gap-2.5">
                <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
                  <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
                </svg>
                <span>Continue with GitHub</span>
              </a>
            </Button>
          </CardContent>
        </Card>

        {/* Security & Isolation Badges */}
        <div className="text-center text-[11px] text-slate-400 flex items-center justify-center gap-3">
          <span>AES-256 Encrypted</span>
          <span className="text-slate-600">•</span>
          <span>Docker Sandboxed</span>
          <span className="text-slate-600">•</span>
          <span>Zero Host Leakage</span>
        </div>
      </div>
    </div>
  )
}
