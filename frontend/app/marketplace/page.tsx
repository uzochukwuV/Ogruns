"use client"

import { useState, useMemo, useEffect } from "react"
import Link from "next/link"
import { motion } from "framer-motion"
import { Search, ArrowUpDown, Filter } from "lucide-react"
import { Navbar } from "@/components/navbar"
import { Footer } from "@/components/footer"
import { apiClient } from "@/lib/api-client"
import { TIER_CONFIG, OUTCOME_COLORS, type Tier, type NodeStats } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

type SortField = "trust_score" | "win_rate" | "sharpe_ratio" | "total_signals"
type SortDirection = "asc" | "desc"

function BlinkDot() {
  return <span className="inline-block h-2 w-2 bg-[#ea580c] animate-blink" />
}

function truncateAddress(address: string) {
  return `${address.slice(0, 6)}...${address.slice(-4)}`
}

function NodeCard({ node, index }: { node: NodeStats; index: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: index * 0.05, duration: 0.4, ease }}
    >
      <Link href={`/node/${node.node_id}`}>
        <div className="border-2 border-foreground hover:border-[#ea580c] transition-colors bg-card h-full">
          {/* Header */}
          <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-2">
            <div className="flex items-center gap-2">
              <div
                className="w-3 h-3"
                style={{ backgroundColor: TIER_CONFIG[node.tier].color }}
              />
              <span className="text-[10px] tracking-widest uppercase font-mono">
                {node.tier}
              </span>
            </div>
            <span className="text-[10px] tracking-widest text-muted-foreground font-mono">
              #{String(index + 1).padStart(3, "0")}
            </span>
          </div>

          {/* Content */}
          <div className="p-4">
            {/* Node ID */}
            <div className="mb-4">
              <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono">
                NODE_ID
              </span>
              <div className="text-sm font-mono font-bold truncate mt-1">
                {truncateAddress(node.node_id)}
              </div>
            </div>

            {/* Trust Score */}
            <div className="mb-4">
              <div className="flex items-center justify-between mb-1">
                <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono">
                  TRUST_SCORE
                </span>
                <span className="text-lg font-mono font-bold">{node.trust_score.toFixed(1)}</span>
              </div>
              <div className="w-full h-2 bg-muted">
                <motion.div
                  initial={{ width: 0 }}
                  animate={{ width: `${node.trust_score}%` }}
                  transition={{ delay: 0.3 + index * 0.05, duration: 0.6, ease }}
                  className="h-full"
                  style={{ backgroundColor: TIER_CONFIG[node.tier].color }}
                />
              </div>
            </div>

            {/* Stats Grid */}
            <div className="grid grid-cols-2 gap-3">
              <div>
                <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono">
                  WIN_RATE
                </span>
                <div
                  className="text-sm font-mono font-bold"
                  style={{ color: node.win_rate >= 0.5 ? OUTCOME_COLORS.green : OUTCOME_COLORS.red }}
                >
                  {(node.win_rate * 100).toFixed(1)}%
                </div>
              </div>
              <div>
                <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono">
                  SHARPE
                </span>
                <div className="text-sm font-mono font-bold">
                  {node.sharpe_ratio.toFixed(2)}
                </div>
              </div>
              <div>
                <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono">
                  SIGNALS
                </span>
                <div className="text-sm font-mono font-bold">
                  {node.total_signals}
                </div>
              </div>
              <div>
                <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono">
                  AVG_EV
                </span>
                <div className="text-sm font-mono font-bold">
                  {node.avg_ev.toFixed(2)}
                </div>
              </div>
            </div>

            {/* Win/Loss bar */}
            <div className="mt-4 pt-3 border-t-2 border-foreground">
              <div className="flex items-center gap-2">
                <div className="flex-1 h-2 bg-muted flex overflow-hidden">
                  <div
                    className="h-full"
                    style={{
                      width: `${(node.win_count / node.total_signals) * 100}%`,
                      backgroundColor: OUTCOME_COLORS.green,
                    }}
                  />
                  <div
                    className="h-full"
                    style={{
                      width: `${(node.expired_count / node.total_signals) * 100}%`,
                      backgroundColor: OUTCOME_COLORS.yellow,
                    }}
                  />
                  <div
                    className="h-full"
                    style={{
                      width: `${(node.loss_count / node.total_signals) * 100}%`,
                      backgroundColor: OUTCOME_COLORS.red,
                    }}
                  />
                </div>
              </div>
              <div className="flex items-center justify-between mt-2">
                <span className="text-[9px] font-mono text-[#22C55E]">{node.win_count}W</span>
                <span className="text-[9px] font-mono text-[#EAB308]">{node.expired_count}E</span>
                <span className="text-[9px] font-mono text-[#EF4444]">{node.loss_count}L</span>
              </div>
            </div>
          </div>
        </div>
      </Link>
    </motion.div>
  )
}

