# Smart Contract Architecture

The 0G Signal Intelligence Network uses smart contracts for transparent signal verification and trust scoring on Arbitrum.

## Overview

The platform consists of three main contract layers:

```
┌─────────────────────────┐
│  SignalRegistry.sol     │  → Records all signals on-chain
│  (Signal Storage)       │     Emits events for indexing
└────────────┬────────────┘
             │
             ↓
┌─────────────────────────┐
│  TrustScorer.sol        │  → Calculates agent trust scores
│  (Reputation System)    │     Tracks WIN/LOSS outcomes
└────────────┬────────────┘
             │
             ↓
┌─────────────────────────┐
│  SignalVerifier.sol     │  → Verifies EIP-191 signatures
│  (Signature Validation) │     Prevents spoofing
└─────────────────────────┘
```

---

## SignalRegistry.sol

Central contract for recording all trading signals.

### Key Functions

#### `submitSignal()`

```solidity
function submitSignal(
    bytes memory signature,
    SignalPayload memory payload
) external returns (bytes32 signalId);
```

**Parameters**:
- `signature`: EIP-191 signature from agent
- `payload`: Signal data (token pair, direction, prices, etc.)

**Returns**:
- `signalId`: Unique identifier for this signal

**Events**:
```solidity
event SignalSubmitted(
    bytes32 indexed signalId,
    address indexed agent,
    string tokenPair,
    string direction,
    uint256 entryPrice,
    uint256 takeProfit,
    uint256 stopLoss,
    uint256 weightPct,
    uint256 timestamp
);
```

#### `resolveSignal()`

```solidity
function resolveSignal(
    bytes32 signalId,
    SignalState newState  // WIN, LOSS, or EXPIRED
) external onlyOracle returns (bool);
```

**Parameters**:
- `signalId`: Signal to resolve
- `newState`: Final state (WIN/LOSS/EXPIRED)

**Events**:
```solidity
event SignalResolved(
    bytes32 indexed signalId,
    address indexed agent,
    SignalState state,
    uint256 returnPct,
    uint256 timestamp
);
```

#### `getSignal()`

```solidity
function getSignal(bytes32 signalId)
    external
    view
    returns (Signal memory);
```

Returns full signal data including current state.

---

## TrustScorer.sol

Calculates and maintains agent trust scores based on performance.

### Key Functions

#### `updateTrustScore()`

```solidity
function updateTrustScore(
    address agent,
    SignalState outcome
) external onlyRegistry returns (uint256 newScore);
```

**Algorithm**:
```solidity
// Simplified scoring logic
function calculateTrustScore(address agent) internal view returns (uint256) {
    Stats memory stats = agentStats[agent];

    // Win rate component (40% weight)
    uint256 winRate = (stats.wins * 100) / (stats.wins + stats.losses);
    uint256 winRateScore = winRate * 40 / 100;

    // Return quality component (30% weight)
    uint256 avgReturn = stats.totalReturn / stats.wins;
    uint256 returnScore = min(avgReturn * 3, 30);

    // Volume component (20% weight)
    uint256 volumeScore = min(stats.totalSignals / 10, 20);

    // Recency component (10% weight)
    uint256 daysSinceLastSignal = (block.timestamp - stats.lastSignalTime) / 86400;
    uint256 recencyScore = daysSinceLastSignal < 7 ? 10 : 5;

    return winRateScore + returnScore + volumeScore + recencyScore;
}
```

#### `getTrustScore()`

```solidity
function getTrustScore(address agent)
    external
    view
    returns (uint256 score, string memory tier);
```

**Returns**:
- `score`: Trust score (0-100)
- `tier`: Agent tier (BRONZE/SILVER/GOLD/DIAMOND)

#### `getAgentStats()`

```solidity
function getAgentStats(address agent)
    external
    view
    returns (
        uint256 totalSignals,
        uint256 wins,
        uint256 losses,
        uint256 winRate,
        uint256 avgReturn
    );
```

---

## SignalVerifier.sol

Validates EIP-191 signatures to ensure signal authenticity.

### Key Functions

#### `verifySignal()`

```solidity
function verifySignal(
    bytes memory signature,
    SignalPayload memory payload,
    address expectedSigner
) public pure returns (bool);
```

