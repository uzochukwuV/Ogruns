/**
 * 0G Signal Intelligence Network - Contract Deployment Script
 *
 * Deployment order matters — each contract depends on the previous:
 *  1. AgentRegistry       (needs verifier + optional Agentic ID contract)
 *  2. ReputationOracle    (standalone, needs verifier address)
 *  3. FeeRouter           (needs AgentRegistry + platform treasury)
 *  4. SubscriptionManager (needs ReputationOracle + FeeRouter)
 *  5. Wire FeeRouter.setSubscriptionManager(SubscriptionManager)
 *  6. PointVault          (v2 off-chain points system, needs verifier)
 *
 * After deployment, copy the printed addresses to your .env:
 *   REGISTRY_CONTRACT_ADDR=...
 *   REPUTATION_CONTRACT_ADDR=...
 *   SUBSCRIPTION_CONTRACT_ADDR=...
 *   POINT_VAULT_ADDR=...
 */

import { ethers } from "hardhat";

async function main() {
  const [deployer] = await ethers.getSigners();
  console.log("Deploying with account:", deployer.address);
  console.log(
    "Balance:",
    ethers.formatEther(await ethers.provider.getBalance(deployer.address)),
    "A0GI"
  );

  // ── 1. AgentRegistry ──────────────────────────────────────────────────────
  const verifierAddress = process.env.VERIFIER_ADDRESS || deployer.address; // fallback to deployer for testnet
  const agenticIdContract = process.env.AGENTIC_ID_CONTRACT || ethers.ZeroAddress; // 0x0 if not using Agentic ID
  console.log("\n[1/5] Deploying AgentRegistry (verifier:", verifierAddress, ")...");
  const AgentRegistry = await ethers.getContractFactory("AgentRegistry");
  const registry = await AgentRegistry.deploy(verifierAddress, agenticIdContract);
  await registry.waitForDeployment();
  console.log("AgentRegistry deployed at:", await registry.getAddress());

  // ── 2. ReputationOracle ───────────────────────────────────────────────────
  console.log("\n[2/5] Deploying ReputationOracle (verifier:", verifierAddress, ")...");
  const ReputationOracle = await ethers.getContractFactory("ReputationOracle");
  const oracle = await ReputationOracle.deploy(verifierAddress);
  await oracle.waitForDeployment();
  console.log("ReputationOracle deployed at:", await oracle.getAddress());

  // ── 3. FeeRouter ─────────────────────────────────────────────────────────
  // Platform treasury = deployer for testnet; use a multisig on mainnet.
  const platformTreasury = process.env.PLATFORM_TREASURY || deployer.address;
  console.log("\n[3/5] Deploying FeeRouter (treasury:", platformTreasury, ")...");
  const FeeRouter = await ethers.getContractFactory("FeeRouter");
  const feeRouter = await FeeRouter.deploy(
    await registry.getAddress(),
    platformTreasury
  );
  await feeRouter.waitForDeployment();
  console.log("FeeRouter deployed at:", await feeRouter.getAddress());

  // ── 4. SubscriptionManager ────────────────────────────────────────────────
  console.log("\n[4/5] Deploying SubscriptionManager...");
  const SubscriptionManager = await ethers.getContractFactory("SubscriptionManager");
  const subManager = await SubscriptionManager.deploy(
    await oracle.getAddress(),
    await feeRouter.getAddress()
  );
  await subManager.waitForDeployment();
  console.log("SubscriptionManager deployed at:", await subManager.getAddress());

  // ── 5. Wire FeeRouter ─────────────────────────────────────────────────────
  console.log("\n[5/6] Wiring FeeRouter → SubscriptionManager...");
  const tx = await feeRouter.setSubscriptionManager(await subManager.getAddress());
  await tx.wait();
  console.log("FeeRouter.subscriptionManager set.");

  // ── 6. PointVault (v2 Off-Chain Points System) ───────────────────────────
  console.log("\n[6/6] Deploying PointVault (verifier:", verifierAddress, ")...");
  const PointVault = await ethers.getContractFactory("PointVault");
  const pointVault = await PointVault.deploy(verifierAddress);
  await pointVault.waitForDeployment();
  console.log("PointVault deployed at:", await pointVault.getAddress());

  // ── Summary ───────────────────────────────────────────────────────────────
  console.log("\n╔══════════════════════════════════════════════════════╗");
  console.log("║         Deployment Complete — Copy to .env           ║");
  console.log("╠══════════════════════════════════════════════════════╣");
  console.log("REGISTRY_CONTRACT_ADDR    =", await registry.getAddress());
  console.log("REPUTATION_CONTRACT_ADDR  =", await oracle.getAddress());
  console.log("SUBSCRIPTION_CONTRACT_ADDR=", await subManager.getAddress());
  console.log("FEE_ROUTER_ADDR           =", await feeRouter.getAddress());
  console.log("POINT_VAULT_ADDR          =", await pointVault.getAddress());
  console.log("╚══════════════════════════════════════════════════════╝");
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
