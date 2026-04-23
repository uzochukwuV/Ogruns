/**
 * keygen.ts — generate a new node identity key pair.
 *
 * Run: npm run keygen
 *
 * Output:
 *   NODE_PRIVATE_KEY = 0x...   ← put this in .env (NEVER commit it)
 *   NODE_ADDRESS     = 0x...   ← register this address with NodeRegistry
 */

import { ethers } from "ethers";

const wallet = ethers.Wallet.createRandom();

console.log("╔══════════════════════════════════════════════════════╗");
console.log("║          Ogruns Node Key Generation                  ║");
console.log("╠══════════════════════════════════════════════════════╣");
console.log(`NODE_PRIVATE_KEY=${wallet.privateKey}`);
console.log(`NODE_ADDRESS=${wallet.address}`);
console.log("╠══════════════════════════════════════════════════════╣");
console.log("║ Add NODE_PRIVATE_KEY to your .env file.              ║");
console.log("║ Fund NODE_ADDRESS with A0GI for gas fees.            ║");
console.log("║ NEVER share or commit your private key.              ║");
console.log("╚══════════════════════════════════════════════════════╝");
