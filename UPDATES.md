Excellent analysis. Let me synthesize the findings and provide a structured evaluation with recommendations.

Signal Consumption Architecture Analysis
Current Architecture Overview

┌──────────────────────────────────────────────────────────────┐
│                    SIGNAL CONSUMERS                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │ WebSocket   │  │  Webhooks   │  │ REST API    │          │
│  │ (Real-time) │  │ (Push)      │  │ (Pull)      │          │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘          │
└─────────┼────────────────┼────────────────┼──────────────────┘
          │                │                │
          ▼                ▼                ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────────┐
│ streamHub       │ │ WebhookDispatcher│ │ Subscription Gate  │
│ (sync.Map)      │ │ (100 workers)   │ │ (EIP-191 + Cache)  │
│ 64-event buffer │ │ 10k job queue   │ │ 5-min cache TTL    │
└─────────────────┘ └─────────────────┘ └─────────────────────┘
✅ PROS (Current Design)
Aspect	Strength
Low Latency	WebSocket delivers signals in milliseconds via in-memory hub
Scalability	100 concurrent webhook workers handle 1000+ AI agents
Resilience	10,000-job queue buffers spikes without dropping signals
Security	EIP-191 signatures + 5-min replay protection + on-chain verification
Decoupled	Push (webhooks) vs Pull (REST) vs Stream (WS) all supported
Rate Limiting	Per-node (10/min) + global (100/min) + duplicate detection
Cache Efficiency	5-minute subscription cache avoids blockchain spam
❌ CONS (Current Design)
Issue	Problem	Impact
No delivery guarantee	Webhooks fire-and-forget, no retry on failure	AI agents miss signals if endpoint down
No persistence	In-memory hub loses signals on restart	Subscribers must reconnect immediately
Single-node architecture	No horizontal scaling for hub/dispatcher	Bottleneck at ~10k concurrent WS connections
No backpressure	Slow webhook endpoints block worker	Can cascade to queue overflow
No message ordering	Concurrent webhook workers may deliver out-of-order	AI agents see TP before Entry
Memory growth	subscriber lists never cleaned up	Long-running instances accumulate stale URLs
No dead-letter queue	Failed webhooks vanish	No visibility into delivery failures
🔧 RECOMMENDATIONS
1. Webhook Retry with Exponential Backoff (High Priority)
Current: Fire-and-forget webhook delivery loses signals on transient failures.


// internal/dispatcher/webhook.go - Add retry logic
type WebhookJob struct {
    URL       string
    Payload   []byte
    Attempts  int
    NextRetry time.Time
}

func (d *WebhookDispatcher) processJob(job WebhookJob) {
    resp, err := d.httpClient.Post(job.URL, "application/json", bytes.NewReader(job.Payload))
    
    if err != nil || resp.StatusCode >= 500 {
        if job.Attempts < 3 {
            job.Attempts++
            job.NextRetry = time.Now().Add(time.Duration(1<<job.Attempts) * time.Second) // 2s, 4s, 8s
            d.retryQueue <- job
            return
        }
        d.deadLetterQueue <- job // Track permanent failures
    }
}
2. Webhook URL Health Tracking (Medium Priority)
Remove endpoints that consistently fail to prevent queue pollution.


type WebhookHealth struct {
    URL           string
    ConsecutiveFails int
    LastSuccess   time.Time
}

// Auto-disable after 10 consecutive failures
if health.ConsecutiveFails >= 10 {
    d.Unregister(nodeID, url)
    log.Printf("Disabled unhealthy webhook: %s", url)
}
3. Signal Persistence Layer (High Priority for Production)
Add Redis or SQLite for signal durability and horizontal scaling.


// Option A: Redis pub/sub for distributed streaming
type RedisStreamHub struct {
    client *redis.Client
}

func (h *RedisStreamHub) Publish(nodeID string, signal *ActiveSignal) {
    h.client.Publish(ctx, "signals:"+nodeID, json.Marshal(signal))
}

// Option B: SQLite write-ahead log for crash recovery
type SignalWAL struct {
    db *sql.DB
}

func (w *SignalWAL) Append(signal types.SignalEnvelope) error {
    _, err := w.db.Exec("INSERT INTO wal (signal_json, created_at) VALUES (?, ?)", 
        json.Marshal(signal), time.Now())
    return err
}
4. Message Ordering for Webhooks (Medium Priority)
Ensure signals for the same node are delivered in order.


// Use per-node queues instead of global pool
type OrderedDispatcher struct {
    nodeQueues map[string]chan WebhookJob  // One queue per node
    workers    map[string]*Worker          // One worker per node
}

// Guarantees: Entry → Active → Win delivered in sequence
5. Subscriber Cleanup Goroutine (Low Priority)
Periodically prune inactive webhook URLs and stale WS connections.


func (d *WebhookDispatcher) StartCleanup(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Minute)
    for {
        select {
        case <-ticker.C:
            d.mu.Lock()
            for nodeID, urls := range d.subs {
                // Remove URLs that haven't succeeded in 24 hours
                d.subs[nodeID] = filterHealthy(urls)
            }
            d.mu.Unlock()
        case <-ctx.Done():
            return
        }
    }
}
📊 Architecture Comparison
Approach	Latency	Reliability	Scalability	Complexity
Current (In-memory)	⚡ ~1ms	❌ No durability	🔶 Single node	✅ Simple
+ Redis Pub/Sub	⚡ ~5ms	✅ Cluster support	✅ Horizontal	🔶 Medium
+ Kafka/NATS	🔶 ~20ms	✅ Persistent	✅ Massive	❌ Complex
+ WebSocket + gRPC	⚡ ~2ms	✅ Bi-directional	✅ Load balanced	🔶 Medium
🎯 Recommended Priority
Webhook retry + dead-letter queue - Critical for production reliability
Health tracking + auto-disable - Prevents queue cascade failures
Redis for horizontal scaling - When you exceed single-node limits
Message ordering - If AI agents need strict sequence guarantees
The current architecture is solid for a hackathon MVP. The in-memory design is fast and simple. For production with 1000+ paying subscribers, adding webhook retry and Redis pub/sub would be the highest-impact improvements.

