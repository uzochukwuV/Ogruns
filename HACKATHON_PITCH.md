# 0G Signal Intelligence Network

## 🎯 One-Liner (Memorize This)

> **"We turn AI trading signals into programmable financial primitives—verifiable, executable, and monetizable on-chain."**

---

## 🔥 The Problem (30 seconds)

The AI trading ecosystem is broken:

| Problem | Impact |
|---------|--------|
| **Best signals are private** | No discovery mechanism |
| **Public signals are fake** | Cherry-picked screenshots, fake PnL |
| **Even good signals are slow** | Manual copy trading, dashboard polling |

**Result:** $50B+ signal industry runs on trust, not proof.

---

## ✅ Our Solution (60 seconds)

**A decentralized marketplace where AI trading agents publish verifiable signals, earn based on performance, and trigger subscriber bots in real-time.**

### Three Pillars:

| Pillar | What We Do | How |
|--------|------------|-----|
| **1. Verifiability** | Prove performance, not claim it | Time-decay Sharpe scoring, on-chain anchoring |
| **2. Execution** | Instant triggers, not dashboards | Webhook fires in <100ms, bots execute automatically |
| **3. Monetization** | Recurring economy, not one-time | Tier-based subscriptions, auto revenue routing |

---

## 🧠 Key Innovation: Signals as Financial Primitives

In our system, a signal is not just a prediction.

It becomes:
- A **verifiable performance asset** (tracked on-chain)
- A **revenue-generating stream** (via subscriptions)
- An **executable trigger** (via webhooks)

```
OLD: Signal → Dashboard → Human reads → Human decides → Human executes
     (Minutes to hours)

NEW: Signal → Condition met → Webhook fires → Bot executes
     (<100 milliseconds)
```

**We transformed signals from information → autonomous execution.**

---

## ⚡ Execution-Grade Signals

Our signals are not advisory. They are:

- **Condition-aware** — entry/TP/SL tracked in real-time
- **Stateful** — PENDING → ACTIVE → WIN/LOSS lifecycle
- **Triggerable** — webhooks fire automatically on state change

This removes the human entirely from the loop.

---

## 📊 Scoring System (Judge This)

```
Trust Score (0-100) = EV(60) + WinRate(25) + Sharpe(15)

Time-decay: λ = 0.05 (half-life ≈ 14 days)
→ Recent signals dominate
→ No cherry-picking old wins
→ Sharpe ratio = risk-adjusted consistency
```

| Tier | Score | Price | Creator Split |
|------|-------|-------|---------------|
| BRONZE | 0-40 | $10/mo | 60% |
| SILVER | 41-70 | $25/mo | 70% |
| GOLD | 71-90 | $50/mo | 80% |
| DIAMOND | 91+ | $100/mo | 85% |

**Agents earn based on provable alpha, not marketing.**

---

## 🔗 0G Integration (Score This)

| 0G Component | Our Usage |
|--------------|-----------|
| **0G Storage** | Zero-gas signal batching (daily Merkle roots) |
| **0G EVM** | 4 smart contracts on Galileo Testnet |
| **0G DA Indexer** | Historical proof retrieval |
| **ERC-7857 Agentic ID** | Verified AI agent identity |

### Deployed Contracts:
- AgentRegistry: `0xe87a81d65a5c03596F9783E17E9b36bDeCe99591`
- ReputationOracle: `0xfcc3e1511DEb19c6039F66CFE6eB35A004844DCd`
- SubscriptionManager: `0x8BD5d799C7B221D3602820493AB691Af9577F31B`
- FeeRouter: `0xD59fB54c76ec892c3Cd912F2ac7afd71addA8F62`

---

## 🏆 Competitive Differentiation

| | Provus | alphatrace | YieldBoost | **Us** |
|--|--------|------------|------------|--------|
| Verifiable performance | TEE attestation | On-chain decisions | Proof-backed | **Time-decay Sharpe + Merkle proofs** |
| Signal distribution | Dashboard | Dashboard | Dashboard | **Instant webhooks** |
| Monetization | Unclear | Unclear | Unclear | **Subscription + auto revenue routing** |
| Scalability | Single user | Single user | Single user | **1000+ concurrent subscribers** |

---

## 🎬 Demo Flow (3 minutes)

### Minute 1: Signal Publication
1. AI agent generates BTC/USDT LONG signal
2. Signs with EIP-191 (cryptographic proof)
3. Submits to L2 API (zero gas)
4. Verification layer monitors market price feed

### Minute 2: Execution & Scoring
1. Entry price hit → Signal ACTIVE
2. Take profit hit → Signal closes as WIN
3. Webhook fires to all subscribers
4. Subscriber bots execute trades instantly
5. Trust score recalculated with time-decay

### Minute 3: Economy
1. Show subscription payment on-chain
2. FeeRouter splits: 80% creator, 20% platform
3. Agent reputation increases → higher tier → higher price

---

## 🔧 Technical Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    HYBRID VERIFICATION                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  OFF-CHAIN (Speed)              ON-CHAIN (Trust)            │
│  ─────────────────              ──────────────────          │
│  • Signal generation            • Performance proofs        │
│  • Price monitoring             • Reputation scores         │
│  • Webhook dispatch             • Subscription economy      │
│  • 100 concurrent workers       • Revenue routing           │
│                                                             │
│  ↓                              ↓                           │
│  High-frequency                 Trustless                   │
│  performance                    verification                │
│                                                             │
└─────────────────────────────────────────────────────────────┘

Market data sources are pluggable (CEX, DEX, oracles).
Not dependent on a single provider.
```

---

## 📈 Traction & Metrics

| Metric | Value |
|--------|-------|
| Smart contracts deployed | 4 on 0G Galileo |
| Webhook capacity | 10,000 jobs buffered |
| Concurrent workers | 100 |
| Signal throughput | 100/min global, 10/min per agent |
| State persistence | AES-256-GCM encrypted |

---

## 🚀 Future Roadmap

| Phase | Feature |
|-------|---------|
| **Post-Hackathon** | Batch on-chain updates (80% gas reduction) |
| **Q3 2026** | Merkle root commitment (99% gas reduction) |
| **Q4 2026** | Multi-chain deployment (Base, Arbitrum) |
| **2027** | Full agent autonomy (agents subscribe to agents) |

---

## 👥 Team

Solo builder for hackathon. Full-stack:
- Go backend (verification layer)
- Solidity contracts (4 deployed)
- TypeScript SDK (node template)
- Next.js frontend (dashboard)

---

## 🎯 Ask

**We're not asking you to trust AI trading signals.**

**We're giving you the infrastructure to verify and execute them.**

---

## 📎 Links

| Resource | Link |
|----------|------|
| GitHub | `[REPO LINK]` |
| Demo Video | `[VIDEO LINK]` |
| Live API | `http://localhost:8080` (local demo) |
| X Post | `[X LINK]` |

---

## 💬 Closing Statement

> *"We didn't just build a trading bot. We built the infrastructure for monetizing and executing AI alpha."*

**Performance is provable. Execution is instant. Monetization is automated.**

That's **programmable financial primitives** for autonomous trading systems.

---

*0G APAC Hackathon 2026 — Track 3: Agentic Economy*
