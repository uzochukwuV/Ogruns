"use client"

import { useState, useMemo } from "react"
import { motion } from "framer-motion"
import { getMockPortfolioSimulation } from "@/lib/mock-data"
import { OUTCOME_COLORS, type PortfolioSimulation } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

type TimeRange = "7d" | "30d" | "90d" | "all"

function EquityCurve({ simulation }: { simulation: PortfolioSimulation }) {
  const { equity_curve } = simulation
  if (equity_curve.length < 2) return null

  const minEquity = Math.min(...equity_curve.map(p => p.equity))
  const maxEquity = Math.max(...equity_curve.map(p => p.equity))
  const range = maxEquity - minEquity || 1

  // Create SVG path
  const width = 100
  const height = 50
  const points = equity_curve.map((p, i) => {
    const x = (i / (equity_curve.length - 1)) * width
    const y = height - ((p.equity - minEquity) / range) * height
    return `${x},${y}`
  })

  const pathD = `M ${points.join(" L ")}`
  const areaD = `${pathD} L ${width},${height} L 0,${height} Z`

  const isPositive = simulation.total_return_percent >= 0

  return (
    <div className="w-full">
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-[150px]" preserveAspectRatio="none">
        {/* Grid lines */}
        {[0.25, 0.5, 0.75].map(ratio => (
          <line
            key={ratio}
            x1="0"
            y1={ratio * height}
            x2={width}
            y2={ratio * height}
            stroke="currentColor"
            strokeOpacity="0.1"
            strokeWidth="0.5"
          />
        ))}

        {/* Area fill */}
        <motion.path
          d={areaD}
          fill={isPositive ? OUTCOME_COLORS.green : OUTCOME_COLORS.red}
          fillOpacity="0.1"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.3, duration: 0.5 }}
        />

        {/* Line */}
        <motion.path
          d={pathD}
          fill="none"
          stroke={isPositive ? OUTCOME_COLORS.green : OUTCOME_COLORS.red}
          strokeWidth="1"
          initial={{ pathLength: 0 }}
          animate={{ pathLength: 1 }}
          transition={{ delay: 0.2, duration: 0.8, ease }}
        />
      </svg>

      {/* X-axis labels */}
      <div className="flex justify-between mt-2">
        <span className="text-[9px] font-mono text-muted-foreground">
          {new Date(equity_curve[0].timestamp * 1000).toLocaleDateString()}
        </span>
        <span className="text-[9px] font-mono text-muted-foreground">
          {new Date(equity_curve[equity_curve.length - 1].timestamp * 1000).toLocaleDateString()}
        </span>
      </div>
    </div>
  )
}

export function PortfolioSimulator({ nodeId }: { nodeId: string }) {
  const [capital, setCapital] = useState(1000)
  const [timeRange, setTimeRange] = useState<TimeRange>("90d")

  const simulation = useMemo(() => {
    const now = Date.now() / 1000
    let fromTimestamp: number | undefined

    switch (timeRange) {
      case "7d":
        fromTimestamp = now - 7 * 86400
        break
      case "30d":
        fromTimestamp = now - 30 * 86400
        break
      case "90d":
        fromTimestamp = now - 90 * 86400
        break
      case "all":
        fromTimestamp = undefined
        break
    }

    return getMockPortfolioSimulation(nodeId, capital, fromTimestamp, now)
  }, [nodeId, capital, timeRange])

  if (!simulation) return null

  const isPositive = simulation.total_return_percent >= 0

  return (
    <div className="border-2 border-foreground">
      {/* Header */}
      <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          portfolio_simulator
        </span>
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          {simulation.total_trades} trades
        </span>
      </div>

      <div className="p-6">
        {/* Controls */}
        <div className="flex flex-col sm:flex-row gap-4 mb-6">
          {/* Capital input */}
          <div className="flex-1">
            <label className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono block mb-2">
              INITIAL_CAPITAL
            </label>
            <div className="flex items-center border-2 border-foreground">
              <span className="px-3 py-2 bg-muted text-sm font-mono">$</span>
              <input
                type="number"
                value={capital}
                onChange={e => setCapital(Math.max(100, Number(e.target.value)))}
                className="flex-1 px-3 py-2 bg-transparent text-sm font-mono outline-none"
                min={100}
                step={100}
              />
            </div>
          </div>

          {/* Time range */}
          <div>
            <label className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono block mb-2">
              TIME_RANGE
            </label>
            <div className="flex gap-0 border-2 border-foreground">
              {(["7d", "30d", "90d", "all"] as TimeRange[]).map(range => (
                <button
                  key={range}
                  onClick={() => setTimeRange(range)}
                  className={`px-3 py-2 text-[10px] tracking-widest uppercase font-mono transition-colors ${
                    timeRange === range
                      ? "bg-foreground text-background"
                      : "hover:bg-muted"
                  }`}
                >
                  {range}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Results Grid */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-0 border-2 border-foreground mb-6">
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1, duration: 0.4, ease }}
            className="p-4 border-r border-b lg:border-b-0 border-foreground/30"
          >
            <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono block mb-1">
              FINAL_VALUE
            </span>
            <span className="text-xl font-mono font-bold">
              ${simulation.final_value.toLocaleString(undefined, { maximumFractionDigits: 0 })}
            </span>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.15, duration: 0.4, ease }}
            className="p-4 border-b lg:border-b-0 lg:border-r border-foreground/30"
          >
            <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono block mb-1">
              TOTAL_RETURN
            </span>
            <span
              className="text-xl font-mono font-bold"
              style={{ color: isPositive ? OUTCOME_COLORS.green : OUTCOME_COLORS.red }}
            >
              {isPositive ? "+" : ""}{simulation.total_return_percent.toFixed(2)}%
            </span>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2, duration: 0.4, ease }}
            className="p-4 border-r border-foreground/30"
          >
            <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono block mb-1">
              MAX_DRAWDOWN
            </span>
            <span className="text-xl font-mono font-bold" style={{ color: OUTCOME_COLORS.red }}>
              -{simulation.max_drawdown_percent.toFixed(2)}%
            </span>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.25, duration: 0.4, ease }}
            className="p-4"
          >
            <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono block mb-1">
              SHARPE_RATIO
            </span>
            <span className="text-xl font-mono font-bold">
              {simulation.sharpe_ratio.toFixed(2)}
            </span>
          </motion.div>
        </div>

        {/* Equity Curve */}
        <div className="border-2 border-foreground p-4">
          <div className="flex items-center justify-between mb-4">
            <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono">
              EQUITY_CURVE
            </span>
            <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono">
              {simulation.timeframe_days} days
            </span>
          </div>
          <EquityCurve simulation={simulation} />
        </div>
      </div>
    </div>
  )
}