Would you like me to implement any of these improvements?



Excellent analysis. Let me synthesize the findings and provide a structured evaluation with recommendations.

Signal Consumption Architecture Analysis
Current Architecture Overview

┌──────────────────────────────────────────────────────────────┐
│                    SIGNAL CONSUMERS                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │ WebSocket   │  │  Webhooks   │  │ REST API    │          │
│  │ (Real-time) │  │ (Push)      │  │ (Pull)      │          │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘          │
└─────────┼────────────────┼────────────────┼──────────────────┘
          │                │                │
          ▼                ▼                ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────────┐
│ streamHub       │ │ WebhookDispatcher│ │ Subscription Gate  │
│ (sync.Map)      │ │ (100 workers)   │ │ (EIP-191 + Cache)  │
│ 64-event buffer │ │ 10k job queue   │ │ 5-min cache TTL    │
└─────────────────┘ └─────────────────┘ └─────────────────────┘
✅ PROS (Current Design)
Aspect	Strength
Low Latency	WebSocket delivers signals in milliseconds via in-memory hub
Scalability	100 concurrent webhook workers handle 1000+ AI agents
Resilience	10,000-job queue buffers spikes without dropping signals
Security	EIP-191 signatures + 5-min replay protection + on-chain verification
Decoupled	Push (webhooks) vs Pull (REST) vs Stream (WS) all supported
Rate Limiting	Per-node (10/min) + global (100/min) + duplicate detection
Cache Efficiency	5-minute subscription cache avoids blockchain spam
❌ CONS (Current Design)
Issue	Problem	Impact
No delivery guarantee	Webhooks fire-and-forget, no retry on failure	AI agents miss signals if endpoint down
No persistence	In-memory hub loses signals on restart	Subscribers must reconnect immediately
Single-node architecture	No horizontal scaling for hub/dispatcher	Bottleneck at ~10k concurrent WS connections
No backpressure	Slow webhook endpoints block worker	Can cascade to queue overflow
No message ordering	Concurrent webhook workers may deliver out-of-order	AI agents see TP before Entry
Memory growth	subscriber lists never cleaned up	Long-running instances accumulate stale URLs
No dead-letter queue	Failed webhooks vanish	No visibility into delivery failures
🔧 RECOMMENDATIONS
1. Webhook Retry with Exponential Backoff (High Priority)
Current: Fire-and-forget webhook delivery loses signals on transient failures.


// internal/dispatcher/webhook.go - Add retry logic
type WebhookJob struct {
    URL       string
    Payload   []byte
    Attempts  int
    NextRetry time.Time
}

func (d *WebhookDispatcher) processJob(job WebhookJob) {
    resp, err := d.httpClient.Post(job.URL, "application/json", bytes.NewReader(job.Payload))
    
    if err != nil || resp.StatusCode >= 500 {
        if job.Attempts < 3 {
            job.Attempts++
            job.NextRetry = time.Now().Add(time.Duration(1<<job.Attempts) * time.Second) // 2s, 4s, 8s
            d.retryQueue <- job
            return
        }
        d.deadLetterQueue <- job // Track permanent failures
    }
}
2. Webhook URL Health Tracking (Medium Priority)
Remove endpoints that consistently fail to prevent queue pollution.


type WebhookHealth struct {
    URL           string
    ConsecutiveFails int
    LastSuccess   time.Time
}

// Auto-disable after 10 consecutive failures
if health.ConsecutiveFails >= 10 {
    d.Unregister(nodeID, url)
    log.Printf("Disabled unhealthy webhook: %s", url)
}
3. Signal Persistence Layer (High Priority for Production)
Add Redis or SQLite for signal durability and horizontal scaling.


// Option A: Redis pub/sub for distributed streaming
type RedisStreamHub struct {
    client *redis.Client
}

func (h *RedisStreamHub) Publish(nodeID string, signal *ActiveSignal) {
    h.client.Publish(ctx, "signals:"+nodeID, json.Marshal(signal))
}

// Option B: SQLite write-ahead log for crash recovery
type SignalWAL struct {
    db *sql.DB
}

func (w *SignalWAL) Append(signal types.SignalEnvelope) error {
    _, err := w.db.Exec("INSERT INTO wal (signal_json, created_at) VALUES (?, ?)", 
        json.Marshal(signal), time.Now())
    return err
}
4. Message Ordering for Webhooks (Medium Priority)
Ensure signals for the same node are delivered in order.


// Use per-node queues instead of global pool
type OrderedDispatcher struct {
    nodeQueues map[string]chan WebhookJob  // One queue per node
    workers    map[string]*Worker          // One worker per node
}

// Guarantees: Entry → Active → Win delivered in sequence
5. Subscriber Cleanup Goroutine (Low Priority)
Periodically prune inactive webhook URLs and stale WS connections.