**Implementation**:
```solidity
function verifySignal(
    bytes memory signature,
    SignalPayload memory payload,
    address expectedSigner
) public pure returns (bool) {
    // Create message hash
    bytes32 messageHash = keccak256(abi.encodePacked(
        payload.tokenPair,
        payload.direction,
        payload.entryPrice,
        payload.takeProfit,
        payload.stopLoss,
        payload.weightPct,
        payload.timestamp
    ));

    // EIP-191 prefix
    bytes32 ethSignedMessageHash = keccak256(abi.encodePacked(
        "\x19Ethereum Signed Message:\n32",
        messageHash
    ));

    // Recover signer
    address recoveredSigner = recoverSigner(ethSignedMessageHash, signature);

    return recoveredSigner == expectedSigner;
}

function recoverSigner(
    bytes32 ethSignedMessageHash,
    bytes memory signature
) internal pure returns (address) {
    require(signature.length == 65, "Invalid signature length");

    bytes32 r;
    bytes32 s;
    uint8 v;

    assembly {
        r := mload(add(signature, 32))
        s := mload(add(signature, 64))
        v := byte(0, mload(add(signature, 96)))
    }

    return ecrecover(ethSignedMessageHash, v, r, s);
}
```

---

## Data Structures

### Signal

```solidity
struct Signal {
    bytes32 signalId;
    address agent;
    string tokenPair;
    string direction;  // "long" or "short"
    uint256 entryPrice;
    uint256 takeProfit;
    uint256 stopLoss;
    uint256 weightPct;
    uint256 expiryTime;
    SignalState state;  // PENDING, WIN, LOSS, EXPIRED
    uint256 submittedAt;
    uint256 resolvedAt;
    uint256 returnPct;
}

enum SignalState {
    PENDING,
    WIN,
    LOSS,
    EXPIRED
}
```

### AgentStats

```solidity
struct AgentStats {
    uint256 totalSignals;
    uint256 wins;
    uint256 losses;
    uint256 expired;
    uint256 totalReturn;  // Sum of all winning returns
    uint256 lastSignalTime;
    uint256 trustScore;
    string tier;
}
```

---

## Contract Addresses

### Arbitrum One (Mainnet)

```
SignalRegistry:   0x... (Coming soon)
TrustScorer:      0x... (Coming soon)
SignalVerifier:   0x... (Coming soon)
```

### Arbitrum Sepolia (Testnet)

```
SignalRegistry:   0x... (Deployed)
TrustScorer:      0x... (Deployed)
SignalVerifier:   0x... (Deployed)
```

---

## Events

### SignalSubmitted

```solidity
event SignalSubmitted(
    bytes32 indexed signalId,
    address indexed agent,
    string tokenPair,
    string direction,
    uint256 entryPrice,
    uint256 takeProfit,
    uint256 stopLoss,
    uint256 weightPct,
    uint256 timestamp
);
```

### SignalResolved

```solidity
event SignalResolved(
    bytes32 indexed signalId,
    address indexed agent,
    SignalState state,
    uint256 returnPct,
    uint256 timestamp
);
```

### TrustScoreUpdated

```solidity
event TrustScoreUpdated(
    address indexed agent,
    uint256 oldScore,
    uint256 newScore,
    string oldTier,
    string newTier,
    uint256 timestamp
);
```

---

## Integration Example

### Reading On-Chain Data

```javascript
const { ethers } = require('ethers');

// Contract ABIs
const SignalRegistryABI = [...];
const TrustScorerABI = [...];

// Connect to contracts
const provider = new ethers.providers.JsonRpcProvider(ARBITRUM_RPC_URL);

const signalRegistry = new ethers.Contract(
  SIGNAL_REGISTRY_ADDRESS,
  SignalRegistryABI,
  provider
);

const trustScorer = new ethers.Contract(
  TRUST_SCORER_ADDRESS,
  TrustScorerABI,
  provider
);

// Get agent trust score
async function getAgentTrust(agentAddress) {
  const [score, tier] = await trustScorer.getTrustScore(agentAddress);
  return { score: score.toNumber(), tier };
}

// Listen for new signals
signalRegistry.on('SignalSubmitted', (
  signalId,
  agent,
  tokenPair,
  direction,
  entryPrice,
  takeProfit,
  stopLoss,
  weightPct,
  timestamp,
  event
) => {
  console.log('New signal:', {
    signalId,
    agent,
    tokenPair,
    direction,
  });
});

// Listen for signal resolutions
signalRegistry.on('SignalResolved', (
  signalId,
  agent,
  state,
  returnPct,
  timestamp,
  event
) => {
  console.log('Signal resolved:', {
    signalId,
    agent,
    state,
    returnPct: returnPct.toNumber(),
  });
});
```

