"use client"

import { useState } from "react"
import { motion } from "framer-motion"
import { ChevronLeft, ChevronRight } from "lucide-react"
import { OUTCOME_COLORS, type Signal } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const
const ITEMS_PER_PAGE = 10

function formatDate(timestamp: number): string {
  return new Date(timestamp * 1000).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function formatDuration(seconds: number): string {
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`
  return `${(seconds / 3600).toFixed(1)}h`
}

export function SignalHistoryTable({ signals }: { signals: Signal[] }) {
  const [page, setPage] = useState(0)
  const totalPages = Math.ceil(signals.length / ITEMS_PER_PAGE)
  const pageSignals = signals.slice(page * ITEMS_PER_PAGE, (page + 1) * ITEMS_PER_PAGE)

  return (
    <div className="border-2 border-foreground">
      {/* Header */}
      <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          signal_history.table
        </span>
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          {signals.length} total
        </span>
      </div>

      {/* Table Header */}
      <div className="hidden lg:grid grid-cols-12 gap-2 px-4 py-2 border-b border-foreground/30 bg-muted/30">
        <div className="col-span-1 text-[9px] tracking-widest uppercase text-muted-foreground font-mono">Status</div>
        <div className="col-span-2 text-[9px] tracking-widest uppercase text-muted-foreground font-mono">Pair</div>
        <div className="col-span-1 text-[9px] tracking-widest uppercase text-muted-foreground font-mono">Dir</div>
        <div className="col-span-2 text-[9px] tracking-widest uppercase text-muted-foreground font-mono text-right">Entry</div>
        <div className="col-span-2 text-[9px] tracking-widest uppercase text-muted-foreground font-mono text-right">Close</div>
        <div className="col-span-1 text-[9px] tracking-widest uppercase text-muted-foreground font-mono text-right">PnL</div>
        <div className="col-span-1 text-[9px] tracking-widest uppercase text-muted-foreground font-mono text-right">Time</div>
        <div className="col-span-2 text-[9px] tracking-widest uppercase text-muted-foreground font-mono text-right">Date</div>
      </div>

      {/* Table Body */}
      <div className="max-h-[500px] overflow-auto">
        {pageSignals.map((signal, i) => (
          <motion.div
            key={signal.id}
            initial={{ opacity: 0, x: -10 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ delay: i * 0.03, duration: 0.3, ease }}
            className="grid grid-cols-4 lg:grid-cols-12 gap-2 px-4 py-3 border-b border-foreground/10 hover:bg-muted/30 transition-colors"
          >
            {/* Outcome indicator */}
            <div className="col-span-1 flex items-center">
              <div
                className="w-3 h-3"
                style={{ backgroundColor: OUTCOME_COLORS[signal.outcome_type] }}
              />
            </div>

            {/* Pair */}
            <div className="col-span-1 lg:col-span-2">
              <span className="text-xs font-mono font-bold">{signal.token_pair}</span>
            </div>

            {/* Direction */}
            <div className="col-span-1">
              <span
                className={`text-[9px] tracking-widest uppercase font-mono px-1.5 py-0.5 ${
                  signal.direction === "LONG"
                    ? "bg-[#22C55E]/20 text-[#22C55E]"
                    : "bg-[#EF4444]/20 text-[#EF4444]"
                }`}
              >
                {signal.direction}
              </span>
            </div>

            {/* Entry price - hidden on mobile */}
            <div className="hidden lg:block col-span-2 text-right">
              <span className="text-xs font-mono text-muted-foreground">
                ${signal.entry_price.toLocaleString(undefined, { maximumFractionDigits: 2 })}
              </span>
            </div>

            {/* Close price - hidden on mobile */}
            <div className="hidden lg:block col-span-2 text-right">
              <span className="text-xs font-mono text-muted-foreground">
                ${signal.closed_price.toLocaleString(undefined, { maximumFractionDigits: 2 })}
              </span>
            </div>

            {/* PnL */}
            <div className="col-span-1 text-right">
              <span
                className="text-xs font-mono font-bold"
                style={{ color: OUTCOME_COLORS[signal.outcome_type] }}
              >
                {signal.pnl_percent > 0 ? "+" : ""}{signal.pnl_percent.toFixed(2)}%
              </span>
            </div>

            {/* Duration - hidden on mobile */}
            <div className="hidden lg:block col-span-1 text-right">
              <span className="text-xs font-mono text-muted-foreground">
                {formatDuration(signal.duration_sec)}
              </span>
            </div>

            {/* Date - hidden on mobile */}
            <div className="hidden lg:block col-span-2 text-right">
              <span className="text-xs font-mono text-muted-foreground">
                {formatDate(signal.created_at)}
              </span>
            </div>
          </motion.div>
        ))}
      </div>

      {/* Pagination */}
      <div className="flex items-center justify-between px-4 py-3 border-t-2 border-foreground">
        <span className="text-[10px] tracking-widest uppercase text-muted-foreground font-mono">
          Page {page + 1} of {totalPages}
        </span>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setPage(p => Math.max(0, p - 1))}
            disabled={page === 0}
            className="p-1 border border-foreground disabled:opacity-30 disabled:cursor-not-allowed hover:bg-muted transition-colors"
          >
            <ChevronLeft size={14} />
          </button>
          <button
            onClick={() => setPage(p => Math.min(totalPages - 1, p + 1))}
            disabled={page >= totalPages - 1}
            className="p-1 border border-foreground disabled:opacity-30 disabled:cursor-not-allowed hover:bg-muted transition-colors"
          >
            <ChevronRight size={14} />
          </button>
        </div>
      </div>
    </div>
  )
}