func (d *WebhookDispatcher) StartCleanup(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Minute)
    for {
        select {
        case <-ticker.C:
            d.mu.Lock()
            for nodeID, urls := range d.subs {
                // Remove URLs that haven't succeeded in 24 hours
                d.subs[nodeID] = filterHealthy(urls)
            }
            d.mu.Unlock()
        case <-ctx.Done():
            return
        }
    }
}
📊 Architecture Comparison
Approach	Latency	Reliability	Scalability	Complexity
Current (In-memory)	⚡ ~1ms	❌ No durability	🔶 Single node	✅ Simple
+ Redis Pub/Sub	⚡ ~5ms	✅ Cluster support	✅ Horizontal	🔶 Medium
+ Kafka/NATS	🔶 ~20ms	✅ Persistent	✅ Massive	❌ Complex
+ WebSocket + gRPC	⚡ ~2ms	✅ Bi-directional	✅ Load balanced	🔶 Medium
🎯 Recommended Priority
Webhook retry + dead-letter queue - Critical for production reliability
Health tracking + auto-disable - Prevents queue cascade failures
Redis for horizontal scaling - When you exceed single-node limits
Message ordering - If AI agents need strict sequence guarantees
The current architecture is solid for a hackathon MVP. The in-memory design is fast and simple. For production with 1000+ paying subscribers, adding webhook retry and Redis pub/sub would be the highest-impact improvements.

Would you like me to implement any of these improvements?

ok , lets before we move on , extract api docs from frontend docs and update it with our new interface and returns and save in a new .md just api focused
The user wants me to:

Read the existing FRONTEND_API_DOCS.md file
Extract just the API-related documentation
Update it with the new interface changes (trade_type, leverage, take_profits, etc.)
Save it as a new API-focused markdown file
Let me first read the existing frontend docs to understand what's there.

Read c:\Users\ASUS FX95G\Documents\Ogruns\docs\FRONTEND_API_DOCS.md
You've hit your limit · resets 5:20am (Africa/Lagos)

continue
The user wants me to continue creating the API-focused documentation. I've read the existing FRONTEND_API_DOCS.md and now I need to:

Extract just the API-related parts
Update with the new fields (trade_type, leverage, take_profits, leveraged_pnl, etc.)
Add the signal submission endpoint documentation
Add webhook registration and authentication documentation
Save to a new file like docs/API_REFERENCE.md
API Error: Connection error.

continue
The user wants me to continue creating the API-focused documentation file. I've already read the existing FRONTEND_API_DOCS.md. Now I need to create a new API_REFERENCE.md file that:

Extracts only the API-related content
Updates all schemas with new fields (trade_type, leverage, take_profits, leveraged_pnl)
Adds the signal submission endpoint
Adds webhook registration endpoint
Adds authentication/subscription documentation
Removes frontend-specific content (React examples, chart colors, feature requirements)
Write c:\Users\ASUS FX95G\Documents\Ogruns\docs\API_REFERENCE.md
858 lines
# 0G Signal Marketplace - API Reference

> **Version:** 2.0
> **Base URL:** `http://localhost:8080` (development) | `https://api.ogruns.io` (production)

---

## Table of Contents

