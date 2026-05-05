"use client"

import Link from "next/link"
import { motion } from "framer-motion"

const ease = [0.22, 1, 0.36, 1] as const

export function Footer() {
  return (
    <motion.footer
      initial={{ opacity: 0 }}
      whileInView={{ opacity: 1 }}
      viewport={{ once: true, margin: "-40px" }}
      transition={{ duration: 0.6, ease }}
      className="w-full border-t-2 border-foreground px-6 py-8 lg:px-12"
    >
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-6">
        <div className="flex flex-col gap-1">
          <Link href="/" className="flex items-center gap-2">
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              className="text-[#ea580c]"
            >
              <circle cx="12" cy="12" r="10" />
              <path d="M12 2a10 10 0 0 1 0 20" strokeDasharray="4 4" />
              <circle cx="12" cy="12" r="3" fill="currentColor" />
            </svg>
            <span className="text-xs font-mono tracking-[0.15em] uppercase font-bold text-foreground">
              0G.SIGNALS
            </span>
          </Link>
          <span className="text-[10px] font-mono tracking-widest text-muted-foreground">
            {"(C) 2026 0G LABS. DECENTRALIZED SIGNAL VERIFICATION."}
          </span>
        </div>
        <div className="flex items-center gap-6">
          {[
            { label: "Dashboard", href: "/dashboard" },
            { label: "Marketplace", href: "/marketplace" },
            { label: "Docs", href: "#" },
            { label: "GitHub", href: "#" },
          ].map((link, i) => (
            <motion.div
              key={link.label}
              initial={{ opacity: 0, y: 6 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: 0.1 + i * 0.06, duration: 0.4, ease }}
            >
              <Link
                href={link.href}
                className="text-[10px] font-mono tracking-widest uppercase text-muted-foreground hover:text-foreground transition-colors duration-200"
              >
                {link.label}
              </Link>
            </motion.div>
          ))}
        </div>
      </div>
    </motion.footer>
  )
}
