"use client"

import React, { useState } from "react"
import { CheckCircle2, ChevronDown, Loader2, Wrench, XCircle } from "lucide-react"
import { cn } from "@/lib/utils"

export type ToolPart = {
  type: string
  state: "input-streaming" | "input-available" | "output-available" | "output-error"
  input?: Record<string, unknown>
  output?: Record<string, unknown>
  toolCallId?: string
  errorText?: string
}

export type ToolProps = {
  toolPart: ToolPart
  defaultOpen?: boolean
  className?: string
}

export const Tool = ({ toolPart, defaultOpen = false, className }: ToolProps) => {
  const [isOpen, setIsOpen] = useState(defaultOpen)
  const { state, input, output, toolCallId } = toolPart

  const getStateIcon = () => {
    switch (state) {
      case "input-streaming":
        return <Loader2 className="h-3.5 w-3.5 animate-spin text-sky-400" />
      case "input-available":
        return <Wrench className="h-3.5 w-3.5 text-amber-400" />
      case "output-available":
        return <CheckCircle2 className="h-3.5 w-3.5 text-emerald-400" />
      case "output-error":
        return <XCircle className="h-3.5 w-3.5 text-red-400" />
      default:
        return <Wrench className="h-3.5 w-3.5 text-slate-400" />
    }
  }

  const getStateBadge = () => {
    const baseClasses = "px-1.5 py-0.5 rounded text-[10px] font-mono uppercase tracking-wider font-semibold"
    switch (state) {
      case "input-streaming":
        return (
          <span className={cn(baseClasses, "bg-sky-500/10 text-sky-400 border border-sky-500/20")}>
            Running
          </span>
        )
      case "input-available":
        return (
          <span className={cn(baseClasses, "bg-amber-500/10 text-amber-300 border border-amber-500/20")}>
            Ready
          </span>
        )
      case "output-available":
        return (
          <span className={cn(baseClasses, "bg-emerald-500/10 text-emerald-400 border border-emerald-500/20")}>
            Completed
          </span>
        )
      case "output-error":
        return (
          <span className={cn(baseClasses, "bg-red-500/10 text-red-400 border border-red-500/20")}>
            Error
          </span>
        )
      default:
        return (
          <span className={cn(baseClasses, "bg-slate-800 text-slate-400 border border-white/5")}>
            Pending
          </span>
        )
    }
  }

  const formatValue = (value: unknown): string => {
    if (value === null) return "null"
    if (value === undefined) return "undefined"
    if (typeof value === "string") return value
    if (typeof value === "object") {
      return JSON.stringify(value, null, 2)
    }
    return String(value)
  }

  return (
    <div
      className={cn(
        "rounded-xl border border-white/10 bg-[#090b10] overflow-hidden my-2 text-xs shadow-sm transition-all",
        className
      )}
    >
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between px-3 py-2 bg-[#0c0f16] hover:bg-[#111622] transition-colors cursor-pointer text-left"
      >
        <div className="flex items-center gap-2">
          {getStateIcon()}
          <span className="font-mono text-xs font-semibold text-zinc-200">
            {toolPart.type}
          </span>
          {getStateBadge()}
        </div>
        <ChevronDown
          className={cn("h-3.5 w-3.5 text-zinc-400 transition-transform duration-200", isOpen && "rotate-180")}
        />
      </button>

      {isOpen && (
        <div className="border-t border-white/10 p-3 space-y-3 bg-[#07080c] font-mono text-[11px]">
          {input && Object.keys(input).length > 0 && (
            <div>
              <div className="text-[10px] uppercase font-bold text-zinc-400 mb-1">Tool Input</div>
              <div className="bg-[#050609] rounded-lg border border-white/5 p-2.5 space-y-1 text-zinc-300">
                {Object.entries(input).map(([key, value]) => (
                  <div key={key} className="flex gap-2">
                    <span className="text-zinc-500">{key}:</span>
                    <span className="text-zinc-200">{formatValue(value)}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {output && (
            <div>
              <div className="text-[10px] uppercase font-bold text-zinc-400 mb-1">Tool Output</div>
              <div className="bg-[#050609] max-h-48 overflow-y-auto rounded-lg border border-white/5 p-2.5 text-zinc-300 custom-scrollbar">
                <pre className="whitespace-pre-wrap">{formatValue(output)}</pre>
              </div>
            </div>
          )}

          {state === "output-error" && toolPart.errorText && (
            <div>
              <div className="text-[10px] uppercase font-bold text-red-400 mb-1">Execution Error</div>
              <div className="bg-red-950/20 border border-red-500/30 rounded-lg p-2.5 text-red-300">
                {toolPart.errorText}
              </div>
            </div>
          )}

          {state === "input-streaming" && (
            <div className="text-zinc-500 flex items-center gap-2">
              <Loader2 className="h-3 w-3 animate-spin text-sky-400" />
              <span>Executing deterministic tool...</span>
            </div>
          )}

          {toolCallId && (
            <div className="border-t border-white/5 pt-2 text-[10px] text-zinc-600 flex items-center justify-between">
              <span>Tool Call ID:</span>
              <span className="font-mono">{toolCallId}</span>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