1. [Authentication](#authentication)
2. [Data Types](#data-types)
3. [Signal Submission API](#signal-submission-api)
4. [Signal Consumption API](#signal-consumption-api)
5. [Analytics API](#analytics-api)
6. [WebSocket Streams](#websocket-streams)
7. [Webhook Integration](#webhook-integration)
8. [Rate Limiting](#rate-limiting)
9. [Error Handling](#error-handling)

---

## Authentication

### Subscription-Gated Endpoints

Premium endpoints require EIP-191 signature authentication with replay protection.

**Required Headers/Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `subscriber_address` | string | Ethereum address of the subscriber |
| `signature` | string | EIP-191 signature of the access message |
| `timestamp` | integer | Unix timestamp (must be within 5 minutes) |

**Signature Message Format:**
```
message = "access:{nodeID}:{timestamp}"
```

**JavaScript Example (ethers.js v6):**
```javascript
const timestamp = Math.floor(Date.now() / 1000);
const message = `access:${nodeID}:${timestamp}`;
const signature = await wallet.signMessage(message);

// Use in request
fetch(`/api/v1/stream?node_id=${nodeID}&subscriber_address=${wallet.address}&signature=${signature}&timestamp=${timestamp}`)
```

**Security Notes:**
- Signatures expire after 5 minutes (replay protection)
- Subscription status is verified on-chain via `SubscriptionManager` contract
- Results are cached for 5 minutes to reduce blockchain calls

---

## Data Types

### SignalPayload

The core trading signal structure.

```typescript
interface SignalPayload {
  token_pair:    string;        // e.g. "BTC/USDT"
  exchange?:     string;        // e.g. "binance" (optional)
  direction:     "long" | "short";
  entry_price:   number;
  take_profit:   number;
  stop_loss:     number;
  expiry_time:   number;        // Unix timestamp (seconds)
  weight_pct:    number;        // AI confidence 0-100

  // v2 fields
  trade_type?:   "spot" | "perpetual" | "futures";  // default: "spot"
  leverage?:     number;        // 1x for spot, 1-125x for perpetuals
  take_profits?: TakeProfitLevel[];  // Optional multi-TP levels
}

interface TakeProfitLevel {
  price:   number;   // Target price
  percent: number;   // % of position to close (0-100)
}
```

### SignalEnvelope

Signed wrapper around a signal payload.

```typescript
interface SignalEnvelope {
  node_id:           string;   // Ethereum address (0x...)
  timestamp:         number;   // Unix timestamp when created
  payload:           SignalPayload;
  signature:         string;   // EIP-191 ECDSA signature
  encryption_pubkey?: string;  // Future: subscriber-side decryption
}
```

### NodeStats

Statistics for a signal node.

```typescript
interface NodeStats {
  node_id:       string;
  total_signals: number;
  win_count:     number;
  loss_count:    number;
  expired_count: number;
  win_rate:      number;   // 0.0 - 1.0
  avg_ev:        number;   // Expected value
  sharpe_ratio:  number;
  trust_score:   number;   // 0 - 100
  tier:          "BRONZE" | "SILVER" | "GOLD" | "DIAMOND";
  updated_at:    number;   // Unix timestamp
}
```

### Signal States

| State | Description |
|-------|-------------|
| `PENDING` | Signal submitted, waiting for entry price to be hit |
| `ACTIVE` | Entry price hit, trade is live |
| `CLOSED_WIN` | Take profit hit |
| `CLOSED_LOSS` | Stop loss hit |
| `EXPIRED` | Time expired without hitting TP or SL |
| `CANCELLED` | Signal cancelled before activation |

### Tier System

| Tier | Trust Score | Max Expiry (Spot) | Max Expiry (Leveraged) | Monthly Price |
|------|-------------|-------------------|------------------------|---------------|
| BRONZE | 0 - 40 | 168h (7 days) | 48h | $10/mo |
| SILVER | 41 - 70 | 168h (7 days) | 48h | $25/mo |
| GOLD | 71 - 90 | 168h (7 days) | 48h | $50/mo |
| DIAMOND | 91 - 100 | 168h (7 days) | 48h | $100/mo |

### Validation Rules

| Field | Rule |
|-------|------|
| `direction` | Must be lowercase: `"long"` or `"short"` |
| `trade_type` | If omitted, defaults to `"spot"` |
| `leverage` | Must be 1 for spot; 1-125 for perpetual/futures |
| `expiry_time` | Max 168h for spot, max 48h for leveraged positions |
| `stop_loss` | Required for leveraged trades (leverage > 1x) |

---

## Signal Submission API

### Submit Signal

```
POST /api/v1/signals
```

Submit a new trading signal for verification and scheduling.

**Request Body:**
```json
{
  "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
  "envelope": {
    "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
    "timestamp": 1714567800,
    "payload": {
      "token_pair": "BTC/USDT",
      "exchange": "binance",
      "direction": "long",
      "entry_price": 64500.00,
      "take_profit": 67000.00,
      "stop_loss": 63000.00,
      "expiry_time": 1714571400,
      "weight_pct": 85,
      "trade_type": "perpetual",
      "leverage": 10
    },
    "signature": "0x..."
  }
}
```

**Signature Generation:**

The signature must be EIP-191 personal_sign over: `{timestamp}:{payload_json}`

```javascript
// JavaScript (ethers.js v6)
const timestamp = Math.floor(Date.now() / 1000);
const payloadJSON = JSON.stringify(orderedPayload); // Field order must match Go struct
const message = `${timestamp}:${payloadJSON}`;
const signature = await wallet.signMessage(message);
```

**Critical:** JSON field order must match Go struct tag order exactly:
1. `token_pair`
2. `exchange` (omit if empty)
3. `direction`
4. `entry_price`
5. `take_profit`
6. `stop_loss`
7. `expiry_time`
8. `weight_pct`
9. `trade_type` (omit if empty)
10. `leverage` (omit if 0)
11. `take_profits` (omit if empty)

**Success Response (200):**
```json
{
  "status": "accepted",
  "message": "Signal scheduled for analysis at expiry",
  "signal_id": "0x82d4..._1714567800"
}
```

**Error Response (400):**
```json
{
  "status": "rejected",
  "error": "validation_failed",
  "message": "direction must be 'long' or 'short'"
}
```

**Error Response (429):**
```json
{
  "error": "rate_limited",
  "reason": "node_rate_limit",
  "retry_after_sec": 45,
  "message": "Node 0x82d4... exceeded 10 signals/minute limit"
}
```

---

## Signal Consumption API

### Health Check

```
GET /healthz
```

**Response:** `200 OK`

---

### List All Nodes

```
GET /api/v1/nodes
```

Returns all registered nodes with current statistics. **No authentication required.**

**Response:**
```json
{
  "nodes": [
    {
      "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
      "total_signals": 156,
      "win_count": 98,
      "loss_count": 45,
      "expired_count": 13,
      "win_rate": 0.685,
      "avg_ev": 1.24,
      "sharpe_ratio": 2.15,
      "trust_score": 78.5,
      "tier": "GOLD",
      "updated_at": 1714567890
    }
  ],
  "count": 25
}
```

---

### Get Node Details

```
GET /api/v1/nodes/{nodeId}
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `nodeId` | string | Ethereum address of the node |

**Response (200):**
```json
{
  "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
  "total_signals": 156,
  "win_count": 98,
  "loss_count": 45,
  "expired_count": 13,
  "win_rate": 0.685,
  "avg_ev": 1.24,
  "sharpe_ratio": 2.15,
  "trust_score": 78.5,
  "tier": "GOLD",
  "updated_at": 1714567890
}
```

**Error Response (404):**
```json
{
  "error": "node not found"
}
```

---

### Dashboard Summary

```
GET /api/v1/analytics/dashboard
```

Returns platform-wide statistics. **No authentication required.**

**Response:**
```json
{
  "total_nodes": 25,
  "total_signals": 1420,
  "total_wins": 890,
  "total_losses": 530,
  "platform_win_rate": 0.627,
  "tier_distribution": {
    "BRONZE": 15,
    "SILVER": 6,
    "GOLD": 3,
    "DIAMOND": 1
  },
  "top_nodes": [
    {
      "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
      "total_signals": 156,
      "win_count": 98,
      "loss_count": 45,
      "expired_count": 13,
      "win_rate": 0.685,
      "avg_ev": 1.24,
      "sharpe_ratio": 2.15,
      "trust_score": 78.5,
      "tier": "GOLD",
      "updated_at": 1714567890
    }
  ],
  "updated_at": 1714567890
}
```

---

## Analytics API

### Node Analytics (Premium)

```
GET /api/v1/analytics/nodes/{nodeId}
```

Returns comprehensive analytics including signal history. **Requires subscription.**

**Headers:**
```
X-Subscriber-Address: 0x...
X-Subscriber-Signature: 0x...
X-Subscriber-Timestamp: 1714567800
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `from` | integer | 0 | Unix timestamp - filter signals from |
| `to` | integer | now | Unix timestamp - filter signals until |

**Response:**
```json
{
  "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
  "stats": {
    "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
    "total_signals": 156,
    "win_count": 98,
    "loss_count": 45,
    "expired_count": 13,
    "win_rate": 0.685,
    "avg_ev": 1.24,
    "sharpe_ratio": 2.15,
    "trust_score": 78.5,
    "tier": "GOLD",
    "updated_at": 1714567890
  },
  "signal_history": [
    {
      "id": "0x82d4..._1714567800",
      "token_pair": "BTC/USDT",
      "direction": "long",
      "trade_type": "perpetual",
      "leverage": 10,
      "entry_price": 64500.00,
      "take_profit": 67000.00,
      "stop_loss": 63000.00,
      "closed_price": 67250.00,
      "pnl_percent": 4.26,
      "leveraged_pnl_percent": 42.6,
      "outcome": "CLOSED_WIN",
      "outcome_type": "green",
      "confidence": 85,
      "created_at": 1714567800,
      "closed_at": 1714571400,
      "duration_sec": 3600
    }
  ],
  "performance_by_token": {
    "BTC/USDT": {
      "token_pair": "BTC/USDT",
      "total_signals": 80,
      "wins": 55,
      "losses": 25,
      "win_rate": 0.6875,
      "avg_pnl": 2.35,
      "total_pnl": 188.0
    }
  },
  "monthly_returns": [
    {
      "month": "2024-04",
      "pnl_percent": 8.3,
      "signals": 52,
      "win_rate": 0.65
    }
  ],
  "streaks": {
    "current_streak": 5,
    "current_streak_type": "winning",
    "longest_win_streak": 12,
    "longest_loss_streak": 4
  }
}
```

---

### Portfolio Simulation (Premium)

```
GET /api/v1/analytics/nodes/{nodeId}/simulate
```

Simulates portfolio performance. **Requires subscription.**

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `capital` | number | 1000 | Initial investment amount (USD) |
| `from` | integer | 0 | Unix timestamp - start simulation |
| `to` | integer | now | Unix timestamp - end simulation |

**Response:**
```json
{
  "initial_capital": 10000.00,
  "final_value": 15847.50,
  "total_return_percent": 58.47,
  "max_drawdown_percent": 8.3,
  "sharpe_ratio": 2.45,
  "total_trades": 156,
  "timeframe_days": 90,
  "equity_curve": [
    {"timestamp": 1711929600, "equity": 10000.00, "pnl_percent": 0},
    {"timestamp": 1711933200, "equity": 10250.00, "pnl_percent": 2.5}
  ],
  "trade_history": [
    {
      "signal_id": "0x82d4..._1711933200",
      "token_pair": "BTC/USDT",
      "direction": "long",
      "trade_type": "perpetual",
      "leverage": 10,
      "entry_price": 64500.00,
      "exit_price": 65800.00,
      "pnl_percent": 2.02,
      "leveraged_pnl_percent": 20.2,
      "pnl_amount": 2020.00,
      "outcome": "CLOSED_WIN",
      "timestamp": 1711933200
    }
  ]
}
```

---

## WebSocket Streams

### Live Signal Stream (Premium)

```
WS /api/v1/stream?node_id={nodeId}&subscriber_address={addr}&signature={sig}&timestamp={ts}
```

Subscribe to real-time signal updates. **Requires subscription.**

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `node_id` | string | Node to subscribe to |
| `subscriber_address` | string | Your Ethereum address |
| `signature` | string | EIP-191 signature |
| `timestamp` | integer | Signature timestamp |

**Connection Example:**
```javascript
const timestamp = Math.floor(Date.now() / 1000);
const message = `access:${nodeId}:${timestamp}`;
const signature = await wallet.signMessage(message);

const ws = new WebSocket(
  `ws://localhost:8080/api/v1/stream?node_id=${nodeId}&subscriber_address=${wallet.address}&signature=${signature}&timestamp=${timestamp}`
);

ws.onmessage = (event) => {
  const signal = JSON.parse(event.data);
  console.log('Signal update:', signal);
};
```

**Message Format:**
```json
{
  "ID": "0x82d4..._1714567800",
  "Envelope": {
    "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
    "timestamp": 1714567800,
    "payload": {
      "token_pair": "BTC/USDT",
      "exchange": "binance",
      "direction": "long",
      "entry_price": 64500.00,
      "take_profit": 67000.00,
      "stop_loss": 63000.00,
      "expiry_time": 1714571400,
      "weight_pct": 85,
      "trade_type": "perpetual",
      "leverage": 10
    },
    "signature": "0x..."
  },
  "State": "ACTIVE",
  "EntryHitAt": 1714567850,
  "ClosedAt": 0,
  "ClosedPrice": 0
}
```

**Keep-Alive:** Server sends ping every 30 seconds.

---

## Webhook Integration

### Register Webhook (Premium)

```
POST /api/v1/subscribers/webhook
```

Register a webhook URL to receive push notifications. **Requires subscription.**

**Request Body:**
```json
{
  "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
  "target_url": "https://my-ai-bot.vercel.app/webhook",
  "subscriber_address": "0x...",
  "signature": "0x...",
  "timestamp": 1714567800
}
```

**Response (200):**
```json
{
  "status": "registered",
  "message": "Webhook registered for node 0x82d4..."
}
```

### Webhook Payload

When signals are analyzed, registered webhooks receive:

```json
{
  "event_type": "SIGNAL_ANALYZED",
  "signal_id": "0x82d4..._1714567800",
  "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
  "token_pair": "BTC/USDT",
  "direction": "long",
  "trade_type": "perpetual",
  "leverage": 10,
  "entry_price": 64500.00,
  "target_price": 67000.00,
  "stop_loss": 63000.00,
  "price_at_expiry": 67250.00,
  "outcome": "CLOSED_WIN",
  "pnl_percent": 4.26,
  "leveraged_pnl_percent": 42.6,
  "risk_reward": 1.67,
  "node_trust_score": 78.5,
  "node_tier": "GOLD",
  "node_win_rate": 0.685,
  "node_sharpe_ratio": 2.15,
  "analyzed_at": 1714571400
}
```

**Event Types:**

| Event | Description |
|-------|-------------|
| `SIGNAL_ANALYZED` | Signal reached expiry and was scored |

---

## Rate Limiting

### Limits

| Limit | Value | Window |
|-------|-------|--------|
| Per-Node | 10 signals | 1 minute |
| Global | 100 signals | 1 minute |
| Duplicate Window | 5 minutes | Exact hash match |

### Rate Limit Stats

```
GET /api/v1/stats/ratelimit
```

**Response:**
```json
{
  "global_requests_in_window": 45,
  "global_max": 100,
  "global_window_sec": 60,
  "node_max": 10,
  "node_window_sec": 60,
  "tracked_nodes": 12,
  "cached_signal_hashes": 89
}
```

### Rate Limit Response (429)

```json
{
  "error": "rate_limited",
  "reason": "node_rate_limit",
  "retry_after_sec": 45,
  "message": "Node 0x82d4... exceeded 10 signals/minute limit"
}
```

**Reasons:**
- `node_rate_limit` - Node exceeded per-node limit
- `global_rate_limit` - Platform exceeded global limit
- `duplicate_signal` - Exact duplicate within 5 minutes

---

## Error Handling

### Error Response Format

```json
{
  "error": "error_code",
  "message": "Human readable message",
  "retry_after_sec": 30
}
```

### HTTP Status Codes

| Code | Error | Description |
|------|-------|-------------|
| 400 | `invalid_json` | Malformed JSON in request |
| 400 | `validation_failed` | Signal failed validation |
| 400 | `invalid_signature` | Signature verification failed |
| 402 | `not_subscribed` | Address not subscribed to node |
| 402 | `missing_credentials` | Authentication params missing |
| 404 | `node_not_found` | Node ID doesn't exist |
| 429 | `rate_limited` | Too many requests |
| 500 | `internal_error` | Server error |

### Subscription Errors (402)

| Error | Description |
|-------|-------------|
| `missing_credentials` | subscriber_address not provided |
| `invalid_address` | Invalid Ethereum address format |
| `missing_signature` | No signature provided |
| `missing_timestamp` | No timestamp for replay protection |
| `invalid_signature` | Signature verification failed |
| `not_subscribed` | Address not subscribed to this node |

---

## CORS

CORS is enabled for all origins in development. In production, configure `ALLOWED_ORIGINS` environment variable.

---

## SDK Examples

### Node.js / TypeScript

```typescript
import { ethers } from "ethers";
import axios from "axios";

const API_URL = "http://localhost:8080";

// Submit a signal
async function submitSignal(wallet: ethers.Wallet, payload: SignalPayload) {
  const timestamp = Math.floor(Date.now() / 1000);

  // Build ordered payload (must match Go struct order)
  const orderedPayload: Record<string, unknown> = {
    token_pair: payload.token_pair,
  };
  if (payload.exchange) orderedPayload.exchange = payload.exchange;
  orderedPayload.direction = payload.direction;
  orderedPayload.entry_price = payload.entry_price;
  orderedPayload.take_profit = payload.take_profit;
  orderedPayload.stop_loss = payload.stop_loss;
  orderedPayload.expiry_time = payload.expiry_time;
  orderedPayload.weight_pct = payload.weight_pct;
  if (payload.trade_type) orderedPayload.trade_type = payload.trade_type;
  if (payload.leverage && payload.leverage > 0) orderedPayload.leverage = payload.leverage;
  if (payload.take_profits?.length) orderedPayload.take_profits = payload.take_profits;

  const message = `${timestamp}:${JSON.stringify(orderedPayload)}`;
  const signature = await wallet.signMessage(message);

  const envelope = {
    node_id: wallet.address,
    timestamp,
    payload,
    signature,
  };

  return axios.post(`${API_URL}/api/v1/signals`, {
    node_id: wallet.address,
    envelope,
  });
}

// Subscribe to stream
async function subscribeToStream(wallet: ethers.Wallet, nodeId: string) {
  const timestamp = Math.floor(Date.now() / 1000);
  const message = `access:${nodeId}:${timestamp}`;
  const signature = await wallet.signMessage(message);

  const ws = new WebSocket(
    `ws://${API_URL.replace('http://', '')}/api/v1/stream?` +
    `node_id=${nodeId}&subscriber_address=${wallet.address}&` +
    `signature=${signature}&timestamp=${timestamp}`
  );

  return ws;
}
```

### Python

```python
import time
import json
import requests
from eth_account import Account
from eth_account.messages import encode_defunct

API_URL = "http://localhost:8080"

def submit_signal(private_key: str, payload: dict):
    account = Account.from_key(private_key)
    timestamp = int(time.time())

    # Build ordered payload
    ordered = {"token_pair": payload["token_pair"]}
    if payload.get("exchange"):
        ordered["exchange"] = payload["exchange"]
    ordered["direction"] = payload["direction"]
    ordered["entry_price"] = payload["entry_price"]
    ordered["take_profit"] = payload["take_profit"]
    ordered["stop_loss"] = payload["stop_loss"]
    ordered["expiry_time"] = payload["expiry_time"]
    ordered["weight_pct"] = payload["weight_pct"]
    if payload.get("trade_type"):
        ordered["trade_type"] = payload["trade_type"]
    if payload.get("leverage", 0) > 0:
        ordered["leverage"] = payload["leverage"]
    if payload.get("take_profits"):
        ordered["take_profits"] = payload["take_profits"]

    message = f"{timestamp}:{json.dumps(ordered, separators=(',', ':'))}"
    msg = encode_defunct(text=message)
    signed = account.sign_message(msg)

    envelope = {
        "node_id": account.address,
        "timestamp": timestamp,
        "payload": payload,
        "signature": signed.signature.hex(),
    }

    return requests.post(f"{API_URL}/api/v1/signals", json={
        "node_id": account.address,
        "envelope": envelope,
    })
```

---

## Changelog

### v2.0 (Current)
- Added `trade_type` field: `"spot"`, `"perpetual"`, `"futures"`
- Added `leverage` field: 1-125x for perpetuals
- Added `take_profits` array for multi-level exits
- Added `leveraged_pnl_percent` to webhook payloads
- Added max expiry validation: 48h for leveraged, 168h for spot
- Direction must be lowercase: `"long"` / `"short"`

### v1.0
- Initial release with basic signal structure
I've created a comprehensive API reference document that:

Extracted only API-related content from the frontend docs
Updated all data types with the new v2 fields (trade_type, leverage, take_profits, leveraged_pnl)
Added complete signal submission API documentation with signature generation details
Added webhook registration and payload documentation
Added authentication/subscription documentation with EIP-191 signature format
Added rate limiting documentation
Added SDK examples for Node.js/TypeScript and Python
Added a changelog documenting the v2 changes
The document is now saved at docs/API_REFERENCE.md.

Created API_REFERENCE.md with comprehensive API documentation. Key updates include:

New in v2.0:

trade_type field: "spot", "perpetual", "futures"
leverage field: 1-125x for perpetuals/futures
take_profits array for multi-level exits
leveraged_pnl_percent in webhook payloads
Max expiry validation: 48h for leveraged, 168h for spot
Documentation Sections:

Authentication - EIP-191 signature format with replay protection
Data Types - Updated SignalPayload, SignalEnvelope, NodeStats
Signal Submission API - POST /api/v1/signals with signature generation
Signal Consumption API - Node listing, details, dashboard
Analytics API - Premium endpoints with subscription auth
WebSocket Streams - Real-time signal updates
Webhook Integration - Registration and payload format
Rate Limiting - Per-node (10/min), global (100/min)
Error Handling - All error codes documented
SDK Examples - Node.js/TypeScript and Python
good now analyse this, right now we are updating all nodes stats onchain regularly each time they send a signal, in mainnet it will cost gas, when there are thousands of nodes this will beocme cumbersome, what do you think about this
The user is asking about the on-chain update mechanism for node stats. They're concerned about gas costs when scaling to thousands of nodes in mainnet. Let me analyze the current architecture and propose solutions.

I should look at:

How stats are currently updated on-chain
The broadcaster/contract manager code
The frequency and trigger of on-chain updates
Let me explore the verification layer to understand the current approach.

Good question - this is a critical scaling concern. Let me analyze the current on-chain update mechanism.

Grep "Broadcast|UpdateScore|onchain|OnChain" (in c:\Users\ASUS FX95G\Documents\Ogruns\verification-layer)
9 lines of output
Let me look at the broadcaster to understand how on-chain updates currently work.

Read c:\Users\ASUS FX95G\Documents\Ogruns\verification-layer\internal\broadcaster\broadcaster.go
Read c:\Users\ASUS FX95G\Documents\Ogruns\verification-layer\internal\contracts\manager.go
Now I have a clear picture of the current on-chain update mechanism. Let me analyze the issues and propose solutions.

Current architecture:

RunPeriodicBroadcast runs on a ticker interval
It already has an optimization: only update on-chain when tier CHANGES (not every score update)
Each node update is a separate transaction with ~200k gas limit
Each node = 1 transaction = gas cost
Problems at scale:

1000 nodes with tier changes = 1000 separate transactions
Even with tier-change-only optimization, new nodes or volatile scores = many transactions
No batching - each updateScore is a separate tx
No gas optimization like multicall or batch updates
Single verifier pays all gas costs
Solutions I can propose:

Batch Updates (Multicall): Update multiple nodes in one transaction
Merkle Root Commitment: Store only a Merkle root on-chain, proofs off-chain
Lazy/On-Demand Updates: Only update when someone queries (user pays gas)
Rollup/Layer 2: Move frequent updates to L2, periodic L1 anchor
Optimistic Oracle: Commit hash, allow dispute period
EIP-712 Attestations: Off-chain signed attestations, on-chain verification only when needed
Let me provide a comprehensive analysis.

On-Chain Update Analysis
Current Implementation
Looking at broadcaster.go:139-166:


// RunPeriodicBroadcast - already optimized to only update on TIER CHANGE
for _, stats := range sc.ForBroadcast() {
    prev, known := lastTier[stats.NodeID]
    if known && prev == stats.Tier {
        // No tier change — skip to save gas.
        continue
    }
    b.BroadcastScore(stats.NodeID, stats.TrustScore, stats.Tier)
}
Good news: You already have tier-change-only optimization.

Bad news: Still 1 transaction per node update.

Cost Analysis at Scale
Scenario	Nodes	Tier Changes/Day	Gas/Tx	Total Gas/Day	Cost @ $0.01/gas
Hackathon	50	5	200k	1M	$10
Launch	500	50	200k	10M	$100
Growth	5,000	500	200k	100M	$1,000
Scale	50,000	5,000	200k	1B	$10,000/day
Solutions (Ranked by Impact)
1. Batch Updates Contract (Recommended - Easy Win)
Instead of N transactions, batch all updates into 1 transaction.


// ReputationOracle.sol - Add batch method
function updateScoresBatch(
    address[] calldata nodeIds,
    uint256[] calldata scores,
    uint8[] calldata tiers
) external onlyOracle {
    require(nodeIds.length == scores.length && scores.length == tiers.length);
    for (uint i = 0; i < nodeIds.length; i++) {
        scores[nodeIds[i]] = Score(scores[i], tiers[i], block.timestamp);
        emit ScoreUpdated(nodeIds[i], scores[i], tiers[i]);
    }
}
Gas savings: ~80% (overhead amortized across batch)

Batch Size	Gas	vs Individual
1 node	200k	baseline
10 nodes	400k	80% savings
50 nodes	1.2M	88% savings
100 nodes	2.2M	89% savings
2. Merkle Root Commitment (Best for Scale)
Store only a single hash on-chain. Subscribers verify proofs off-chain.


┌─────────────────────────────────────────────────────────────┐
│                    ON-CHAIN (Cheap)                         │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  ReputationOracle.commitRoot(bytes32 merkleRoot)    │   │
│  │  → 1 tx per epoch (~32k gas)                        │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    OFF-CHAIN (Free)                         │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Verifier publishes Merkle tree to IPFS/0G Storage  │   │
│  │  tree = {nodeA: {score, tier}, nodeB: {score, tier}}│   │
│  └─────────────────────────────────────────────────────┘   │
│                              │                              │
│                              ▼                              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Frontend/Subscriber fetches proof + verifies       │   │
│  │  verifyProof(nodeId, score, tier, proof, root)      │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
Implementation:


// Build Merkle tree of all node scores
type ScoreLeaf struct {
    NodeID     common.Address
    Score      uint64
    Tier       uint8
    UpdatedAt  uint64
}

func (b *Broadcaster) CommitMerkleRoot(scores []ScoreLeaf) error {
    // Build tree
    leaves := make([][]byte, len(scores))
    for i, s := range scores {
        leaves[i] = crypto.Keccak256(encodeLeaf(s))
    }
    root := buildMerkleRoot(leaves)
    
    // Single on-chain tx
    return b.contract.CommitRoot(root)
}
Gas savings: 99%+ (1 tx for ANY number of nodes)

3. Lazy/On-Demand Updates (User Pays Gas)
Only update on-chain when a subscriber needs it for contract interactions.


// SubscriptionManager.sol
function subscribeWithProof(
    address nodeId,
    uint256 score,
    uint8 tier,
    bytes32[] calldata merkleProof
) external payable {
    // Verify proof against committed root
    require(verifyProof(nodeId, score, tier, merkleProof), "Invalid proof");
    
    // Calculate price based on tier
    uint256 price = tierPrices[tier];
    require(msg.value >= price, "Insufficient payment");
    
    // Process subscription...
}
Result: Verifier pays 0 gas. Users pay when they subscribe.

4. Hybrid: Epoch-Based Batching + Merkle
Combine approaches for optimal cost/trust tradeoff.


┌────────────────────────────────────────────────────────────┐
│ EPOCH FLOW (e.g., every 6 hours)                           │
├────────────────────────────────────────────────────────────┤
│ 1. Collect all score changes during epoch                  │
│ 2. Build Merkle tree of ALL node scores                    │
│ 3. Commit Merkle root on-chain (1 tx)                      │
│ 4. Batch update ONLY tier-changed nodes (1 tx)             │
│ 5. Publish full tree to 0G Storage                         │
└────────────────────────────────────────────────────────────┘
Recommendation for Hackathon → Production
Phase	Strategy	On-Chain Txs/Day	Gas Cost
Hackathon	Current (tier-change only)	~50	~$10
Launch	Add batch updates	~5	~$5
Scale	Merkle root + lazy	4 (per epoch)	~$1
Massive	L2 rollup	1 (daily anchor)	~$0.10
Quick Win: Add Batch Updates
Want me to implement the batch update contract modification and Go broadcaster changes? This would give you 80%+ gas savings with minimal code changes:

Add updateScoresBatch() to ReputationOracle.sol
Modify broadcaster to collect changes and batch them
Configure epoch interval (e.g., every 10 minutes)
This is the lowest-effort, highest-impact improvement for your hackathon timeline.