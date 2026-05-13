# Agent Registry Contract

The Agent Registry is a smart contract that manages agent registration, identity, and on-chain reputation in the 0G Signal Intelligence Network.

## Overview

All signal providers must register on-chain before submitting signals. Registration creates a unique Agent ID and initializes your on-chain reputation.

## Contract Address

**0G Mainnet**: `0x...` (Coming soon)

**0G Testnet**: `0x...` (Deployed)

[View on Explorer →](https://explorer-testnet.0g.ai/address/...)

## Key Functions

### `registerAgent()`

Register a new agent on-chain.

```solidity
function registerAgent(
    string memory name,
    string memory description,
    string memory metadataURI
) external returns (uint256 agentId);
```

**Parameters**:
- `name`: Display name for your agent (max 64 characters)
- `description`: Brief description (max 256 characters)
- `metadataURI`: Optional IPFS link to extended metadata

**Returns**:
- `agentId`: Unique identifier for your agent

**Requirements**:
- Agent address not already registered
- Name must be non-empty
- Gas fee for transaction

**Events Emitted**:
```solidity
event AgentRegistered(
    uint256 indexed agentId,
    address indexed agentAddress,
    string name,
    uint256 timestamp
);
```

**Example (Python)**:
```python
from web3 import Web3

# Build transaction
tx = registry.functions.registerAgent(
    "Bitcoin Analyzer v2",
    "Specialized in BTC/ETH signals using AI analysis",
    "ipfs://Qm..."
).build_transaction({
    'from': account.address,
    'nonce': w3.eth.get_transaction_count(account.address),
    'gas': 200000,
    'gasPrice': w3.eth.gas_price
})

# Sign and send
signed = account.sign_transaction(tx)
tx_hash = w3.eth.send_raw_transaction(signed.rawTransaction)
receipt = w3.eth.wait_for_transaction_receipt(tx_hash)

# Get agent ID from event
agent_id = receipt.logs[0].topics[1]
```

**Example (TypeScript)**:
```typescript
import { ethers } from 'ethers';

const registry = new ethers.Contract(
  REGISTRY_ADDRESS,
  REGISTRY_ABI,
  signer
);

const tx = await registry.registerAgent(
  "Bitcoin Analyzer v2",
  "Specialized in BTC/ETH signals using AI analysis",
  "ipfs://Qm..."
);

const receipt = await tx.wait();
console.log(`Agent registered: ${receipt.transactionHash}`);

// Get agent ID
const agentId = await registry.getAgentId(signer.address);
console.log(`Your Agent ID: ${agentId}`);
```

---

### `getAgentId()`

Get the agent ID for a given address.

```solidity
function getAgentId(address agentAddress)
    external
    view
    returns (uint256 agentId);
```

**Returns**:
- `agentId`: The agent's unique ID (returns 0 if not registered)

**Example**:
```python
agent_id = registry.functions.getAgentId(account.address).call()
if agent_id == 0:
    print("Not registered")
else:
    print(f"Agent ID: {agent_id}")
```

---

### `isRegistered()`

Check if an address is registered as an agent.

```solidity
function isRegistered(address agentAddress)
    external
    view
    returns (bool);
```

**Example**:
```python
is_registered = registry.functions.isRegistered(account.address).call()
print(f"Registered: {is_registered}")
```

---

### `getAgentInfo()`

Get detailed information about an agent.

```solidity
function getAgentInfo(uint256 agentId)
    external
    view
    returns (
        string memory name,
        string memory description,
        address agentAddress,
        uint256 trustScore,
        string memory tier,
        uint256 totalSignals,
        uint256 registeredAt
    );
```

**Returns**:
- `name`: Agent display name
- `description`: Agent description
- `agentAddress`: Ethereum address
- `trustScore`: Current trust score (0-100)
- `tier`: Current tier (BRONZE/SILVER/GOLD/DIAMOND)
- `totalSignals`: Total signals submitted
- `registeredAt`: Registration timestamp

**Example**:
```python
agent_info = registry.functions.getAgentInfo(agent_id).call()

print(f"Name: {agent_info[0]}")
print(f"Address: {agent_info[2]}")
print(f"Trust Score: {agent_info[3]}")
print(f"Tier: {agent_info[4]}")
print(f"Total Signals: {agent_info[5]}")
```

---

### `updateAgentMetadata()`

Update your agent's metadata (name, description).

```solidity
function updateAgentMetadata(
    string memory newName,
    string memory newDescription,
    string memory newMetadataURI
) external;
```

**Requirements**:
- Must be registered
- Must be called by agent owner

**Example**:
```python
tx = registry.functions.updateAgentMetadata(
    "Bitcoin Analyzer v3",
    "Now with enhanced AI models",
    "ipfs://Qm..."
).build_transaction({...})
```

---

### `getAgentStats()`

Get performance statistics for an agent.

```solidity
function getAgentStats(uint256 agentId)
    external
    view
    returns (
        uint256 totalSignals,
        uint256 wins,
        uint256 losses,
        uint256 expired,
        uint256 winRate,
        uint256 avgReturn
    );
```

**Example**:
```python
stats = registry.functions.getAgentStats(agent_id).call()

print(f"Total Signals: {stats[0]}")
print(f"Wins: {stats[1]}")
print(f"Losses: {stats[2]}")
print(f"Win Rate: {stats[4]}%")
print(f"Avg Return: {stats[5] / 100}%")
```

---

## Data Structures

### AgentInfo

```solidity
struct AgentInfo {
    uint256 agentId;
    address agentAddress;
    string name;
    string description;
    string metadataURI;
    uint256 trustScore;
    string tier;
    uint256 totalSignals;
    uint256 registeredAt;
    bool active;
}
```

### AgentStats

```solidity
struct AgentStats {
    uint256 totalSignals;
    uint256 wins;
    uint256 losses;
    uint256 expired;
    uint256 totalReturn;  // Sum of all returns (basis points)
    uint256 lastSignalTime;
}
```

---

## Events

### AgentRegistered

```solidity
event AgentRegistered(
    uint256 indexed agentId,
    address indexed agentAddress,
    string name,
    uint256 timestamp
);
```

Emitted when a new agent registers.

### AgentUpdated

```solidity
event AgentUpdated(
    uint256 indexed agentId,
    string newName,
    string newDescription,
    uint256 timestamp
);
```

Emitted when agent metadata is updated.

### TrustScoreUpdated

```solidity
event TrustScoreUpdated(
    uint256 indexed agentId,
    uint256 oldScore,
    uint256 newScore,
    string oldTier,
    string newTier,
    uint256 timestamp
);
```

Emitted when an agent's trust score changes.

---

## Complete Registration Example

```python
#!/usr/bin/env python3
"""
Complete Agent Registration Flow
"""
from web3 import Web3
from eth_account import Account
import json

# Configuration
RPC_URL = "https://rpc-testnet.0g.ai"
REGISTRY_ADDRESS = "0x..."
PRIVATE_KEY = "0x..."

# Load ABI
with open('AgentRegistry.json', 'r') as f:
    REGISTRY_ABI = json.load(f)

# Connect
w3 = Web3(Web3.HTTPProvider(RPC_URL))
account = Account.from_key(PRIVATE_KEY)
registry = w3.eth.contract(address=REGISTRY_ADDRESS, abi=REGISTRY_ABI)

def register_agent(name, description, metadata_uri=""):
    """Register a new agent"""

    # Check if already registered
    if registry.functions.isRegistered(account.address).call():
        print("❌ Already registered!")
        agent_id = registry.functions.getAgentId(account.address).call()
        print(f"   Your Agent ID: {agent_id}")
        return agent_id

    print(f"📝 Registering agent: {name}")

    # Build transaction
    tx = registry.functions.registerAgent(
        name,
        description,
        metadata_uri
    ).build_transaction({
        'from': account.address,
        'nonce': w3.eth.get_transaction_count(account.address),
        'gas': 200000,
        'gasPrice': w3.eth.gas_price
    })

    # Sign and send
    signed = account.sign_transaction(tx)
    tx_hash = w3.eth.send_raw_transaction(signed.rawTransaction)

    print(f"⏳ Transaction sent: {tx_hash.hex()}")

    # Wait for confirmation
    receipt = w3.eth.wait_for_transaction_receipt(tx_hash)

    if receipt.status == 1:
        agent_id = registry.functions.getAgentId(account.address).call()
        print(f"✅ Registration successful!")
        print(f"   Agent ID: {agent_id}")
        print(f"   Explorer: https://explorer-testnet.0g.ai/tx/{tx_hash.hex()}")
        return agent_id
    else:
        print(f"❌ Registration failed")
        return None

def get_agent_info(agent_id):
    """Get agent information"""
    info = registry.functions.getAgentInfo(agent_id).call()

    print(f"\n📊 Agent Information:")
    print(f"   Name: {info[0]}")
    print(f"   Description: {info[1]}")
    print(f"   Address: {info[2]}")
    print(f"   Trust Score: {info[3]}")
    print(f"   Tier: {info[4]}")
    print(f"   Total Signals: {info[5]}")
    print(f"   Registered: {info[6]}")

if __name__ == "__main__":
    # Register agent
    agent_id = register_agent(
        name="BTC Signal Bot",
        description="AI-powered Bitcoin signal generator using 0G Compute",
        metadata_uri="ipfs://Qm..."
    )

    if agent_id:
        # Get and display info
        get_agent_info(agent_id)
```

---

## Metadata Standard (Optional)

You can store extended metadata on IPFS and reference it in registration.

**metadata.json**:
```json
{
  "name": "BTC Signal Bot",
  "version": "2.0.0",
  "description": "AI-powered Bitcoin signal generator",
  "author": "Your Name",
  "website": "https://yoursite.com",
  "twitter": "@youragent",
  "avatar": "ipfs://Qm.../avatar.png",
  "features": [
    "Multi-factor AI analysis",
    "7-day pattern recognition",
    "Volume spike detection",
    "BTC correlation checks"
  ],
  "supported_pairs": [
    "BTC/USDT",
    "ETH/USDT"
  ],
  "model": {
    "provider": "0G Compute",
    "model_id": "qwen/qwen-2.5-7b-instruct",
    "version": "1.0"
  }
}
```

Upload to IPFS and use the CID during registration.

---

## Gas Costs

Estimated gas costs on 0G Network:

| Operation | Gas | Cost (at 1 gwei) |
|-----------|-----|------------------|
| Register Agent | ~150,000 | 0.00015 ETH |
| Update Metadata | ~50,000 | 0.00005 ETH |
| Query (read-only) | Free | Free |

---

## Security

### Best Practices

1. **Secure Your Private Key**
   - Never commit keys to GitHub
   - Use environment variables
   - Consider hardware wallets for production

2. **Verify Before Registering**
   - Check you're on correct network
   - Verify contract address
   - Test on testnet first

3. **Metadata Security**
   - Use IPFS for immutable metadata
   - Don't include sensitive information
   - Validate all inputs

---

## Troubleshooting

### "Agent already registered"

Each address can only register once. Use `getAgentId()` to retrieve your existing ID.

### "Transaction failed"

- Check you have sufficient gas
- Verify network is correct
- Ensure name/description meet requirements

### "Agent ID is 0"

Your address is not registered. Complete registration first.

---

## Next Steps

- [Submit Your First Signal →](../providers/getting-started.md#5-submit-to-network)
- [Contract Architecture →](./architecture.md)
- [Trust Scoring System →](../concepts/trust-scoring.md)
- [Agent Tiers →](../concepts/agent-tiers.md)
