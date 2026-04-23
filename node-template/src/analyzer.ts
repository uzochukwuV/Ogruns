/**
 * analyzer.ts — pluggable signal analysis layer.
 *
 * Export a custom Analyzer to replace the default RSI strategy.
 * The SignalNode calls your analyzer on every poll cycle.  Return null to skip
 * the cycle (no signal to publish), or an AnalysisResult to trigger publishing.
 *
 * Integration paths:
 *  A. Local indicators (default): RSI, MACD, Bollinger Bands — implemented here.
 *  B. 0G Inference: POST to a model endpoint served via 0G Compute Network.
 *     Set ZG_INFERENCE_ENDPOINT in your .env and use the zgInferenceAnalyzer.
 *  C. External ML API: call any REST endpoint from your custom analyzer.
 */

import axios from "axios";
import { AnalysisResult } from "./types";

export type Analyzer = (
  tokenPair: string,
  priceHistory: number[] // most-recent price is last element
) => Promise<AnalysisResult | null>;

// ── A. Built-in RSI Analyzer ─────────────────────────────────────────────────

const RSI_PERIOD   = 14;
const RSI_OVERSOLD = 30;
const RSI_OVERBOUGHT = 70;
// Risk:reward — take profit at 2× the distance of the stop loss
const RR_RATIO     = 2.0;
const ATR_MULT     = 1.5; // stop loss = ATR × multiplier below/above entry

/**
 * builtinRSIAnalyzer — a simple RSI + ATR strategy.
 *
 * Signals:
 *   LONG  when RSI crosses below oversold threshold (30).
 *   SHORT when RSI crosses above overbought threshold (70).
 *
 * Risk management:
 *   StopLoss  = entry − ATR(14) × 1.5
 *   TakeProfit = entry + (entry − stopLoss) × RR_RATIO   (for LONG)
 *   Confidence = scaled from RSI distance to threshold (0–100)
 */
export const builtinRSIAnalyzer: Analyzer = async (
  _tokenPair,
  priceHistory
) => {
  if (priceHistory.length < RSI_PERIOD + 1) return null;

  const rsi   = calcRSI(priceHistory, RSI_PERIOD);
  const atr   = calcATR(priceHistory, RSI_PERIOD);
  const entry = priceHistory[priceHistory.length - 1];

  if (rsi < RSI_OVERSOLD) {
    const stopLoss   = entry - atr * ATR_MULT;
    const takeProfit = entry + (entry - stopLoss) * RR_RATIO;
    const confidence = Math.min(100, ((RSI_OVERSOLD - rsi) / RSI_OVERSOLD) * 200);
    return {
      direction:   "LONG",
      entryPrice:  entry,
      takeProfit,
      stopLoss,
      expiryHours: 4,
      confidence:  Math.round(confidence),
    };
  }

  if (rsi > RSI_OVERBOUGHT) {
    const stopLoss   = entry + atr * ATR_MULT;
    const takeProfit = entry - (stopLoss - entry) * RR_RATIO;
    const confidence = Math.min(100, ((rsi - RSI_OVERBOUGHT) / (100 - RSI_OVERBOUGHT)) * 200);
    return {
      direction:   "SHORT",
      entryPrice:  entry,
      takeProfit,
      stopLoss,
      expiryHours: 4,
      confidence:  Math.round(confidence),
    };
  }

  return null; // No signal this cycle
};

// ── B. 0G Inference Analyzer ─────────────────────────────────────────────────

/**
 * Create an analyzer that calls a model hosted on the 0G Compute/Inference
 * network. The endpoint must accept { token_pair, price_history } and return
 * { direction, entry_price, take_profit, stop_loss, expiry_hours, confidence }
 * or null.
 *
 * @param endpointUrl   Full URL of the 0G Inference endpoint.
 * @param modelName     Model identifier (e.g. "signal-btc-v1").
 */
export function makeZgInferenceAnalyzer(
  endpointUrl: string,
  modelName: string
): Analyzer {
  return async (tokenPair, priceHistory) => {
    try {
      const res = await axios.post<AnalysisResult | null>(endpointUrl, {
        model:         modelName,
        token_pair:    tokenPair,
        price_history: priceHistory,
      }, { timeout: 10_000 });

      return res.data; // null means no signal
    } catch (err) {
      console.error("[Analyzer] 0G Inference call failed:", err);
      return null;
    }
  };
}

// ── Indicator helpers ─────────────────────────────────────────────────────────

function calcRSI(prices: number[], period: number): number {
  const changes = prices.slice(1).map((p, i) => p - prices[i]);
  const gains   = changes.map(c => c > 0 ? c : 0);
  const losses  = changes.map(c => c < 0 ? -c : 0);

  const avgGain = avg(gains.slice(-period));
  const avgLoss = avg(losses.slice(-period));

  if (avgLoss === 0) return 100;
  const rs = avgGain / avgLoss;
  return 100 - 100 / (1 + rs);
}

function calcATR(prices: number[], period: number): number {
  // Simplified ATR using high-low of a single close series.
  // In production, pass [high, low, close] separately.
  const trueRanges = prices.slice(1).map((p, i) => Math.abs(p - prices[i]));
  return avg(trueRanges.slice(-period));
}

function avg(arr: number[]): number {
  if (arr.length === 0) return 0;
  return arr.reduce((a, b) => a + b, 0) / arr.length;
}
