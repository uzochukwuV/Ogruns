import type { AgentConfig, ActiveSignal, TradeExecution } from "../core/types.js";
import { GmxApiSdk, PrivateKeySigner } from "@gmx-io/sdk/v2";
import type { PrepareOrderRequest } from "@gmx-io/sdk/v2";
import { createPublicClient, createWalletClient, http, parseAbi, formatEther } from "viem";
import { arbitrum, arbitrumSepolia } from "viem/chains";
import { privateKeyToAccount } from "viem/accounts";

// ERC20 ABI for approvals
const ERC20_ABI = parseAbi([
  "function approve(address spender, uint256 amount) returns (bool)",
  "function allowance(address owner, address spender) view returns (uint256)",
  "function balanceOf(address) view returns (uint256)"
]);

// Contract addresses per chain
const CONTRACTS = {
  // Arbitrum Sepolia (Testnet)
  421614: {
    collateralToken: "USDC.SG",
    collateralAddress: "0x3253a335E7bFfB4790Aa4C25C4250d206E9b9773",
    router: "0xEd50B2A1eF0C35DAaF08Da6486971180237909c3",
    orderVault: "0x1b8ac606de71686fd2a1aedecb6e0efba28909a2",
    syntheticsRouter: "0x72f13a44c8ba16a678cad549f17bc9e06d2b8bd2",
    tokenMapping: {
      "BTC/USDT": "BTC/USD [BTC-USDC.SG]",
      "ETH/USDT": "ETH/USD [WETH-USDC.SG]",
    }
  },
  // Arbitrum One (Mainnet)
  42161: {
    collateralToken: "USDC",
    collateralAddress: "0xaf88d065e77c8cC2239327C5EDb3A432268e5831", // USDC on Arbitrum
    router: "0x7C68C7866A64FA2160F78EEaE12217FFbf871fa8",
    orderVault: "0x31eF83a530Fde1B38EE9A18093A333D8Bbbc40D5",
    syntheticsRouter: "0x3B070aA6847bd0fB56eFAdB351f49BBb7619dbc2",
    tokenMapping: {
      "BTC/USDT": "BTC/USD [WBTC-USDC]",
      "ETH/USDT": "ETH/USD [WETH-USDC]",
      "SOL/USDT": "SOL/USD [SOL-USDC]",
      "ARB/USDT": "ARB/USD [ARB-USDC]",
      "DOGE/USDT": "DOGE/USD [WETH-USDC]",
      "LINK/USDT": "LINK/USD [LINK-USDC]",
    }
  }
};

interface WalletBalance {
  eth: bigint;
  collateral: bigint;
}

interface OrderStatus {
  requestId: string;
  status: string;
  txHash?: string;
  error?: string;
}

export class GMXExecutor {
  private config: AgentConfig;
  private sdk: GmxApiSdk | null = null;
  private signer: PrivateKeySigner | null = null;
  private walletAddress: string = "";
  private activePositions: Map<string, TradeExecution> = new Map();
  private executionHistory: TradeExecution[] = [];
  private pendingOrders: Map<string, OrderStatus> = new Map();
  private contracts: typeof CONTRACTS[421614];
  private publicClient: any;
  private walletClient: any;
  private account: any;

  constructor(config: AgentConfig) {
    this.config = config;
    this.contracts = CONTRACTS[config.chainId as 421614 | 42161] || CONTRACTS[421614];
  }

