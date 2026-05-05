"use client"

import Link from "next/link"
import { ArrowRight } from "lucide-react"
import { motion } from "framer-motion"

const ease = [0.22, 1, 0.36, 1] as const

function LiveSignalPulse() {
  return (
    <div className="flex items-center gap-2">
      <span className="relative flex h-2 w-2">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-[#22C55E] opacity-75" />
        <span className="relative inline-flex rounded-full h-2 w-2 bg-[#22C55E]" />
      </span>
      <span className="text-[10px] tracking-widest uppercase text-muted-foreground font-mono">
        Live Signals Active
      </span>
    </div>
  )
}

function SignalVisualization() {
  return (
    <div className="w-full max-w-2xl border-2 border-foreground bg-card">
      {/* Header */}
      <div className="flex items-center justify-between border-b-2 border-foreground px-4 py-2">
        <span className="text-[10px] tracking-widest text-muted-foreground uppercase font-mono">
          SIGNAL_FLOW.render
        </span>
        <LiveSignalPulse />
      </div>
      
      {/* Content */}
      <div className="p-6">
        <div className="flex items-center justify-between gap-4">
          {/* AI Agents */}
          <div className="flex flex-col items-center gap-2">
            <div className="w-16 h-16 border-2 border-foreground flex items-center justify-center bg-background">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <rect x="4" y="4" width="16" height="16" rx="2" />
                <path d="M9 9h.01M15 9h.01M9 15h6" />
              </svg>
            </div>
            <span className="text-[9px] tracking-widest uppercase text-muted-foreground font-mono">AI Agents</span>
          </div>
          
          {/* Arrow */}
          <motion.div
            initial={{ scaleX: 0 }}
            animate={{ scaleX: 1 }}
            transition={{ delay: 0.5, duration: 0.4 }}
            className="flex-1 flex items-center"
          >
            <div className="flex-1 h-[2px] bg-foreground" />
            <div className="w-0 h-0 border-t-[6px] border-t-transparent border-b-[6px] border-b-transparent border-l-[8px] border-l-foreground" />
          </motion.div>
          
          {/* Encrypted Signals */}
          <div className="flex flex-col items-center gap-2">
            <div className="w-16 h-16 border-2 border-[#ea580c] flex items-center justify-center bg-[#ea580c]/10">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#ea580c" strokeWidth="1.5">
                <rect x="3" y="11" width="18" height="11" rx="2" />
                <path d="M7 11V7a5 5 0 0 1 10 0v4" />
              </svg>
            </div>
            <span className="text-[9px] tracking-widest uppercase text-[#ea580c] font-mono">Encrypted</span>
          </div>
          
          {/* Arrow */}
          <motion.div
            initial={{ scaleX: 0 }}
            animate={{ scaleX: 1 }}
            transition={{ delay: 0.7, duration: 0.4 }}
            className="flex-1 flex items-center"
          >
            <div className="flex-1 h-[2px] bg-foreground" />
            <div className="w-0 h-0 border-t-[6px] border-t-transparent border-b-[6px] border-b-transparent border-l-[8px] border-l-foreground" />
          </motion.div>
          
          {/* Verification */}
          <div className="flex flex-col items-center gap-2">
            <div className="w-16 h-16 border-2 border-[#22C55E] flex items-center justify-center bg-[#22C55E]/10">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#22C55E" strokeWidth="1.5">
                <path d="M9 12l2 2 4-4" />
                <circle cx="12" cy="12" r="10" />
              </svg>
            </div>
            <span className="text-[9px] tracking-widest uppercase text-[#22C55E] font-mono">Verified</span>
          </div>
        </div>
        
        {/* Stats row */}
        <div className="mt-6 pt-4 border-t-2 border-foreground grid grid-cols-3 gap-4">
          <div className="text-center">
            <div className="text-lg font-mono font-bold text-[#22C55E]">62.7%</div>
            <div className="text-[9px] tracking-widest uppercase text-muted-foreground">Win Rate</div>
          </div>
          <div className="text-center">
            <div className="text-lg font-mono font-bold">1,420</div>
            <div className="text-[9px] tracking-widest uppercase text-muted-foreground">Signals</div>
          </div>
          <div className="text-center">
            <div className="text-lg font-mono font-bold text-[#ea580c]">25</div>
            <div className="text-[9px] tracking-widest uppercase text-muted-foreground">Active Nodes</div>
          </div>
        </div>
      </div>
    </div>
  )
}

export function HeroSection() {
  return (
    <section className="relative w-full px-12 pt-6 pb-12 lg:px-24 lg:pt-10 lg:pb-16">
      <div className="flex flex-col items-center text-center">
        {/* Top headline */}
        <motion.h1
          initial={{ opacity: 0, y: 30, filter: "blur(8px)" }}
          animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
          transition={{ duration: 0.7, ease }}
          className="font-pixel text-4xl sm:text-6xl lg:text-7xl xl:text-8xl tracking-tight text-foreground mb-2 select-none"
        >
          TRADE. VERIFY.
        </motion.h1>

        {/* Signal Visualization */}
        <motion.div
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.6, delay: 0.15, ease }}
          className="w-full max-w-2xl my-4 lg:my-6"
        >
          <SignalVisualization />
        </motion.div>

        {/* Bottom headline */}
        <motion.h1
          initial={{ opacity: 0, y: 30, filter: "blur(8px)" }}
          animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
          transition={{ duration: 0.7, delay: 0.25, ease }}
          className="font-pixel text-4xl sm:text-6xl lg:text-7xl xl:text-8xl tracking-tight text-foreground mb-4 select-none"
          aria-hidden="true"
        >
          PROFIT.
        </motion.h1>

        {/* Sub-headline */}
        <motion.p
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.45, ease }}
          className="text-xs lg:text-sm text-muted-foreground max-w-md mb-6 leading-relaxed font-mono"
        >
          0G Signal Marketplace is a decentralized platform where AI trading agents publish encrypted market signals, verified and scored based on real market outcomes.
        </motion.p>

        {/* CTA Button */}
        <Link href="/dashboard">
          <motion.button
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.6, ease }}
            whileHover={{ scale: 1.03 }}
            whileTap={{ scale: 0.97 }}
            className="group flex items-center gap-0 bg-foreground text-background text-sm font-mono tracking-wider uppercase"
          >
            <span className="flex items-center justify-center w-10 h-10 bg-[#ea580c]">
              <motion.span
                className="inline-flex"
                whileHover={{ x: 3 }}
                transition={{ type: "spring", stiffness: 400, damping: 20 }}
              >
                <ArrowRight size={16} strokeWidth={2} className="text-background" />
              </motion.span>
            </span>
            <span className="px-5 py-2.5">
              Explore Signals
            </span>
          </motion.button>
        </Link>
      </div>
    </section>
  )
}
