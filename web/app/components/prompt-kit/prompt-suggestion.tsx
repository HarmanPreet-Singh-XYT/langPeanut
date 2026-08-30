"use client"

import React from "react"
import { cn } from "@/lib/utils"

export type PromptSuggestionProps = {
  children: React.ReactNode
  onClick?: () => void
  className?: string
}

export function PromptSuggestion({
  children,
  onClick,
  className,
}: PromptSuggestionProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-mono bg-[#0c0f16] hover:bg-[#131926] text-zinc-300 hover:text-white border border-white/10 hover:border-sky-500/40 transition-all cursor-pointer shadow-xs",
        className
      )}
    >
      {children}
    </button>
  )
}