  async initialize(): Promise<void> {
    try {
      const chain = this.config.chainId === 421614 ? arbitrumSepolia : arbitrum;

      // Initialize viem clients for direct contract calls
      this.account = privateKeyToAccount(this.config.privateKey as `0x${string}`);
      this.publicClient = createPublicClient({
        chain,
        transport: http(this.config.arbitrumRpcUrl)
      });
      this.walletClient = createWalletClient({
        account: this.account,
        chain,
        transport: http(this.config.arbitrumRpcUrl)
      });

      // Initialize GMX API SDK
      this.sdk = new GmxApiSdk({
        chainId: this.config.chainId as 42161 | 421614
      });

      // Initialize signer for GMX SDK
      this.signer = new PrivateKeySigner(
        this.config.privateKey as `0x${string}`,
        { rpcUrl: this.config.arbitrumRpcUrl }
      );
      this.walletAddress = this.signer.address;

      const isTestnet = this.config.chainId === 421614;
      const networkName = isTestnet ? "Arbitrum Sepolia (TESTNET)" : "Arbitrum One (MAINNET)";

      console.log("✅ GMX SDK initialized");
      console.log(`   Wallet: ${this.walletAddress}`);
      console.log(`   Chain: ${networkName}`);
      console.log(`   Collateral: ${this.contracts.collateralToken}`);

      if (isTestnet) {
        console.log(`   🧪 TESTNET MODE - No real funds at risk`);
      } else {
        console.log(`   ⚠️  MAINNET MODE - Real funds will be used!`);
      }

      // Check and set up approvals
      await this.ensureApprovals();

      // Fetch current balances and positions
      await this.syncBalances();
      await this.syncPositions();

    } catch (error) {
      console.error("Failed to initialize GMX SDK:", error);
      throw error;
    }
  }

  // Get wallet balances
  async getBalances(): Promise<WalletBalance> {
    const eth = await this.publicClient.getBalance({ address: this.walletAddress });
    const collateral = await this.publicClient.readContract({
      address: this.contracts.collateralAddress as `0x${string}`,
      abi: ERC20_ABI,
      functionName: "balanceOf",
      args: [this.walletAddress]
    });
    return { eth, collateral: collateral as bigint };
  }

  // Sync and display balances
  async syncBalances(): Promise<void> {
    const balances = await this.getBalances();
    console.log(`💰 Wallet Balances:`);
    console.log(`   ETH: ${formatEther(balances.eth)}`);
    console.log(`   ${this.contracts.collateralToken}: ${Number(balances.collateral) / 1e6}`);
  }

  // Ensure all required token approvals are in place
  async ensureApprovals(): Promise<void> {
    console.log("🔐 Checking token approvals...");

    const maxApproval = BigInt("115792089237316195423570985008687907853269984665640564039457584007913129639935");
    const spenders = [
      { name: "Router", address: this.contracts.router },
      { name: "OrderVault", address: this.contracts.orderVault },
      { name: "SyntheticsRouter", address: this.contracts.syntheticsRouter },
    ];

    for (const spender of spenders) {
      const allowance = await this.publicClient.readContract({
        address: this.contracts.collateralAddress as `0x${string}`,
        abi: ERC20_ABI,
        functionName: "allowance",
        args: [this.walletAddress, spender.address]
      }) as bigint;

      if (allowance < BigInt(1e30)) {
        console.log(`   Approving ${spender.name}...`);
        const hash = await this.walletClient.writeContract({
          address: this.contracts.collateralAddress as `0x${string}`,
          abi: ERC20_ABI,
          functionName: "approve",
          args: [spender.address as `0x${string}`, maxApproval]
        });
        await this.publicClient.waitForTransactionReceipt({ hash });
        console.log(`   ✅ ${spender.name} approved`);
      }
    }
    console.log("   ✅ All approvals in place");
  }

  // Swap ETH to collateral token when needed
  async swapEthToCollateral(ethAmount: bigint): Promise<boolean> {
    if (!this.sdk || !this.signer) return false;

    console.log(`🔄 Swapping ${formatEther(ethAmount)} ETH to ${this.contracts.collateralToken}...`);

    try {
      const swapRequest: PrepareOrderRequest = {
        kind: "swap",
        orderType: "market",
        collateralToPay: {
          amount: ethAmount,
          token: "ETH"
        },
        receiveToken: this.contracts.collateralToken,
        mode: "classic",
        from: this.walletAddress,
      };

      const prepared = await this.sdk.prepareOrder(swapRequest);

      if (prepared.payloadType === "transaction") {
        const tx = prepared.payload as { to: string; data: string; value?: bigint };
        const txHash = await this.signer.sendTransaction({
          to: tx.to,
          data: tx.data,
          value: tx.value
        });
        const hash = typeof txHash === "string" ? txHash : txHash.hash;
        console.log(`   ✅ Swap TX: ${hash}`);

        // Wait for swap to complete
        await new Promise(r => setTimeout(r, 5000));
        return true;
      }
    } catch (error: any) {
      console.error(`   ❌ Swap failed: ${error.message}`);
    }
    return false;
  }

