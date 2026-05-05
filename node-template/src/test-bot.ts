/**
 * test-bot.ts — A simple bot to send test signals to the Verifier API
 *
 * This bot generates random trading signals and submits them to the
 * Verification Layer's REST API for scheduled analysis testing.
 *
 * Usage:
 *   1. Set VERIFIER_API_URL and NODE_PRIVATE_KEY in .env
 *   2. npm run test-bot
 *
 * The bot will:
 *   - Generate a realistic trading signal (LONG or SHORT)
 *   - Sign it with the node's private key
 *   - Submit to POST /api/v1/signals
 *   - The verifier will schedule analysis at expiry time
 */

import { ethers } from "ethers";
import * as dotenv from "dotenv";
import axios from "axios";
import * as readline from "readline";

import { SignalPayload, SignalEnvelope, TradeType, Direction, TakeProfitLevel } from "./types";
import { signEnvelope } from "./signer";

dotenv.config();

// ── Trade type constants ───────────────────────────────────────────────────────

const TRADE_TYPES: TradeType[] = ["spot", "perpetual", "futures"];
const DIRECTIONS: Direction[] = ["long", "short"];

// Max expiry hours based on trade type (matching Go validation)
const MAX_EXPIRY_HOURS: Record<TradeType, number> = {
  spot: 168,       // 7 days
  perpetual: 48,   // 48 hours
  futures: 48,     // 48 hours
};

// Common leverage values for perpetuals
const LEVERAGE_OPTIONS = [1, 2, 3, 5, 10, 20, 25, 50, 75, 100, 125];

// ── Configuration ─────────────────────────────────────────────────────────────

interface TestBotConfig {
  privateKeyHex: string;
  verifierApiUrl: string;
  tokenPair: string;
  exchange: string;
}

function loadConfig(): TestBotConfig {
  const pk = process.env.NODE_PRIVATE_KEY;
  if (!pk) throw new Error("NODE_PRIVATE_KEY not set — run: npm run keygen");

  return {
    privateKeyHex: pk,
    verifierApiUrl: process.env.VERIFIER_API_URL || "http://localhost:8080",
    tokenPair: process.env.TOKEN_PAIR || "BTC/USDT",
    exchange: process.env.EXCHANGE || "binance",
  };
}

// ── Price fetching ────────────────────────────────────────────────────────────

async function fetchCurrentPrice(tokenPair: string): Promise<number> {
  const coinGeckoIds: Record<string, string> = {
    "BTC/USDT": "bitcoin",
    "ETH/USDT": "ethereum",
    "SOL/USDT": "solana",
    "BNB/USDT": "binancecoin",
    "XRP/USDT": "ripple",
    "ADA/USDT": "cardano",
    "DOGE/USDT": "dogecoin",
    "LINK/USDT": "chainlink",
    "AVAX/USDT": "avalanche-2",
    "DOT/USDT": "polkadot",
  };

  // Try CoinGecko first (more reliable, no API key needed for basic use)
  const coinId = coinGeckoIds[tokenPair];
  if (coinId) {
    try {
      const url = `https://api.coingecko.com/api/v3/simple/price?ids=${coinId}&vs_currencies=usd`;
      const res = await axios.get(url, { timeout: 10000 });
      if (res.data[coinId]?.usd) {
        return res.data[coinId].usd;
      }
    } catch (err: any) {
      console.log(`  CoinGecko failed: ${err.message}, trying Binance...`);
    }
  }

  // Fallback to Binance
  try {
    const symbol = tokenPair.replace("/", "");
    const url = `https://api.binance.com/api/v3/ticker/price?symbol=${symbol}`;
    const res = await axios.get(url, { timeout: 5000 });
    return parseFloat(res.data.price);
  } catch (err: any) {
    throw new Error(`Failed to fetch price for ${tokenPair}: ${err.message}`);
  }
}

// ── Signal generation ─────────────────────────────────────────────────────────

function randomChoice<T>(arr: T[]): T {
  return arr[Math.floor(Math.random() * arr.length)];
}

