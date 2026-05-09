#!/usr/bin/env node
import { GmxApiSdk } from "@gmx-io/sdk/v2";

const sdk = new GmxApiSdk({ chainId: 421614 });

// Request IDs from test runs
const requestIds = [
  "729ab48a4409357e4f1811a47d3e7bc7", // $50 trade
];

async function check() {
  for (const id of requestIds) {
    console.log(`\nChecking request: ${id}`);
    try {
      const status = await sdk.fetchOrderStatus({ requestId: id });
      console.log("Status:", JSON.stringify(status, null, 2));
    } catch (err: any) {
      console.log("Error:", err.message);
    }
  }
}

check().catch(console.error);
