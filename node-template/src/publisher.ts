/**
 * publisher.ts — uploads a SignalEnvelope to 0G Storage and announces
 *               the root hash to the NodeRegistry smart contract.
 *
 * Two-step process:
 *  1. Upload JSON blob to 0G Storage → receive Merkle root hash.
 *  2. Call NodeRegistry.publishSignal(rootHash) on-chain so the Verifier
 *     Layer can discover and ingest the signal.
 *
 * This keeps the bulk data off-chain (cheap + scalable) while the on-chain
 * event log provides ordered, tamper-proof discoverability.
 */

import { ethers } from "ethers";
import { Indexer, ZgFile, getFlowContract } from "@0glabs/0g-ts-sdk";
import { SignalEnvelope } from "./types";

// Minimal ABI fragment for NodeRegistry.publishSignal
const REGISTRY_ABI = [
  "function publishSignal(bytes32 rootHash) external",
  "function registerNode(string name, string tokenFocus, string description) external",
  "function nodes(address) external view returns (string, string, string, address, bool, uint256)",
];

/**
 * Upload a SignalEnvelope blob to 0G Storage and publish its root hash
 * to the NodeRegistry contract. Returns the hex root hash on success.
 */
export async function publishSignal(
  wallet: ethers.Wallet,
  zgIndexerUrl: string,
  registryAddress: string,
  envelope: SignalEnvelope
): Promise<string> {
  const content = Buffer.from(JSON.stringify(envelope), "utf-8");

  // ── Step 1: Upload to 0G Storage ─────────────────────────────────────────
  const [zgFile, fileErr] = await ZgFile.fromNodeBuffer(content);
  if (fileErr !== null) {
    throw new Error(`ZgFile creation failed: ${fileErr}`);
  }

  const [tree, treeErr] = await zgFile.merkleTree();
  if (treeErr !== null) {
    throw new Error(`Merkle tree failed: ${treeErr}`);
  }

  const rootHash: string = tree.rootHash();
  console.log(`[Publisher] 0G Storage root hash: ${rootHash}`);

  const indexer = new Indexer(zgIndexerUrl);
  const provider = wallet.provider as ethers.JsonRpcProvider;
  const rpcUrl = provider._getConnection().url;

  const [_txHash, uploadErr] = await indexer.upload(
    zgFile,
    rpcUrl,
    wallet.privateKey
  );
  if (uploadErr !== null) {
    throw new Error(`0G Storage upload failed: ${uploadErr}`);
  }

  console.log(`[Publisher] Blob uploaded successfully.`);

  // ── Step 2: Announce root hash on-chain ──────────────────────────────────
  const registry = new ethers.Contract(registryAddress, REGISTRY_ABI, wallet);

  // Convert hex string root hash to bytes32
  const rootHashBytes32 = rootHash.startsWith("0x")
    ? rootHash.padEnd(66, "0")
    : "0x" + rootHash.padEnd(64, "0");

  const tx = await registry.publishSignal(rootHashBytes32);
  await tx.wait();
  console.log(`[Publisher] On-chain publishSignal tx: ${tx.hash}`);

  return rootHash;
}

/**
 * Register this node in the NodeRegistry. Only needs to be called once.
 * Subsequent calls will revert with AlreadyRegistered().
 */
export async function registerNode(
  wallet: ethers.Wallet,
  registryAddress: string,
  name: string,
  tokenFocus: string,
  description: string
): Promise<void> {
  const registry = new ethers.Contract(registryAddress, REGISTRY_ABI, wallet);
  const tx = await registry.registerNode(name, tokenFocus, description);
  await tx.wait();
  console.log(`[Publisher] Node registered. tx: ${tx.hash}`);
}

/**
 * Check if this node is already registered on-chain.
 */
export async function isRegistered(
  wallet: ethers.Wallet,
  registryAddress: string
): Promise<boolean> {
  const registry = new ethers.Contract(registryAddress, REGISTRY_ABI, wallet);
  const [, , , , , registeredAt] = await registry.nodes(wallet.address);
  return BigInt(registeredAt) > 0n;
}
