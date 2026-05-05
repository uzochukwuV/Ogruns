/**
 * Core type definitions — mirrors the Go types in pkg/types exactly.
 * The JSON field names must match byte-for-byte for ECDSA signature verification.
 */

export type TradeType = "spot" | "perpetual" | "futures";
export type Direction = "long" | "short";

export interface TakeProfitLevel {
  price:   number;  // Target price
  percent: number;  // Percentage of position to close (0-100)
}

export interface SignalPayload {
  token_pair:   string;          // e.g. "BTC/USDT"
  exchange?:    string;          // e.g. "binance"
  direction:    Direction;       // "long" or "short" (lowercase)
  entry_price:  number;
  take_profit:  number;
  stop_loss:    number;
  expiry_time:  number;          // Unix timestamp (seconds)
  weight_pct:   number;          // AI confidence 0–100; scales reputation impact

  // v2 fields - trade type and leverage
  trade_type?:   TradeType;      // "spot", "perpetual", "futures" (default: "spot")
  leverage?:     number;         // 1x for spot, 1-125x for perpetuals

  // Optional: multiple take profit levels
  take_profits?: TakeProfitLevel[];
}

export interface SignalEnvelope {
  node_id:           string;   // Ethereum address (0x…) derived from private key
  timestamp:         number;   // Unix timestamp when the signal was created
  payload:           SignalPayload;
  signature:         string;   // EIP-191 ECDSA signature over timestamp:payload_json
  encryption_pubkey?: string;  // Future: subscriber-side decryption
}

/**
 * The result your Analyzer must return.
 * Return null to skip this polling cycle (no signal to publish).
 */
export interface AnalysisResult {
  direction:   Direction;
  entryPrice:  number;
  takeProfit:  number;
  stopLoss:    number;
  expiryHours: number;         // How many hours until the signal expires
  confidence:  number;         // 0–100; maps directly to weight_pct
  tradeType?:  TradeType;      // Optional: "spot", "perpetual", "futures"
  leverage?:   number;         // Optional: leverage multiplier (1-125x)
}

export interface NodeConfig {
  tokenPair:          string;
  exchange:           string;
  privateKeyHex:      string;  // Node private key (hex, with or without 0x)
  zgRpcUrl:           string;
  zgIndexerUrl:       string;
  registryAddress:    string;
  pollIntervalSec:    number;
  inferenceEndpoint?: string;  // Optional 0G Inference URL
}
