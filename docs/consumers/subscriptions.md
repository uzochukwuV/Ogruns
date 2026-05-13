# Subscription Management

Point-based access control for premium signals and features (v2 Coming Soon).

---

## Current Version (v1)

**Status**: ✅ Active

All signals are currently **free and publicly accessible**:
- No subscription required
- No authentication needed
- Open WebSocket stream
- Public REST API

Access at:
- REST: `https://api.0g-signals.network/api/v1/`
- WebSocket: `ws://api.0g-signals.network/api/v1/stream/public`

---

## Version 2 (Coming Q3 2026)

**Status**: 🚧 Planned

v2 introduces **point-based subscriptions** with API key authentication.

### How It Works

1. **Register** → Get API key
2. **Purchase Points** → Send ETH to payment contract
3. **Spend Points** → Access premium features
4. **Track Balance** → Monitor usage

### Point System Overview

**Purchase Points**:
```
100 points   = 0.01 ETH    (no bonus)
500 points   = 0.045 ETH   (+10% bonus = 550 points)
1,000 points = 0.08 ETH    (+20% bonus = 1,200 points)
5,000 points = 0.35 ETH    (+30% bonus = 6,500 points)
```

**Spend Points On**:
- Premium signals (TBD points/signal)
- Agent subscriptions (TBD points/month)
- Historical data queries (TBD points/query)
- API calls (TBD points/request)

*Final pricing will be announced before v2 launch.*

### Authentication

**API Key in Requests**:
```typescript
const response = await fetch(
  'https://api.0g-signals.network/api/v2/signals/premium',
  {
    headers: {
      'Authorization': `Bearer ${API_KEY}`
    }
  }
);
```

**API Key in WebSocket**:
```typescript
const ws = new WebSocket(
  `wss://api.0g-signals.network/api/v2/stream?api_key=${API_KEY}`
);
```

**Limits**:
- WebSocket: 500 concurrent connections
- Beyond 500: Use HTTP polling

### Purchasing Points

**Step 1**: Send ETH to payment contract

```typescript
import { ethers } from 'ethers';

const PAYMENT_CONTRACT = "0x..."; // Will be announced
const signer = new ethers.Wallet(PRIVATE_KEY, provider);

// Purchase 500 points (0.045 ETH)
const tx = await signer.sendTransaction({
  to: PAYMENT_CONTRACT,
  value: ethers.utils.parseEther("0.045"),
  data: ethers.utils.defaultAbiCoder.encode(
    ["uint256", "string"],
    [500, "my-user-id"]  // points, userId
  )
});

await tx.wait();
```

**Step 2**: Backend auto-credits points

The backend monitors payment contract events and automatically credits your account within 1-2 minutes.

**Step 3**: Verify balance

```bash
GET /api/v2/users/me/balance
Authorization: Bearer sk_live_abc123...

{
  "points_balance": 550,
  "points_total": 550,
  "points_spent": 0
}
```

### Using Points

Points are automatically deducted when you access premium endpoints:

```typescript
// Each request costs points
const response = await fetch(
  'https://api.0g-signals.network/api/v2/signals/premium',
  {
    headers: { 'Authorization': `Bearer ${API_KEY}` }
  }
);

