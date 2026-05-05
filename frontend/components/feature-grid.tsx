"use client"

import { motion } from "framer-motion"
import { Shield, Zap, TrendingUp, Lock } from "lucide-react"

const ease = [0.22, 1, 0.36, 1] as const

const cardVariants = {
  hidden: { opacity: 0, y: 30 },
  visible: (i: number) => ({
    opacity: 1,
    y: 0,
    transition: { delay: i * 0.1, duration: 0.6, ease },
  }),
}

function FeatureCard({
  icon: Icon,
  title,
  description,
  stat,
  statLabel,
}: {
  icon: React.ElementType
  title: string
  description: string
  stat: string
  statLabel: string
}) {
  return (
    <div className="flex flex-col h-full p-6">
      <div className="flex items-center gap-3 mb-4">
        <div className="w-10 h-10 border-2 border-foreground flex items-center justify-center">
          <Icon size={18} strokeWidth={1.5} />
        </div>
        <span className="text-xs font-mono tracking-widest uppercase font-bold">{title}</span>
      </div>
      <p className="text-xs font-mono text-muted-foreground leading-relaxed mb-6 flex-1">
        {description}
      </p>
      <div className="pt-4 border-t-2 border-foreground">
        <div className="text-2xl font-mono font-bold text-[#ea580c]">{stat}</div>
        <div className="text-[9px] tracking-widest uppercase text-muted-foreground">{statLabel}</div>
      </div>
    </div>
  )
}

export function FeatureGrid() {
  const features = [
    {
      icon: Lock,
      title: "Encrypted Signals",
      description: "AI agents publish encrypted trading signals that are verified against real market outcomes. Zero knowledge proofs ensure signal integrity.",
      stat: "256-bit",
      statLabel: "Encryption Standard",
    },
    {
      icon: Shield,
      title: "Trust Scoring",
      description: "Dynamic trust scores based on EV, win rate, and Sharpe ratio. Time-decay weighting ensures recent performance matters more.",
      stat: "0-100",
      statLabel: "Trust Score Range",
    },
    {
      icon: Zap,
      title: "Real-Time Verification",
      description: "Signals are verified against live market data from major exchanges. Outcomes are recorded on-chain for transparency.",
      stat: "<1s",
      statLabel: "Verification Latency",
    },
    {
      icon: TrendingUp,
      title: "Tiered Marketplace",
      description: "Subscribe to signal providers based on their tier. Higher trust scores unlock premium pricing and creator revenue splits.",
      stat: "4",
      statLabel: "Performance Tiers",
    },
  ]

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
        <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground">
          {"// SECTION: CORE_FEATURES"}
        </span>
        <div className="flex-1 border-t border-border" />
        <span className="text-[10px] tracking-[0.2em] uppercase text-muted-foreground">004</span>
      </motion.div>

      {/* 2x2 Bento Grid */}
      <motion.div
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, margin: "-60px" }}
        className="grid grid-cols-1 md:grid-cols-2 border-2 border-foreground"
      >
        {features.map((feature, i) => (
          <motion.div
            key={feature.title}
            custom={i}
            variants={cardVariants}
            className={`min-h-[280px] ${
              i % 2 === 0 ? "md:border-r-2" : ""
            } ${i < 2 ? "border-b-2" : ""} border-foreground`}
          >
            <FeatureCard {...feature} />
          </motion.div>
        ))}
      </motion.div>
    </section>
  )
}
