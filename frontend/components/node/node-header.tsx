"use client"

import { motion } from "framer-motion"
import { Copy, Check } from "lucide-react"
import { useState } from "react"
import { TIER_CONFIG, type NodeStats } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

export function NodeHeader({ stats }: { stats: NodeStats }) {
  const [copied, setCopied] = useState(false)

  const copyAddress = () => {
    navigator.clipboard.writeText(stats.node_id)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5, ease }}
      className="border-2 border-foreground mb-8"
    >
      <div className="flex flex-col lg:flex-row lg:items-center justify-between p-6">
        {/* Left - Node info */}
        <div className="flex items-start gap-4">
          {/* Tier badge */}
          <div
            className="w-16 h-16 flex items-center justify-center text-xl font-mono font-bold"
            style={{
              backgroundColor: TIER_CONFIG[stats.tier].color,
              color: stats.tier === "DIAMOND" ? "#0f0f0f" : "#fff",
            }}
          >
            {stats.tier.charAt(0)}
          </div>

          <div>
            <div className="flex items-center gap-2 mb-1">
              <span
                className="text-[10px] tracking-widest uppercase font-mono px-2 py-0.5"
                style={{
                  backgroundColor: TIER_CONFIG[stats.tier].color,
                  color: stats.tier === "DIAMOND" ? "#0f0f0f" : "#fff",
                }}
              >
                {stats.tier} TIER
              </span>
              <span className="text-[10px] tracking-widest uppercase font-mono text-muted-foreground">
                ${TIER_CONFIG[stats.tier].monthly_price}/mo
              </span>
            </div>

            <div className="flex items-center gap-2">
              <h1 className="text-lg lg:text-xl font-mono font-bold">
                {stats.node_id.slice(0, 10)}...{stats.node_id.slice(-8)}
              </h1>
              <button
                onClick={copyAddress}
                className="p-1 hover:bg-muted transition-colors"
              >
                {copied ? (
                  <Check size={14} className="text-[#22C55E]" />
                ) : (
                  <Copy size={14} className="text-muted-foreground" />
                )}
              </button>
            </div>

            <p className="text-[10px] tracking-widest uppercase font-mono text-muted-foreground mt-1">
              Last updated: {new Date(stats.updated_at * 1000).toLocaleString()}
            </p>
          </div>
        </div>

        {/* Right - Trust score */}
        <div className="mt-4 lg:mt-0 flex flex-col items-end">
          <span className="text-[10px] tracking-widest uppercase text-muted-foreground font-mono">
            TRUST_SCORE
          </span>
          <div className="flex items-baseline gap-2">
            <span
              className="text-4xl lg:text-5xl font-mono font-bold"
              style={{ color: TIER_CONFIG[stats.tier].color }}
            >
              {stats.trust_score.toFixed(1)}
            </span>
            <span className="text-sm font-mono text-muted-foreground">/100</span>
          </div>
          <div className="w-32 h-2 bg-muted mt-2">
            <motion.div
              initial={{ width: 0 }}
              animate={{ width: `${stats.trust_score}%` }}
              transition={{ delay: 0.3, duration: 0.6, ease }}
              className="h-full"
              style={{ backgroundColor: TIER_CONFIG[stats.tier].color }}
            />
          </div>
        </div>
      </div>
    </motion.div>
  )
}
