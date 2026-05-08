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

// AgentRegistry ABI definitions
// Includes Agentic ID verification methods and backward-compatible node methods
const agentRegistryABI = `[
  {"inputs":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"tokenFocus","type":"string"},{"internalType":"string","name":"description","type":"string"}],"name":"registerNode","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"tokenFocus","type":"string"},{"internalType":"string","name":"description","type":"string"}],"name":"registerAgent","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"tokenFocus","type":"string"},{"internalType":"string","name":"description","type":"string"},{"internalType":"uint256","name":"agenticId","type":"uint256"}],"name":"registerVerifiedAgent","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"bytes32","name":"rootHash","type":"bytes32"}],"name":"publishBatch","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"nodes","outputs":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"tokenFocus","type":"string"},{"internalType":"string","name":"description","type":"string"},{"internalType":"address","name":"creator","type":"address"},{"internalType":"bool","name":"active","type":"bool"},{"internalType":"uint256","name":"registeredAt","type":"uint256"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"agents","outputs":[{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"tokenFocus","type":"string"},{"internalType":"string","name":"description","type":"string"},{"internalType":"address","name":"creator","type":"address"},{"internalType":"bool","name":"active","type":"bool"},{"internalType":"uint256","name":"registeredAt","type":"uint256"},{"internalType":"uint256","name":"agenticId","type":"uint256"},{"internalType":"bool","name":"isVerified","type":"bool"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"internalType":"address","name":"agentId","type":"address"}],"name":"isVerifiedAgent","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},
  {"inputs":[{"internalType":"address","name":"agentId","type":"address"}],"name":"getAgenticId","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}
]`

