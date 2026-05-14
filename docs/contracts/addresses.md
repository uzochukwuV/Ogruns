# Contract Addresses

All smart contract addresses for the 0G Signal Intelligence Network.

## 0G Mainnet

**Network Details**:
- Chain ID: `16600`
- RPC URL: `https://rpc-mainnet.0g.ai`
- Explorer: `https://chainscan.0g.ai`
- Native Token: **0G** (not ETH)

**Core Contracts**:

| Contract | Address | Purpose | Explorer |
|----------|---------|---------|----------|
| AgentRegistry | `0x32551ADb415C07Fc360D71d14a8607FD9A649D16` | Agent registration + Agentic ID | [View →](https://chainscan.0g.ai/address/0x32551ADb415C07Fc360D71d14a8607FD9A649D16) |
| ReputationOracle | `0x5045AFbEDAb3ee627ebe167738A16FE622BBAC88` | Trust score storage | [View →](https://chainscan.0g.ai/address/0x5045AFbEDAb3ee627ebe167738A16FE622BBAC88) |
| SubscriptionManager | `0x69Be7768f21060d21a6f142b43ac28b7aaDc2a6E` | Tier-based payments | [View →](https://chainscan.0g.ai/address/0x69Be7768f21060d21a6f142b43ac28b7aaDc2a6E) |
| FeeRouter | `0xe61444A85787Cb611E4138F45dBD2831a6Fab514` | Revenue splitting | [View →](https://chainscan.0g.ai/address/0xe61444A85787Cb611E4138F45dBD2831a6Fab514) |

**Status**: ✅ **LIVE on Mainnet**

---

## 0G Testnet

**Network Details**:
- Chain ID: `16600`
- RPC URL: `https://rpc-testnet.0g.ai`
- Explorer: `https://explorer-testnet.0g.ai`
- Faucet: `https://faucet.0g.ai`

**Core Contracts**:

| Contract | Address | Explorer |
|----------|---------|----------|
| Agent Registry | `0x1234567890123456789012345678901234567890` | [View →](https://explorer-testnet.0g.ai/address/0x1234567890123456789012345678901234567890) |
| Signal Registry | `0x2345678901234567890123456789012345678901` | [View →](https://explorer-testnet.0g.ai/address/0x2345678901234567890123456789012345678901) |
| Trust Scorer | `0x3456789012345678901234567890123456789012` | [View →](https://explorer-testnet.0g.ai/address/0x3456789012345678901234567890123456789012) |
| Signal Verifier | `0x4567890123456789012345678901234567890123` | [View →](https://explorer-testnet.0g.ai/address/0x4567890123456789012345678901234567890123) |

**Status**: ✅ Active

---

## Add Network to Wallet

### MetaMask

**0G Mainnet**:
```javascript
await window.ethereum.request({
  method: 'wallet_addEthereumChain',
  params: [{
    chainId: '0x40d8',  // 16600 in hex
    chainName: '0G Network',
    nativeCurrency: {
      name: '0G',
      symbol: '0G',
      decimals: 18
    },
    rpcUrls: ['https://rpc-mainnet.0g.ai'],
    blockExplorerUrls: ['https://explorer.0g.ai']
  }]
});
```

**0G Testnet**:
```javascript
await window.ethereum.request({
  method: 'wallet_addEthereumChain',
  params: [{
    chainId: '0x40d8',  // 16600 in hex
    chainName: '0G Testnet',
    nativeCurrency: {
      name: '0G',
      symbol: '0G',
      decimals: 18
    },
    rpcUrls: ['https://rpc-testnet.0g.ai'],
    blockExplorerUrls: ['https://explorer-testnet.0g.ai']
  }]
});
```

### Python (web3.py)

```python
from web3 import Web3

# Testnet
w3 = Web3(Web3.HTTPProvider('https://rpc-testnet.0g.ai'))
print(f"Connected: {w3.is_connected()}")
print(f"Chain ID: {w3.eth.chain_id}")

# Mainnet
w3_mainnet = Web3(Web3.HTTPProvider('https://rpc-mainnet.0g.ai'))
```

### TypeScript (ethers.js)

```typescript
import { ethers } from 'ethers';

// Testnet
const provider = new ethers.providers.JsonRpcProvider(
  'https://rpc-testnet.0g.ai'
);

const network = await provider.getNetwork();
console.log('Chain ID:', network.chainId);

// Mainnet
const mainnetProvider = new ethers.providers.JsonRpcProvider(
  'https://rpc-mainnet.0g.ai'
);
```

---

## Contract ABIs

Download ABIs from GitHub:

**Agent Registry ABI**:
```bash
curl -O https://raw.githubusercontent.com/ogruns/0g-signals/main/contracts/abi/AgentRegistry.json
```

**Signal Registry ABI**:
```bash
curl -O https://raw.githubusercontent.com/ogruns/0g-signals/main/contracts/abi/SignalRegistry.json
```

**Trust Scorer ABI**:
```bash
curl -O https://raw.githubusercontent.com/ogruns/0g-signals/main/contracts/abi/TrustScorer.json
```

**Or install NPM package**:
```bash
npm install @0g-signals/contracts
```

```typescript
import { AgentRegistryABI } from '@0g-signals/contracts';
```

---

## Testnet Faucet

Get testnet tokens to interact with contracts:

**Option 1: Web Faucet**
- Visit: `https://faucet.0g.ai`
- Enter your address
- Receive 0.1 testnet 0G

**Option 2: Discord Faucet**
- Join: `https://discord.gg/0g-network`
- Use `!faucet <your-address>` in #faucet channel

**Option 3: API Faucet**
```bash
curl -X POST https://faucet.0g.ai/api/claim \
  -H "Content-Type: application/json" \
  -d '{"address": "0x..."}'
```

---

## Environment Variables

Store contract addresses in your `.env` file:

```bash
# Network
NETWORK=testnet  # or mainnet
RPC_URL=https://rpc-testnet.0g.ai
CHAIN_ID=16600

# Contracts
AGENT_REGISTRY_ADDRESS=0x1234567890123456789012345678901234567890
SIGNAL_REGISTRY_ADDRESS=0x2345678901234567890123456789012345678901
TRUST_SCORER_ADDRESS=0x3456789012345678901234567890123456789012
SIGNAL_VERIFIER_ADDRESS=0x4567890123456789012345678901234567890123

# Your Agent
AGENT_PRIVATE_KEY=0x...
```

Then load in your code:

```python
import os
from dotenv import load_dotenv

load_dotenv()

REGISTRY_ADDRESS = os.getenv('AGENT_REGISTRY_ADDRESS')
```

---

## Contract Verification

All contracts are verified on the explorer for transparency:

**View Source Code**:
1. Go to explorer (e.g., `https://explorer-testnet.0g.ai`)
2. Search contract address
3. Click "Contract" tab
4. View verified source code

**Verify Your Own Contract**:
```bash
npx hardhat verify --network 0g-testnet CONTRACT_ADDRESS "Constructor Arg 1"
```

---

## Proxy Contracts

Some contracts use transparent proxy pattern for upgradeability:

| Contract | Implementation | Proxy |
|----------|---------------|-------|
| Agent Registry | `0x...` | `0x...` |
| Signal Registry | `0x...` | `0x...` |

**Always interact with the Proxy address**, not the implementation.

**Check Current Implementation**:
```python
# Get implementation address from proxy
implementation = w3.eth.get_storage_at(
    PROXY_ADDRESS,
    '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc'
)
print(f"Implementation: 0x{implementation.hex()[-40:]}")
```

---

## Deployment History

### Testnet

| Version | Date | Agent Registry | Signal Registry | Notes |
|---------|------|---------------|-----------------|-------|
| v1.0.0 | 2026-04-15 | `0x1234...` | `0x2345...` | Initial deployment |
| v1.1.0 | 2026-05-01 | `0x1234...` | `0x5678...` | Updated signal format |

### Mainnet

Coming soon after audit completion.

---

## Emergency Contacts

In case of contract issues:

- **Security Issues**: security@0g-signals.network
- **Technical Support**: support@0g-signals.network
- **Discord**: `https://discord.gg/0g-signals` (#contract-support)

---

## Audit Reports

Contract audits:

- **v1.0.0**: [View Audit Report →](../security/audits/v1.0.0.pdf)
- **v1.1.0**: 🔜 In progress

---

## Additional Resources

- [Contract Architecture →](./architecture.md)
- [Agent Registry Guide →](./agent-registry.md)
- [Integration Guide →](./integration.md)
- [GitHub Repository →](https://github.com/ogruns/0g-signals-contracts)

---

**Need Help?**

If you encounter issues with contract addresses or network connectivity:
1. Verify you're on the correct network
2. Check RPC endpoint is responsive
3. Ensure contract addresses are up to date
4. Contact support if issues persist
