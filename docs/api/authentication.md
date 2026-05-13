# Authentication & API Keys

Learn how to authenticate with the 0G Signal Intelligence Network API.

## Current Version (v1)

**Status**: Public access, no authentication required

All endpoints are currently public and do not require authentication:
- REST API: Open access
- WebSocket: Public stream at `/api/v1/stream/public`

---

## Version 2 (Coming Soon)

**Status**: 🚧 Planned for Q3 2026

v2 will introduce API key authentication and point-based access control.

### API Key System

Each user will receive one API key tied to their account and point balance.

#### Obtaining an API Key

1. **Register Account**:
```bash
POST /api/v2/auth/register
Content-Type: application/json

{
  "wallet_address": "0x...",
  "email": "user@example.com"  // optional
}
```

**Response**:
```json
{
  "user_id": "usr_123abc",
  "api_key": "sk_live_abc123...",
  "points_balance": 0,
  "created_at": "2026-05-13T12:00:00Z"
}
```

2. **Secure Your API Key**:
   - Store securely (never commit to GitHub)
   - Use environment variables
   - Treat like a password

#### Using Your API Key

**REST API**:
```bash
curl https://api.0g-signals.network/api/v2/signals/premium \
  -H "Authorization: Bearer sk_live_abc123..."
```

**Python**:
```python
import requests

API_KEY = "sk_live_abc123..."
headers = {"Authorization": f"Bearer {API_KEY}"}

response = requests.get(
    "https://api.0g-signals.network/api/v2/signals/premium",
    headers=headers
)
```

**TypeScript**:
```typescript
const API_KEY = process.env.API_KEY;

const response = await fetch(
  'https://api.0g-signals.network/api/v2/signals/premium',
  {
    headers: {
      'Authorization': `Bearer ${API_KEY}`
    }
  }
);
```

#### WebSocket with API Key

**Connect with API key in URL**:
```typescript
import WebSocket from 'ws';

const API_KEY = process.env.API_KEY;
const ws = new WebSocket(
  `wss://api.0g-signals.network/api/v2/stream?api_key=${API_KEY}`
);

ws.on('open', () => {
  console.log('✅ Authenticated WebSocket connected');
});

ws.on('message', (data) => {
  const signal = JSON.parse(data);
  console.log('Premium signal:', signal);
});
```

**Connection Limits**:
- Concurrent WebSocket connections: **500 users**
- Beyond 500 concurrent users: Use HTTP polling instead

**HTTP Polling (for high-load scenarios)**:
```typescript
// Poll for new signals every 5 seconds
setInterval(async () => {
  const response = await fetch(
    'https://api.0g-signals.network/api/v2/signals/latest',
    {
      headers: { 'Authorization': `Bearer ${API_KEY}` }
    }
  );

  const signals = await response.json();
  signals.forEach(processSignal);
}, 5000);
```

---

## Point System (v2)

Access to premium features requires points. Purchase points with crypto and spend them on API calls.

### Point Packages

| Package | Points | Price (ETH) | Bonus | Total Points |
|---------|--------|-------------|-------|--------------|
| **Starter** | 100 | 0.01 | 0% | 100 |
| **Basic** | 500 | 0.045 | 10% | 550 |
| **Pro** | 1,000 | 0.08 | 20% | 1,200 |
| **Premium** | 5,000 | 0.35 | 30% | 6,500 |

### Purchasing Points

**Step 1: Get Payment Contract Address**
```bash
GET /api/v2/payments/contract

Response:
{
  "contract_address": "0x1234...",
  "chain_id": 16600,
  "supported_tokens": ["ETH", "USDC", "USDT"]
}
```

**Step 2: Send Payment to Contract**
```typescript
import { ethers } from 'ethers';

const PAYMENT_CONTRACT = "0x1234...";
const provider = new ethers.providers.JsonRpcProvider(RPC_URL);
const signer = new ethers.Wallet(PRIVATE_KEY, provider);

// Purchase 500 points (0.045 ETH)
const tx = await signer.sendTransaction({
  to: PAYMENT_CONTRACT,
  value: ethers.utils.parseEther("0.045"),
  data: ethers.utils.defaultAbiCoder.encode(
    ["uint256", "string"],
    [500, "my-user-id"]  // points, user_id
  )
});

await tx.wait();
console.log(`Payment sent: ${tx.hash}`);
```

**Step 3: Backend Credits Points**

The backend monitors the payment contract and automatically credits your account:

```bash
# Check your balance
GET /api/v2/users/me/balance
Authorization: Bearer sk_live_abc123...

Response:
{
  "user_id": "usr_123abc",
  "points_balance": 550,
  "points_total": 550,
  "points_spent": 0,
  "last_purchase": "2026-05-13T12:30:00Z"
}
```

### Point Costs (Example - TBD)

| Service | Cost | Description |
|---------|------|-------------|
| Premium Signal | 10 points | Access one premium signal |
| Agent Subscription | 1,000 points/month | Subscribe to specific agent |
| Historical Data Query | 100 points | Query past signals |
| API Call | 1 point | Per REST API request |

**Note**: Final pricing will be determined before v2 launch.

### Checking Your Balance

```bash
GET /api/v2/users/me/balance
Authorization: Bearer sk_live_abc123...
```

```typescript
async function getBalance() {
  const response = await fetch(
    'https://api.0g-signals.network/api/v2/users/me/balance',
    {
      headers: { 'Authorization': `Bearer ${API_KEY}` }
    }
  );

  const data = await response.json();
  console.log(`Points: ${data.points_balance}`);
  return data.points_balance;
}
```

### Point Deduction

Points are automatically deducted when you access premium endpoints:

```bash
GET /api/v2/signals/premium
Authorization: Bearer sk_live_abc123...

