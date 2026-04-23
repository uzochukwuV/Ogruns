/**
 * Core type definitions — mirrors the Go types in pkg/types exactly.
 * The JSON field names must match byte-for-byte for ECDSA signature verification.
 */

export interface SignalPayload {
  token_pair:  string;          // e.g. "BTC/USDT"
  exchange:    string;          // e.g. "binance"
  direction:   "LONG" | "SHORT";
  entry_price: number;
  take_profit: number;
  stop_loss:   number;
  expiry_time: number;          // Unix timestamp (seconds)
  weight_pct:  number;          // AI confidence 0–100; scales reputation impact
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
  direction:   "LONG" | "SHORT";
  entryPrice:  number;
  takeProfit:  number;
  stopLoss:    number;
  expiryHours: number;         // How many hours until the signal expires
  confidence:  number;         // 0–100; maps directly to weight_pct
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
