#!/usr/bin/env node
/**
 * Test swap on GMX testnet: ETH -> USDC.SG
 */

import dotenv from "dotenv";
import { GmxApiSdk, PrivateKeySigner } from "@gmx-io/sdk/v2";
import type { PrepareOrderRequest } from "@gmx-io/sdk/v2";

dotenv.config();

const CHAIN_ID = 421614;
const RPC_URL = "https://sepolia-rollup.arbitrum.io/rpc";

async function testSwap() {
  console.log("🔄 GMX Testnet Swap Test: ETH → USDC.SG\n");

  const privateKey = process.env.PRIVATE_KEY;
  if (!privateKey) throw new Error("PRIVATE_KEY not set");

  // Initialize
  console.log("1️⃣ Initializing...");
  const sdk = new GmxApiSdk({ chainId: CHAIN_ID });
  const signer = new PrivateKeySigner(privateKey as `0x${string}`, { rpcUrl: RPC_URL });
  console.log(`   Wallet: ${signer.address}`);

  // Check available tokens
  console.log("\n2️⃣ Available tokens for swap:");
  try {
    const tokens = await sdk.fetchTokens();
    for (const t of tokens) {
      console.log(`   - ${t.symbol} (${t.address})`);
    }
  } catch (err: any) {
    console.log(`   Error: ${err.message}`);
  }

  // Swap ETH to USDC.SG
  const swapAmount = BigInt(10) * BigInt(10 ** 15); // 0.01 ETH

  console.log("\n3️⃣ Executing swap...");
  console.log(`   From: 0.01 ETH`);
  console.log(`   To: USDC.SG`);

  const swapRequest: PrepareOrderRequest = {
    kind: "swap",
    orderType: "market",
    collateralToPay: {
      amount: swapAmount,
      token: "ETH"
    },
    receiveToken: "USDC.SG",
    mode: "classic",
    from: signer.address,
  };

  try {
    console.log("\n   Preparing swap order...");
    const prepared = await sdk.prepareOrder(swapRequest);
    console.log(`   Payload type: ${prepared.payloadType}`);
    console.log(`   Request ID: ${prepared.requestId}`);

    if (prepared.estimates) {
      console.log(`   Estimated output: ${prepared.estimates}`);
    }

    if (prepared.payloadType === "transaction") {
      console.log("\n   Sending transaction...");
      const tx = prepared.payload as { to: string; data: string; value?: bigint };
      const txHash = await signer.sendTransaction({
        to: tx.to,
        data: tx.data,
        value: tx.value
      });

      const hash = typeof txHash === "string" ? txHash : txHash.hash;
      console.log(`\n   ✅ Swap transaction sent!`);
      console.log(`   TX Hash: ${hash}`);
      console.log(`   Explorer: https://sepolia.arbiscan.io/tx/${hash}`);
    } else {
      console.log("\n   Signing order...");
      const signature = await sdk.signOrder(prepared, signer);
      const result = await sdk.submitOrder({
        mode: prepared.mode,
        requestId: prepared.requestId,
        signature
      });
      console.log(`\n   ✅ Swap submitted! Status: ${result.status}`);
      if (result.txHash) {
        console.log(`   TX: https://sepolia.arbiscan.io/tx/${result.txHash}`);
      }
    }

  } catch (err: any) {
    console.error("\n   ❌ Swap failed:", err.message);
  }

  console.log("\n🏁 Done!");
}

testSwap().catch(console.error);
