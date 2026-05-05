"use client"

import { motion } from "framer-motion"
import { Navbar } from "@/components/navbar"
import { Footer } from "@/components/footer"
import { DashboardStats } from "@/components/dashboard/dashboard-stats"
import { TierDistributionChart } from "@/components/dashboard/tier-distribution"
import { TopNodesLeaderboard } from "@/components/dashboard/top-nodes"
import { SignalActivityFeed } from "@/components/dashboard/signal-feed"

const ease = [0.22, 1, 0.36, 1] as const

function BlinkDot() {
  return <span className="inline-block h-2 w-2 bg-[#ea580c] animate-blink" />
}

export default function DashboardPage() {
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
            {"// DASHBOARD: PLATFORM_OVERVIEW"}
          </span>
          <div className="flex-1 border-t border-border" />
          <BlinkDot />
          <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">001</span>
        </motion.div>

        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease }}
          className="mb-8"
        >
          <h1 className="text-2xl lg:text-3xl font-mono font-bold tracking-tight uppercase text-foreground mb-2">
            Signal <span className="text-[#ea580c]">Dashboard</span>
          </h1>
          <p className="text-xs lg:text-sm font-mono text-muted-foreground">
            Platform-wide statistics and real-time signal activity
          </p>
        </motion.div>

        {/* Stats Grid */}
        <DashboardStats />

        {/* Main Content Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-0 mt-8 border-2 border-foreground">
          {/* Tier Distribution */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2, duration: 0.5, ease }}
            className="border-b-2 lg:border-b-0 lg:border-r-2 border-foreground"
          >
            <TierDistributionChart />
          </motion.div>

          {/* Top Nodes Leaderboard */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.3, duration: 0.5, ease }}
            className="lg:col-span-2"
          >
            <TopNodesLeaderboard />
          </motion.div>
        </div>

        {/* Signal Activity Feed */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4, duration: 0.5, ease }}
          className="mt-8"
        >
          <SignalActivityFeed />
        </motion.div>
      </main>
      <Footer />
    </div>
  )
}
