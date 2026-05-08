// ─── Signal Types (from Verification Layer) ─────────────────────────────────

export interface SignalPayload {
  token_pair: string;
  exchange: string;
  direction: "long" | "short" | "LONG" | "SHORT";
  entry_price: number;
  take_profit: number;
  stop_loss: number;
  expiry_time: number;
  weight_pct: number;
}

export interface SignalEnvelope {
  node_id: string;
  timestamp: number;
  payload: SignalPayload;
  signature: string;
}

export interface ActiveSignal {
  id: string;
  envelope: SignalEnvelope;
  state: "ACTIVE" | "WIN" | "LOSS" | "EXPIRED";
  entry_hit_at: number;
  closed_at: number;
  closed_price: number;
}

export interface NodeStats {
  node_id: string;
  total_signals: number;
  win_count: number;
  loss_count: number;
  expired_count: number;
  win_rate: number;
  avg_ev: number;
  sharpe_ratio: number;
  trust_score: number;
  tier: "BRONZE" | "SILVER" | "GOLD" | "DIAMOND";
  updated_at: number;
  is_verified: boolean;
}

// ─── WebSocket Event Types ───────────────────────────────────────────────────

export interface WSSignalEvent {
  type: "signal_update";
  data: ActiveSignal;
}

// ─── GMX Types ───────────────────────────────────────────────────────────────

export interface GMXPosition {
  symbol: string;
  direction: "long" | "short";
  sizeUsd: bigint;
  collateralUsd: bigint;
  entryPrice: bigint;
  markPrice: bigint;
  pnlUsd: bigint;
  leverage: number;
}

export interface TradeExecution {
  signalId: string;
  nodeId: string;
  symbol: string;
  direction: "long" | "short";
  sizeUsd: bigint;
  collateralUsd: bigint;
  entryPrice: number;
  stopLoss: number;
  takeProfit: number;
  txHash?: string;
  status: "pending" | "executed" | "failed";
  timestamp: number;
  error?: string;
}

// ─── Agent Configuration ─────────────────────────────────────────────────────

export interface AgentConfig {
  // Wallet
  privateKey: string;

  // Signal Platform
  signalApiUrl: string;
  signalWsUrl: string;

  // GMX
  arbitrumRpcUrl: string;
  chainId: number;

  // Trading Filters
  minTrustScore: number;
  minTier: NodeStats["tier"];
  maxPositionUsd: number;
  collateralToken: string;

  // Risk Management
  maxOpenPositions: number;
  autoStopLossPct: number;
  autoTakeProfitPct: number;
}

// ─── Token Mapping (GMX Symbols) ─────────────────────────────────────────────

export const TOKEN_TO_GMX_SYMBOL: Record<string, string> = {
  "BTC/USDT": "BTC/USD [WBTC-USDC]",
  "ETH/USDT": "ETH/USD [WETH-USDC]",
  "SOL/USDT": "SOL/USD [SOL-USDC]",
  "ARB/USDT": "ARB/USD [ARB-USDC]",
  "DOGE/USDT": "DOGE/USD [WETH-USDC]", // DOGE uses ETH pool on GMX
  "XRP/USDT": "XRP/USD [WETH-USDC]",   // May not be available
  "LINK/USDT": "LINK/USD [LINK-USDC]",
  "AVAX/USDT": "AVAX/USD [WAVAX-USDC]",
};

export const TIER_PRIORITY: Record<NodeStats["tier"], number> = {
  BRONZE: 1,
  SILVER: 2,
  GOLD: 3,
  DIAMOND: 4,
};
