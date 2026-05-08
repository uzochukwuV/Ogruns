#!/usr/bin/env node

import { loadConfig, validateConfig } from "./core/config.js";
import { SignalConsumer } from "./signals/consumer.js";
import { GMXExecutor } from "./gmx/executor.js";
import { PaperExecutor } from "./gmx/paper-executor.js";
import type { ActiveSignal, NodeStats } from "./core/types.js";

// Parse command line args
const args = process.argv.slice(2);
const PAPER_MODE = args.includes("--paper") || process.env.PAPER_MODE === "true";

// ─── ASCII Banner ────────────────────────────────────────────────────────────

const BANNER = `
╔═══════════════════════════════════════════════════════════════════╗
║                                                                   ║
║   ███████╗███████╗███╗   ██╗████████╗██╗███╗   ██╗███████╗██╗     ║
║   ██╔════╝██╔════╝████╗  ██║╚══██╔══╝██║████╗  ██║██╔════╝██║     ║
║   ███████╗█████╗  ██╔██╗ ██║   ██║   ██║██╔██╗ ██║█████╗  ██║     ║
║   ╚════██║██╔══╝  ██║╚██╗██║   ██║   ██║██║╚██╗██║██╔══╝  ██║     ║
║   ███████║███████╗██║ ╚████║   ██║   ██║██║ ╚████║███████╗███████╗║
║   ╚══════╝╚══════╝╚═╝  ╚═══╝   ╚═╝   ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝║
║                                                                   ║
║   Autonomous AI Trading Agent                                     ║
║   Powered by 0G Compute • Executing on GMX/Arbitrum               ║
║                                                                   ║
╚═══════════════════════════════════════════════════════════════════╝
`;

// ─── Main Agent ──────────────────────────────────────────────────────────────

class SentinelAgent {
  private config;
  private consumer: SignalConsumer;
  private executor: GMXExecutor | PaperExecutor;
  private isRunning = false;
  private signalsReceived = 0;
  private signalsFollowed = 0;
  private paperMode: boolean;

  constructor(paperMode: boolean = false) {
    this.config = loadConfig();
    this.paperMode = paperMode;
    this.consumer = new SignalConsumer(this.config);

    // Use paper executor for testing, real GMX executor for live trading
    this.executor = paperMode
      ? new PaperExecutor(this.config)
      : new GMXExecutor(this.config);
  }

  async start(): Promise<void> {
    console.log(BANNER);

    // Validate configuration
    validateConfig(this.config);

    // Initialize GMX executor
    console.log("\n🔧 Initializing GMX connection...");
    await this.executor.initialize();

    // Fetch initial node stats
    console.log("\n📊 Fetching node reputations...");
    await this.consumer.fetchNodeStats();

    // Register signal handler
    this.consumer.onNewSignal(async (signal: ActiveSignal, nodeStats: NodeStats | null) => {
      await this.handleSignal(signal, nodeStats);
    });

    // Connect to signal stream
    console.log("\n🔌 Connecting to signal stream...");
    this.consumer.connect();

    // Fetch any existing active signals we might have missed
    console.log("\n📥 Checking for existing active signals...");
    await this.consumer.processExistingSignals();

    this.isRunning = true;

    // Print config summary
    console.log("\n" + "═".repeat(60));
    console.log("⚙️  CONFIGURATION");
    console.log("═".repeat(60));
    console.log(`   Mode: ${this.paperMode ? "📝 PAPER TRADING" : "💰 LIVE TRADING"}`);
    console.log(`   Min Trust Score: ${this.config.minTrustScore}`);
    console.log(`   Min Tier: ${this.config.minTier}`);
    console.log(`   Max Position: $${this.config.maxPositionUsd}`);
    console.log(`   Max Open Positions: ${this.config.maxOpenPositions}`);
    console.log(`   Auto Stop Loss: ${this.config.autoStopLossPct}%`);
    console.log(`   Auto Take Profit: ${this.config.autoTakeProfitPct}%`);
    console.log("═".repeat(60));

    const modeEmoji = this.paperMode ? "📝" : "🟢";
    console.log(`\n${modeEmoji} SENTINEL ACTIVE - Waiting for signals...\n`);

    // Periodic status update
    setInterval(() => this.printStatus(), 60000);
  }

  private async handleSignal(signal: ActiveSignal, nodeStats: NodeStats | null): Promise<void> {
    this.signalsReceived++;

    // Execute the trade
    const execution = await this.executor.executeSignal(signal);

    if (execution.status === "executed") {
      this.signalsFollowed++;
    }
  }

  private printStatus(): void {
    const stats = this.executor.getStats();
    const positions = this.executor.getActivePositionsCount();

    console.log("\n" + "─".repeat(40));
    console.log("📈 STATUS UPDATE");
    console.log("─".repeat(40));
    console.log(`   Signals Received: ${this.signalsReceived}`);
    console.log(`   Signals Followed: ${this.signalsFollowed}`);
    console.log(`   Trades Executed: ${stats.executed}`);
    console.log(`   Trades Failed: ${stats.failed}`);
    console.log(`   Active Positions: ${positions}/${this.config.maxOpenPositions}`);

    // Paper trading specific stats
    if (this.paperMode && this.executor instanceof PaperExecutor) {
      console.log(`   💰 Virtual Balance: $${this.executor.getVirtualBalance().toFixed(2)}`);
      const pnl = this.executor.getVirtualPnL();
      console.log(`   📊 Total P&L: ${pnl >= 0 ? "+" : ""}$${pnl.toFixed(2)}`);
    }

    console.log("─".repeat(40) + "\n");
  }

  stop(): void {
    console.log("\n🛑 Shutting down Sentinel...");
    this.consumer.disconnect();
    this.isRunning = false;
    console.log("✅ Shutdown complete");
  }
}

// ─── Entry Point ─────────────────────────────────────────────────────────────

async function main(): Promise<void> {
  console.log(PAPER_MODE ? "🧪 Starting in PAPER TRADING mode..." : "💰 Starting in LIVE TRADING mode...");
  const agent = new SentinelAgent(PAPER_MODE);

  // Handle graceful shutdown
  process.on("SIGINT", () => {
    agent.stop();
    process.exit(0);
  });

  process.on("SIGTERM", () => {
    agent.stop();
    process.exit(0);
  });

  try {
    await agent.start();
  } catch (error) {
    console.error("Fatal error:", error);
    process.exit(1);
  }
}

main();
