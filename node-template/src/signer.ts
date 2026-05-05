/**
 * signer.ts — deterministic ECDSA signing of signal envelopes.
 *
 * The message format is identical to what the Go VerifySignal function expects:
 *   message = `${timestamp}:${JSON.stringify(payload)}`
 *   prefixedHash = keccak256("\x19Ethereum Signed Message:\n" + len(message) + message)
 *
 * ethers v6 Wallet.signMessage() produces exactly this format (EIP-191 personal_sign).
 *
 * CRITICAL: JSON.stringify must produce the same byte sequence as Go's
 * encoding/json.Marshal. To guarantee this:
 *  - Use a fixed key ordering (TypeScript object literal insertion order is stable).
 *  - Never add extra whitespace.
 *  - Ensure the SignalPayload interface fields are declared in the SAME order as
 *    the Go struct tags (see pkg/types/signal.go).
 */

import { ethers } from "ethers";
import { SignalPayload, SignalEnvelope } from "./types";

/**
 * Sign a payload and wrap it in a SignalEnvelope ready for upload.
 *
 * @param wallet    ethers Wallet initialised with the node's private key.
 * @param payload   The trading signal parameters.
 * @param timestamp Unix timestamp in seconds (use Math.floor(Date.now()/1000)).
 */
export async function signEnvelope(
  wallet: ethers.Wallet,
  payload: SignalPayload,
  timestamp: number
): Promise<SignalEnvelope> {
  // Serialise with the same field order as Go's json.Marshal (struct tag order).
  // CRITICAL: Fields with `omitempty` in Go must only be included if they have values.
  // Go excludes: empty strings, 0 numbers, nil/empty slices.

  const orderedPayload: Record<string, unknown> = {
    token_pair: payload.token_pair,
  };

  // exchange: omitempty - only include if non-empty string
  if (payload.exchange) {
    orderedPayload.exchange = payload.exchange;
  }

  // Required fields (no omitempty)
  orderedPayload.direction = payload.direction;
  orderedPayload.entry_price = payload.entry_price;
  orderedPayload.take_profit = payload.take_profit;
  orderedPayload.stop_loss = payload.stop_loss;
  orderedPayload.expiry_time = payload.expiry_time;
  orderedPayload.weight_pct = payload.weight_pct;

  // trade_type: omitempty - only include if non-empty string
  if (payload.trade_type) {
    orderedPayload.trade_type = payload.trade_type;
  }

  // leverage: omitempty - only include if > 0
  if (payload.leverage && payload.leverage > 0) {
    orderedPayload.leverage = payload.leverage;
  }

  // take_profits: omitempty - only include if non-empty array
  if (payload.take_profits && payload.take_profits.length > 0) {
    orderedPayload.take_profits = payload.take_profits;
  }

  const payloadJSON = JSON.stringify(orderedPayload);
  const message = `${timestamp}:${payloadJSON}`;

  // ethers v6 signMessage applies EIP-191 prefix automatically.
  const signature = await wallet.signMessage(message);

  return {
    node_id:   wallet.address,
    timestamp,
    payload,
    signature,
  };
}
