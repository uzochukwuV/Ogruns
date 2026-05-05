"use client"

import { useEffect, useState } from "react"
import { motion, AnimatePresence } from "framer-motion"
import { OUTCOME_COLORS, type Signal } from "@/lib/types"

const ease = [0.22, 1, 0.36, 1] as const

function truncateAddress(address: string) {
  return `${address.slice(0, 6)}...${address.slice(-4)}`
}

function formatTimeAgo(timestamp: number): string {
  const now = Date.now() / 1000
  const diff = now - timestamp

  if (diff < 60) return `${Math.floor(diff)}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

interface FeedItem extends Signal {
  node_id: string
}

export function SignalActivityFeed() {
  const [signals, setSignals] = useState<FeedItem[]>([])
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    // TODO: Implement WebSocket connection for real-time signals
    // For now, show placeholder
    setConnected(false)
  }, [])

  return (
    <div className="border-2 border-foreground">
      {/* Header */}
      <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-3">
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          signal_activity.feed
        </span>
        <div className="flex items-center gap-2">
          <span className="inline-block h-2 w-2 bg-[#ea580c] animate-pulse" />
          <span className="text-[10px] tracking-widest uppercase font-mono text-muted-foreground">
            {connected ? 'LIVE' : 'CONNECTING'}
          </span>
        </div>
      </div>

      {/* Placeholder for WebSocket integration */}
      <div className="flex-1 flex items-center justify-center p-6">
        <div className="text-center">
          <div className="text-sm font-mono text-muted-foreground mb-2">
            Real-time signal feed
          </div>
          <div className="text-xs font-mono text-muted-foreground/70">
            WebSocket integration pending
          </div>
        </div>
      </div>
    </div>
  )
}