function generateSignal(
  tokenPair: string,
  exchange: string,
  currentPrice: number,
  expiryMinutes: number = 2
): { payload: SignalPayload; description: string } {
  // Randomly choose direction and trade type
  const direction: Direction = randomChoice(DIRECTIONS);
  const tradeType: TradeType = randomChoice(TRADE_TYPES);

  // Leverage: 1x for spot, random for perpetual/futures
  let leverage = 1;
  if (tradeType !== "spot") {
    leverage = randomChoice(LEVERAGE_OPTIONS.filter(l => l <= 50)); // Cap at 50x for safety
  }

  // Validate expiry doesn't exceed max for trade type
  const maxExpiryMinutes = MAX_EXPIRY_HOURS[tradeType] * 60;
  const actualExpiryMinutes = Math.min(expiryMinutes, maxExpiryMinutes);

  // Calculate risk/reward levels (realistic trading parameters)
  // Higher leverage = tighter stops
  const baseRiskPercent = 1 + Math.random() * 2; // 1-3% base risk
  const riskPercent = leverage > 10 ? baseRiskPercent / 2 : baseRiskPercent; // Tighter for high leverage
  const rewardMultiplier = 1.5 + Math.random() * 1.5; // 1.5-3x reward
  const rewardPercent = riskPercent * rewardMultiplier;

  let entryPrice: number;
  let takeProfit: number;
  let stopLoss: number;

  if (direction === "long") {
    // Entry slightly above current (simulating a breakout entry)
    entryPrice = currentPrice * (1 + (Math.random() * 0.001)); // 0-0.1% above
    takeProfit = entryPrice * (1 + rewardPercent / 100);
    stopLoss = entryPrice * (1 - riskPercent / 100);
  } else {
    // short
    entryPrice = currentPrice * (1 - (Math.random() * 0.001)); // 0-0.1% below
    takeProfit = entryPrice * (1 - rewardPercent / 100);
    stopLoss = entryPrice * (1 + riskPercent / 100);
  }

  const confidence = 50 + Math.floor(Math.random() * 50); // 50-99%
  const expiryTime = Math.floor(Date.now() / 1000) + (actualExpiryMinutes * 60);

  // Build payload with new v2 fields
  const payload: SignalPayload = {
    token_pair: tokenPair,
    exchange: exchange,
    direction: direction,
    entry_price: parseFloat(entryPrice.toFixed(2)),
    take_profit: parseFloat(takeProfit.toFixed(2)),
    stop_loss: parseFloat(stopLoss.toFixed(2)),
    expiry_time: expiryTime,
    weight_pct: confidence,
    trade_type: tradeType,
    leverage: leverage,
  };

  // Optionally add multiple take profit levels (20% chance)
  if (Math.random() < 0.2) {
    const tp1 = entryPrice + (takeProfit - entryPrice) * 0.5;
    const tp2 = takeProfit;
    payload.take_profits = [
      { price: parseFloat(tp1.toFixed(2)), percent: 50 },
      { price: parseFloat(tp2.toFixed(2)), percent: 50 },
    ];
  }

  // Build description
  const leverageStr = leverage > 1 ? ` ${leverage}x` : "";
  const typeStr = tradeType === "spot" ? "" : ` [${tradeType.toUpperCase()}${leverageStr}]`;

  const description =
    `${direction.toUpperCase()} ${tokenPair}${typeStr} @ $${entryPrice.toFixed(2)} | ` +
    `TP: $${takeProfit.toFixed(2)} (${rewardPercent.toFixed(1)}%) | ` +
    `SL: $${stopLoss.toFixed(2)} (${riskPercent.toFixed(1)}%) | ` +
    `R:R ${rewardMultiplier.toFixed(1)}:1 | ` +
    `Confidence: ${confidence}% | ` +
    `Expires in ${actualExpiryMinutes} min`;

  return { payload, description };
}

// ── API submission ────────────────────────────────────────────────────────────

interface SubmitResponse {
  status: string;
  message: string;
}

async function submitSignal(
  apiUrl: string,
  nodeId: string,
  envelope: SignalEnvelope
): Promise<SubmitResponse> {
  const url = `${apiUrl}/api/v1/signals`;

  const body = {
    node_id: nodeId,
    envelope: envelope,
  };

  const res = await axios.post<SubmitResponse>(url, body, {
    headers: { "Content-Type": "application/json" },
    timeout: 10000,
  });

  return res.data;
}

// ── Interactive CLI ───────────────────────────────────────────────────────────

async function prompt(question: string): Promise<string> {
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  return new Promise((resolve) => {
    rl.question(question, (answer) => {
      rl.close();
      resolve(answer.trim());
    });
  });
}

// ── Main ──────────────────────────────────────────────────────────────────────

async function main(): Promise<void> {
  console.log("==================================================");
  console.log("🤖 Signal Test Bot - Sends Signals to Verifier API");
  console.log("==================================================\n");

  const config = loadConfig();
  const wallet = new ethers.Wallet(config.privateKeyHex);
  const nodeId = wallet.address;

  console.log(`Node ID (Address): ${nodeId}`);
  console.log(`Verifier API: ${config.verifierApiUrl}`);
  console.log(`Token Pair: ${config.tokenPair}`);
  console.log("");

  // Test API connectivity
  try {
    const healthUrl = `${config.verifierApiUrl}/healthz`;
    await axios.get(healthUrl, { timeout: 5000 });
    console.log("✅ Verifier API is reachable\n");
  } catch (err) {
    console.error("❌ Cannot reach Verifier API. Is it running?");
    console.error(`   Tried: ${config.verifierApiUrl}/healthz`);
    process.exit(1);
  }

  // Interactive loop
  while (true) {
    console.log("\n─────────────────────────────────────");
    console.log("Options:");
    console.log("  [1] Send a random signal (expires in 2 min)");
    console.log("  [2] Send a random signal (expires in 5 min)");
    console.log("  [3] Send a random signal (expires in 10 min)");
    console.log("  [4] Send multiple random signals");
    console.log("  [5] Check node stats");
    console.log("  [q] Quit");
    console.log("─────────────────────────────────────");

    const choice = await prompt("\nEnter choice: ");

    if (choice === "q" || choice === "quit") {
      console.log("\nGoodbye! 👋");
      break;
    }

    try {
      if (choice === "1" || choice === "2" || choice === "3") {
        const expiryMinutes = choice === "1" ? 2 : choice === "2" ? 5 : 10;
        await sendSingleSignal(config, wallet, nodeId, expiryMinutes);
      } else if (choice === "4") {
        const countStr = await prompt("How many signals to send? ");
        const count = parseInt(countStr, 10);
        if (isNaN(count) || count < 1) {
          console.log("Invalid number");
          continue;
        }
        await sendMultipleSignals(config, wallet, nodeId, count);
      } else if (choice === "5") {
        await checkNodeStats(config.verifierApiUrl, nodeId);
      } else {
        console.log("Invalid choice");
      }
    } catch (err: any) {
      console.error(`\n❌ Error: ${err.message}`);
    }
  }
}

