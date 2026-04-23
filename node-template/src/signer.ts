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
  const orderedPayload = {
    token_pair:  payload.token_pair,
    exchange:    payload.exchange,
    direction:   payload.direction,
    entry_price: payload.entry_price,
    take_profit: payload.take_profit,
    stop_loss:   payload.stop_loss,
    expiry_time: payload.expiry_time,
    weight_pct:  payload.weight_pct,
  };

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
