"use client"

import Link from "next/link"
import { useEffect, useState } from "react"
import { motion } from "framer-motion"
import { ArrowRight } from "lucide-react"
import { apiClient } from "@/lib/api-client"
import { TIER_CONFIG, OUTCOME_COLORS, type NodeStats } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

function truncateAddress(address: string) {
  return `${address.slice(0, 6)}...${address.slice(-4)}`
}

export function TopNodesLeaderboard() {
  const [nodes, setNodes] = useState<NodeStats[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchData = async () => {
      try {
        const dashboardData = await apiClient.getDashboardSummary()
        setNodes(dashboardData.top_nodes)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load top nodes')
        console.error('Top nodes error:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [])

  if (loading) {
    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
          <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
            top_nodes.leaderboard
          </span>
          <Link href="/marketplace" className="flex items-center gap-1 text-[10px] tracking-widest uppercase font-mono text-[#ea580c] hover:underline">
            View All <ArrowRight size={10} />
          </Link>
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
            top_nodes.leaderboard
          </span>
          <Link href="/marketplace" className="flex items-center gap-1 text-[10px] tracking-widest uppercase font-mono text-[#ea580c] hover:underline">
            View All <ArrowRight size={10} />
          </Link>
        </div>
        <div className="flex-1 p-6 flex items-center justify-center">
          <div className="text-muted-foreground text-xs">{error}</div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          top_nodes.leaderboard
        </span>
        <Link href="/marketplace" className="flex items-center gap-1 text-[10px] tracking-widest uppercase font-mono text-[#ea580c] hover:underline">
          View All <ArrowRight size={10} />
        </Link>
      </div>

      {/* Table Header */}
      <div className="grid grid-cols-12 gap-2 px-4 py-2 border-b border-foreground/30 bg-muted/30">
        <div className="col-span-1 text-[9px] tracking-widest uppercase text-muted-foreground font-mono">#</div>
        <div className="col-span-3 text-[9px] tracking-widest uppercase text-muted-foreground font-mono">Node</div>
        <div className="col-span-2 text-[9px] tracking-widest uppercase text-muted-foreground font-mono text-right">Trust</div>
        <div className="col-span-2 text-[9px] tracking-widest uppercase text-muted-foreground font-mono text-right">Win Rate</div>
        <div className="col-span-2 text-[9px] tracking-widest uppercase text-muted-foreground font-mono text-right">Sharpe</div>
        <div className="col-span-2 text-[9px] tracking-widest uppercase text-muted-foreground font-mono text-right">Signals</div>
      </div>

      {/* Table Body */}
      <div className="flex-1 overflow-auto">
        {nodes.map((node, i) => (
          <motion.div
            key={node.node_id}
            initial={{ opacity: 0, x: -10 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ delay: 0.1 + i * 0.05, duration: 0.4, ease }}
          >
            <Link
              href={`/node/${node.node_id}`}
              className="grid grid-cols-12 gap-2 px-4 py-3 border-b border-foreground/10 hover:bg-muted/50 transition-colors"
            >
              <div className="col-span-1 text-xs font-mono text-muted-foreground">
                {String(i + 1).padStart(2, "0")}
              </div>
              <div className="col-span-3 flex items-center gap-2">
                <div
                  className="w-2 h-2 shrink-0"
                  style={{ backgroundColor: TIER_CONFIG[node.tier].color }}
                />
                <span className="text-xs font-mono truncate">{truncateAddress(node.node_id)}</span>
              </div>
              <div className="col-span-2 text-right">
                <span className="text-xs font-mono font-bold">{node.trust_score.toFixed(1)}</span>
              </div>
              <div className="col-span-2 text-right">
                <span
                  className="text-xs font-mono font-bold"
                  style={{ color: node.win_rate >= 0.5 ? OUTCOME_COLORS.green : OUTCOME_COLORS.red }}
                >
                  {(node.win_rate * 100).toFixed(1)}%
                </span>
              </div>
              <div className="col-span-2 text-right">
                <span className="text-xs font-mono">{node.sharpe_ratio.toFixed(2)}</span>
              </div>
              <div className="col-span-2 text-right">
                <span className="text-xs font-mono text-muted-foreground">{node.total_signals}</span>
              </div>
            </Link>
          </motion.div>
        ))}
      </div>
    </div>
  )
}
