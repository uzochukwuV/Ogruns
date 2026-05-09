#!/usr/bin/env node
import dotenv from "dotenv";
import { createPublicClient, http, formatEther, formatUnits } from "viem";
import { arbitrumSepolia } from "viem/chains";
import { GmxApiSdk } from "@gmx-io/sdk/v2";

dotenv.config();

async function checkBalance() {
  const privateKey = process.env.PRIVATE_KEY;
  if (!privateKey) throw new Error("PRIVATE_KEY not set");

  // Derive address from private key
  const { privateKeyToAccount } = await import("viem/accounts");
  const account = privateKeyToAccount(privateKey as `0x${string}`);

  const client = createPublicClient({
    chain: arbitrumSepolia,
    transport: http("https://sepolia-rollup.arbitrum.io/rpc")
  });

  const balance = await client.getBalance({ address: account.address });

  console.log(`Wallet: ${account.address}`);
  console.log(`\nNative ETH Balance: ${formatEther(balance)} ETH`);

  // Check GMX testnet token balances
  console.log("\nGMX Testnet Token Balances:");
  const sdk = new GmxApiSdk({ chainId: 421614 });

  try {
    const tokens = await sdk.fetchTokens();
    const balances = await sdk.fetchWalletBalances({ address: account.address });

    for (const bal of balances) {
      const amount = Number(bal.balance) / 10 ** bal.decimals;
      console.log(`  ${bal.symbol}: ${amount.toFixed(4)}`);
    }

    if (balances.every(b => Number(b.balance) === 0)) {
      console.log("  (No tokens found)");
    }

    // Show faucet info
    console.log("\n📱 To get testnet tokens:");
    console.log("   1. Go to https://app.gmx.io/#/faucet");
    console.log("   2. Connect your wallet");
    console.log("   3. Request testnet USDC.SG and WETH");
    console.log("\n   Or use GMX testnet: https://app.gmxtest.io");
  } catch (err: any) {
    console.log(`  Error fetching balances: ${err.message}`);
  }
}

checkBalance().catch(console.error);
