// Package broadcaster periodically writes finalized Node Trust Scores from the
// Scorer to the ReputationOracle smart contract on 0G Network. This is the
// single trust anchor that the SubscriptionManager and FeeRouter read from to
// determine pricing and fee splits.
//
// Flow:
//  1. Scorer.ForBroadcast() → []NodeStats with current scores + tiers.
//  2. For each node, call ReputationOracle.updateScore(nodeId, score, tier).
//  3. Transaction is signed with the verifier's private key (the contract's
//     authorised "oracle" address).
package broadcaster

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/0xprotocol/verification-layer/internal/scorer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// reputationOracleABI is the minimal ABI fragment for ReputationOracle.sol.
// Only the functions this broadcaster calls are required here.
const reputationOracleABI = `[
  {
    "inputs": [
      {"internalType": "address", "name": "nodeId",  "type": "address"},
      {"internalType": "uint256", "name": "score",   "type": "uint256"},
      {"internalType": "uint8",   "name": "tier",    "type": "uint8"}
    ],
    "name": "updateScore",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  }
]`

// Broadcaster signs and submits on-chain score updates.
type Broadcaster struct {
	client       *ethclient.Client
	contractAddr common.Address
	parsedABI    abi.ABI
	auth         *bind.TransactOpts
	chainID      *big.Int
}

// New creates a Broadcaster connected to the given RPC endpoint.
// privateKey is the verifier's hex-encoded secp256k1 key (no 0x prefix).
// contractAddr is the deployed ReputationOracle address.
func New(rpcURL, privateKeyHex, contractAddr string) (*Broadcaster, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("broadcaster: dial rpc: %w", err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("broadcaster: get chain id: %w", err)
	}

	privKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("broadcaster: parse private key: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("broadcaster: create transactor: %w", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(reputationOracleABI))
	if err != nil {
		return nil, fmt.Errorf("broadcaster: parse abi: %w", err)
	}

	return &Broadcaster{
		client:       client,
		contractAddr: common.HexToAddress(contractAddr),
		parsedABI:    parsedABI,
		auth:         auth,
		chainID:      chainID,
	}, nil
}

// BroadcastScore sends a single updateScore transaction for one node.
func (b *Broadcaster) BroadcastScore(nodeID string, trustScore float64, tier scorer.Tier) error {
	// Convert score to uint256 with 2 decimal places of precision (e.g. 75.42 → 7542).
	scoreBig := big.NewInt(int64(trustScore * 100))
	tierUint8 := scorer.TierIndex[tier]
	nodeAddr := common.HexToAddress(nodeID)

	// Encode the ABI call data.
	data, err := b.parsedABI.Pack("updateScore", nodeAddr, scoreBig, tierUint8)
	if err != nil {
		return fmt.Errorf("broadcaster: pack abi: %w", err)
	}

	nonce, err := b.client.PendingNonceAt(context.Background(), b.auth.From)
	if err != nil {
		return fmt.Errorf("broadcaster: nonce: %w", err)
	}

	gasPrice, err := b.client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("broadcaster: gas price: %w", err)
	}

	tx := types.NewTransaction(
		nonce,
		b.contractAddr,
		big.NewInt(0), // no ETH value
		200_000,       // gas limit — updateScore is a simple mapping write
		gasPrice,
		data,
	)

	signedTx, err := b.auth.Signer(b.auth.From, tx)
	if err != nil {
		return fmt.Errorf("broadcaster: sign tx: %w", err)
	}

	if err := b.client.SendTransaction(context.Background(), signedTx); err != nil {
		return fmt.Errorf("broadcaster: send tx for %s: %w", nodeID, err)
	}

	log.Printf("Broadcaster: updated %s score=%.2f tier=%s tx=%s",
		nodeID, trustScore, tier, signedTx.Hash().Hex())
	return nil
}

// RunPeriodicBroadcast calls BroadcastScore for every tracked node on each
// tick of the given interval. Intended to be run in a goroutine.
//
// Trade-off: broadcasting every node on every tick costs gas. In production,
// only broadcast when the tier *changes* (upgrade/downgrade) — not the score
// itself — since only tier transitions affect subscription pricing.
func (b *Broadcaster) RunPeriodicBroadcast(sc *scorer.Scorer, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Track the last-broadcast tier per node so we only submit on change.
	lastTier := make(map[string]scorer.Tier)

	for range ticker.C {
		for _, stats := range sc.ForBroadcast() {
			prev, known := lastTier[stats.NodeID]
			if known && prev == stats.Tier {
				// No tier change — skip to save gas.
				continue
			}
			if err := b.BroadcastScore(stats.NodeID, stats.TrustScore, stats.Tier); err != nil {
				log.Printf("Broadcaster: error for %s: %v", stats.NodeID, err)
				continue
			}
			lastTier[stats.NodeID] = stats.Tier
		}
	}
}
