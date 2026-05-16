package vault

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// VaultContract handles on-chain interactions with the PointVault smart contract.
type VaultContract struct {
	client      *ethclient.Client
	contractABI *abi.ABI
	address     common.Address
	privateKey  *ecdsa.PrivateKey
	chainID     *big.Int
}

// Config holds configuration for VaultContract initialization.
type Config struct {
	RPCEndpoint     string
	ContractAddress string
	PrivateKey      string
}

// New creates a new VaultContract instance.
func New(ctx context.Context, cfg *Config) (*VaultContract, error) {
	client, err := ethclient.DialContext(ctx, cfg.RPCEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	contractABI, err := abi.JSON(strings.NewReader(PointVaultABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %w", err)
	}

	return &VaultContract{
		client:      client,
		contractABI: &contractABI,
		address:     common.HexToAddress(cfg.ContractAddress),
		privateKey:  privateKey,
		chainID:     chainID,
	}, nil
}

// Close closes the RPC client connection.
func (vc *VaultContract) Close() {
	if vc.client != nil {
		vc.client.Close()
	}
}

// GetBalance returns the current balance of the PointVault contract in wei.
func (vc *VaultContract) GetBalance(ctx context.Context) (*big.Int, error) {
	balance, err := vc.client.BalanceAt(ctx, vc.address, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault balance: %w", err)
	}
	return balance, nil
}

// GetAutoThreshold returns the current auto-approval threshold in wei.
func (vc *VaultContract) GetAutoThreshold(ctx context.Context) (*big.Int, error) {
	data, err := vc.contractABI.Pack("autoThreshold")
	if err != nil {
		return nil, fmt.Errorf("failed to pack autoThreshold call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &vc.address,
		Data: data,
	}

	result, err := vc.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call autoThreshold: %w", err)
	}

	threshold := new(big.Int).SetBytes(result)
	return threshold, nil
}

// Withdraw executes an immediate withdrawal for amounts <= autoThreshold.
// Returns the transaction hash.
func (vc *VaultContract) Withdraw(ctx context.Context, provider common.Address, amount *big.Int, pointsRedeemed *big.Int) (string, error) {
	data, err := vc.contractABI.Pack("withdraw", provider, amount, pointsRedeemed)
	if err != nil {
		return "", fmt.Errorf("failed to pack withdraw call: %w", err)
	}

	txHash, err := vc.sendTransaction(ctx, data)
	if err != nil {
		return "", fmt.Errorf("withdraw transaction failed: %w", err)
	}

	return txHash, nil
}

// QueueWithdrawal queues a withdrawal for amounts > autoThreshold.
// Returns the on-chain withdrawal ID and transaction hash.
func (vc *VaultContract) QueueWithdrawal(ctx context.Context, provider common.Address, amount *big.Int, pointsRedeemed *big.Int) (uint64, string, error) {
	data, err := vc.contractABI.Pack("queueWithdrawal", provider, amount, pointsRedeemed)
	if err != nil {
		return 0, "", fmt.Errorf("failed to pack queueWithdrawal call: %w", err)
	}

	txHash, err := vc.sendTransaction(ctx, data)
	if err != nil {
		return 0, "", fmt.Errorf("queueWithdrawal transaction failed: %w", err)
	}

	// Parse the transaction receipt to extract the withdrawal ID from WithdrawalQueued event
	receipt, err := vc.waitForReceipt(ctx, txHash)
	if err != nil {
		return 0, txHash, fmt.Errorf("failed to get receipt: %w", err)
	}

	// Find WithdrawalQueued event and extract ID
	withdrawalID, err := vc.parseWithdrawalQueuedEvent(receipt)
	if err != nil {
		return 0, txHash, fmt.Errorf("failed to parse withdrawal ID: %w", err)
	}

	return withdrawalID, txHash, nil
}

// ExecuteQueuedWithdrawal executes a queued withdrawal after the timelock has passed.
// Returns the transaction hash.
func (vc *VaultContract) ExecuteQueuedWithdrawal(ctx context.Context, withdrawalID uint64) (string, error) {
	data, err := vc.contractABI.Pack("executeQueuedWithdrawal", new(big.Int).SetUint64(withdrawalID))
	if err != nil {
		return "", fmt.Errorf("failed to pack executeQueuedWithdrawal call: %w", err)
	}

	txHash, err := vc.sendTransaction(ctx, data)
	if err != nil {
		return "", fmt.Errorf("executeQueuedWithdrawal transaction failed: %w", err)
	}

	return txHash, nil
}

// PublishSolvencyProof publishes a solvency proof on-chain.
// Returns the transaction hash.
func (vc *VaultContract) PublishSolvencyProof(ctx context.Context, merkleRoot [32]byte, totalPoints *big.Int, totalValueLocked *big.Int) (string, error) {
	data, err := vc.contractABI.Pack("publishSolvencyProof", merkleRoot, totalPoints, totalValueLocked)
	if err != nil {
		return "", fmt.Errorf("failed to pack publishSolvencyProof call: %w", err)
	}

	txHash, err := vc.sendTransaction(ctx, data)
	if err != nil {
		return "", fmt.Errorf("publishSolvencyProof transaction failed: %w", err)
	}

	return txHash, nil
}

// sendTransaction sends a transaction to the PointVault contract using EIP-155 signing.
// Returns the transaction hash as a hex string.
func (vc *VaultContract) sendTransaction(ctx context.Context, data []byte) (string, error) {
	fromAddress := crypto.PubkeyToAddress(vc.privateKey.PublicKey)

	// Get nonce
	nonce, err := vc.client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	// Estimate gas
	gasLimit, err := vc.client.EstimateGas(ctx, ethereum.CallMsg{
		From: fromAddress,
		To:   &vc.address,
		Data: data,
	})
	if err != nil {
		return "", fmt.Errorf("failed to estimate gas: %w", err)
	}

	// Get gas price
	gasPrice, err := vc.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Create transaction
	tx := types.NewTransaction(nonce, vc.address, big.NewInt(0), gasLimit, gasPrice, data)

	// Sign with EIP-155
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(vc.chainID), vc.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	if err := vc.client.SendTransaction(ctx, signedTx); err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return signedTx.Hash().Hex(), nil
}

// waitForReceipt waits for a transaction receipt.
func (vc *VaultContract) waitForReceipt(ctx context.Context, txHash string) (*types.Receipt, error) {
	hash := common.HexToHash(txHash)
	receipt, err := vc.client.TransactionReceipt(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}
	return receipt, nil
}

// parseWithdrawalQueuedEvent parses the WithdrawalQueued event from a transaction receipt.
func (vc *VaultContract) parseWithdrawalQueuedEvent(receipt *types.Receipt) (uint64, error) {
	eventSignature := []byte("WithdrawalQueued(uint256,address,uint256,uint256,uint256,uint256)")
	eventHash := crypto.Keccak256Hash(eventSignature)

	for _, log := range receipt.Logs {
		if log.Topics[0] == eventHash {
			// First topic is event signature, second is withdrawal ID
			if len(log.Topics) < 2 {
				continue
			}
			withdrawalID := new(big.Int).SetBytes(log.Topics[1].Bytes())
			return withdrawalID.Uint64(), nil
		}
	}

	return 0, fmt.Errorf("WithdrawalQueued event not found in receipt")
}

// PointVaultABI is the ABI for the PointVault contract.
const PointVaultABI = `[
  {
    "type": "function",
    "name": "autoThreshold",
    "inputs": [],
    "outputs": [{"name": "", "type": "uint256"}],
    "stateMutability": "view"
  },
  {
    "type": "function",
    "name": "withdraw",
    "inputs": [
      {"name": "provider", "type": "address"},
      {"name": "amount", "type": "uint256"},
      {"name": "pointsRedeemed", "type": "uint256"}
    ],
    "outputs": [],
    "stateMutability": "nonpayable"
  },
  {
    "type": "function",
    "name": "queueWithdrawal",
    "inputs": [
      {"name": "provider", "type": "address"},
      {"name": "amount", "type": "uint256"},
      {"name": "pointsRedeemed", "type": "uint256"}
    ],
    "outputs": [{"name": "withdrawalId", "type": "uint256"}],
    "stateMutability": "nonpayable"
  },
  {
    "type": "function",
    "name": "executeQueuedWithdrawal",
    "inputs": [
      {"name": "withdrawalId", "type": "uint256"}
    ],
    "outputs": [],
    "stateMutability": "nonpayable"
  },
  {
    "type": "function",
    "name": "publishSolvencyProof",
    "inputs": [
      {"name": "merkleRoot", "type": "bytes32"},
      {"name": "totalPoints", "type": "uint256"},
      {"name": "totalValueLocked", "type": "uint256"}
    ],
    "outputs": [],
    "stateMutability": "nonpayable"
  },
  {
    "type": "event",
    "name": "Deposit",
    "inputs": [
      {"name": "user", "type": "address", "indexed": true},
      {"name": "userId", "type": "string", "indexed": false},
      {"name": "amount", "type": "uint256", "indexed": false}
    ]
  },
  {
    "type": "event",
    "name": "WithdrawalQueued",
    "inputs": [
      {"name": "withdrawalId", "type": "uint256", "indexed": true},
      {"name": "provider", "type": "address", "indexed": true},
      {"name": "amount", "type": "uint256", "indexed": false},
      {"name": "pointsRedeemed", "type": "uint256", "indexed": false},
      {"name": "queuedAt", "type": "uint256", "indexed": false},
      {"name": "executeAfter", "type": "uint256", "indexed": false}
    ]
  }
]`