// Check remaining points in response
const remaining = response.headers.get('X-Points-Remaining');
console.log(`Points left: ${remaining}`);
```

### Rate Limits by Points Tier

| Tier | Points Balance | Rate Limit |
|------|---------------|------------|
| Free | 0 | 10 req/min |
| Basic | 100+ | 60 req/min |
| Pro | 1,000+ | 300 req/min |
| Premium | 5,000+ | 1,000 req/min |

---

## Migration Plan (v1 → v2)

When v2 launches:

### For Existing Users

1. **Continue using v1** (free tier):
   - Access to basic public signals
   - Limited rate (10 req/min)
   - No authentication required

2. **Upgrade to v2** (premium):
   - Register for API key
   - Purchase points
   - Access premium signals
   - Higher rate limits

### Code Changes Needed

**Before (v1)**:
```typescript
// No authentication
const ws = new WebSocket('ws://api.com/stream/public');
```

**After (v2)**:
```typescript
// With API key
const ws = new WebSocket(
  `wss://api.com/stream?api_key=${API_KEY}`
);
```

---

## Features Comparison

| Feature | v1 (Current) | v2 (Coming) |
|---------|-------------|-------------|
| Public Signals | ✅ Free | ✅ Free (limited) |
| Premium Signals | ❌ | ✅ Points required |
| Agent Subscriptions | ❌ | ✅ Points required |
| Historical Data | ❌ | ✅ Points required |
| Rate Limit | 100 req/min | Tier-based |
| WebSocket | ✅ Public | ✅ Auth required (500 max) |
| API Key | ❌ | ✅ Required |
| Points System | ❌ | ✅ Purchase with ETH |

---

## Payment Contract

**Simple ETH payment contract for purchasing points:**

```solidity
// PointPayment.sol
contract PointPayment {
    event PointsPurchased(
        address indexed user,
        uint256 points,
        uint256 amountPaid,
        string userId,
        uint256 timestamp
    );

    function purchasePoints(uint256 points, string calldata userId)
        external
        payable
    {
        require(msg.value > 0, "Payment required");
        require(points > 0, "Invalid points");

        emit PointsPurchased(
            msg.sender,
            points,
            msg.value,
            userId,
            block.timestamp
        );
    }
}
```

**Contract Address**: Will be announced before v2 launch

**Backend Flow**:
1. User sends ETH to contract → Emits `PointsPurchased` event
2. Backend listens for events → Extracts `userId` and `points`
3. Credits user's off-chain account → Updates database
4. User can now spend points via API

---

## Off-Chain Point Tracking

Points are tracked in the backend database (PostgreSQL):

```sql
-- users table
CREATE TABLE users (
    id VARCHAR PRIMARY KEY,
    wallet_address VARCHAR UNIQUE,
    api_key VARCHAR UNIQUE,
    points_balance INTEGER DEFAULT 0,
    points_total INTEGER DEFAULT 0,
    points_spent INTEGER DEFAULT 0,
    created_at TIMESTAMP
);

-- transactions table
CREATE TABLE point_transactions (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR REFERENCES users(id),
    type VARCHAR,  -- 'purchase', 'spend', 'grant'
    points INTEGER,
    description TEXT,
    created_at TIMESTAMP
);
```

**Why Off-Chain?**
- ✅ Cheaper (no gas for point deduction)
- ✅ Faster (instant API responses)
- ✅ Flexible (easy to adjust pricing)
- ✅ Better UX (no wallet signing per API call)

---

## Admin Functions

### Grant Points (Manual)

```bash
POST /api/v2/admin/users/{userId}/grant-points
Authorization: Bearer admin_key

{
  "points": 1000,
  "reason": "Welcome bonus"
}
```

### View User Balance

```bash
GET /api/v2/admin/users/{userId}/balance
Authorization: Bearer admin_key

{
  "user_id": "usr_123",
  "points_balance": 1550,
  "points_total": 2000,
  "points_spent": 450
}
```

### Analytics

```bash
GET /api/v2/admin/analytics/points
Authorization: Bearer admin_key

{
  "total_revenue_eth": "2.45",
  "total_points_sold": 50000,
  "total_points_spent": 12500,
  "active_users": 127
}
```

---

## Roadmap

### Phase 1: v2 Launch (Q3 2026)
- ✅ API key authentication
- ✅ Point purchase contract
- ✅ Off-chain point tracking
- ✅ Premium signal access
- ✅ WebSocket with auth (500 users)
- ✅ HTTP polling fallback

### Phase 2: Enhanced Features (Q4 2026)
- 🔜 Auto-renewal subscriptions
- 🔜 Team accounts (shared points)
- 🔜 Usage analytics dashboard
- 🔜 Credit card payments (Stripe)
- 🔜 Referral program (earn points)

### Phase 3: Advanced (2027)
- 🔜 Revenue sharing with signal providers
- 🔜 DAO governance for pricing
- 🔜 Token-based payments (beyond ETH)
- 🔜 Mobile app with point management

---

## FAQ

**Q: Will v1 still be free after v2 launches?**
A: Yes! Basic public signals will remain free with limited rate (10 req/min).

**Q: Can I use USDC instead of ETH?**
A: Not initially. v2 launch supports ETH only. USDC/USDT coming in Phase 2.

**Q: What happens if I run out of points mid-request?**
A: The request will fail with `402 Insufficient Points`. Top up and retry.

**Q: Do points expire?**
A: No expiry! Points are valid forever.

**Q: Can I transfer points to another user?**
A: Not in v2. This may be added in future versions.

**Q: What if the payment transaction fails?**
A: Your ETH stays in your wallet. No points are credited. Safe to retry.

---

## Next Steps

- [Authentication Guide (v2 Details) →](../api/authentication.md)
- [REST API Reference →](../api/rest.md)
- [WebSocket API →](../api/websocket.md)
- [Getting Started →](./getting-started.md)

---

**Want early access to v2?**

Join our Discord for beta testing opportunities:
https://discord.gg/0g-signals
