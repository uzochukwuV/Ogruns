import dotenv from "dotenv";
import type { AgentConfig, NodeStats } from "./types.js";

dotenv.config();

function requireEnv(key: string): string {
  const value = process.env[key];
  if (!value) {
    throw new Error(`Missing required environment variable: ${key}`);
  }
  return value;
}

function optionalEnv(key: string, defaultValue: string): string {
  return process.env[key] || defaultValue;
}

export function loadConfig(): AgentConfig {
  return {
    // Wallet
    privateKey: requireEnv("PRIVATE_KEY"),

    // Signal Platform
    signalApiUrl: optionalEnv("SIGNAL_API_URL", "http://localhost:8080"),
    signalWsUrl: optionalEnv("SIGNAL_WS_URL", "ws://localhost:8080/api/v1/stream"),

    // GMX - Default to Arbitrum Sepolia testnet
    arbitrumRpcUrl: optionalEnv("ARBITRUM_RPC_URL", "https://sepolia-rollup.arbitrum.io/rpc"),
    chainId: parseInt(optionalEnv("CHAIN_ID", "421614")),

    // Trading Filters
    minTrustScore: parseInt(optionalEnv("MIN_TRUST_SCORE", "50")),
    minTier: optionalEnv("MIN_TIER", "SILVER") as NodeStats["tier"],
    maxPositionUsd: parseInt(optionalEnv("MAX_POSITION_USD", "100")),
    collateralToken: optionalEnv("COLLATERAL_TOKEN", "USDC"),

    // Risk Management
    maxOpenPositions: parseInt(optionalEnv("MAX_OPEN_POSITIONS", "3")),
    autoStopLossPct: parseFloat(optionalEnv("AUTO_STOP_LOSS_PCT", "5")),
    autoTakeProfitPct: parseFloat(optionalEnv("AUTO_TAKE_PROFIT_PCT", "10")),
  };
}

export function validateConfig(config: AgentConfig): void {
  // Validate private key format
  if (!config.privateKey.startsWith("0x") || config.privateKey.length !== 66) {
    throw new Error("Invalid private key format");
  }

  // Validate chain ID
  const supportedChains = [42161, 421614]; // Arbitrum One, Arbitrum Sepolia
  if (!supportedChains.includes(config.chainId)) {
    throw new Error(`Unsupported chain ID: ${config.chainId}. Supported: ${supportedChains.join(", ")}`);
  }

  // Validate risk parameters
  if (config.maxPositionUsd <= 0) {
    throw new Error("MAX_POSITION_USD must be positive");
  }

  if (config.autoStopLossPct <= 0 || config.autoStopLossPct > 50) {
    throw new Error("AUTO_STOP_LOSS_PCT must be between 0 and 50");
  }

  console.log("✅ Configuration validated");
}
