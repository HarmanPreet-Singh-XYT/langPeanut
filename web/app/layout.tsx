import type { Metadata } from 'next'
import { Roboto, Roboto_Mono } from 'next/font/google'
import Navbar from './components/Navbar'
import './globals.css'

const roboto = Roboto({
  weight: ['300', '400', '500', '700', '900'],
  subsets: ['latin'],
  variable: '--font-roboto',
  display: 'swap',
})

const robotoMono = Roboto_Mono({
  weight: ['400', '500', '600', '700'],
  subsets: ['latin'],
  variable: '--font-roboto-mono',
  display: 'swap',
})

export const metadata: Metadata = {
  title: 'langPeanut Cloud — Universal Multi-Agent Localization Platform',
  description: 'Zero-defect automated localization for modern engineering teams. Connect your GitHub repository, customize settings, and get verified, compiler-clean pull requests.',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`dark ${roboto.variable} ${robotoMono.variable}`}>
      <body className="bg-[#030712] text-slate-100 min-h-screen font-sans antialiased selection:bg-sky-500/30 selection:text-sky-100 bg-grid-pattern relative">
        {/* Subtle Ambient Glow */}
        <div className="fixed top-0 left-1/2 -translate-x-1/2 w-[1100px] h-[350px] bg-gradient-to-b from-sky-500/8 via-blue-600/5 to-transparent blur-[140px] pointer-events-none -z-10" />

        {/* Global Navigation Bar */}
        <Navbar />

        {/* Main Content */}
        <main className="max-w-7xl mx-auto px-6 py-8">{children}</main>

        {/* Global Footer */}
        <footer className="border-t border-white/[0.08] mt-28 py-12 text-center text-xs text-slate-500">
          <div className="max-w-7xl mx-auto px-6 flex flex-col sm:flex-row items-center justify-between gap-4">
            <p>© 2026 langPeanut — Universal Multi-Agent Localization Workflow & Studio.</p>
            <div className="flex items-center gap-4 text-slate-400 text-[11px]">
              <span>Tree-Sitter AST</span>
              <span>•</span>
              <span>ICU Syntax Parity</span>
              <span>•</span>
              <span>Compiler Self-Healing</span>
              <span>•</span>
              <span>Docker Sandboxing</span>
            </div>
          </div>
        </footer>
      </body>
    </html>
  )
}
