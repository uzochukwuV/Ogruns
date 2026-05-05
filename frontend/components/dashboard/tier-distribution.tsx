"use client"

import { useEffect, useState } from "react"
import { motion } from "framer-motion"
import { apiClient } from "@/lib/api-client"
import { TIER_CONFIG, type Tier, type TierDistribution } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

export function TierDistributionChart() {
  const [data, setData] = useState<TierDistribution | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchData = async () => {
      try {
        const dashboardData = await apiClient.getDashboardSummary()
        setData(dashboardData.tier_distribution)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load tier distribution')
        console.error('Tier distribution error:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [])

  if (loading || !data) {
    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
          <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
            tier_distribution.chart
          </span>
          <span className="inline-block h-2 w-2 bg-[#ea580c]" />
        </div>
        <div className="flex-1 p-6 flex items-center justify-center">
          <div className="animate-pulse text-muted-foreground text-xs">Loading...</div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
          <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
            tier_distribution.chart
          </span>
          <span className="inline-block h-2 w-2 bg-[#ea580c]" />
        </div>
        <div className="flex-1 p-6 flex items-center justify-center">
          <div className="text-muted-foreground text-xs">{error}</div>
        </div>
      </div>
    )
  }

  const total = Object.values(data).reduce((sum, n) => sum + n, 0)

  const tiers: { tier: Tier; count: number; percent: number }[] = [
    { tier: "DIAMOND", count: data.DIAMOND, percent: (data.DIAMOND / total) * 100 },
    { tier: "GOLD", count: data.GOLD, percent: (data.GOLD / total) * 100 },
    { tier: "SILVER", count: data.SILVER, percent: (data.SILVER / total) * 100 },
    { tier: "BRONZE", count: data.BRONZE, percent: (data.BRONZE / total) * 100 },
  ]

  // Calculate angles for pie chart
  let cumulativeAngle = 0
  const segments = tiers.map(t => {
    const startAngle = cumulativeAngle
    const angle = (t.count / total) * 360
    cumulativeAngle += angle
    return { ...t, startAngle, angle }
  })

  function polarToCartesian(cx: number, cy: number, radius: number, angleInDegrees: number) {
    const angleInRadians = ((angleInDegrees - 90) * Math.PI) / 180
    return {
      x: cx + radius * Math.cos(angleInRadians),
      y: cy + radius * Math.sin(angleInRadians),
    }
  }

  function describeArc(cx: number, cy: number, radius: number, startAngle: number, endAngle: number) {
    const start = polarToCartesian(cx, cy, radius, endAngle)
    const end = polarToCartesian(cx, cy, radius, startAngle)
    const largeArcFlag = endAngle - startAngle <= 180 ? "0" : "1"
    return [
      "M", cx, cy,
      "L", start.x, start.y,
      "A", radius, radius, 0, largeArcFlag, 0, end.x, end.y,
      "Z"
    ].join(" ")
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          tier_distribution.chart
        </span>
        <span className="inline-block h-2 w-2 bg-[#ea580c]" />
      </div>

      {/* Content */}
      <div className="flex-1 p-6">
        {/* Pie Chart */}
        <div className="flex justify-center mb-6">
          <svg width="140" height="140" viewBox="0 0 100 100">
            {segments.map((seg, i) => (
              <motion.path
                key={seg.tier}
                d={describeArc(50, 50, 45, seg.startAngle, seg.startAngle + seg.angle - 0.5)}
                fill={TIER_CONFIG[seg.tier].color}
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ delay: 0.2 + i * 0.1, duration: 0.4, ease }}
              />
            ))}
            {/* Center circle */}
            <circle cx="50" cy="50" r="25" className="fill-background" />
            <text
              x="50"
              y="48"
              textAnchor="middle"
              className="fill-foreground font-mono text-[10px] font-bold"
            >
              {total}
            </text>
            <text
              x="50"
              y="56"
              textAnchor="middle"
              className="fill-muted-foreground text-[5px] uppercase tracking-widest"
            >
              NODES
            </text>
          </svg>
        </div>

        {/* Legend */}
        <div className="flex flex-col gap-3">
          {tiers.map((t, i) => (
            <motion.div
              key={t.tier}
              initial={{ opacity: 0, x: -10 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: 0.4 + i * 0.08, duration: 0.4, ease }}
              className="flex items-center justify-between"
            >
              <div className="flex items-center gap-2">
                <div
                  className="w-3 h-3"
                  style={{ backgroundColor: TIER_CONFIG[t.tier].color }}
                />
                <span className="text-[10px] tracking-widest uppercase font-mono text-foreground">
                  {t.tier}
                </span>
              </div>
              <div className="flex items-center gap-3">
                <span className="text-xs font-mono font-bold">{t.count}</span>
                <span className="text-[10px] font-mono text-muted-foreground w-12 text-right">
                  {t.percent.toFixed(0)}%
                </span>
              </div>
            </motion.div>
          ))}
        </div>

        {/* Tier pricing info */}
        <div className="mt-6 pt-4 border-t-2 border-foreground">
          <div className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono mb-2">
            MONTHLY SUBSCRIPTION
          </div>
          <div className="grid grid-cols-4 gap-2">
            {(["BRONZE", "SILVER", "GOLD", "DIAMOND"] as Tier[]).map(tier => (
              <div key={tier} className="text-center">
                <div
                  className="text-xs font-mono font-bold"
                  style={{ color: TIER_CONFIG[tier].color }}
                >
                  ${TIER_CONFIG[tier].monthly_price}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
