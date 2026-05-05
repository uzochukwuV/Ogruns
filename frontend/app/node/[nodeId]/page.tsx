"use client"

import { use, useMemo, useEffect, useState } from "react"
import Link from "next/link"
import { motion } from "framer-motion"
import { ArrowLeft } from "lucide-react"
import { Navbar } from "@/components/navbar"
import { Footer } from "@/components/footer"
import { apiClient } from "@/lib/api-client"
import { NodeHeader } from "@/components/node/node-header"
import { NodeStats } from "@/components/node/node-stats"
import { SignalHistoryTable } from "@/components/node/signal-history"
import { PerformanceByToken } from "@/components/node/performance-by-token"
import { MonthlyReturnsChart } from "@/components/node/monthly-returns"
import { StreaksDisplay } from "@/components/node/streaks-display"
import { PortfolioSimulator } from "@/components/node/portfolio-simulator"
import type { NodeAnalytics } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

function BlinkDot() {
  return <span className="inline-block h-2 w-2 bg-[#ea580c] animate-blink" />
}

export default function NodeDetailPage({
  params,
}: {
  params: Promise<{ nodeId: string }>
}) {
  const { nodeId } = use(params)
  const [analytics, setAnalytics] = useState<NodeAnalytics | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchAnalytics = async () => {
      try {
        const data = await apiClient.getNodeAnalytics(nodeId)
        setAnalytics(data)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load node analytics')
        console.error('Node analytics error:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchAnalytics()
  }, [nodeId])

  if (loading) {
    return (
      <div className="min-h-screen dot-grid-bg">
        <Navbar />
        <main className="w-full px-6 py-12 lg:px-12">
          <div className="text-center py-20 border-2 border-foreground">
            <div className="animate-pulse text-sm font-mono text-muted-foreground">
              Loading node data...
            </div>
          </div>
        </main>
        <Footer />
      </div>
    )
  }

  if (error || !analytics) {
    return (
      <div className="min-h-screen dot-grid-bg">
        <Navbar />
        <main className="w-full px-6 py-12 lg:px-12">
          <div className="text-center py-20 border-2 border-foreground">
            <span className="text-sm font-mono text-muted-foreground">
              {error || 'Node not found'}
            </span>
          </div>
        </main>
        <Footer />
      </div>
    )
  }

  return (
    <div className="min-h-screen dot-grid-bg">
      <Navbar />
      <main className="w-full px-6 py-12 lg:px-12">
        {/* Back link */}
        <motion.div
          initial={{ opacity: 0, x: -20 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.4, ease }}
        >
          <Link
            href="/marketplace"
            className="inline-flex items-center gap-2 text-[10px] tracking-widest uppercase font-mono text-muted-foreground hover:text-foreground transition-colors mb-6"
          >
            <ArrowLeft size={12} /> Back to Marketplace
          </Link>
        </motion.div>

        {/* Section label */}
        <motion.div
          initial={{ opacity: 0, x: -20 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.5, ease }}
          className="flex items-center gap-4 mb-8"
        >
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
            {"// NODE: DETAIL_VIEW"}
          </span>
          <div className="flex-1 border-t border-border" />
          <BlinkDot />
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">003</span>
        </motion.div>

        {/* Node Header */}
        <NodeHeader stats={analytics.stats} />

        {/* Stats Grid */}
        <NodeStats stats={analytics.stats} />

        {/* Main Content */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-0 mt-8">
          {/* Left Column - Performance & Streaks */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2, duration: 0.5, ease }}
            className="lg:col-span-1 border-2 border-foreground"
          >
            <PerformanceByToken performance={analytics.performance_by_token} />
            <div className="border-t-2 border-foreground">
              <StreaksDisplay streaks={analytics.streaks} />
            </div>
          </motion.div>

          {/* Right Column - Monthly Returns */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.3, duration: 0.5, ease }}
            className="lg:col-span-2 border-2 lg:border-l-0 border-foreground"
          >
            <MonthlyReturnsChart returns={analytics.monthly_returns} />
          </motion.div>
        </div>

        {/* Portfolio Simulator */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4, duration: 0.5, ease }}
          className="mt-8"
        >
          <PortfolioSimulator nodeId={nodeId} />
        </motion.div>

        {/* Signal History */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.5, duration: 0.5, ease }}
          className="mt-8"
        >
          <SignalHistoryTable signals={analytics.signal_history} />
        </motion.div>
      </main>
      <Footer />
    </div>
  )
}
