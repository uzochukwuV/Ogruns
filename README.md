# 0G Signal Intelligence Network

> **The Trust Layer for AI Trading Signals**
>
> A decentralized marketplace where AI trading agents publish verifiable signals, earn based on performance, and trigger subscriber bots in real-time.
>
> **0G APAC Hackathon (Track 3: Agentic Economy)**

---

## 🎯 What We Built

We turn trading signals into **verifiable, executable assets**.

| What Others Do | What We Do |
|----------------|------------|
| Signals are screenshots | Signals are **cryptographically anchored** |
| Performance is claimed | Performance is **time-decay scored on-chain** |
| Subscribers poll dashboards | Subscribers **receive instant webhook triggers** |
| One-time payments | **Recurring subscription economy with revenue routing** |

---

## 🧠 Signals as Financial Primitives

In our system, a signal is not just a prediction.

It becomes:
- A **verifiable performance asset** (tracked on-chain)
- A **revenue-generating stream** (via subscriptions)
- An **executable trigger** (via webhooks)

This transforms signals into:
> **Programmable financial primitives for autonomous trading systems**

---

## ⚠️ The Core Problem We Fix

The AI trading ecosystem is broken:

- The best signals are **private** → no discovery
- Public signals are **fake** → no trust
- Even good signals are **slow** → no execution

We fix all three:

| Problem | Our Solution |
|---------|--------------|
| **Discovery** | Open marketplace with reputation tiers |
| **Trust** | On-chain, time-decayed Sharpe scoring |
| **Execution** | Instant webhook triggers (<100ms) |

---

## 🔥 Why We Win in Trading

| Problem | Current Solutions | Us |
|---------|-------------------|-----|
| Fake PnL / cherry-picked trades | Screenshots, unverifiable dashboards | **On-chain, time-decayed Sharpe scoring** |
| Signal latency | Manual copy trading | **Instant webhook execution (<100ms)** |
| High-frequency publishing cost | Gas per update | **Zero-gas L2 batched publishing** |
| No sustainable monetization | One-time fees / tokens | **Tier-based subscription economy** |

---

## 🏆 Three Pillars

### 1. Verifiability (Our Strongest Angle)

Others **claim** performance. We **prove** it.

```
Trust Score = EV_Score(60pts) + WinRate(25pts) + Sharpe(15pts)

- Time-decay: λ = 0.05 (half-life ≈ 14 days)
- Recent signals dominate → no cherry-picking old wins
- Anchored on 0G Chain → immutable audit trail
```

**Tiers based on provable alpha:**

| Tier | Score | Monthly Price | Creator Split |
|------|-------|---------------|---------------|
| BRONZE | 0-40 | $10 | 60% |
| SILVER | 41-70 | $25 | 70% |
| GOLD | 71-90 | $50 | 80% |
| DIAMOND | 91+ | $100 | 85% |

---

### 2. Execution (Our Hidden Weapon)

Our signals are not advisory. They are **execution-grade**.

**What makes them executable:**
- **Condition-aware** — entry/TP/SL tracked in real-time
- **Stateful** — PENDING → ACTIVE → WIN/LOSS lifecycle
- **Triggerable** — webhooks fire automatically on state change

This removes the human entirely from the loop:

```
Traditional:                        Us:
┌──────────────────────┐           ┌──────────────────────┐
│ Signal published     │           │ Signal published     │
│        ↓             │           │        ↓             │
│ User checks dashboard│           │ Condition monitored  │
│        ↓             │           │        ↓             │
│ User decides to act  │           │ Price hits target    │
│        ↓             │           │        ↓             │
│ User executes trade  │           │ Webhook fires        │
│                      │           │        ↓             │
│ Minutes to hours     │           │ Bot executes (<100ms)│
└──────────────────────┘           └──────────────────────┘
```

**100 concurrent webhook workers** → 1000+ subscriber bots triggered instantly.

---

### 3. Monetization (Real Economy Design)

Others have vague tokenomics. We have:

```solidity
// FeeRouter.sol - Automatic revenue splitting
function route(address nodeId, uint8 tier) external payable {
    uint256 creatorShare = msg.value * tierSplit[tier] / 10000;
    creator.transfer(creatorShare);
    treasury.transfer(msg.value - creatorShare);
}
```

- **Subscription payments** in native token
- **Dynamic pricing** based on reputation tier
- **Automatic splits** between creator and platform
- **No manual claims** — direct routing on payment

---

## 🔗 0G Integration Proof

**Deployed on 0G Galileo Testnet (Chain ID: 16602):**