export default function MarketplacePage() {
  const [nodes, setNodes] = useState<NodeStats[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState("")
  const [sortField, setSortField] = useState<SortField>("trust_score")
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc")
  const [tierFilter, setTierFilter] = useState<Tier | "ALL">("ALL")

  useEffect(() => {
    const fetchNodes = async () => {
      try {
        const response = await apiClient.getNodes()
        setNodes(response.nodes)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load nodes')
        console.error('Marketplace nodes error:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchNodes()
  }, [])

  const filteredNodes = useMemo(() => {
    let filtered = [...nodes]

    // Filter by search
    if (search) {
      filtered = filtered.filter(n =>
        n.node_id.toLowerCase().includes(search.toLowerCase())
      )
    }

    // Filter by tier
    if (tierFilter !== "ALL") {
      filtered = filtered.filter(n => n.tier === tierFilter)
    }

    // Sort
    filtered.sort((a, b) => {
      const aVal = a[sortField]
      const bVal = b[sortField]
      return sortDirection === "desc" ? bVal - aVal : aVal - bVal
    })

    return filtered
  }, [nodes, search, sortField, sortDirection, tierFilter])

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(d => d === "desc" ? "asc" : "desc")
    } else {
      setSortField(field)
      setSortDirection("desc")
    }
  }

  return (
    <div className="min-h-screen dot-grid-bg">
      <Navbar />
      <main className="w-full px-6 py-12 lg:px-12">
        {/* Section label */}
        <motion.div
          initial={{ opacity: 0, x: -20 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.5, ease }}
          className="flex items-center gap-4 mb-8"
        >
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            {"// MARKETPLACE: NODE_DIRECTORY"}
          </span>
          <div className="flex-1 border-t border-border" />
          <BlinkDot />
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">002</span>
        </motion.div>

        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease }}
          className="mb-8"
        >
          <h1 className="text-2xl lg:text-3xl font-mono font-bold tracking-tight uppercase text-foreground mb-2">
            Node <span className="text-[#ea580c]">Marketplace</span>
          </h1>
          <p className="text-xs lg:text-sm font-mono text-muted-foreground">
            Browse and filter all signal nodes by performance metrics
          </p>
        </motion.div>

        {/* Filters */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1, duration: 0.5, ease }}
          className="border-2 border-foreground p-4 mb-8"
        >
          <div className="flex flex-col lg:flex-row gap-4">
            {/* Search */}
            <div className="flex-1">
              <div className="flex items-center gap-2 border-2 border-foreground bg-background px-3 py-2">
                <Search size={14} className="text-muted-foreground" />
                <input
                  type="text"
                  placeholder="Search by node address..."
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                  className="flex-1 bg-transparent text-xs font-mono outline-none placeholder:text-muted-foreground"
                />
              </div>
            </div>

            {/* Tier Filter */}
            <div className="flex items-center gap-2">
              <Filter size={14} className="text-muted-foreground" />
              <div className="flex gap-0 border-2 border-foreground">
                {(["ALL", "DIAMOND", "GOLD", "SILVER", "BRONZE"] as const).map(tier => (
                  <button
                    key={tier}
                    onClick={() => setTierFilter(tier)}
                    className={`px-3 py-1.5 text-[10px] tracking-widest uppercase font-mono transition-colors ${
                      tierFilter === tier
                        ? "bg-foreground text-background"
                        : "hover:bg-muted"
                    }`}
                  >
                    {tier}
                  </button>
                ))}
              </div>
            </div>

            {/* Sort */}
            <div className="flex items-center gap-2">
              <ArrowUpDown size={14} className="text-muted-foreground" />
              <div className="flex gap-0 border-2 border-foreground">
                {([
                  { field: "trust_score", label: "Trust" },
                  { field: "win_rate", label: "Win %" },
                  { field: "sharpe_ratio", label: "Sharpe" },
                  { field: "total_signals", label: "Signals" },
                ] as const).map(({ field, label }) => (
                  <button
                    key={field}
                    onClick={() => toggleSort(field)}
                    className={`px-3 py-1.5 text-[10px] tracking-widest uppercase font-mono transition-colors ${
                      sortField === field
                        ? "bg-foreground text-background"
                        : "hover:bg-muted"
                    }`}
                  >
                    {label}
                    {sortField === field && (
                      <span className="ml-1">{sortDirection === "desc" ? "↓" : "↑"}</span>
                    )}
                  </button>
                ))}
              </div>
            </div>
          </div>
        </motion.div>

        {/* Results count */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.2, duration: 0.4 }}
          className="flex items-center gap-2 mb-6"
        >
          <span className="text-[10px] tracking-widest uppercase text-muted-foreground font-mono">
            SHOWING {filteredNodes.length} NODES
          </span>
          <div className="flex-1 border-t border-border" />
        </motion.div>

        {/* Loading state */}
        {loading && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <div key={i} className="border-2 border-foreground bg-card h-64 animate-pulse">
                <div className="p-4">
                  <div className="h-4 bg-muted rounded mb-4"></div>
                  <div className="h-8 bg-muted rounded mb-4"></div>
                  <div className="grid grid-cols-2 gap-3 mb-4">
                    <div className="h-6 bg-muted rounded"></div>
                    <div className="h-6 bg-muted rounded"></div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Error state */}
        {error && !loading && (
          <div className="text-center py-20 border-2 border-foreground">
            <div className="text-muted-foreground text-sm mb-2">Failed to load nodes</div>
            <div className="text-muted-foreground/70 text-xs">{error}</div>
          </div>
        )}

        {/* Grid */}
        {!loading && !error && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {filteredNodes.map((node, i) => (
              <NodeCard key={node.node_id} node={node} index={i} />
            ))}
          </div>
        )}

        {!loading && !error && filteredNodes.length === 0 && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="text-center py-20 border-2 border-foreground"
          >
            <span className="text-sm font-mono text-muted-foreground">
              No nodes found matching your criteria
            </span>
          </motion.div>
        )}
      </main>
      <Footer />
    </div>
  )
}
