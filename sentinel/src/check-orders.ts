#!/usr/bin/env node
import { GmxApiSdk } from "@gmx-io/sdk/v2";

const sdk = new GmxApiSdk({ chainId: 421614 });
const addr = "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4";

async function check() {
  console.log("📋 Checking GMX Orders & Positions\n");

  console.log("1️⃣ Pending Orders:");
  const orders = await sdk.fetchOrders({ address: addr });
  console.log(`   Found: ${orders.length} orders`);
  for (const o of orders) {
    console.log(`   - Type: ${o.orderType}, Market: ${o.marketName}`);
  }

  console.log("\n2️⃣ Open Positions:");
  const positions = await sdk.fetchPositionsInfo({ address: addr });
  console.log(`   Found: ${positions.length} positions`);
  for (const p of positions) {
    console.log(`   - ${p.indexName} ${p.isLong ? "LONG" : "SHORT"}`);
    console.log(`     Size: $${(Number(p.sizeInUsd) / 1e30).toFixed(2)}`);
    console.log(`     PnL: $${(Number(p.pnlAfterFees) / 1e30).toFixed(2)}`);
  }

  console.log("\n3️⃣ Balances:");
  const balances = await sdk.fetchWalletBalances({ address: addr });
  for (const b of balances) {
    if (Number(b.balance) > 0) {
      console.log(`   - ${b.symbol}: ${(Number(b.balance) / 10 ** b.decimals).toFixed(4)}`);
    }
  }
}

check().catch(console.error);
