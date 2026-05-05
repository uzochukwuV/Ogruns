"use client"

import { useEffect, useState, useRef } from "react"
import { motion, useInView } from "framer-motion"
import { apiClient } from "@/lib/api-client"
import type { DashboardData } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const



function ScrambleNumber({ target, label, suffix = "" }: { target: string; label: string; suffix?: string }) {
  const [display, setDisplay] = useState(target.replace(/[0-9]/g, "0"))
  const ref = useRef<HTMLDivElement>(null)
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
    <div ref={ref} className="flex flex-col gap-1">
      <span
        className="text-2xl lg:text-3xl font-mono font-bold tracking-tight text-foreground"
        style={{ fontVariantNumeric: "tabular-nums" }}
      >
        {display}{suffix}
      </span>
      <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground">
        {label}
      </span>
    </div>
  )
}

export function DashboardStats() {
  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchData = async () => {
      try {
        const dashboardData = await apiClient.getDashboardSummary()
        setData(dashboardData)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load dashboard data')
        console.error('Dashboard stats error:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [])

  if (loading) {
    return (
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease }}
        className="grid grid-cols-2 lg:grid-cols-4 gap-0 border-2 border-foreground"
      >
        {Array.from({ length: 4 }).map((_, i) => (
          <div
            key={i}
            className={`p-6 ${i < 3 ? "border-r-2 border-foreground" : ""} ${i < 2 ? "border-b-2 lg:border-b-0 border-foreground" : ""}`}
          >
            <div className="animate-pulse">
              <div className="h-8 bg-muted rounded mb-2"></div>
              <div className="h-3 bg-muted rounded"></div>
            </div>
          </div>
        ))}
      </motion.div>
    )
  }

  if (error || !data) {
    return (
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease }}
        className="grid grid-cols-2 lg:grid-cols-4 gap-0 border-2 border-foreground"
      >
        <div className="col-span-full p-6 text-center text-muted-foreground">
          {error || 'No data available'}
        </div>
      </motion.div>
    )
  }

  const stats = [
    { value: data.total_nodes.toString(), label: "Active Nodes", suffix: "" },
    { value: data.total_signals.toLocaleString(), label: "Total Signals", suffix: "" },
    { value: (data.platform_win_rate * 100).toFixed(1), label: "Platform Win Rate", suffix: "%" },
    { value: data.total_wins.toLocaleString(), label: "Total Wins", suffix: "" },
  ]

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5, ease }}
      className="grid grid-cols-2 lg:grid-cols-4 gap-0 border-2 border-foreground"
    >
      {stats.map((stat, i) => (
        <motion.div
          key={stat.label}
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 + i * 0.08, duration: 0.5, ease }}
          className={`p-6 ${i < stats.length - 1 ? "border-r-2 border-foreground" : ""} ${i < 2 ? "border-b-2 lg:border-b-0 border-foreground" : ""}`}
        >
          <ScrambleNumber target={stat.value} label={stat.label} suffix={stat.suffix} />
        </motion.div>
      ))}
    </motion.div>
  )
}
