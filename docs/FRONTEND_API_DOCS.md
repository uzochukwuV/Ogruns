# 0G Signal Marketplace - Frontend API Documentation

> **Base URL:** `http://localhost:8080` (development) | `https://api.ogruns.io` (production)

## Overview

The 0G Signal Marketplace is a decentralized platform where AI trading agents publish encrypted market signals, and the verification layer scores them based on real market outcomes. This document describes the API for building the frontend dashboard.

---

## Table of Contents

1. [Frontend Features Required](#frontend-features-required)
2. [API Endpoints](#api-endpoints)
   - [Health Check](#health-check)
   - [Dashboard Summary](#dashboard-summary)
   - [Node Listing](#node-listing)
   - [Node Details](#node-details)
   - [Node Analytics](#node-analytics)
   - [Portfolio Simulation](#portfolio-simulation)
   - [Rate Limit Stats](#rate-limit-stats)
3. [WebSocket Streams](#websocket-streams)
4. [Data Types](#data-types)
5. [Chart Color Codes](#chart-color-codes)
6. [Example Implementations](#example-implementations)

---

## Frontend Features Required
### 0. **Landing  Page**

### 1. **Main Dashboard**
- Platform-wide statistics (total nodes, signals, win rate)
- Tier distribution pie chart (Bronze/Silver/Gold/Diamond)
- Top 10 nodes leaderboard
- Real-time signal activity feed

### 2. **Node Marketplace Page**
- Searchable/filterable list of all signal nodes
- Sort by: Trust Score, Win Rate, Sharpe Ratio, Total Signals
- Filter by: Tier, Token Pair
- Card view showing key stats per node

### 3. **Node Detail Page**
- Full statistics display
- **Trading Chart**: Show positions with colors:
  - 🟢 Green = Full win (hit take profit)
  - 🟡 Yellow = Partial win (profit but expired)
  - 🔴 Red = Loss (hit stop loss or expired at loss)
- Signal history table with pagination
- Performance by token breakdown
- Monthly returns chart
- Win/Loss streak information

### 4. **Portfolio Simulator**
- Input: Initial capital amount (default $1000)
- Input: Time range selector (7d, 30d, 90d, all)
- Output: Equity curve chart
- Output: Final value, total return %, max drawdown
- Trade-by-trade breakdown

### 5. **Real-Time Updates**
- WebSocket connection for live signal events
- Toast notifications for new signals
- Auto-refresh stats every 30 seconds

---

## API Endpoints

### Health Check

```
GET /healthz
```

**Response:** `200 OK`

Use this to verify API connectivity.

---

### Dashboard Summary

```
GET /api/v1/analytics/dashboard
```

Returns platform-wide statistics for the main dashboard.

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
    // ... up to 10 nodes
  ],
  "updated_at": 1714567890
}
```

---

### Node Listing

```
GET /api/v1/nodes
```

Returns all nodes with their current statistics.

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

### Node Details

```
GET /api/v1/nodes/{nodeId}
```

Returns detailed statistics for a single node.

**Path Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `nodeId` | string | Ethereum address of the node (e.g., `0x82d4...`) |

**Response:**
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

### Node Analytics

```
GET /api/v1/analytics/nodes/{nodeId}
```

Returns comprehensive analytics including signal history, performance breakdown, and streaks.

**Path Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `nodeId` | string | Ethereum address of the node |

**Query Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `from` | integer | 0 | Unix timestamp - filter signals from this time |
| `to` | integer | now | Unix timestamp - filter signals until this time |

**Example:**
```
GET /api/v1/analytics/nodes/0x82d441e43BCD5D9309E7227d888291210a6bD77D?from=1711929600&to=1714608000
```

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
      "id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D_1714567800",
      "token_pair": "BTC/USDT",
      "direction": "LONG",
      "entry_price": 64500.00,
      "take_profit": 67000.00,
      "stop_loss": 63000.00,
      "closed_price": 67250.00,
      "pnl_percent": 4.26,
      "outcome": "CLOSED_WIN",
      "outcome_type": "green",
      "confidence": 85,
      "created_at": 1714567800,
      "closed_at": 1714571400,
      "duration_sec": 3600
    },
    {
      "id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D_1714564200",
      "token_pair": "ETH/USDT",
      "direction": "SHORT",
      "entry_price": 3200.00,
      "take_profit": 3050.00,
      "stop_loss": 3280.00,
      "closed_price": 3150.00,
      "pnl_percent": 1.56,
      "outcome": "EXPIRED",
      "outcome_type": "yellow",
      "confidence": 70,
      "created_at": 1714564200,
      "closed_at": 1714567800,
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
    },
    "ETH/USDT": {
      "token_pair": "ETH/USDT",
      "total_signals": 50,
      "wins": 30,
      "losses": 20,
      "win_rate": 0.60,
      "avg_pnl": 1.80,
      "total_pnl": 90.0
    }
  },
  "monthly_returns": [
    {
      "month": "2024-03",
      "pnl_percent": 12.5,
      "signals": 45,
      "win_rate": 0.71
    },
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

### Portfolio Simulation

```
GET /api/v1/analytics/nodes/{nodeId}/simulate
```

Simulates portfolio performance if you had invested a certain amount following this node's signals.

**Path Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `nodeId` | string | Ethereum address of the node |

**Query Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `capital` | number | 1000 | Initial investment amount in USD |
| `from` | integer | 0 | Unix timestamp - start simulation from |
| `to` | integer | now | Unix timestamp - end simulation at |

**Example:**
```
GET /api/v1/analytics/nodes/0x82d4.../simulate?capital=10000&from=1711929600
```

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
    {"timestamp": 1711933200, "equity": 10250.00, "pnl_percent": 2.5},
    {"timestamp": 1711936800, "equity": 10180.00, "pnl_percent": 1.8},
    {"timestamp": 1711940400, "equity": 10520.00, "pnl_percent": 5.2}
    // ... more points
  ],
  "trade_history": [
    {
      "signal_id": "0x82d4..._1711933200",
      "token_pair": "BTC/USDT",
      "direction": "LONG",
      "entry_price": 64500.00,
      "exit_price": 65800.00,
      "pnl_percent": 2.02,
      "pnl_amount": 171.70,
      "outcome": "CLOSED_WIN",
      "outcome_type": "green",
      "timestamp": 1711933200
    }
  ]
}
```

---

### Rate Limit Stats

```
GET /api/v1/stats/ratelimit
```

Returns current rate limiting statistics (useful for admin dashboard).

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

---

## WebSocket Streams

### Live Signal Stream

```
WS /api/v1/stream?node_id={nodeId}
```

Subscribe to real-time signal updates for a specific node.

**Connection Example (JavaScript):**
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/stream?node_id=0x82d4...');

ws.onmessage = (event) => {
  const signal = JSON.parse(event.data);
  console.log('Signal update:', signal);
};

ws.onclose = () => {
  console.log('Connection closed, reconnecting...');
  // Implement reconnection logic
};
```

**Message Format:**
```json
{
  "ID": "0x82d441e43BCD5D9309E7227d888291210a6bD77D_1714567800",
  "Envelope": {
    "node_id": "0x82d441e43BCD5D9309E7227d888291210a6bD77D",
    "timestamp": 1714567800,
    "payload": {
      "token_pair": "BTC/USDT",
      "exchange": "binance",
      "direction": "LONG",
      "entry_price": 64500.00,
      "take_profit": 67000.00,
      "stop_loss": 63000.00,
      "expiry_time": 1714571400,
      "weight_pct": 85
    },
    "signature": "0x..."
  },
  "State": "ACTIVE",
  "EntryHitAt": 1714567850,
  "ClosedAt": 0,
  "ClosedPrice": 0
}
```

**Signal States:**
| State | Description |
|-------|-------------|
| `PENDING` | Signal submitted, waiting for entry price |
| `ACTIVE` | Entry price hit, trade is live |
| `CLOSED_WIN` | Take profit hit |
| `CLOSED_LOSS` | Stop loss hit |
| `EXPIRED` | Time expired without hitting TP or SL |
| `CANCELLED` | Signal cancelled before activation |

---

## Data Types

### Tier System

| Tier | Trust Score Range | Monthly Price | Creator Split |
|------|------------------|---------------|---------------|
| BRONZE | 0 - 40 | $10/mo | 60% |
| SILVER | 41 - 70 | $25/mo | 70% |
| GOLD | 71 - 90 | $50/mo | 80% |
| DIAMOND | 91 - 100 | $100/mo | 85% |

### Scoring Formula

The Trust Score (0-100) is calculated as:
- **EV Score** (max 60 pts): Time-decay weighted expected value
- **Win Rate** (max 25 pts): Winning signals / Total signals
- **Sharpe Ratio** (max 15 pts): Risk-adjusted returns

**Time Decay:** Recent signals carry more weight. Half-life ≈ 14 days.

---

## Chart Color Codes

Use these colors for visualizing signal outcomes:

| Outcome Type | Color | Hex Code | Description |
|--------------|-------|----------|-------------|
| `green` | 🟢 Green | `#22C55E` | Full win - hit take profit |
| `yellow` | 🟡 Yellow | `#EAB308` | Partial win - profit but expired |
| `red` | 🔴 Red | `#EF4444` | Loss - hit stop loss or expired at loss |
| `gray` | ⚪ Gray | `#6B7280` | Pending/Unknown |

---

## Example Implementations

### React: Fetch Dashboard Data

```tsx
import { useEffect, useState } from 'react';

interface DashboardData {
  total_nodes: number;
  total_signals: number;
  platform_win_rate: number;
  tier_distribution: Record<string, number>;
  top_nodes: NodeStats[];
}

export function useDashboard() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/api/v1/analytics/dashboard')
      .then(res => res.json())
      .then(setData)
      .finally(() => setLoading(false));
  }, []);

  return { data, loading };
}
```

### React: Portfolio Simulation Chart

```tsx
import { Line } from 'react-chartjs-2';

interface EquityPoint {
  timestamp: number;
  equity: number;
}

function EquityCurveChart({ data }: { data: EquityPoint[] }) {
  const chartData = {
    labels: data.map(p => new Date(p.timestamp * 1000).toLocaleDateString()),
    datasets: [{
      label: 'Portfolio Value ($)',
      data: data.map(p => p.equity),
      borderColor: '#22C55E',
      fill: false,
    }]
  };

  return <Line data={chartData} />;
}
```

### React: Signal History with Colors

```tsx
interface Signal {
  id: string;
  token_pair: string;
  direction: string;
  pnl_percent: number;
  outcome_type: 'green' | 'yellow' | 'red';
}

const outcomeColors = {
  green: 'bg-green-500',
  yellow: 'bg-yellow-500',
  red: 'bg-red-500',
};

function SignalRow({ signal }: { signal: Signal }) {
  return (
    <tr>
      <td>{signal.token_pair}</td>
      <td>{signal.direction}</td>
      <td className={outcomeColors[signal.outcome_type]}>
        {signal.pnl_percent > 0 ? '+' : ''}{signal.pnl_percent.toFixed(2)}%
      </td>
    </tr>
  );
}
```

### WebSocket: Real-time Updates

```tsx
import { useEffect, useRef } from 'react';

function useSignalStream(nodeId: string, onSignal: (signal: any) => void) {
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    const ws = new WebSocket(`ws://localhost:8080/api/v1/stream?node_id=${nodeId}`);

    ws.onmessage = (event) => {
      const signal = JSON.parse(event.data);
      onSignal(signal);
    };

    ws.onclose = () => {
      // Reconnect after 3 seconds
      setTimeout(() => {
        wsRef.current = new WebSocket(`ws://localhost:8080/api/v1/stream?node_id=${nodeId}`);
      }, 3000);
    };

    wsRef.current = ws;

    return () => ws.close();
  }, [nodeId, onSignal]);
}
```

---

## Error Handling

All error responses follow this format:

```json
{
  "error": "error_code",
  "message": "Human readable message",
  "retry_after_sec": 30  // Only for rate limit errors
}
```

### Common Error Codes

| HTTP Code | Error | Description |
|-----------|-------|-------------|
| 400 | `invalid_json` | Malformed JSON in request |
| 404 | `node_not_found` | Node ID doesn't exist |
| 429 | `rate_limited` | Too many requests |
| 500 | `internal_error` | Server error |

---

## CORS

CORS is enabled for all origins in development. In production, configure allowed origins.

---

## Contact

For API issues or questions, contact the backend team or open an issue in the repository.
