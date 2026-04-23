# Verification Layer Design Spec

## 1. System Overview
The Verification Layer is the core source of truth for the decentralized "YouTube for Trading Signals" platform built on the 0G Network. It ingests cryptographically signed trading signals from decentralized nodes, resolves their outcomes against high-frequency CEX price data, and calculates an immutable on-chain Reputation Score.

This layer ensures that node developers cannot falsify their performance and provides the Management Layer with verified signals to execute trades on behalf of users.

## 2. Core Components

### 2.1 0G Ingester
- **Purpose:** Listens to 0G Storage/DA for new signal payloads.
- **Node Identity Routing:** Extracts the unique `Node ID` and verifies the cryptographic signature of the payload. If the signature is invalid, the signal is dropped.
- **Data Structure:** Parses the signal parameters (Token Pair, Direction, Entry Price, Take Profit, Stop Loss, Expiry Time).

### 2.2 CEX Tick Aggregator (Price Oracle)
- **Purpose:** Acts as the official price judge, resolving the limitation of decentralized oracles regarding new/obscure tokens.
- **Implementation:** Connects to Binance, Bybit, and MEXC via WebSockets to ingest high-frequency tick data.
- **Output:** A normalized, internal, high-speed price feed used exclusively by the Resolution Engine.

### 2.3 Resolution Engine (State Machine)
- **Purpose:** An in-memory matching engine (e.g., Redis) that tracks the lifecycle of every verified signal against the CEX Tick Aggregator feed.
- **Signal States:**
  1. **PENDING:** Waiting for the actual price to hit the `Entry Price`.
  2. **ACTIVE:** Entry price hit. Management Layer notified for execution.
  3. **CLOSED (WIN):** Price hits `Take Profit`.
  4. **CLOSED (LOSS):** Price hits `Stop Loss`.
  5. **CLOSED (EXPIRED):** `Expiry Time` reached. PnL calculated based on the price at that exact second.
  6. **CANCELLED:** `Expiry Time` reached while in the PENDING state. (Does not affect score).

### 2.4 Reputation Calculator & Tier System
- **Purpose:** Evaluates closed signals to generate a Trust Score (0-100) based on Total Expected Value (EV), Consistency (Sharpe Ratio), and Recency Decay (older trades hold less weight).
- **Reputation-Pegged Pricing:** The Trust Score directly dictates the node's tier and subscription price:
  - **Bronze (e.g., Score 0-40):** $10/mo
  - **Silver (e.g., Score 41-70):** $25/mo
  - **Gold (e.g., Score 71-90):** $50/mo
  - **Diamond (e.g., Score 91-100):** $100/mo
- *Note:* Upgrades/Downgrades in tiers automatically adjust the price for new subscribers. Existing subscribers are grandfathered until their current cycle ends.

### 2.5 On-Chain Broadcaster
- **Purpose:** Periodically writes the finalized Trust Scores and Tier statuses to the platform's Smart Contract. This dictates how the Management Layer routes subscription funds.

## 3. Data Flow
1. Node developer signs and pushes signal $\rightarrow$ 0G Storage.
2. 0G Ingester reads, verifies signature, and registers signal as PENDING.
3. Resolution Engine monitors CEX Aggregator feed.
4. Signal hits Entry Price $\rightarrow$ State changes to ACTIVE $\rightarrow$ Management Layer executes trade for users subscribed to that specific Node ID.
5. Signal hits TP/SL/Expiry $\rightarrow$ State changes to CLOSED.
6. Reputation Calculator updates the Node's Trust Score.
7. On-Chain Broadcaster syncs the new score to the smart contract.

## 4. Constraints & Trade-offs
- **Centralized Price Aggregator:** While the nodes and storage are decentralized via 0G, the Verification Layer relies on a platform-run CEX aggregator for price resolution. This trade-off is accepted to allow nodes to trade long-tail tokens and memecoins immediately upon listing.
- **Memory Intensive:** The Resolution Engine must hold thousands of PENDING and ACTIVE signals in memory while processing high-frequency websocket ticks. Efficient data structures (like Redis sorted sets) will be required.