async function sendSingleSignal(
  config: TestBotConfig,
  wallet: ethers.Wallet,
  nodeId: string,
  expiryMinutes: number
): Promise<void> {
  console.log(`\nFetching current price for ${config.tokenPair}...`);
  const currentPrice = await fetchCurrentPrice(config.tokenPair);
  console.log(`Current price: $${currentPrice.toFixed(2)}`);

  console.log(`\nGenerating signal (expires in ${expiryMinutes} min)...`);
  const { payload, description } = generateSignal(
    config.tokenPair,
    config.exchange,
    currentPrice,
    expiryMinutes
  );

  console.log(`\n📊 Signal: ${description}`);

  const timestamp = Math.floor(Date.now() / 1000);
  const envelope = await signEnvelope(wallet, payload, timestamp);

  console.log(`\nSubmitting to ${config.verifierApiUrl}...`);
  const response = await submitSignal(config.verifierApiUrl, nodeId, envelope);

  console.log(`\n✅ ${response.status}: ${response.message}`);
  console.log(`   Signal will be analyzed at expiry: ${new Date(payload.expiry_time * 1000).toLocaleTimeString()}`);
}

async function sendMultipleSignals(
  config: TestBotConfig,
  wallet: ethers.Wallet,
  nodeId: string,
  count: number
): Promise<void> {
  console.log(`\nFetching current price for ${config.tokenPair}...`);
  const currentPrice = await fetchCurrentPrice(config.tokenPair);
  console.log(`Current price: $${currentPrice.toFixed(2)}`);

  console.log(`\nSending ${count} signals with varying expiry times...`);

  let successes = 0;
  let failures = 0;

  for (let i = 0; i < count; i++) {
    // Random expiry between 1-10 minutes
    const expiryMinutes = 1 + Math.floor(Math.random() * 10);
    const { payload, description } = generateSignal(
      config.tokenPair,
      config.exchange,
      currentPrice,
      expiryMinutes
    );

    const timestamp = Math.floor(Date.now() / 1000) + i; // Unique timestamps

    try {
      const envelope = await signEnvelope(wallet, payload, timestamp);
      await submitSignal(config.verifierApiUrl, nodeId, envelope);
      console.log(`  [${i + 1}/${count}] ✅ ${payload.direction} signal (expires in ${expiryMinutes}min)`);
      successes++;
    } catch (err: any) {
      console.log(`  [${i + 1}/${count}] ❌ Failed: ${err.message}`);
      failures++;
    }

    // Small delay to avoid overwhelming the API
    await new Promise(r => setTimeout(r, 200));
  }

  console.log(`\nDone! Successes: ${successes}, Failures: ${failures}`);
}

async function checkNodeStats(apiUrl: string, nodeId: string): Promise<void> {
  const url = `${apiUrl}/api/v1/nodes/${nodeId}`;

  try {
    const res = await axios.get(url, { timeout: 5000 });
    const stats = res.data;

    console.log("\n📈 Node Statistics:");
    console.log(`   Node ID:       ${stats.node_id || nodeId}`);
    console.log(`   Trust Score:   ${stats.trust_score?.toFixed(1) || 0}/100`);
    console.log(`   Tier:          ${stats.tier || "BRONZE"}`);
    console.log(`   Total Signals: ${stats.total_signals || 0}`);
    console.log(`   Win Rate:      ${((stats.win_rate || 0) * 100).toFixed(1)}%`);
    console.log(`   Sharpe Ratio:  ${stats.sharpe_ratio?.toFixed(2) || 0}`);
    console.log(`   Avg EV:        ${stats.avg_ev?.toFixed(2) || 0}`);
    console.log(`   Wins:          ${stats.win_count || 0}`);
    console.log(`   Losses:        ${stats.loss_count || 0}`);
    console.log(`   Expired:       ${stats.expired_count || 0}`);
  } catch (err: any) {
    if (err.response?.status === 404) {
      console.log("\n📈 Node Statistics: No data yet (send some signals first!)");
    } else {
      throw err;
    }
  }
}

main().catch((err) => {
  console.error("[TestBot] Fatal error:", err);
  process.exit(1);
});