  // Get GMX symbol for a token pair
  private getGMXSymbol(tokenPair: string): string | null {
    return this.contracts.tokenMapping[tokenPair as keyof typeof this.contracts.tokenMapping] || null;
  }

  // Sync current positions from GMX
  async syncPositions(): Promise<void> {
    if (!this.sdk) return;

    try {
      const positions = await this.sdk.fetchPositionsInfo({ address: this.walletAddress });

      console.log(`📊 Current GMX Positions: ${positions.length}`);
      for (const pos of positions) {
        const size = Number(pos.sizeInUsd) / 1e30;
        const pnl = Number(pos.pnlAfterFees) / 1e30;
        console.log(`   ${pos.indexName} ${pos.isLong ? "LONG" : "SHORT"} - Size: $${size.toFixed(2)}, PnL: $${pnl.toFixed(2)}`);
      }
    } catch (error) {
      console.error("Failed to sync positions:", error);
    }
  }

  // Get positions data for dashboard
  async getPositionsForDashboard(): Promise<{ positions: Array<{ symbol: string; direction: string; sizeUsd: number; pnl: number; entryPrice: number }>; totalPnL: number }> {
    if (!this.sdk) return { positions: [], totalPnL: 0 };

    try {
      const rawPositions = await this.sdk.fetchPositionsInfo({ address: this.walletAddress });
      let totalPnL = 0;

      const positions = rawPositions.map(pos => {
        const pnl = Number(pos.pnlAfterFees) / 1e30;
        totalPnL += pnl;
        return {
          symbol: pos.indexName || "Unknown",
          direction: pos.isLong ? "long" : "short",
          sizeUsd: Number(pos.sizeInUsd) / 1e30,
          pnl,
          entryPrice: Number(pos.entryPrice) / 1e30
        };
      });

      return { positions, totalPnL };
    } catch (error) {
      console.error("Failed to fetch positions for dashboard:", error);
      return { positions: [], totalPnL: 0 };
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

  // Poll order status until completion
  async waitForOrderExecution(requestId: string, maxAttempts: number = 10): Promise<OrderStatus> {
    if (!this.sdk) throw new Error("SDK not initialized");

    for (let i = 0; i < maxAttempts; i++) {
      await new Promise(r => setTimeout(r, 3000));

      try {
        const status = await this.sdk.fetchOrderStatus({ requestId });
        console.log(`   Order status: ${status.status}`);

        if (status.status === "executed") {
          return {
            requestId,
            status: "executed",
            txHash: status.executionTxnHash
          };
        } else if (status.status === "cancelled" || status.status === "relay_failed" || status.status === "relay_reverted") {
          return {
            requestId,
            status: "failed",
            error: status.cancellationReason || "Order cancelled by keeper"
          };
        }
      } catch (error: any) {
        console.log(`   Status check error: ${error.message}`);
      }
    }

    return { requestId, status: "timeout", error: "Order status check timed out" };
  }

  // Execute a trade based on signal
  async executeSignal(signal: ActiveSignal): Promise<TradeExecution> {
    // Calculate position size and collateral
    const positionSizeUsd = this.config.maxPositionUsd;
    const collateralUsd = Math.floor(positionSizeUsd / 5); // 5x leverage
    const minCollateral = 10; // Minimum $10 collateral for GMX

    const execution: TradeExecution = {
      signalId: signal.id,
      nodeId: signal.envelope.node_id,
      symbol: signal.envelope.payload.token_pair,
      direction: signal.envelope.payload.direction.toLowerCase() as "long" | "short",
      sizeUsd: BigInt(positionSizeUsd) * BigInt(10 ** 30),
      collateralUsd: BigInt(Math.max(collateralUsd, minCollateral)) * BigInt(10 ** 6),
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
      // Check balances and swap if needed
      const balances = await this.getBalances();
      const requiredCollateral = execution.collateralUsd;
      const collateralBalance = balances.collateral;

      console.log(`\n🚀 EXECUTING TRADE`);
      console.log(`   Symbol: ${gmxSymbol}`);
      console.log(`   Direction: ${execution.direction.toUpperCase()}`);
      console.log(`   Size: $${positionSizeUsd}`);
      console.log(`   Collateral: $${Number(requiredCollateral) / 1e6} ${this.contracts.collateralToken}`);

      // Check if we have enough collateral
      if (collateralBalance < requiredCollateral) {
        console.log(`   ⚠️ Insufficient collateral (have: $${Number(collateralBalance) / 1e6})`);

        // Try to swap ETH for collateral
        const ethNeeded = BigInt(Math.ceil(Number(requiredCollateral - collateralBalance) / 2000 * 1.2)) * BigInt(10 ** 15); // Rough ETH estimate with buffer
        const minEthForSwap = BigInt(5) * BigInt(10 ** 14); // 0.0005 ETH minimum

        if (balances.eth > ethNeeded + BigInt(10 ** 15)) { // Keep 0.001 ETH for gas
          const swapAmount = ethNeeded > minEthForSwap ? ethNeeded : minEthForSwap;
          const swapped = await this.swapEthToCollateral(swapAmount);
          if (!swapped) {
            execution.status = "failed";
            execution.error = "Failed to swap ETH for collateral";
            this.executionHistory.push(execution);
            return execution;
          }
        } else {
          execution.status = "failed";
          execution.error = `Insufficient funds: need $${Number(requiredCollateral) / 1e6} ${this.contracts.collateralToken}, have $${Number(collateralBalance) / 1e6}`;
          this.executionHistory.push(execution);
          return execution;
        }
      }

      // Build the order request
      const orderRequest: PrepareOrderRequest = {
        kind: "increase",
        symbol: gmxSymbol,
        direction: execution.direction,
        orderType: "market",
        size: execution.sizeUsd,
        collateralToken: this.contracts.collateralToken,
        collateralToPay: {
          amount: execution.collateralUsd,
          token: this.contracts.collateralToken
        },
        mode: "classic", // Use classic mode (express requires Gelato auth)
        from: this.walletAddress,
      };

      // Prepare and send the order
      console.log(`   Preparing order...`);
      const prepared = await this.sdk.prepareOrder(orderRequest);

      if (prepared.payloadType === "transaction") {
        console.log(`   Sending transaction...`);
        const tx = prepared.payload as { to: string; data: string; value?: bigint };
        const txHash = await this.signer.sendTransaction({
          to: tx.to,
          data: tx.data,
          value: tx.value
        });

        const hash = typeof txHash === "string" ? txHash : txHash.hash;
        execution.txHash = hash;
        console.log(`   ✅ Order TX: ${hash}`);

        // Wait for order execution
        console.log(`   Waiting for execution...`);
        const orderStatus = await this.waitForOrderExecution(prepared.requestId);

        if (orderStatus.status === "executed") {
          execution.status = "executed";
          console.log(`   ✅ Trade executed!`);
          this.activePositions.set(signal.id, execution);
        } else {
          execution.status = "failed";
          execution.error = orderStatus.error || "Order not executed";
          console.log(`   ❌ Trade failed: ${execution.error}`);
        }
      } else {
        execution.status = "failed";
        execution.error = "Unexpected payload type";
      }

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

  // Get wallet balance info for status display
  async getBalanceInfo(): Promise<{ eth: string; collateral: string; collateralSymbol: string }> {
    const balances = await this.getBalances();
    return {
      eth: formatEther(balances.eth),
      collateral: (Number(balances.collateral) / 1e6).toFixed(2),
      collateralSymbol: this.contracts.collateralToken
    };
  }

  // Get wallet address
  getWalletAddress(): string {
    return this.walletAddress;
  }
}
