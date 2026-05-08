import type { AgentConfig, ActiveSignal, TradeExecution, TOKEN_TO_GMX_SYMBOL } from "../core/types.js";

// GMX SDK types (will be properly typed when SDK is installed)
interface GMXSdk {
  executeExpressOrder(params: any, signer: any): Promise<any>;
  fetchPositionsInfo(params: { address: string }): Promise<any[]>;
  fetchMarketsTickers(params: { symbols: string[] }): Promise<any[]>;
}

interface PrivateKeySigner {
  address: string;
}

export class GMXExecutor {
  private config: AgentConfig;
  private sdk: GMXSdk | null = null;
  private signer: PrivateKeySigner | null = null;
  private walletAddress: string = "";
  private activePositions: Map<string, TradeExecution> = new Map();
  private executionHistory: TradeExecution[] = [];

  // Token mapping from signal format to GMX symbols
  private readonly tokenMapping: Record<string, string> = {
    "BTC/USDT": "BTC/USD [WBTC-USDC]",
    "ETH/USDT": "ETH/USD [WETH-USDC]",
    "SOL/USDT": "SOL/USD [SOL-USDC]",
    "ARB/USDT": "ARB/USD [ARB-USDC]",
    "DOGE/USDT": "DOGE/USD [WETH-USDC]",
    "LINK/USDT": "LINK/USD [LINK-USDC]",
  };

  constructor(config: AgentConfig) {
    this.config = config;
  }

  async initialize(): Promise<void> {
    try {
      // Dynamic import of GMX SDK
      const { GmxApiSdk, PrivateKeySigner } = await import("@gmx-io/sdk/v2");

      this.sdk = new GmxApiSdk({ chainId: this.config.chainId });
      this.signer = new PrivateKeySigner(this.config.privateKey);
      this.walletAddress = this.signer?.address!;

      const isTestnet = this.config.chainId === 421614;
      const networkName = isTestnet ? "Arbitrum Sepolia (TESTNET)" : "Arbitrum One (MAINNET)";

      console.log("✅ GMX SDK initialized");
      console.log(`   Wallet: ${this.walletAddress}`);
      console.log(`   Chain: ${networkName}`);

      if (isTestnet) {
        console.log(`   🧪 TESTNET MODE - No real funds at risk`);
        console.log(`   📱 GMX Testnet UI: https://app.gmxtest.io`);
        console.log(`   💧 Get testnet ETH: https://www.alchemy.com/faucets/arbitrum-sepolia`);
      } else {
        console.log(`   ⚠️  MAINNET MODE - Real funds will be used!`);
      }

      // Fetch current positions
      await this.syncPositions();
    } catch (error) {
      console.error("Failed to initialize GMX SDK:", error);
      throw error;
    }
  }

  // Get GMX symbol for a token pair
  private getGMXSymbol(tokenPair: string): string | null {
    return this.tokenMapping[tokenPair] || null;
  }

  // Sync current positions from GMX
  async syncPositions(): Promise<void> {
    if (!this.sdk) return;

    try {
      const positions = await this.sdk.fetchPositionsInfo({ address: this.walletAddress });

      console.log(`📊 Current GMX Positions: ${positions.length}`);
      for (const pos of positions) {
        console.log(`   ${pos.indexName} ${pos.isLong ? "LONG" : "SHORT"} - $${Number(pos.sizeInUsd) / 1e30}`);
      }
    } catch (error) {
      console.error("Failed to sync positions:", error);
    }
  }

  // Check if we can open a new position
  canOpenPosition(): { allowed: boolean; reason: string } {
    if (this.activePositions.size >= this.config.maxOpenPositions) {
      return {
        allowed: false,
        reason: `Max positions reached (${this.activePositions.size}/${this.config.maxOpenPositions})`
      };
    }
    return { allowed: true, reason: "OK" };
  }

  // Execute a trade based on signal
  async executeSignal(signal: ActiveSignal): Promise<TradeExecution> {
    const execution: TradeExecution = {
      signalId: signal.id,
      nodeId: signal.envelope.node_id,
      symbol: signal.envelope.payload.token_pair,
      direction: signal.envelope.payload.direction.toLowerCase() as "long" | "short",
      sizeUsd: BigInt(this.config.maxPositionUsd) * BigInt(10 ** 30),
      collateralUsd: BigInt(this.config.maxPositionUsd) * BigInt(10 ** 6), // USDC has 6 decimals
      entryPrice: signal.envelope.payload.entry_price,
      stopLoss: signal.envelope.payload.stop_loss,
      takeProfit: signal.envelope.payload.take_profit,
      status: "pending",
      timestamp: Date.now(),
    };

    // Get GMX symbol
    const gmxSymbol = this.getGMXSymbol(signal.envelope.payload.token_pair);
    if (!gmxSymbol) {
      execution.status = "failed";
      execution.error = `Token pair ${signal.envelope.payload.token_pair} not supported on GMX`;
      this.executionHistory.push(execution);
      return execution;
    }

    // Check if we can open position
    const { allowed, reason } = this.canOpenPosition();
    if (!allowed) {
      execution.status = "failed";
      execution.error = reason;
      this.executionHistory.push(execution);
      return execution;
    }

    if (!this.sdk || !this.signer) {
      execution.status = "failed";
      execution.error = "SDK not initialized";
      this.executionHistory.push(execution);
      return execution;
    }

    try {
      console.log(`\n🚀 EXECUTING TRADE`);
      console.log(`   Symbol: ${gmxSymbol}`);
      console.log(`   Direction: ${execution.direction.toUpperCase()}`);
      console.log(`   Size: $${this.config.maxPositionUsd}`);
      console.log(`   Stop Loss: $${execution.stopLoss}`);
      console.log(`   Take Profit: $${execution.takeProfit}`);

      // Get current mark price for TP/SL calculation
      const tickers = await this.sdk.fetchMarketsTickers({ symbols: [gmxSymbol] });
      const markPrice = BigInt(tickers[0]?.maxPrice || 0);

      // Calculate TP/SL prices
      const slPrice = BigInt(Math.floor(execution.stopLoss * 10 ** 30));
      const tpPrice = BigInt(Math.floor(execution.takeProfit * 10 ** 30));

      // Execute the order
      const result = await this.sdk.executeExpressOrder({
        kind: "increase",
        symbol: gmxSymbol,
        direction: execution.direction,
        orderType: "market",
        size: execution.sizeUsd,
        collateralToken: this.config.collateralToken,
        collateralToPay: {
          amount: execution.collateralUsd,
          token: this.config.collateralToken
        },
        mode: "express",
        from: this.walletAddress,
        tpsl: [
          { type: "take-profit", triggerPrice: tpPrice, size: execution.sizeUsd },
          { type: "stop-loss", triggerPrice: slPrice, size: execution.sizeUsd },
        ],
      }, this.signer);

      execution.txHash = result?.hash;
      execution.status = "executed";

      console.log(`   ✅ Trade executed! TX: ${execution.txHash || "pending"}`);

      // Track active position
      this.activePositions.set(signal.id, execution);

    } catch (error: any) {
      execution.status = "failed";
      execution.error = error.message || "Unknown error";
      console.error(`   ❌ Trade failed: ${execution.error}`);
    }

    this.executionHistory.push(execution);
    return execution;
  }

  // Get execution stats
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

  // Get active positions count
  getActivePositionsCount(): number {
    return this.activePositions.size;
  }
}