### Verifying Signals Off-Chain

```javascript
const { ethers } = require('ethers');

function verifySignal(envelope, payload) {
  // Reconstruct message
  const message = JSON.stringify({
    agent: envelope.agent,
    timestamp: envelope.timestamp,
    payload: payload,
  }, null, 0);  // No spaces

  // Hash message
  const messageHash = ethers.utils.hashMessage(message);

  // Recover signer
  const recoveredAddress = ethers.utils.verifyMessage(
    message,
    envelope.signature
  );

  // Check if signer matches claimed agent
  return recoveredAddress.toLowerCase() === envelope.agent.toLowerCase();
}
```

---

## Security Considerations

### 1. Signature Validation

Always verify signatures before processing signals:

```solidity
require(
    SignalVerifier.verifySignal(signature, payload, msg.sender),
    "Invalid signature"
);
```

### 2. Replay Protection

Use timestamps to prevent replay attacks:

```solidity
require(
    block.timestamp - payload.timestamp < 300,  // 5 minutes
    "Signal too old"
);

require(
    !processedSignals[signalId],
    "Signal already processed"
);
```

### 3. Oracle Security

Signal resolution is controlled by a trusted oracle:

```solidity
modifier onlyOracle() {
    require(msg.sender == oracleAddress, "Only oracle can resolve");
    _;
}
```

### 4. Rate Limiting

Prevent spam with per-agent rate limits:

```solidity
mapping(address => uint256) public lastSubmissionTime;

require(
    block.timestamp - lastSubmissionTime[msg.sender] > 60,
    "Rate limit: 1 signal per minute"
);
```

---

## Gas Optimization

### 1. Batch Operations

Submit multiple signals in one transaction:

```solidity
function batchSubmitSignals(
    bytes[] memory signatures,
    SignalPayload[] memory payloads
) external returns (bytes32[] memory signalIds);
```

### 2. Storage Optimization

Use packed structs to save gas:

```solidity
struct PackedSignal {
    address agent;           // 20 bytes
    uint96 entryPrice;       // 12 bytes
    uint32 timestamp;        // 4 bytes
    uint8 weightPct;         // 1 byte
    SignalState state;       // 1 byte
}
```

### 3. Event Indexing

Index key fields for efficient querying:

```solidity
event SignalSubmitted(
    bytes32 indexed signalId,
    address indexed agent,
    string tokenPair,  // Not indexed (long string)
    ...
);
```

---

## Upgradeability

Contracts use transparent proxy pattern for upgrades:

```
┌──────────────┐
│ ProxyContract│  → User-facing address (immutable)
└──────┬───────┘
       │
       ↓
┌──────────────┐
│ Implementation│  → Logic contract (upgradeable)
│   Contract    │
└──────────────┘
```

**Upgrade Process**:
1. Deploy new implementation contract
2. Call `upgradeTo(newImplementation)` on proxy
3. All storage preserved, new logic active

---

## Roadmap

### Phase 1 (Current)
- ✅ Signal submission and verification
- ✅ Trust score calculation
- ✅ Basic event emissions

### Phase 2 (Q3 2026)
- 🔜 On-chain signal aggregation
- 🔜 Staking for signal providers
- 🔜 Revenue sharing mechanism

### Phase 3 (Q4 2026)
- 🔜 Cross-chain signal bridge
- 🔜 Decentralized oracle network
- 🔜 DAO governance

---

## Next Steps

- [Integration Guide →](./integration.md)
- [Contract Source Code →](https://github.com/ogruns/0g-signals-contracts)
- [Arbiscan →](https://arbiscan.io/address/...)
- [Audit Reports →](../security/audits.md)
