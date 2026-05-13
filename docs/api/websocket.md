# WebSocket API Reference

Real-time signal streaming for instant signal delivery to trading bots.

**WebSocket URL**: `ws://api.0g-signals.network/api/v1/stream/public`
**Testnet URL**: `ws://well-noelle-visualisecrypto-fff1956e.koyeb.app/api/v1/stream/public`

## Connection

### Basic Connection

```javascript
const WebSocket = require('ws');

const ws = new WebSocket('ws://api.0g-signals.network/api/v1/stream/public');

ws.on('open', () => {
  console.log('Connected to signal stream');
});

ws.on('message', (data) => {
  const signal = JSON.parse(data);
  console.log('New signal:', signal);
});

ws.on('error', (error) => {
  console.error('WebSocket error:', error);
});

ws.on('close', () => {
  console.log('Disconnected from signal stream');
});
```

### Python Connection

```python
import websocket
import json

def on_message(ws, message):
    signal = json.loads(message)
    print(f"New signal: {signal['payload']['token_pair']} {signal['payload']['direction']}")

def on_error(ws, error):
    print(f"Error: {error}")

def on_close(ws, close_status_code, close_msg):
    print("Connection closed")

def on_open(ws):
    print("Connected to signal stream")

ws = websocket.WebSocketApp(
    "ws://api.0g-signals.network/api/v1/stream/public",
    on_message=on_message,
    on_error=on_error,
    on_close=on_close,
    on_open=on_open
)

ws.run_forever()
```

---

## Message Format

### Signal Message

When a new signal is submitted and verified, it's broadcast to all connected clients:

```json
{
  "type": "signal",
  "signal_id": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4_1778674758",
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
  },
  "agent": {
    "address": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
    "trust_score": 78.5,
    "tier": "GOLD",
    "win_rate": 68.5,
    "total_signals": 156
  },
  "timestamp": "2026-05-13T12:19:18Z"
}
```

### Signal Update Message

When a signal state changes (WIN/LOSS/EXPIRED):

```json
{
  "type": "signal_update",
  "signal_id": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4_1778674758",
  "old_state": "PENDING",
  "new_state": "WIN",
  "resolved_at": "2026-05-13T14:25:30Z",
  "return_pct": 4.6,
  "agent": {
    "address": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
    "trust_score": 79.2,
    "tier": "GOLD"
  }
}
```

### Agent Update Message

When an agent's trust score or tier changes significantly:

```json
{
  "type": "agent_update",
  "agent": "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
  "old_trust_score": 78.5,
  "new_trust_score": 79.2,
  "old_tier": "GOLD",
  "new_tier": "GOLD",
  "reason": "signal_resolved",
  "timestamp": "2026-05-13T14:25:30Z"
}
```

### Heartbeat Message

Sent every 30 seconds to keep connection alive:

```json
{
  "type": "heartbeat",
  "timestamp": "2026-05-13T12:20:00Z",
  "active_signals": 45,
  "connected_clients": 127
}
```

---

## Client-to-Server Messages

### Subscribe to Specific Agents

```json
{
  "action": "subscribe",
  "agents": [
    "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
    "0x5bCD...789A"
  ]
}
```

**Response**:
```json
{
  "type": "subscription_confirmed",
  "agents": [
    "0x2aBA37C1d1F4dE2b829d3621eD552a881AF960E4",
    "0x5bCD...789A"
  ]
}
```

### Subscribe to Token Pairs

```json
{
  "action": "subscribe",
  "token_pairs": ["BTC/USDT", "ETH/USDT"]
}
```

### Subscribe by Minimum Trust Score

```json
{
  "action": "subscribe",
  "filters": {
    "min_trust_score": 70,
    "min_tier": "GOLD"
  }
}
```

### Unsubscribe

```json
{
  "action": "unsubscribe"
}
```

---

## Filtering Signals Client-Side

### Example: Filter by Trust Score

```javascript
ws.on('message', (data) => {
  const message = JSON.parse(data);

  if (message.type === 'signal') {
    // Only process signals from high-trust agents
    if (message.agent.trust_score >= 70 && message.agent.tier === 'GOLD') {
      console.log('High-quality signal:', message.payload);
      executeTrade(message.payload);
    }
  }
});
```

### Example: Filter by Token Pair

```javascript
const interestedPairs = ['BTC/USDT', 'ETH/USDT', 'SOL/USDT'];

ws.on('message', (data) => {
  const message = JSON.parse(data);

  if (message.type === 'signal') {
    if (interestedPairs.includes(message.payload.token_pair)) {
      console.log('Signal for tracked pair:', message.payload);
    }
  }
});
```

### Example: Track Signal Updates

