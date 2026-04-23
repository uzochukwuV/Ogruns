package contracts

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ReputationOracle ABI definition for `updateScore(address,uint256,uint8)`
const reputationOracleABI = `[{"inputs":[{"internalType":"address","name":"nodeId","type":"address"},{"internalType":"uint256","name":"score","type":"uint256"},{"internalType":"uint8","name":"tier","type":"uint8"}],"name":"updateScore","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

// NodeRegistry ABI definition for `publishBatch(bytes32)`
const nodeRegistryABI = `[{"inputs":[{"internalType":"bytes32","name":"rootHash","type":"bytes32"}],"name":"publishBatch","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

type ContractManager struct {
	client           *ethclient.Client
	privateKey       *ecdsa.PrivateKey
	address          common.Address
	oracleABI        abi.ABI
	oracleAddress    common.Address
	registryABI      abi.ABI
	registryAddress  common.Address
}

func NewContractManager(rpcURL, privKeyHex, oracleAddr, registryAddr string) (*ContractManager, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	privateKey, err := crypto.HexToECDSA(privKeyHex)
	if err != nil {
		return nil, err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	parsedOracleABI, err := abi.JSON(strings.NewReader(reputationOracleABI))
	if err != nil {
		return nil, err
	}

	parsedRegistryABI, err := abi.JSON(strings.NewReader(nodeRegistryABI))
	if err != nil {
		return nil, err
	}

	return &ContractManager{
		client:          client,
		privateKey:      privateKey,
		address:         address,
		oracleABI:       parsedOracleABI,
		oracleAddress:   common.HexToAddress(oracleAddr),
		registryABI:     parsedRegistryABI,
		registryAddress: common.HexToAddress(registryAddr),
	}, nil
}

// PublishBatchHash anchors the daily 0G Storage Root Hash on-chain
func (m *ContractManager) PublishBatchHash(ctx context.Context, rootHashHex string) error {
	if m.registryAddress == (common.Address{}) {
		return fmt.Errorf("registry address not configured")
	}

	var rootHash [32]byte
	copy(rootHash[:], common.FromHex(rootHashHex))

	data, err := m.registryABI.Pack("publishBatch", rootHash)
	if err != nil {
		return err
	}

	return m.sendTransaction(ctx, m.registryAddress, data)
}

// UpdateNodeScore pushes a Node's new trust score to the ReputationOracle contract
func (m *ContractManager) UpdateNodeScore(ctx context.Context, nodeID string, score float64, tier uint8) error {
	if m.oracleAddress == (common.Address{}) {
		return fmt.Errorf("oracle address not configured")
	}

	nodeAddr := common.HexToAddress(nodeID)
	// Convert score (e.g. 75.42) to uint256 with 2 decimals (7542)
	scoreInt := big.NewInt(int64(score * 100))

	data, err := m.oracleABI.Pack("updateScore", nodeAddr, scoreInt, tier)
	if err != nil {
		return err
	}

	return m.sendTransaction(ctx, m.oracleAddress, data)
}

func (m *ContractManager) sendTransaction(ctx context.Context, to common.Address, data []byte) error {
	nonce, err := m.client.PendingNonceAt(ctx, m.address)
	if err != nil {
		return err
	}

	gasPrice, err := m.client.SuggestGasPrice(ctx)
	if err != nil {
		return err
	}

	// 0G Testnet typical gas limits
	gasLimit := uint64(300000) 

	tx := types.NewTransaction(nonce, to, big.NewInt(0), gasLimit, gasPrice, data)

	chainID, err := m.client.NetworkID(ctx)
	if err != nil {
		return err
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), m.privateKey)
	if err != nil {
		return err
	}

	err = m.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return err
	}

	// Wait for receipt (simplified for MVP)
	_, err = bind.WaitMined(ctx, m.client, signedTx)
	return err
}
