#!/usr/bin/env node
/**
 * Approve USDC.SG for GMX router
 */

import dotenv from "dotenv";
import { createWalletClient, createPublicClient, http, parseAbi } from "viem";
import { privateKeyToAccount } from "viem/accounts";
import { arbitrumSepolia } from "viem/chains";

dotenv.config();

// USDC.SG token address on Arbitrum Sepolia
const USDC_SG_ADDRESS = "0x3253a335E7bFfB4790Aa4C25C4250d206E9b9773";
// GMX Exchange Router on Arbitrum Sepolia
const GMX_ROUTER_ADDRESS = "0xEd50B2A1eF0C35DAaF08Da6486971180237909c3";
// GMX Order Vault (where tokens are actually transferred to)
const GMX_ORDER_VAULT = "0x1b8ac606de71686fd2a1aedecb6e0efba28909a2";

const ERC20_ABI = parseAbi([
  "function approve(address spender, uint256 amount) returns (bool)",
  "function allowance(address owner, address spender) view returns (uint256)"
]);

async function approveToken() {
  console.log("🔐 Approving USDC.SG for GMX Router\n");

  const privateKey = process.env.PRIVATE_KEY as `0x${string}`;
  if (!privateKey) throw new Error("PRIVATE_KEY not set");

  const account = privateKeyToAccount(privateKey);

  const publicClient = createPublicClient({
    chain: arbitrumSepolia,
    transport: http("https://sepolia-rollup.arbitrum.io/rpc")
  });

  const walletClient = createWalletClient({
    account,
    chain: arbitrumSepolia,
    transport: http("https://sepolia-rollup.arbitrum.io/rpc")
  });

  console.log(`Wallet: ${account.address}`);
  console.log(`Token: USDC.SG (${USDC_SG_ADDRESS})`);

  const approveAmount = BigInt("115792089237316195423570985008687907853269984665640564039457584007913129639935"); // max uint256

  // Approve Router, Order Vault, and Synthetics Router (from SDK)
  const SDK_SYNTHETICS_ROUTER = "0x72f13a44c8ba16a678cad549f17bc9e06d2b8bd2";

  const spenders = [
    { name: "GMX Router", address: GMX_ROUTER_ADDRESS },
    { name: "GMX Order Vault", address: GMX_ORDER_VAULT },
    { name: "SDK Synthetics Router", address: SDK_SYNTHETICS_ROUTER }
  ];

  for (const spender of spenders) {
    console.log(`\n--- Approving ${spender.name} (${spender.address}) ---`);

    const currentAllowance = await publicClient.readContract({
      address: USDC_SG_ADDRESS,
      abi: ERC20_ABI,
      functionName: "allowance",
      args: [account.address, spender.address as `0x${string}`]
    });

    console.log(`Current allowance: ${Number(currentAllowance) > 1e30 ? "Unlimited" : Number(currentAllowance) / 1e6 + " USDC.SG"}`);

    if (Number(currentAllowance) > 1e30) {
      console.log("Already approved, skipping...");
      continue;
    }

    console.log("Sending approval transaction...");

    const hash = await walletClient.writeContract({
      address: USDC_SG_ADDRESS,
      abi: ERC20_ABI,
      functionName: "approve",
      args: [spender.address as `0x${string}`, approveAmount]
    });

    console.log(`TX Hash: ${hash}`);

    const receipt = await publicClient.waitForTransactionReceipt({ hash });
    console.log(`Status: ${receipt.status === "success" ? "✅ Confirmed" : "❌ Failed"}`);
  }

  console.log("\n✅ All approvals complete!");
}

approveToken().catch(console.error);
