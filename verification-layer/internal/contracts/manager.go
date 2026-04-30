package contracts

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ReputationOracle ABI definition for `updateScore(address,uint256,uint8)`
const reputationOracleABI = `[{"inputs":[{"internalType":"address","name":"nodeId","type":"address"},{"internalType":"uint256","name":"score","type":"uint256"},{"internalType":"uint8","name":"tier","type":"uint8"}],"name":"updateScore","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

// NodeRegistry ABI definitions for `registerNode`, `publishBatch`, and `nodes`
const nodeRegistryABI = `[{"inputs":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"tokenFocus","type":"string"},{"internalType":"string","name":"description","type":"string"}],"name":"registerNode","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"rootHash","type":"bytes32"}],"name":"publishBatch","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"nodes","outputs":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"tokenFocus","type":"string"},{"internalType":"string","name":"description","type":"string"},{"internalType":"address","name":"creator","type":"address"},{"internalType":"bool","name":"active","type":"bool"},{"internalType":"uint256","name":"registeredAt","type":"uint256"}],"stateMutability":"view","type":"function"}]`

type ContractManager struct {
	client          *ethclient.Client
	privateKey      *ecdsa.PrivateKey
	address         common.Address
	oracleABI       abi.ABI
	oracleAddress   common.Address
	registryABI     abi.ABI
	registryAddress common.Address
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

// RegisterNode registers a new signal node on the NodeRegistry contract
func (m *ContractManager) RegisterNode(ctx context.Context, name, tokenFocus, description string) error {
	if m.registryAddress == (common.Address{}) {
		return fmt.Errorf("registry address not configured")
	}

	data, err := m.registryABI.Pack("registerNode", name, tokenFocus, description)
	if err != nil {
		return err
	}

	return m.sendTransaction(ctx, m.registryAddress, data)
}

// GetNodeInfo queries the NodeRegistry mapping for the given node address.
func (m *ContractManager) GetNodeInfo(ctx context.Context, nodeID string) (string, string, string, common.Address, bool, *big.Int, error) {
	if m.registryAddress == (common.Address{}) {
		return "", "", "", common.Address{}, false, nil, fmt.Errorf("registry address not configured")
	}

	nodeAddr := common.HexToAddress(nodeID)
	data, err := m.registryABI.Pack("nodes", nodeAddr)
	if err != nil {
		return "", "", "", common.Address{}, false, nil, err
	}

	callMsg := ethereum.CallMsg{
		To:   &m.registryAddress,
		Data: data,
	}
	result, err := m.client.CallContract(ctx, callMsg, nil)
	if err != nil {
		return "", "", "", common.Address{}, false, nil, err
	}

	var out []interface{}
	if err := m.registryABI.UnpackIntoInterface(&out, "nodes", result); err != nil {
		return "", "", "", common.Address{}, false, nil, err
	}

	name := out[0].(string)
	tokenFocus := out[1].(string)
	description := out[2].(string)
	creator := out[3].(common.Address)
	active := out[4].(bool)
	registeredAt := out[5].(*big.Int)

	return name, tokenFocus, description, creator, active, registeredAt, nil
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

	// Wait for receipt and verify the transaction was not reverted.
	receipt, err := bind.WaitMined(ctx, m.client, signedTx)
	if err != nil {
		return err
	}
	if receipt.Status == 0 {
		return fmt.Errorf("transaction %s reverted (status=0)", signedTx.Hash().Hex())
	}
	return nil
}