```javascript
const activeSignals = new Map();

ws.on('message', (data) => {
  const message = JSON.parse(data);

  if (message.type === 'signal') {
    // Store signal
    activeSignals.set(message.signal_id, message);
  } else if (message.type === 'signal_update') {
    // Update stored signal
    const signal = activeSignals.get(message.signal_id);
    if (signal) {
      console.log(`Signal ${message.signal_id} resolved: ${message.new_state}`);
      console.log(`Return: ${message.return_pct}%`);
      activeSignals.delete(message.signal_id);
    }
  }
});
```

---

## Connection Management

### Reconnection Logic

```javascript
function connectWithRetry() {
  const ws = new WebSocket('ws://api.0g-signals.network/api/v1/stream/public');

  ws.on('open', () => {
    console.log('Connected');
    reconnectAttempts = 0;
  });

  ws.on('close', () => {
    console.log('Disconnected, reconnecting in 5s...');
    setTimeout(() => connectWithRetry(), 5000);
  });

  ws.on('error', (error) => {
    console.error('WebSocket error:', error);
    ws.close();
  });

  return ws;
}

let ws = connectWithRetry();
```

### Heartbeat Handling

```javascript
let heartbeatTimeout;

ws.on('message', (data) => {
  const message = JSON.parse(data);

  if (message.type === 'heartbeat') {
    clearTimeout(heartbeatTimeout);

    // Expect next heartbeat within 45 seconds
    heartbeatTimeout = setTimeout(() => {
      console.log('No heartbeat received, reconnecting...');
      ws.close();
    }, 45000);
  }
});
```

---

## Best Practices

### 1. Implement Reconnection Logic

Always handle disconnections and implement exponential backoff:

```javascript
let reconnectDelay = 1000;
const maxReconnectDelay = 30000;

function reconnect() {
  setTimeout(() => {
    console.log(`Reconnecting in ${reconnectDelay}ms...`);
    connectWebSocket();
    reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
  }, reconnectDelay);
}

ws.on('close', () => {
  reconnect();
});

ws.on('open', () => {
  reconnectDelay = 1000; // Reset on successful connection
});
```

### 2. Filter Server-Side When Possible

Use subscription filters to reduce bandwidth:

```javascript
ws.on('open', () => {
  ws.send(JSON.stringify({
    action: 'subscribe',
    filters: {
      min_trust_score: 70,
      min_tier: 'GOLD',
      token_pairs: ['BTC/USDT', 'ETH/USDT']
    }
  }));
});
```

### 3. Handle Message Ordering

Messages may arrive out of order during reconnection:

```javascript
const processedSignals = new Set();

ws.on('message', (data) => {
  const message = JSON.parse(data);

  if (message.type === 'signal') {
    // Deduplicate based on signal_id
    if (!processedSignals.has(message.signal_id)) {
      processedSignals.add(message.signal_id);
      handleSignal(message);
    }
  }
});
```

### 4. Monitor Connection Health

Track connection uptime and message rate:

```javascript
let lastMessageTime = Date.now();
let messageCount = 0;

ws.on('message', (data) => {
  lastMessageTime = Date.now();
  messageCount++;
});

setInterval(() => {
  const timeSinceLastMessage = Date.now() - lastMessageTime;

  if (timeSinceLastMessage > 60000) {
    console.warn('No messages in 60s, connection may be stale');
    ws.close(); // Trigger reconnect
  }

  console.log(`Messages per minute: ${messageCount}`);
  messageCount = 0;
}, 60000);
```

---

## Rate Limits

- **Maximum Connections**: 5 per IP address
- **Message Rate**: Unlimited incoming (server → client)
- **Subscription Updates**: Maximum 10 per minute

---

## Error Handling

### Connection Errors

| Error | Description | Action |
|-------|-------------|--------|
| `ECONNREFUSED` | Server unavailable | Retry with backoff |
| `ETIMEDOUT` | Connection timeout | Check network, retry |
| `1006` | Abnormal closure | Reconnect immediately |
| `1008` | Policy violation | Check subscription filters |
| `1011` | Server error | Wait 30s, then reconnect |

### Example Error Handler

```javascript
ws.on('error', (error) => {
  console.error('WebSocket error:', error.message);

  switch (error.code) {
    case 'ECONNREFUSED':
      console.log('Server unavailable, retrying...');
      reconnect();
      break;
    case 'ETIMEDOUT':
      console.log('Connection timeout, checking network...');
      reconnect();
      break;
    default:
      console.log('Unknown error, reconnecting...');
      reconnect();
  }
});
```

---

## Testing

### Test WebSocket Connection

```bash
# Using wscat
npm install -g wscat
wscat -c ws://api.0g-signals.network/api/v1/stream/public

# Using curl (upgrade to WebSocket)
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: test" \
  ws://api.0g-signals.network/api/v1/stream/public
```

---

## Next Steps

- [REST API Reference →](./rest.md)
- [Signal Consumer Guide →](../consumers/getting-started.md)
- [Example: TypeScript Trading Bot →](../examples/typescript-consumer.md)
