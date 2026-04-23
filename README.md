# 0G Verification Layer & Signal Marketplace

This repository contains the full stack for the Decentralized AI Signal Marketplace built on the **0G Network**. It includes the high-frequency Go Verification Engine, the L2 Batching Architecture, and the Solidity Smart Contracts.

## 🚀 0G Galileo Testnet Deployment (April 2026)

### Testnet Wallet
This wallet is used by the Go Backend (Verifier Identity) and was used to deploy the Smart Contracts. It contains testnet `A0GI` for gas.

*   **Address:** `0x3E99444912Ff7549A1581Baf0b0C8EB1e930729D`
*   **Private Key:** `f6c5489042890316e9e73d79c41c4c79d1441469ae30203f63b7ea3c449baa37`

### Smart Contract Addresses (0G Galileo Testnet - ChainID: 16602)
*   **NodeRegistry:** `0xc9297E1a79F28f35BfbA335671cd655C4D104125`
*   **ReputationOracle:** `0xB63CeDc10F0Fe0475171e50493Fde8cD015f60C2`
*   **FeeRouter:** `0xA90f5392BCA1E0a7A261D91f31c0acB3a69fEe96`
*   **SubscriptionManager:** `0x61C10990B28990C09D895860C0Ab70A49042Ad92`

---

## 🛠 Project Architecture

The platform is designed to solve two massive problems in Web3/AI trading:
1.  **Cost of Data:** Decentralized Storage is cheap, but *uploading* to it costs gas. We built an **L2 Batcher** that allows AI Agents to push signals to our API for free. We bundle them and upload them to 0G Storage once a day, absorbing the cost.
2.  **Cost of Compute:** AI Agents waste millions of LLM tokens constantly polling APIs. We built a **Webhook Dispatcher** that allows AI Agents to sleep. The millisecond a signal hits its target, we push an HTTP POST to wake them up.

### 1. The Verification Layer (Go Backend)
Located in `/verification-layer`. This is a high-frequency, concurrent engine.
*   **Ingestion:** Listens for AES-encrypted signals on `/api/v1/signals`.
*   **Resolution Engine:** Streams live Binance `!miniTicker@arr` WebSockets and matches incoming prices against the signals in memory.
*   **Scorer:** Uses a time-decayed Risk-Adjusted Return (Sharpe Ratio) formula to assign `BRONZE` through `DIAMOND` tiers to Node Creators based on their historical accuracy.
*   **L2 Batcher & Broadcaster:** Writes the daily 0G Storage Root Hashes and the Reputation Scores directly to the EVM Smart Contracts.

### 2. The Smart Contracts (Solidity)
Located in `/contracts`.
*   **NodeRegistry.sol:** Tracks active AI signal nodes. The Verification Layer anchors the daily 0G Storage Batch proofs here.
*   **ReputationOracle.sol:** The single source of truth for a Node's Trust Score (0-100). Only the Verification Layer can write to this.
*   **SubscriptionManager.sol:** Allows users to subscribe to Nodes using ERC20 tokens (e.g., USDC). Prices dynamically scale based on the Oracle tier.
*   **FeeRouter.sol:** Splits the subscription revenue between the Platform Treasury and the Node Creator. Higher reputation = higher creator split (up to 85% for DIAMOND).

## 💻 How to Run

### 1. Run the Verification Layer (Backend)
Ensure you have Go 1.25+ installed.
```bash
cd verification-layer
go run ./cmd/verifier
```

### 2. Simulate an AI Node Creator
This script simulates an AI Agent generating a cryptographic identity, registering, and pushing an encrypted signal to the L2 API.
```bash
cd verification-layer
go run ./cmd/test_api
```

### 3. Simulate the 0G Storage Engine
This script tests the direct 0G Storage upload and download logic.
```bash
cd verification-layer
go run ./cmd/test_storage
```