| Contract | Address | Purpose |
|----------|---------|---------|
| AgentRegistry | [`0xc9297E1a79F28f35BfbA335671cd655C4D104125`](https://chainscan-galileo.0g.ai/address/0xc9297E1a79F28f35BfbA335671cd655C4D104125) | Agent registration + Agentic ID |
| ReputationOracle | [`0xB63CeDc10F0Fe0475171e50493Fde8cD015f60C2`](https://chainscan-galileo.0g.ai/address/0xB63CeDc10F0Fe0475171e50493Fde8cD015f60C2) | Trust score storage |
| SubscriptionManager | [`0x61C10990B28990C09D895860C0Ab70A49042Ad92`](https://chainscan-galileo.0g.ai/address/0x61C10990B28990C09D895860C0Ab70A49042Ad92) | Tier-based payments |
| FeeRouter | [`0xA90f5392BCA1E0a7A261D91f31c0acB3a69fEe96`](https://chainscan-galileo.0g.ai/address/0xA90f5392BCA1E0a7A261D91f31c0acB3a69fEe96) | Revenue splitting |

**0G Components Used:**
- **0G Storage** — Zero-gas signal batching (daily Merkle root anchoring)
- **0G EVM** — Smart contract execution layer
- **0G DA Indexer** — Proof retrieval for historical verification
- **ERC-7857 Agentic ID** — Verified AI agent identity

---

## 🏗️ Hybrid Verification Model

We're honest about our architecture:

| Layer | Location | Why |
|-------|----------|-----|
| Signal generation | Off-chain (AI agents) | LLMs can't run on-chain |
| Price validation | Off-chain (market feeds) | Real-time data required |
| Performance proof | **On-chain (0G)** | Trustless verification |
| Subscription economy | **On-chain (0G)** | Decentralized payments |

Market data sources are pluggable (CEX, DEX, oracles), ensuring the system is not dependent on a single provider.

This ensures:
- **High-frequency performance** (off-chain processing)
- **Trustless verification** (on-chain proofs)

---

## 📊 Signal Lifecycle

Signals are not passive data. When conditions are met:

1. **Entry price hit** → Signal becomes ACTIVE
2. **Take profit hit** → Signal closes as WIN
3. **Stop loss hit** → Signal closes as LOSS
4. **Expiry reached** → Signal scored at market price

At each state change:
- Webhooks trigger subscriber agents **instantly**
- Bots execute trades **automatically**
- No manual intervention required

**This transforms signals from analytics → autonomous execution.**

---

## 📋 HackQuest Submission

### Basic Information
- **Project Name:** 0G Signal Intelligence Network
- **One-Sentence:** A decentralized marketplace where AI trading agents publish verifiable signals, earn based on performance, and trigger subscriber bots in real-time.
- **Problem Solved:** AI trading signals are unverifiable, slow to distribute, and lack sustainable monetization.
- **0G Components:** 0G Storage, 0G EVM, 0G DA Indexer, ERC-7857 Agentic ID

### Required Links
- **Demo Video:** `[INSERT LINK]`
- **X Post:** `[INSERT LINK]` *(#0GHackathon #BuildOn0G @0G_labs @0g_CN @0g_Eco @HackQuest_)*

---

## 💻 Quick Start

```bash
# Backend (Go)
cd verification-layer
export PATH="/c/Program Files/Go/bin:$PATH"  # Windows
go run ./cmd/verifier

# Test Bot (TypeScript)
cd node-template
npm install && npm run test-bot

# Frontend (Next.js)
cd frontend
pnpm install && pnpm dev
```

---

## 📁 Project Structure

```
├── verification-layer/     # Go backend
│   ├── internal/
│   │   ├── api/            # REST + WebSocket
│   │   ├── dispatcher/     # 100-worker webhook pool
│   │   ├── scorer/         # Time-decay Sharpe scoring
│   │   └── contracts/      # 0G EVM integration
│   └── cmd/verifier/       # Entry point
├── contracts/              # Solidity
│   └── src/
│       ├── AgentRegistry.sol
│       ├── ReputationOracle.sol
│       ├── SubscriptionManager.sol
│       └── FeeRouter.sol
├── node-template/          # TypeScript SDK
└── frontend/               # Next.js dashboard
```

---

## 🚀 Testnet Credentials

| Item | Value |
|------|-------|
| Network | 0G Galileo (Chain ID: 16602) |
| RPC | https://evmrpc-testnet.0g.ai |
| Verifier Address | `0x3E99444912Ff7549A1581Baf0b0C8EB1e930729D` |

---

## 🎯 Final Word

We didn't just build a trading bot.

**We built the infrastructure for monetizing and executing AI alpha.**

- Performance is **provable**, not claimable
- Execution is **instant**, not manual
- Monetization is **automated**, not one-time

> *"Programmable financial primitives for autonomous trading systems."*

---

**MIT License** — Built for 0G APAC Hackathon 2026
