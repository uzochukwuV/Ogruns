/**
 * node.ts — the main loop of an Ogruns Signal Node.
 *
 * QUICK START:
 *   1. cp .env.example .env && fill in your keys
 *   2. npm run keygen          # generates NODE_PRIVATE_KEY
 *   3. npm run dev             # starts the node loop
 *
 * WHAT HAPPENS:
 *   Every POLL_INTERVAL_SEC seconds:
 *     a. Fetch recent price history for TOKEN_PAIR from Binance REST API.
 *     b. Call the configured Analyzer (default: RSI strategy).
 *     c. If the analyzer returns a signal → sign it and publish to 0G Storage.
 *     d. The SignalPublished on-chain event notifies the Verifier Layer.
 *
 * CUSTOMISE:
 *   - Replace builtinRSIAnalyzer with your own Analyzer function.
 *   - Switch to 0G Inference by using makeZgInferenceAnalyzer().
 *   - Add more token pairs by running multiple SignalNode instances.
 */

import { ethers } from "ethers";
import * as dotenv from "dotenv";
import axios from "axios";

import { NodeConfig, SignalPayload, AnalysisResult } from "./types";
import { signEnvelope } from "./signer";
import { publishSignal, registerNode, isRegistered } from "./publisher";
import { builtinRSIAnalyzer, makeZgInferenceAnalyzer, Analyzer } from "./analyzer";

dotenv.config();

// ── Configuration ─────────────────────────────────────────────────────────────

function loadConfig(): NodeConfig {
  const pk = process.env.NODE_PRIVATE_KEY;
  if (!pk) throw new Error("NODE_PRIVATE_KEY not set — run: npm run keygen");

  const registryAddr = process.env.REGISTRY_CONTRACT_ADDR;
  if (!registryAddr) throw new Error("REGISTRY_CONTRACT_ADDR not set");

  return {
    tokenPair:         process.env.TOKEN_PAIR || "BTC/USDT",
    exchange:          process.env.EXCHANGE || "binance",
    privateKeyHex:     pk,
    zgRpcUrl:          process.env.ZG_RPC_URL || "https://evmrpc-testnet.0g.ai",
    zgIndexerUrl:      process.env.ZG_INDEXER_URL || "https://indexer-storage-testnet-turbo.0g.ai",
    registryAddress:   registryAddr,
    pollIntervalSec:   parseInt(process.env.POLL_INTERVAL_SEC || "60", 10),
    inferenceEndpoint: process.env.ZG_INFERENCE_ENDPOINT,
  };
}

// ── Price feed ────────────────────────────────────────────────────────────────

/**
 * Fetch the last `limit` 1-minute closing prices from Binance REST API.
 * Returns prices in chronological order (oldest first, newest last).
 */
async function fetchPriceHistory(
  tokenPair: string,
  limit = 100
): Promise<number[]> {
  // Convert "BTC/USDT" → "BTCUSDT"
  const symbol = tokenPair.replace("/", "");
  const url = `https://api.binance.com/api/v3/klines?symbol=${symbol}&interval=1m&limit=${limit}`;

  const res = await axios.get<Array<[number, string, string, string, string]>>(url, {
    timeout: 5000,
  });

  // Kline format: [openTime, open, high, low, close, ...]
  return res.data.map((k) => parseFloat(k[4]));
}

// ── SignalNode class ──────────────────────────────────────────────────────────

export class SignalNode {
  private wallet:   ethers.Wallet;
  private config:   NodeConfig;
  private analyzer: Analyzer;
  private provider: ethers.JsonRpcProvider;

  constructor(config: NodeConfig, analyzer?: Analyzer) {
    this.config   = config;
    this.provider = new ethers.JsonRpcProvider(config.zgRpcUrl);
    this.wallet   = new ethers.Wallet(config.privateKeyHex, this.provider);

    // Use 0G Inference if an endpoint is configured, otherwise fall back to RSI.
    this.analyzer = analyzer
      ?? (config.inferenceEndpoint
          ? makeZgInferenceAnalyzer(
              config.inferenceEndpoint,
              process.env.ZG_INFERENCE_MODEL || "signal-v1"
            )
          : builtinRSIAnalyzer);
  }

  async start(): Promise<void> {
    console.log(`[Node] Starting — address: ${this.wallet.address}`);
    console.log(`[Node] Token pair: ${this.config.tokenPair} | Exchange: ${this.config.exchange}`);

    // Auto-register if not yet on-chain.
    await this.ensureRegistered();

    console.log(`[Node] Poll interval: ${this.config.pollIntervalSec}s`);
    this.poll(); // run immediately, then on interval

    setInterval(() => this.poll(), this.config.pollIntervalSec * 1000);
  }

  private async ensureRegistered(): Promise<void> {
    const registered = await isRegistered(this.wallet, this.config.registryAddress);
    if (registered) {
      console.log(`[Node] Already registered on NodeRegistry.`);
      return;
    }
    console.log(`[Node] Registering on NodeRegistry...`);
    await registerNode(
      this.wallet,
      this.config.registryAddress,
      `${this.config.tokenPair} Signal Node`,
      this.config.tokenPair,
      `Automated signal node for ${this.config.tokenPair} on ${this.config.exchange}`
    );
    console.log(`[Node] Registration complete.`);
  }

  private async poll(): Promise<void> {
    try {
      const prices = await fetchPriceHistory(this.config.tokenPair);
      if (prices.length < 20) {
        console.log("[Node] Not enough price history yet.");
        return;
      }

      const result: AnalysisResult | null = await this.analyzer(
        this.config.tokenPair,
        prices
      );

      if (!result) {
        console.log(`[Node] ${new Date().toISOString()} — no signal this cycle.`);
        return;
      }

      await this.publishResult(result);
    } catch (err) {
      console.error("[Node] Poll error:", err);
    }
  }

  private async publishResult(result: AnalysisResult): Promise<void> {
    const now        = Math.floor(Date.now() / 1000);
    const expiryTime = now + result.expiryHours * 3600;

    const payload: SignalPayload = {
      token_pair:  this.config.tokenPair,
      exchange:    this.config.exchange,
      direction:   result.direction,
      entry_price: result.entryPrice,
      take_profit: result.takeProfit,
      stop_loss:   result.stopLoss,
      expiry_time: expiryTime,
      weight_pct:  result.confidence,
    };

    console.log(
      `[Node] Publishing ${result.direction} signal | ` +
      `entry=${result.entryPrice.toFixed(4)} TP=${result.takeProfit.toFixed(4)} ` +
      `SL=${result.stopLoss.toFixed(4)} conf=${result.confidence}%`
    );

    const envelope = await signEnvelope(this.wallet, payload, now);
    const rootHash = await publishSignal(
      this.wallet,
      this.config.zgIndexerUrl,
      this.config.registryAddress,
      envelope
    );

    console.log(`[Node] Signal published. Root hash: ${rootHash}`);
  }
}

// ── Entry point ───────────────────────────────────────────────────────────────

async function main(): Promise<void> {
  const config = loadConfig();
  const node   = new SignalNode(config);
  await node.start();
}

main().catch((err) => {
  console.error("[Node] Fatal error:", err);
  process.exit(1);
});
