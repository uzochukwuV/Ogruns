#!/usr/bin/env node
/**
 * Test script for GMX testnet trade execution
 * Run with: npx tsx src/test-trade.ts
 */

import dotenv from "dotenv";
import { GmxApiSdk, PrivateKeySigner } from "@gmx-io/sdk/v2";
import type { PrepareOrderRequest } from "@gmx-io/sdk/v2";

dotenv.config();

const CHAIN_ID = 421614; // Arbitrum Sepolia
const RPC_URL = "https://sepolia-rollup.arbitrum.io/rpc";

async function testTrade() {
  console.log("🧪 GMX Testnet Trade Test\n");

  // Get private key from env
  const privateKey = process.env.PRIVATE_KEY;
  if (!privateKey) {
    throw new Error("PRIVATE_KEY not set in .env");
  }

  // Initialize SDK and signer
  console.log("1️⃣ Initializing GMX SDK...");
  const sdk = new GmxApiSdk({ chainId: CHAIN_ID });
  const signer = new PrivateKeySigner(privateKey as `0x${string}`, { rpcUrl: RPC_URL });

  console.log(`   Wallet: ${signer.address}`);
  console.log(`   Chain: Arbitrum Sepolia (${CHAIN_ID})`);

  // Check wallet balances
  console.log("\n2️⃣ Checking wallet balances...");
  try {
    const balances = await sdk.fetchWalletBalances({ address: signer.address });
    console.log("   Balances:");
    for (const bal of balances) {
      if (Number(bal.balance) > 0) {
        console.log(`   - ${bal.symbol}: ${Number(bal.balance) / 10 ** bal.decimals}`);
      }
    }
  } catch (err: any) {
    console.log(`   ⚠️ Could not fetch balances: ${err.message}`);
  }

  // Check current positions
  console.log("\n3️⃣ Checking current positions...");
  try {
    const positions = await sdk.fetchPositionsInfo({ address: signer.address });
    console.log(`   Open positions: ${positions.length}`);
    for (const pos of positions) {
      console.log(`   - ${pos.indexName} ${pos.isLong ? "LONG" : "SHORT"}`);
    }
  } catch (err: any) {
    console.log(`   ⚠️ Could not fetch positions: ${err.message}`);
  }

  // Fetch available markets
  console.log("\n4️⃣ Fetching available markets...");
  let marketSymbol = "ETH/USD [WETH-USDC]"; // default
  try {
    const markets = await sdk.fetchMarkets();
    console.log(`   Available markets: ${markets.length}`);
    for (const m of markets) {
      console.log(`   - ${m.symbol}`);
    }
    // Find ETH or BTC market
    const ethMarket = markets.find(m => m.symbol?.startsWith("ETH/"));
    const btcMarket = markets.find(m => m.symbol?.startsWith("BTC/"));
    if (ethMarket?.symbol) {
      marketSymbol = ethMarket.symbol;
    } else if (btcMarket?.symbol) {
      marketSymbol = btcMarket.symbol;
    }
  } catch (err: any) {
    console.log(`   ⚠️ Could not fetch markets: ${err.message}`);
  }

  // Use USDC.SG as collateral (we have ~1.14 from swap)
  const collateralToken = "USDC.SG";
  const collateralAmount = BigInt(1) * BigInt(10 ** 6); // 1 USDC.SG (6 decimals)

  // Check and set approval for USDC.SG
  console.log("\n5️⃣ Checking token approval...");
  try {
    const allowances = await sdk.fetchAllowances({
      address: signer.address,
      spender: "router"
    });
    const usdcAllowance = allowances.find(a => a.symbol === "USDC.SG");
    console.log(`   Current USDC.SG allowance: ${usdcAllowance ? Number(usdcAllowance.allowance) / 1e6 : 0}`);

    if (!usdcAllowance || BigInt(usdcAllowance.allowance) < collateralAmount) {
      console.log("   Approving USDC.SG for router...");
      const approvalTx = sdk.buildApproveTransaction({
        tokenSymbol: "USDC.SG",
        spender: "router",
        amount: BigInt(10) * BigInt(10 ** 6) // Approve 10 USDC.SG
      });

      const txHash = await signer.sendTransaction({
        to: approvalTx.to,
        data: approvalTx.data
      });
      const hash = typeof txHash === "string" ? txHash : txHash.hash;
      console.log(`   ✅ Approval TX: ${hash}`);

      // Wait a bit for approval to be confirmed
      console.log("   Waiting for approval confirmation...");
      await new Promise(r => setTimeout(r, 5000));
    } else {
      console.log("   ✅ Already approved");
    }
  } catch (err: any) {
    console.log(`   ⚠️ Approval check failed: ${err.message}`);
  }

  // Execute test trade - minimum is ~$10 on GMX, use 10x leverage
  console.log("\n6️⃣ Executing test trade...");
  console.log(`   Symbol: ${marketSymbol}`);
  console.log("   Direction: LONG");
  console.log("   Size: $10 USD (10x leverage)");
  console.log(`   Collateral: 1 ${collateralToken}`);

  const orderRequest: PrepareOrderRequest = {
    kind: "increase",
    symbol: marketSymbol,
    direction: "long",
    orderType: "market",
    size: BigInt(10) * BigInt(10 ** 30), // $10 USD (30 decimals) - 10x leverage
    collateralToken: collateralToken,
    collateralToPay: {
      amount: collateralAmount,
      token: collateralToken
    },
    mode: "classic",
    from: signer.address,
  };

  try {
    console.log("\n   Preparing order...");
    const prepared = await sdk.prepareOrder(orderRequest);
    console.log(`   Payload type: ${prepared.payloadType}`);
    console.log(`   Request ID: ${prepared.requestId}`);

    if (prepared.payloadType === "transaction") {
      // Classic mode - send transaction directly
      console.log("\n   Sending transaction...");
      const tx = prepared.payload as { to: string; data: string; value?: bigint };
      const txHash = await signer.sendTransaction({
        to: tx.to,
        data: tx.data,
        value: tx.value
      });

      const hash = typeof txHash === "string" ? txHash : txHash.hash;
      console.log(`\n   ✅ Transaction sent!`);
      console.log(`   TX Hash: ${hash}`);
      console.log(`   Explorer: https://sepolia.arbiscan.io/tx/${hash}`);
    } else {
      // Express mode - sign and submit
      console.log("\n   Signing order...");
      const signature = await sdk.signOrder(prepared, signer);

      console.log("   Submitting order...");
      const result = await sdk.submitOrder({
        mode: "express",
        requestId: prepared.requestId,
        signature
      });

      console.log(`\n   ✅ Order submitted!`);
      console.log(`   Status: ${result.status}`);
      if (result.txHash) {
        console.log(`   TX Hash: ${result.txHash}`);
        console.log(`   Explorer: https://sepolia.arbiscan.io/tx/${result.txHash}`);
      }
    }

  } catch (err: any) {
    console.error("\n   ❌ Trade failed:", err.message);
    if (err.response) {
      console.error("   Response:", JSON.stringify(err.response, null, 2));
    }
  }

  console.log("\n🏁 Test complete!");
}

testTrade().catch(console.error);