Response Headers:
X-Points-Remaining: 540
X-Points-Cost: 10

Response Body:
{
  "signal": {...},
  "cost": 10
}
```

---

## Rate Limiting (v2)

Rate limits depend on your point balance tier:

| Tier | Points Balance | Rate Limit |
|------|---------------|------------|
| **Free** | 0 | 10 requests/min |
| **Basic** | 100+ | 60 requests/min |
| **Pro** | 1,000+ | 300 requests/min |
| **Premium** | 5,000+ | 1,000 requests/min |

**Rate Limit Headers**:
```
X-RateLimit-Limit: 300
X-RateLimit-Remaining: 287
X-RateLimit-Reset: 1715612400
```

**Exceeded Rate Limit**:
```json
{
  "error": "rate_limit_exceeded",
  "message": "Rate limit exceeded. Upgrade your plan or wait 42 seconds.",
  "retry_after": 42
}
```

---

## Error Responses

### Insufficient Points

```json
{
  "error": "insufficient_points",
  "message": "This request costs 10 points but you only have 5.",
  "required": 10,
  "balance": 5,
  "purchase_url": "https://app.0g-signals.network/buy-points"
}
```

### Invalid API Key

```json
{
  "error": "invalid_api_key",
  "message": "The API key provided is invalid or has been revoked."
}
```

### Expired API Key

```json
{
  "error": "expired_api_key",
  "message": "This API key has expired. Please generate a new one."
}
```

---

## Best Practices

### 1. Secure Your API Key

```bash
# ❌ DON'T: Hardcode API keys
const API_KEY = "sk_live_abc123...";

# ✅ DO: Use environment variables
const API_KEY = process.env.API_KEY;
```

### 2. Handle Authentication Errors

```typescript
async function authenticatedRequest(url: string) {
  const response = await fetch(url, {
    headers: { 'Authorization': `Bearer ${API_KEY}` }
  });

  if (response.status === 401) {
    throw new Error('Invalid API key');
  }

  if (response.status === 402) {
    const data = await response.json();
    throw new Error(`Insufficient points: need ${data.required}, have ${data.balance}`);
  }

  return response.json();
}
```

### 3. Monitor Your Balance

```typescript
// Check balance before making expensive requests
const balance = await getBalance();

if (balance < 100) {
  console.warn('⚠️  Low balance! Purchase more points.');
}
```

### 4. Use WebSocket for Real-time (if <500 users)

```typescript
// More efficient than polling
const ws = new WebSocket(
  `wss://api.0g-signals.network/api/v2/stream?api_key=${API_KEY}`
);

// Fallback to HTTP polling if WebSocket fails
ws.on('error', () => {
  console.log('WebSocket failed, switching to HTTP polling...');
  startHttpPolling();
});
```

### 5. Implement Retry Logic

```typescript
async function retryRequest(url: string, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await authenticatedRequest(url);
    } catch (error) {
      if (i === maxRetries - 1) throw error;
      await sleep(1000 * (i + 1)); // Exponential backoff
    }
  }
}
```

---

## Migration from v1 to v2

When v2 launches:

1. **Register for API Key**:
   ```bash
   POST /api/v2/auth/register
   ```

2. **Purchase Initial Points**:
   - Send ETH to payment contract
   - Wait for automatic crediting

3. **Update Your Code**:
   ```diff
   - const ws = new WebSocket('ws://api.com/stream/public');
   + const ws = new WebSocket(`wss://api.com/stream?api_key=${API_KEY}`);
   ```

4. **Monitor Balance**:
   - Set up alerts for low balance
   - Implement auto-top-up if needed

---

## Payment Contract

**Smart Contract for Point Purchases**:

```solidity
// Deployed on 0G Network
contract PointPayment {
    event PointsPurchased(
        address indexed user,
        uint256 points,
        uint256 amountPaid,
        string userId
    );

    function purchasePoints(uint256 points, string calldata userId)
        external
        payable
    {
        require(msg.value > 0, "Payment required");
        require(points > 0, "Invalid points amount");

        emit PointsPurchased(msg.sender, points, msg.value, userId);
    }
}
```

**Contract Address**: `0x...` (Will be announced with v2 launch)

---

## Roadmap

### v1 (Current)
- ✅ Public REST API
- ✅ Public WebSocket stream
- ✅ No authentication required

### v2 (Q3 2026)
- 🚧 API key authentication
- 🚧 Point-based access control
- 🚧 Smart contract payments
- 🚧 Rate limiting by tier
- 🚧 WebSocket with auth (500 concurrent users)
- 🚧 HTTP polling fallback

### v3 (Future)
- 🔜 OAuth integration
- 🔜 Team accounts & shared keys
- 🔜 Usage analytics dashboard
- 🔜 Auto-renewal subscriptions

---

## Support

**Having issues with authentication?**

- Check your API key is correct
- Verify you have sufficient points
- Ensure you're using the correct endpoint
- Contact: support@0g-signals.network

**Want to be notified when v2 launches?**

Join our Discord: https://discord.gg/0g-signals

---

## Next Steps

- [REST API Reference →](./rest.md)
- [WebSocket API →](./websocket.md)
- [Rate Limits →](./rate-limits.md)
- [Subscription Management →](../consumers/subscriptions.md)
