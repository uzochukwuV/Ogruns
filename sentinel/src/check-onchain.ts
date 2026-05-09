#!/usr/bin/env node
import { createPublicClient, http, formatUnits, parseAbi } from "viem";
import { arbitrumSepolia } from "viem/chains";

const client = createPublicClient({
  chain: arbitrumSepolia,
  transport: http("https://sepolia-rollup.arbitrum.io/rpc")
});

const USDC_SG = "0x3253a335E7bFfB4790Aa4C25C4250d206E9b9773";
const wallet = "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4";

const abi = parseAbi(["function balanceOf(address) view returns (uint256)"]);

async function check() {
  const balance = await client.readContract({
    address: USDC_SG,
    abi,
    functionName: "balanceOf",
    args: [wallet]
  });

  console.log("USDC.SG on-chain balance:", formatUnits(balance, 6));
}

check().catch(console.error);
