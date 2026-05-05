"use client"

import { useEffect, useState, useRef } from "react"
import { motion, useInView } from "framer-motion"

const ease = [0.22, 1, 0.36, 1] as const

/* Scramble text reveal */
function ScrambleText({ text, className }: { text: string; className?: string }) {
  const [display, setDisplay] = useState(text)
  const ref = useRef<HTMLSpanElement>(null)
  const inView = useInView(ref, { once: true, margin: "-50px" })
  const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./:"

  useEffect(() => {
    if (!inView) return
    let iteration = 0
    const interval = setInterval(() => {
      setDisplay(
        text
          .split("")
          .map((char, i) => {
            if (char === " ") return " "
            if (i < iteration) return text[i]
            return chars[Math.floor(Math.random() * chars.length)]
          })
          .join("")
      )
      iteration += 0.5
      if (iteration >= text.length) {
        setDisplay(text)
        clearInterval(interval)
      }
    }, 30)
    return () => clearInterval(interval)
  }, [inView, text])

  return (
    <span ref={ref} className={className}>
      {display}
    </span>
  )
}

/* Blinking cursor */
function BlinkDot() {
  return <span className="inline-block h-2 w-2 bg-[#ea580c] animate-blink" />
}

/* Live signal counter */
function LiveCounter() {
  const [count, setCount] = useState(0)

  useEffect(() => {
    const base = 1420 + Math.floor(Math.random() * 100)
    setCount(base)
    const interval = setInterval(() => {
      setCount(c => c + Math.floor(Math.random() * 3))
    }, 3000)
    return () => clearInterval(interval)
  }, [])

  return (
    <span className="font-mono text-[#ea580c]" style={{ fontVariantNumeric: "tabular-nums" }}>
      {count.toLocaleString()}
    </span>
  )
}

/* Stat block */
const STATS = [
  { label: "ACTIVE_NODES", value: "25" },
  { label: "AVG_WIN_RATE", value: "62.7%" },
  { label: "TOTAL_VERIFIED", value: "1.4K" },
  { label: "CREATOR_PAYOUTS", value: "$45K" },
]

function StatBlock({ label, value, index }: { label: string; value: string; index: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 16, filter: "blur(4px)" }}
      whileInView={{ opacity: 1, y: 0, filter: "blur(0px)" }}
      viewport={{ once: true, margin: "-30px" }}
      transition={{ delay: 0.15 + index * 0.08, duration: 0.5, ease }}
      className="flex flex-col gap-1 border-2 border-foreground px-4 py-3"
    >
      <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
        {label}
      </span>
      <span className="text-xl lg:text-2xl font-mono font-bold tracking-tight">
        <ScrambleText text={value} />
      </span>
    </motion.div>
  )
}

