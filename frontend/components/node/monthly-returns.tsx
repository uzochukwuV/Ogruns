"use client"

import { motion } from "framer-motion"
import { OUTCOME_COLORS, type MonthlyReturn } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

export function MonthlyReturnsChart({ returns }: { returns: MonthlyReturn[] }) {
  const maxPnl = Math.max(...returns.map(r => Math.abs(r.pnl_percent)), 20)

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          monthly_returns.chart
        </span>
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          {returns.length} months
        </span>
      </div>

      {/* Chart */}
      <div className="flex-1 p-6">
        <div className="flex items-end justify-around h-[200px] gap-4">
          {returns.map((month, i) => {
            const height = (Math.abs(month.pnl_percent) / maxPnl) * 100
            const isPositive = month.pnl_percent >= 0

            return (
              <div key={month.month} className="flex flex-col items-center flex-1">
                {/* Bar container */}
                <div className="flex-1 w-full flex flex-col justify-end items-center">
                  {/* Value label */}
                  <span
                    className="text-xs font-mono font-bold mb-1"
                    style={{ color: isPositive ? OUTCOME_COLORS.green : OUTCOME_COLORS.red }}
                  >
                    {isPositive ? "+" : ""}{month.pnl_percent.toFixed(1)}%
                  </span>

                  {/* Bar */}
                  <motion.div
                    initial={{ height: 0 }}
                    animate={{ height: `${height}%` }}
                    transition={{ delay: 0.2 + i * 0.1, duration: 0.5, ease }}
                    className="w-full max-w-[60px]"
                    style={{
                      backgroundColor: isPositive ? OUTCOME_COLORS.green : OUTCOME_COLORS.red,
                      minHeight: "4px",
                    }}
                  />
                </div>

                {/* Month label */}
                <span className="text-[10px] font-mono text-muted-foreground mt-2">
                  {month.month.split("-")[1]}/{month.month.split("-")[0].slice(2)}
                </span>
              </div>
            )
          })}
        </div>

        {/* Summary stats */}
        <div className="grid grid-cols-3 gap-4 mt-6 pt-4 border-t-2 border-foreground">
          {returns.map((month, i) => (
            <motion.div
              key={month.month}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.4 + i * 0.05, duration: 0.4, ease }}
              className="text-center"
            >
              <div className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono mb-1">
                {month.month}
              </div>
              <div className="text-xs font-mono">
                <span className="text-muted-foreground">{month.signals} signals</span>
                <span className="mx-1">|</span>
                <span style={{ color: month.win_rate >= 0.5 ? OUTCOME_COLORS.green : OUTCOME_COLORS.red }}>
                  {(month.win_rate * 100).toFixed(0)}% WR
                </span>
              </div>
            </motion.div>
          ))}
        </div>
      </div>
    </div>
  )
}