// SubscriptionManager ABI for subscription verification
const subscriptionManagerABI = `[{"inputs":[{"internalType":"address","name":"user","type":"address"},{"internalType":"address","name":"nodeId","type":"address"}],"name":"isSubscribed","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"user","type":"address"},{"internalType":"address","name":"nodeId","type":"address"}],"name":"subscriptionEnd","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"nodeId","type":"address"}],"name":"priceFor","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

type ContractManager struct {
	client              *ethclient.Client
	privateKey          *ecdsa.PrivateKey
	address             common.Address
	oracleABI           abi.ABI
	oracleAddress       common.Address
	registryABI         abi.ABI
	registryAddress     common.Address
	subscriptionABI     abi.ABI
	subscriptionAddress common.Address
}

func NewContractManager(rpcURL, privKeyHex, oracleAddr, registryAddr string) (*ContractManager, error) {
	return NewContractManagerWithSubscription(rpcURL, privKeyHex, oracleAddr, registryAddr, "")
}

func NewContractManagerWithSubscription(rpcURL, privKeyHex, oracleAddr, registryAddr, subscriptionAddr string) (*ContractManager, error) {
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

	parsedRegistryABI, err := abi.JSON(strings.NewReader(agentRegistryABI))
	if err != nil {
		return nil, err
	}

	parsedSubscriptionABI, err := abi.JSON(strings.NewReader(subscriptionManagerABI))
	if err != nil {
		return nil, err
	}

	return &ContractManager{
		client:              client,
		privateKey:          privateKey,
		address:             address,
		oracleABI:           parsedOracleABI,
		oracleAddress:       common.HexToAddress(oracleAddr),
		registryABI:         parsedRegistryABI,
		registryAddress:     common.HexToAddress(registryAddr),
		subscriptionABI:     parsedSubscriptionABI,
		subscriptionAddress: common.HexToAddress(subscriptionAddr),
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

// RegisterNode registers a new signal agent on the AgentRegistry contract
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

// GetNodeInfo queries the AgentRegistry mapping for the given agent address.
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

// ── Subscription Manager Methods ─────────────────────────────────────────────

// SubscriptionInfo contains details about a user's subscription to a node
type SubscriptionInfo struct {
	IsSubscribed bool
	EndTime      *big.Int
}

// IsSubscribed checks if a user address is subscribed to a node on-chain
func (m *ContractManager) IsSubscribed(ctx context.Context, userAddr, nodeID string) (bool, error) {
	if m.subscriptionAddress == (common.Address{}) {
		// No subscription contract configured - allow access (dev mode)
		return true, nil
	}

	user := common.HexToAddress(userAddr)
	node := common.HexToAddress(nodeID)

	data, err := m.subscriptionABI.Pack("isSubscribed", user, node)
	if err != nil {
		return false, fmt.Errorf("failed to pack isSubscribed: %w", err)
	}

	callMsg := ethereum.CallMsg{
		To:   &m.subscriptionAddress,
		Data: data,
	}

	result, err := m.client.CallContract(ctx, callMsg, nil)
	if err != nil {
		return false, fmt.Errorf("contract call failed: %w", err)
	}

	var subscribed bool
	if err := m.subscriptionABI.UnpackIntoInterface(&subscribed, "isSubscribed", result); err != nil {
		return false, fmt.Errorf("failed to unpack result: %w", err)
	}

	return subscribed, nil
}

// GetSubscriptionEnd returns the subscription end timestamp for a user-node pair
func (m *ContractManager) GetSubscriptionEnd(ctx context.Context, userAddr, nodeID string) (*big.Int, error) {
	if m.subscriptionAddress == (common.Address{}) {
		return big.NewInt(0), nil
	}

	user := common.HexToAddress(userAddr)
	node := common.HexToAddress(nodeID)

	data, err := m.subscriptionABI.Pack("subscriptionEnd", user, node)
	if err != nil {
		return nil, err
	}

	callMsg := ethereum.CallMsg{
		To:   &m.subscriptionAddress,
		Data: data,
	}

	result, err := m.client.CallContract(ctx, callMsg, nil)
	if err != nil {
		return nil, err
	}

	var endTime *big.Int
	if err := m.subscriptionABI.UnpackIntoInterface(&endTime, "subscriptionEnd", result); err != nil {
		return nil, err
	}

	return endTime, nil
}

// GetSubscriptionPrice returns the price to subscribe to a node
func (m *ContractManager) GetSubscriptionPrice(ctx context.Context, nodeID string) (*big.Int, error) {
	if m.subscriptionAddress == (common.Address{}) {
		return big.NewInt(0), nil
	}

	node := common.HexToAddress(nodeID)

	data, err := m.subscriptionABI.Pack("priceFor", node)
	if err != nil {
		return nil, err
	}

	callMsg := ethereum.CallMsg{
		To:   &m.subscriptionAddress,
		Data: data,
	}

	result, err := m.client.CallContract(ctx, callMsg, nil)
	if err != nil {
		return nil, err
	}

	var price *big.Int
	if err := m.subscriptionABI.UnpackIntoInterface(&price, "priceFor", result); err != nil {
		return nil, err
	}

	return price, nil
}

// HasSubscriptionContract returns whether the subscription contract is configured
func (m *ContractManager) HasSubscriptionContract() bool {
	return m.subscriptionAddress != (common.Address{})
}

// ── Agent Verification Methods (Agentic ID Integration) ─────────────────────

// AgentInfo contains details about a registered AI trading signal agent
type AgentInfo struct {
	Name         string
	TokenFocus   string
	Description  string
	Creator      common.Address
	Active       bool
	RegisteredAt *big.Int
	AgenticId    *big.Int
	IsVerified   bool
}

// IsVerifiedAgent checks if an agent has a linked ERC-7857 Agentic ID
func (m *ContractManager) IsVerifiedAgent(ctx context.Context, agentID string) (bool, error) {
	if m.registryAddress == (common.Address{}) {
		return false, nil
	}

	agentAddr := common.HexToAddress(agentID)
	data, err := m.registryABI.Pack("isVerifiedAgent", agentAddr)
	if err != nil {
		// Method might not exist on older contracts, return false
		return false, nil
	}

	callMsg := ethereum.CallMsg{
		To:   &m.registryAddress,
		Data: data,
	}

	result, err := m.client.CallContract(ctx, callMsg, nil)
	if err != nil {
		// Contract might not have this method, return false
		return false, nil
	}

	var isVerified bool
	if err := m.registryABI.UnpackIntoInterface(&isVerified, "isVerifiedAgent", result); err != nil {
		return false, nil
	}

	return isVerified, nil
}

// GetAgenticId returns the ERC-7857 token ID for an agent (0 if not verified)
func (m *ContractManager) GetAgenticId(ctx context.Context, agentID string) (*big.Int, error) {
	if m.registryAddress == (common.Address{}) {
		return big.NewInt(0), nil
	}

	agentAddr := common.HexToAddress(agentID)
	data, err := m.registryABI.Pack("getAgenticId", agentAddr)
	if err != nil {
		return big.NewInt(0), nil
	}

	callMsg := ethereum.CallMsg{
		To:   &m.registryAddress,
		Data: data,
	}

	result, err := m.client.CallContract(ctx, callMsg, nil)
	if err != nil {
		return big.NewInt(0), nil
	}

	var agenticId *big.Int
	if err := m.registryABI.UnpackIntoInterface(&agenticId, "getAgenticId", result); err != nil {
		return big.NewInt(0), nil
	}

	return agenticId, nil
}

// GetAgentInfo returns full agent information including Agentic ID status
func (m *ContractManager) GetAgentInfo(ctx context.Context, agentID string) (*AgentInfo, error) {
	if m.registryAddress == (common.Address{}) {
		return nil, fmt.Errorf("registry address not configured")
	}

	agentAddr := common.HexToAddress(agentID)
	data, err := m.registryABI.Pack("agents", agentAddr)
	if err != nil {
		// Fall back to old "nodes" method
		return m.getNodeInfoAsAgent(ctx, agentID)
	}

	callMsg := ethereum.CallMsg{
		To:   &m.registryAddress,
		Data: data,
	}

	result, err := m.client.CallContract(ctx, callMsg, nil)
	if err != nil {
		// Fall back to old "nodes" method
		return m.getNodeInfoAsAgent(ctx, agentID)
	}

	var out []interface{}
	if err := m.registryABI.UnpackIntoInterface(&out, "agents", result); err != nil {
		return m.getNodeInfoAsAgent(ctx, agentID)
	}

	return &AgentInfo{
		Name:         out[0].(string),
		TokenFocus:   out[1].(string),
		Description:  out[2].(string),
		Creator:      out[3].(common.Address),
		Active:       out[4].(bool),
		RegisteredAt: out[5].(*big.Int),
		AgenticId:    out[6].(*big.Int),
		IsVerified:   out[7].(bool),
	}, nil
}

// getNodeInfoAsAgent falls back to the old nodes() method for backward compatibility
func (m *ContractManager) getNodeInfoAsAgent(ctx context.Context, agentID string) (*AgentInfo, error) {
	name, tokenFocus, description, creator, active, registeredAt, err := m.GetNodeInfo(ctx, agentID)
	if err != nil {
		return nil, err
	}

	return &AgentInfo{
		Name:         name,
		TokenFocus:   tokenFocus,
		Description:  description,
		Creator:      creator,
		Active:       active,
		RegisteredAt: registeredAt,
		AgenticId:    big.NewInt(0),
		IsVerified:   false,
	}, nil
}
