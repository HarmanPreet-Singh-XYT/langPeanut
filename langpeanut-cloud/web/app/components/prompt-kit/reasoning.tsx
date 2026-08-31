"use client"

import React, { useState } from "react"
import { ChevronDown, Sparkles } from "lucide-react"
import { cn } from "@/lib/utils"

export type ReasoningProps = {
  children: React.ReactNode
  isStreaming?: boolean
  defaultOpen?: boolean
  className?: string
}

export function Reasoning({
  children,
  isStreaming = false,
  defaultOpen = false,
  className,
}: ReasoningProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen)

  return (
    <div
      className={cn(
        "rounded-xl border border-white/5 bg-[#090b10]/60 overflow-hidden my-2 text-xs",
        className
      )}
    >
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between px-3 py-2 text-zinc-400 hover:text-zinc-200 transition-colors text-left cursor-pointer"
      >
        <div className="flex items-center gap-2 font-mono text-[11px]">
          <Sparkles className={cn("h-3.5 w-3.5 text-purple-400", isStreaming && "animate-pulse")} />
          <span>{isStreaming ? "Thinking & reasoning..." : "Reasoning process"}</span>
        </div>
        <ChevronDown
          className={cn("h-3.5 w-3.5 text-zinc-500 transition-transform", isOpen && "rotate-180")}
        />
      </button>

      {isOpen && (
        <div className="px-3 pb-3 pt-1 border-t border-white/5 text-zinc-400 font-sans text-xs leading-relaxed">
          {children}
        </div>
      )}
    </div>
  )
}
