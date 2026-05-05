"use client"

import { motion } from "framer-motion"
import { OUTCOME_COLORS, type TokenPerformance } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

export function PerformanceByToken({
  performance,
}: {
  performance: Record<string, TokenPerformance>
}) {
  const tokens = Object.values(performance).sort((a, b) => b.total_signals - a.total_signals)

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          performance_by_token
        </span>
        <span className="inline-block h-2 w-2 bg-[#ea580c]" />
      </div>

      {/* Content */}
      <div className="p-4">
        {tokens.map((token, i) => (
          <motion.div
            key={token.token_pair}
            initial={{ opacity: 0, x: -10 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ delay: 0.1 + i * 0.05, duration: 0.4, ease }}
            className={`py-3 ${i < tokens.length - 1 ? "border-b border-foreground/20" : ""}`}
          >
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-mono font-bold">{token.token_pair}</span>
              <span
                className="text-sm font-mono font-bold"
                style={{ color: token.total_pnl >= 0 ? OUTCOME_COLORS.green : OUTCOME_COLORS.red }}
              >
                {token.total_pnl > 0 ? "+" : ""}{token.total_pnl.toFixed(1)}%
              </span>
            </div>

            {/* Win rate bar */}
            <div className="w-full h-2 bg-muted mb-2">
              <div
                className="h-full"
                style={{
                  width: `${token.win_rate * 100}%`,
                  backgroundColor: token.win_rate >= 0.5 ? OUTCOME_COLORS.green : OUTCOME_COLORS.red,
                }}
              />
            </div>

            {/* Stats row */}
            <div className="flex items-center justify-between text-[10px] font-mono text-muted-foreground">
              <span>{token.total_signals} signals</span>
              <span className="text-[#22C55E]">{token.wins}W</span>
              <span className="text-[#EF4444]">{token.losses}L</span>
              <span>{(token.win_rate * 100).toFixed(0)}% WR</span>
            </div>
          </motion.div>
        ))}

        {tokens.length === 0 && (
          <div className="text-center py-6 text-sm font-mono text-muted-foreground">
            No token data available
          </div>
        )}
      </div>
    </div>
  )
}
