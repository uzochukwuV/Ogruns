import type { AgentConfig, ActiveSignal, TradeExecution } from "../core/types.js";

/**
 * Paper Trading Executor - Simulates GMX trades without real funds
 * Perfect for demos and testing the signal flow
 */
export class PaperExecutor {
  private config: AgentConfig;
  private activePositions: Map<string, TradeExecution> = new Map();
  private executionHistory: TradeExecution[] = [];
  private virtualBalance: number = 10000; // Starting balance in USD
  private virtualPnL: number = 0;

  constructor(config: AgentConfig) {
    this.config = config;
  }

  async initialize(): Promise<void> {
    console.log("✅ Paper Trading Mode initialized");
    console.log(`   Virtual Balance: $${this.virtualBalance.toFixed(2)}`);
    console.log(`   ⚠️  No real trades will be executed`);
  }

  canOpenPosition(): { allowed: boolean; reason: string } {
    if (this.activePositions.size >= this.config.maxOpenPositions) {
      return {
        allowed: false,
        reason: `Max positions reached (${this.activePositions.size}/${this.config.maxOpenPositions})`
      };
    }

    if (this.virtualBalance < this.config.maxPositionUsd) {
      return {
        allowed: false,
        reason: `Insufficient balance ($${this.virtualBalance.toFixed(2)} < $${this.config.maxPositionUsd})`
      };
    }

    return { allowed: true, reason: "OK" };
  }

  async executeSignal(signal: ActiveSignal): Promise<TradeExecution> {
    const execution: TradeExecution = {
      signalId: signal.id,
      nodeId: signal.envelope.node_id,
      symbol: signal.envelope.payload.token_pair,
      direction: signal.envelope.payload.direction.toLowerCase() as "long" | "short",
      sizeUsd: BigInt(this.config.maxPositionUsd) * BigInt(10 ** 30),
      collateralUsd: BigInt(this.config.maxPositionUsd) * BigInt(10 ** 6),
      entryPrice: signal.envelope.payload.entry_price,
      stopLoss: signal.envelope.payload.stop_loss,
      takeProfit: signal.envelope.payload.take_profit,
      status: "pending",
      timestamp: Date.now(),
    };

    // Check if we can open position
    const { allowed, reason } = this.canOpenPosition();
    if (!allowed) {
      execution.status = "failed";
      execution.error = reason;
      this.executionHistory.push(execution);
      return execution;
    }

    // Simulate execution
    console.log(`\n📝 PAPER TRADE EXECUTED`);
    console.log(`   Symbol: ${execution.symbol}`);
    console.log(`   Direction: ${execution.direction.toUpperCase()}`);
    console.log(`   Size: $${this.config.maxPositionUsd}`);
    console.log(`   Entry: $${execution.entryPrice}`);
    console.log(`   Stop Loss: $${execution.stopLoss}`);
    console.log(`   Take Profit: $${execution.takeProfit}`);

    // Generate fake tx hash
    execution.txHash = `0x${Math.random().toString(16).slice(2)}${Math.random().toString(16).slice(2)}`;
    execution.status = "executed";

    // Deduct from virtual balance
    this.virtualBalance -= this.config.maxPositionUsd;

    // Track position
    this.activePositions.set(signal.id, execution);

    console.log(`   ✅ Paper trade recorded (TX: ${execution.txHash.slice(0, 18)}...)`);
    console.log(`   💰 Virtual Balance: $${this.virtualBalance.toFixed(2)}`);

    // Schedule position close simulation (at expiry or TP/SL)
    this.schedulePositionClose(signal, execution);

    this.executionHistory.push(execution);
    return execution;
  }

  private schedulePositionClose(signal: ActiveSignal, execution: TradeExecution): void {
    const expiryMs = (signal.envelope.payload.expiry_time * 1000) - Date.now();
    const closeDelay = Math.max(expiryMs, 30000); // At least 30 seconds

    setTimeout(() => {
      this.closePosition(signal.id, execution);
    }, Math.min(closeDelay, 300000)); // Max 5 minutes for demo
  }

  private closePosition(signalId: string, execution: TradeExecution): void {
    if (!this.activePositions.has(signalId)) return;

    // Simulate random outcome based on signal confidence
    const confidence = 0.6; // Simplified - could use actual node win rate
    const isWin = Math.random() < confidence;

    // Calculate simulated P&L
    const pnlPercent = isWin
      ? Math.random() * 5 // 0-5% profit
      : -Math.random() * 3; // 0-3% loss

    const pnlAmount = this.config.maxPositionUsd * (pnlPercent / 100);
    this.virtualPnL += pnlAmount;
    this.virtualBalance += this.config.maxPositionUsd + pnlAmount;

    console.log(`\n📊 PAPER POSITION CLOSED`);
    console.log(`   Symbol: ${execution.symbol}`);
    console.log(`   Outcome: ${isWin ? "✅ WIN" : "❌ LOSS"}`);
    console.log(`   P&L: ${pnlAmount >= 0 ? "+" : ""}$${pnlAmount.toFixed(2)} (${pnlPercent.toFixed(2)}%)`);
    console.log(`   💰 Virtual Balance: $${this.virtualBalance.toFixed(2)}`);
    console.log(`   📈 Total P&L: ${this.virtualPnL >= 0 ? "+" : ""}$${this.virtualPnL.toFixed(2)}`);

    this.activePositions.delete(signalId);
  }

  getStats(): { total: number; executed: number; failed: number; pending: number } {
    const stats = { total: 0, executed: 0, failed: 0, pending: 0 };

    for (const exec of this.executionHistory) {
      stats.total++;
      if (exec.status === "executed") stats.executed++;
      else if (exec.status === "failed") stats.failed++;
      else stats.pending++;
    }

    return stats;
  }

  getActivePositionsCount(): number {
    return this.activePositions.size;
  }

  getVirtualBalance(): number {
    return this.virtualBalance;
  }

  getVirtualPnL(): number {
    return this.virtualPnL;
  }

  // Sync positions (no-op for paper trading)
  async syncPositions(): Promise<void> {
    console.log(`📊 Paper Positions: ${this.activePositions.size}`);
  }
}