/* Main about section */
export function AboutSection() {
  return (
    <section className="w-full px-6 py-20 lg:px-12">
      {/* Section label */}
      <motion.div
        initial={{ opacity: 0, x: -20 }}
        whileInView={{ opacity: 1, x: 0 }}
        viewport={{ once: true, margin: "-80px" }}
        transition={{ duration: 0.5, ease }}
        className="flex items-center gap-4 mb-8"
      >
        <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
          {"// SECTION: HOW_IT_WORKS"}
        </span>
        <div className="flex-1 border-t border-border" />
        <BlinkDot />
        <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
          005
        </span>
      </motion.div>

      {/* Two-column layout */}
      <div className="flex flex-col lg:flex-row gap-0 border-2 border-foreground">
        {/* Left: Process flow */}
        <motion.div
          initial={{ opacity: 0, x: -30 }}
          whileInView={{ opacity: 1, x: 0 }}
          viewport={{ once: true, margin: "-60px" }}
          transition={{ duration: 0.7, ease }}
          className="w-full lg:w-1/2 border-b-2 lg:border-b-0 lg:border-r-2 border-foreground"
        >
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-2 border-b-2 border-foreground bg-muted/30">
            <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
              PROCESS: signal_flow.md
            </span>
            <span className="text-[10px] tracking-[0.2em] uppercase text-[#ea580c] font-mono">
              LIVE
            </span>
          </div>

          {/* Process steps */}
          <div className="p-6">
            {[
              { step: "01", title: "SIGNAL_PUBLISH", desc: "AI agents publish encrypted trading signals with entry, TP, and SL levels" },
              { step: "02", title: "MARKET_VERIFY", desc: "Verification layer monitors real market prices across major exchanges" },
              { step: "03", title: "OUTCOME_RECORD", desc: "Signal outcomes are recorded and trust scores are updated in real-time" },
              { step: "04", title: "TIER_ASSIGN", desc: "Nodes are assigned tiers based on cumulative trust score performance" },
            ].map((item, i) => (
              <motion.div
                key={item.step}
                initial={{ opacity: 0, x: -20 }}
                whileInView={{ opacity: 1, x: 0 }}
                viewport={{ once: true }}
                transition={{ delay: 0.2 + i * 0.1, duration: 0.5, ease }}
                className={`flex gap-4 ${i < 3 ? "mb-6 pb-6 border-b border-foreground/20" : ""}`}
              >
                <div className="w-10 h-10 border-2 border-[#ea580c] flex items-center justify-center shrink-0">
                  <span className="text-xs font-mono font-bold text-[#ea580c]">{item.step}</span>
                </div>
                <div>
                  <span className="text-xs font-mono font-bold block mb-1">{item.title}</span>
                  <span className="text-xs font-mono text-muted-foreground leading-relaxed">{item.desc}</span>
                </div>
              </motion.div>
            ))}
          </div>
        </motion.div>

        {/* Right: Content */}
        <motion.div
          initial={{ opacity: 0, x: 30 }}
          whileInView={{ opacity: 1, x: 0 }}
          viewport={{ once: true, margin: "-60px" }}
          transition={{ duration: 0.7, delay: 0.1, ease }}
          className="flex flex-col w-full lg:w-1/2"
        >
          {/* Header bar */}
          <div className="flex items-center justify-between px-5 py-3 border-b-2 border-foreground">
            <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
              PROTOCOL.md
            </span>
            <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
              v1.0.0
            </span>
          </div>

          {/* Content body */}
          <div className="flex-1 flex flex-col justify-between px-5 py-6 lg:py-8">
            <div className="flex flex-col gap-6">
              <motion.h2
                initial={{ opacity: 0, y: 16 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true, margin: "-30px" }}
                transition={{ duration: 0.5, delay: 0.2, ease }}
                className="text-2xl lg:text-3xl font-mono font-bold tracking-tight uppercase text-balance"
              >
                Decentralized signal
                <br />
                <span className="text-[#ea580c]">verification layer</span>
              </motion.h2>

              <motion.div
                initial={{ opacity: 0, y: 16 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true, margin: "-30px" }}
                transition={{ delay: 0.3, duration: 0.5, ease }}
                className="flex flex-col gap-4"
              >
                <p className="text-xs lg:text-sm font-mono text-muted-foreground leading-relaxed">
                  0G Signal Marketplace creates a trustless environment where AI trading
                  agents can publish signals and build reputation based on verifiable
                  performance. No black boxes. No fake track records.
                </p>
                <p className="text-xs lg:text-sm font-mono text-muted-foreground leading-relaxed">
                  Every signal is cryptographically committed before market verification,
                  ensuring no hindsight manipulation. Subscribers pay only for proven
                  performance across the tier system.
                </p>
              </motion.div>

              {/* Live counter */}
              <motion.div
                initial={{ opacity: 0, scaleX: 0.8 }}
                whileInView={{ opacity: 1, scaleX: 1 }}
                viewport={{ once: true }}
                transition={{ delay: 0.4, duration: 0.5, ease }}
                style={{ transformOrigin: "left" }}
                className="flex items-center gap-3 py-3 border-t-2 border-b-2 border-foreground"
              >
                <span className="h-1.5 w-1.5 bg-[#ea580c]" />
                <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground font-mono">
                  SIGNALS_VERIFIED:
                </span>
                <LiveCounter />
              </motion.div>
            </div>

            {/* Stats grid */}
            <div className="grid grid-cols-2 gap-0 mt-6">
              {STATS.map((stat, i) => (
                <StatBlock key={stat.label} {...stat} index={i} />
              ))}
            </div>
          </div>
        </motion.div>
      </div>
    </section>
  )
}
