"use client"

import { useEffect, useState, useRef } from "react"
import { motion, useInView } from "framer-motion"
import { OUTCOME_COLORS, type NodeStats as NodeStatsType } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

function ScrambleNumber({ target, suffix = "" }: { target: string; suffix?: string }) {
  const [display, setDisplay] = useState(target.replace(/[0-9]/g, "0"))
  const ref = useRef<HTMLSpanElement>(null)
  const inView = useInView(ref, { once: true, margin: "-50px" })

  useEffect(() => {
    if (!inView) return
    let iterations = 0
    const maxIterations = 20

    const interval = setInterval(() => {
      if (iterations >= maxIterations) {
        setDisplay(target)
        clearInterval(interval)
        return
      }

      setDisplay(
        target
          .split("")
          .map((char, i) => {
            if (!/[0-9]/.test(char)) return char
            if (iterations > maxIterations - 5 && i < iterations - (maxIterations - 5)) return char
            return String(Math.floor(Math.random() * 10))
          })
          .join("")
      )
      iterations++
    }, 50)

    return () => clearInterval(interval)
  }, [inView, target])

  return (
    <span ref={ref} style={{ fontVariantNumeric: "tabular-nums" }}>
      {display}{suffix}
    </span>
  )
}

export function NodeStats({ stats }: { stats: NodeStatsType }) {
  const statItems = [
    {
      label: "WIN_RATE",
      value: (stats.win_rate * 100).toFixed(1),
      suffix: "%",
      color: stats.win_rate >= 0.5 ? OUTCOME_COLORS.green : OUTCOME_COLORS.red,
    },
    {
      label: "TOTAL_SIGNALS",
      value: stats.total_signals.toString(),
      suffix: "",
      color: undefined,
    },
    {
      label: "SHARPE_RATIO",
      value: stats.sharpe_ratio.toFixed(2),
      suffix: "",
      color: stats.sharpe_ratio >= 1.5 ? OUTCOME_COLORS.green : undefined,
    },
    {
      label: "AVG_EV",
      value: stats.avg_ev.toFixed(2),
      suffix: "",
      color: stats.avg_ev >= 1 ? OUTCOME_COLORS.green : undefined,
    },
    {
      label: "WINS",
      value: stats.win_count.toString(),
      suffix: "",
      color: OUTCOME_COLORS.green,
    },
    {
      label: "LOSSES",
      value: stats.loss_count.toString(),
      suffix: "",
      color: OUTCOME_COLORS.red,
    },
  ]

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.1, duration: 0.5, ease }}
      className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-0 border-2 border-foreground"
    >
      {statItems.map((item, i) => (
        <motion.div
          key={item.label}
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.15 + i * 0.05, duration: 0.4, ease }}
          className={`p-4 ${i < statItems.length - 1 ? "border-r border-foreground/30" : ""} ${i < 4 ? "border-b md:border-b-0 border-foreground/30" : ""}`}
        >
          <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono block mb-1">
            {item.label}
          </span>
          <span
            className="text-xl font-mono font-bold"
            style={{ color: item.color }}
          >
            <ScrambleNumber target={item.value} suffix={item.suffix} />
          </span>
        </motion.div>
      ))}
    </motion.div>
  )
}
