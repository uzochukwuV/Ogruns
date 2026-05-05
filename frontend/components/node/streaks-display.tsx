"use client"

import { motion } from "framer-motion"
import { OUTCOME_COLORS, type Streaks } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

export function StreaksDisplay({ streaks }: { streaks: Streaks }) {
  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          win_loss_streaks
        </span>
        <span className="inline-block h-2 w-2 bg-[#ea580c]" />
      </div>

      {/* Content */}
      <div className="p-4">
        {/* Current streak */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1, duration: 0.4, ease }}
          className="mb-4 pb-4 border-b border-foreground/20"
        >
          <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono block mb-2">
            CURRENT_STREAK
          </span>
          <div className="flex items-center gap-2">
            <span
              className="text-2xl font-mono font-bold"
              style={{
                color: streaks.current_streak_type === "winning"
                  ? OUTCOME_COLORS.green
                  : OUTCOME_COLORS.red,
              }}
            >
              {streaks.current_streak}
            </span>
            <span
              className="text-[10px] tracking-widest uppercase font-mono px-2 py-0.5"
              style={{
                backgroundColor: streaks.current_streak_type === "winning"
                  ? `${OUTCOME_COLORS.green}20`
                  : `${OUTCOME_COLORS.red}20`,
                color: streaks.current_streak_type === "winning"
                  ? OUTCOME_COLORS.green
                  : OUTCOME_COLORS.red,
              }}
            >
              {streaks.current_streak_type}
            </span>
          </div>
        </motion.div>

        {/* Best streaks */}
        <div className="grid grid-cols-2 gap-4">
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2, duration: 0.4, ease }}
          >
            <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono block mb-1">
              BEST_WIN_STREAK
            </span>
            <span className="text-xl font-mono font-bold" style={{ color: OUTCOME_COLORS.green }}>
              {streaks.longest_win_streak}
            </span>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.25, duration: 0.4, ease }}
          >
            <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono block mb-1">
              WORST_LOSS_STREAK
            </span>
            <span className="text-xl font-mono font-bold" style={{ color: OUTCOME_COLORS.red }}>
              {streaks.longest_loss_streak}
            </span>
          </motion.div>
        </div>
      </div>
    </div>
  )
}
