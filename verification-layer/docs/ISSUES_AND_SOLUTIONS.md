# Verification Layer - Issues & Solutions

This document captures the key issues encountered during development and how they were resolved.

---

## 1. Signal Privacy at Rest

### Problem
Active signals stored in plaintext could be read directly from 0G Storage or local disk by malicious actors, allowing them to front-run or copy trades before expiry.

### Analysis
- **Active signals** contain actionable information (entry, TP, SL, direction)
- **Closed signals** are historical and safe to be public (proof of track record)
- Encrypting for each subscriber doesn't scale for high-frequency delivery

### Solution
**Verifier as Trusted Gateway Architecture:**
- Active signals encrypted locally using AES-256-GCM derived from verifier's private key
- Signals decrypted only when delivering to authenticated subscribers
- HTTPS provides transport encryption
- After expiry, signals uploaded to 0G Storage in plaintext (public audit trail)

**Implementation:**
- `internal/crypto/encryption.go` - AES-256-GCM encryption utilities
- `EncryptedSignalStore` - Persistent encrypted storage at `~/.0g-verifier/signals/`
- Auto-saves every 2 minutes + graceful shutdown save
- Restores and re-schedules unexpired signals on restart

---

## 2. Subscription Signature Replay Attacks

### Problem
Original signature format `access:{nodeID}` could be captured and reused indefinitely by attackers to access premium endpoints without paying.

### Solution
Added timestamp-based replay protection:

**New Signature Format:**
```
message = "access:{nodeID}:{timestamp}"
```

**Validation Rules:**
- Timestamp must be within 5 minutes of current time
- Prevents replay of old signatures
- Signature still proves address ownership via EIP-191 personal_sign

**Client Example (ethers.js):**
```javascript
const timestamp = Math.floor(Date.now() / 1000);
const message = `access:${nodeID}:${timestamp}`;
const signature = await signer.signMessage(message);

// Include in request
fetch(`/api/v1/stream?node_id=${nodeID}&subscriber_address=${address}&signature=${signature}&timestamp=${timestamp}`)
```

---

## 3. Subscription Cache Memory Leak

### Problem
`sync.Map` cache for subscription status would grow unbounded as users accessed premium endpoints, never releasing memory for inactive users.

### Solution
Added periodic cache cleanup:
- Background goroutine runs every 10 minutes
- Removes cache entries older than 5-minute TTL
- Stops gracefully when server context is cancelled
- Logs cleanup activity for monitoring

---

## 4. Long-Expiry Signal Manipulation (NEW)

### Problem
Signals with long expiry times (e.g., 1 week) can be gamed:
1. Signal: "BUY BTC at $50k, TP $55k, SL $48k, expiry 7 days"
2. During week: BTC drops to $45k (real traders stopped out at $48k)
3. BTC recovers to $55k before expiry
4. System marks signal as "WIN" at expiry
5. **Reality:** Traders who followed the signal lost money

This creates a disconnect between signal "score" and actual trader outcomes.

### Root Cause
Current system only checks price AT expiry time, not DURING the signal lifetime.

### Solution: First-Exit Analysis

Instead of only checking at expiry, track which exit condition was hit FIRST:

```
Outcome = FIRST of:
  1. Price hits Take Profit → WIN
  2. Price hits Stop Loss → LOSS
  3. Expiry time reached → Check final price vs entry
```

**Implementation approach:**
1. When signal is scheduled, also schedule SL/TP price checks
2. Use historical candle data (1m or 5m) to detect if SL/TP was breached
3. Record the FIRST exit event as the outcome
4. Store max drawdown for risk analysis

### Mitigation Until Full Implementation
- Add `max_expiry_hours` validation (e.g., max 24-48 hours)
- Reject signals with unreasonable time ranges
- Add warning flag for signals with >12h expiry

---

## 5. Missing Signal Metadata

### Problem
Current `SignalPayload` lacks important trading context:
- No distinction between spot and futures/perpetuals
- No leverage information for risk calculation
- No position sizing guidance

### Impact
- Cannot accurately simulate leveraged PnL
- Cannot separate strategies (spot hodl vs perp scalping)
- Risk metrics don't account for leverage

### Solution
See `SignalPayload` enhancement proposal below.

---

## Appendix: Enhanced SignalPayload Schema

```go
type SignalPayload struct {
    // Existing fields
    TokenPair  string  `json:"token_pair"`
    Exchange   string  `json:"exchange,omitempty"`
    Direction  string  `json:"direction"`      // "long" or "short"
    EntryPrice float64 `json:"entry_price"`
    TakeProfit float64 `json:"take_profit"`
    StopLoss   float64 `json:"stop_loss"`
    ExpiryTime int64   `json:"expiry_time"`
    WeightPct  float64 `json:"weight_pct"`

    // NEW: Trade type and leverage
    TradeType  string  `json:"trade_type,omitempty"`  // "spot", "perpetual", "futures"
    Leverage   float64 `json:"leverage,omitempty"`    // 1x for spot, 1-125x for perps

    // NEW: Risk management
    MaxExpiryHours int `json:"max_expiry_hours,omitempty"` // Soft limit suggestion

    // NEW: Multiple take profit levels
    TakeProfits []TakeProfitLevel `json:"take_profits,omitempty"`
}

type TakeProfitLevel struct {
    Price   float64 `json:"price"`
    Percent float64 `json:"percent"` // % of position to close at this level
}
```

### Validation Rules
| Field | Rule |
|-------|------|
| `trade_type` | If omitted, defaults to "spot" |
| `leverage` | Must be 1 for spot, 1-125 for perpetual |
| `expiry_time` | Recommended max 48h for leveraged positions |
| `stop_loss` | Required for leveraged trades (>1x) |

---

## Security Considerations

1. **Private Key Protection**: Encryption key derived from verifier's private key - if compromised, all active signals are exposed
2. **Timestamp Skew**: 5-minute window allows for clock drift but limits replay window
3. **Cache Invalidation**: On subscription events (subscribe/unsubscribe), call `ClearCache()` to force fresh on-chain check
