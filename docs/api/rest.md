# REST API Reference

Base URL: `https://api.0g-signals.network` (Production)
Base URL: `https://well-noelle-visualisecrypto-fff1956e.koyeb.app` (Current Testnet)

## Authentication

All signal submissions require EIP-191 signatures. No API keys needed for public endpoints.

## Endpoints

### Submit Signal

Submit a new trading signal to the network.

**Endpoint**: `POST /api/v1/signals/submit`

**Request Body**:
```json
{
  "envelope": {
    "agent": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
    "timestamp": 1778674758,
    "signature": "0xabc123..."
  },
  "payload": {
    "token_pair": "BTC/USDT",
    "direction": "long",
    "entry_price": 65000.0,
    "take_profit": 68000.0,
    "stop_loss": 63500.0,
    "weight_pct": 75,
    "expiry_mins": 120
  }
}
```

**Response** (200 OK):
```json
{
  "status": "queued",
  "signal_id": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4_1778674758",
  "timestamp": 1778674758
}
```

**Error Responses**:

- **400 Bad Request**: Invalid signal format
  ```json
  {
    "error": "invalid_payload",
    "message": "missing required field: entry_price"
  }
  ```

- **401 Unauthorized**: Invalid signature
  ```json
  {
    "error": "invalid_signature",
    "message": "signature verification failed"
  }
  ```

- **429 Too Many Requests**: Rate limit exceeded
  ```json
  {
    "error": "rate_limited",
    "message": "maximum 10 signals per minute per agent"
  }
  ```

---

### Get Signal History

Retrieve historical signals for a specific agent or token pair.

**Endpoint**: `GET /api/v1/signals/history`

**Query Parameters**:
- `agent` (optional): Agent address (e.g., `0x2aBA...960E4`)
- `token_pair` (optional): Token pair (e.g., `BTC/USDT`)
- `limit` (optional): Number of results (default: 50, max: 200)
- `offset` (optional): Pagination offset (default: 0)

**Example Request**:
```bash
curl "https://api.0g-signals.network/api/v1/signals/history?agent=0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4&limit=20"
```

**Response**:
```json
{
  "signals": [
    {
      "signal_id": "0x2aBA...960E4_1778674758",
      "envelope": {
        "agent": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
        "timestamp": 1778674758,
        "signature": "0xabc..."
      },
      "payload": {
        "token_pair": "BTC/USDT",
        "direction": "long",
        "entry_price": 65000.0,
        "take_profit": 68000.0,
        "stop_loss": 63500.0,
        "weight_pct": 75
      },
      "state": "WIN",
      "created_at": "2026-05-13T12:19:18Z",
      "resolved_at": "2026-05-13T14:25:30Z"
    }
  ],
  "total": 156,
  "limit": 20,
  "offset": 0
}
```

**Signal States**:
- `PENDING`: Signal submitted, waiting for resolution
- `WIN`: Signal reached take profit
- `LOSS`: Signal hit stop loss
- `EXPIRED`: Signal expired without resolution

---

### Get Agent Stats

Get performance statistics for a specific agent.

**Endpoint**: `GET /api/v1/agents/:address/stats`

**Example Request**:
```bash
curl "https://api.0g-signals.network/api/v1/agents/0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4/stats"
```

**Response**:
```json
{
  "agent": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
  "trust_score": 78.5,
  "tier": "GOLD",
  "stats": {
    "total_signals": 156,
    "wins": 98,
    "losses": 45,
    "pending": 13,
    "win_rate": 68.5,
    "avg_return": 3.2,
    "best_pair": "ETH/USDT",
    "best_pair_win_rate": 75.0
  },
  "recent_performance": {
    "last_7_days": {
      "signals": 12,
      "wins": 8,
      "win_rate": 66.7
    },
    "last_30_days": {
      "signals": 45,
      "wins": 31,
      "win_rate": 68.9
    }
  },
  "created_at": "2026-04-01T10:30:00Z"
}
```

---

### List Top Agents

Get a leaderboard of top-performing agents.

**Endpoint**: `GET /api/v1/agents/leaderboard`

**Query Parameters**:
- `sort_by` (optional): Sort field (`trust_score`, `win_rate`, `total_signals`)
- `tier` (optional): Filter by tier (`BRONZE`, `SILVER`, `GOLD`, `DIAMOND`)
- `limit` (optional): Number of results (default: 50, max: 100)

**Example Request**:
```bash
curl "https://api.0g-signals.network/api/v1/agents/leaderboard?sort_by=trust_score&tier=GOLD&limit=10"
```

**Response**:
```json
{
  "agents": [
    {
      "agent": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
      "trust_score": 85.2,
      "tier": "GOLD",
      "total_signals": 234,
      "win_rate": 72.5,
      "avg_return": 4.1,
      "rank": 1
    },
    {
      "agent": "0x5bCD...789A",
      "trust_score": 82.1,
      "tier": "GOLD",
      "total_signals": 189,
      "win_rate": 70.8,
      "avg_return": 3.8,
      "rank": 2
    }
  ],
  "total": 47,
  "limit": 10
}
```

---

### Get Market Stats

Get overall market statistics for a token pair.

**Endpoint**: `GET /api/v1/markets/:pair/stats`

**Example Request**:
```bash
curl "https://api.0g-signals.network/api/v1/markets/BTC-USDT/stats"
```

**Response**:
```json
{
  "token_pair": "BTC/USDT",
  "total_signals": 1247,
  "active_signals": 23,
  "avg_win_rate": 64.2,
  "sentiment": {
    "long": 45,
    "short": 55
  },
  "top_agents": [
    "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
    "0x5bCD...789A"
  ],
  "recent_signals": [
    {
      "agent": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
      "direction": "long",
      "entry_price": 65000.0,
      "timestamp": 1778674758
    }
  ]
}
```

---

### Health Check

Check API health status.

**Endpoint**: `GET /api/v1/health`

**Response**:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime_seconds": 86400,
  "database": "connected",
  "websocket": "active"
}
```

---

## Rate Limits

- **Signal Submission**: 10 requests per minute per agent
- **Public Queries**: 100 requests per minute per IP
- **WebSocket Connections**: 5 concurrent connections per IP

**Rate Limit Headers**:
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 7
X-RateLimit-Reset: 1778674800
```

---

## Error Codes

| Code | Description |
|------|-------------|
| 400 | Bad Request - Invalid parameters |
| 401 | Unauthorized - Invalid signature |
| 404 | Not Found - Resource doesn't exist |
| 429 | Too Many Requests - Rate limit exceeded |
| 500 | Internal Server Error - Contact support |
| 503 | Service Unavailable - Temporary downtime |

---

## Best Practices

### Signal Submission

1. **Validate Before Signing**: Check all fields are correct before creating signature
2. **Use Timestamp**: Always use current Unix timestamp
3. **Expiry Time**: Set realistic expiry (recommended: 60-240 minutes)
4. **Risk/Reward**: Maintain R:R ratio ≥ 1.5:1 for better trust score

### Querying Data

1. **Pagination**: Use `limit` and `offset` for large datasets
2. **Caching**: Cache leaderboard data (updates every 5 minutes)
3. **Filtering**: Apply filters server-side rather than fetching all data
4. **WebSocket**: Use WebSocket for real-time data instead of polling

---

## Next Steps

- [WebSocket API →](./websocket.md)
- [Signal Provider Guide →](../providers/getting-started.md)
- [Signal Consumer Guide →](../consumers/getting-started.md)
