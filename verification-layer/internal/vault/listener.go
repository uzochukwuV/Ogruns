package vault

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/0xprotocol/verification-layer/internal/database"
)

// DepositListener polls the blockchain for PointVault Deposit events
// and credits user points in the database.
type DepositListener struct {
	client              *ethclient.Client
	db                  *database.DB
	contractAddress     common.Address
	depositEventHash    common.Hash
	pollInterval        time.Duration
	confirmations       uint64
	pointConversionRate int64 // points per 1 0G (default: 1000)
	stopChan            chan struct{}
	doneChan            chan struct{}
}

// DepositListenerConfig holds configuration for the DepositListener.
type DepositListenerConfig struct {
	RPCEndpoint         string
	ContractAddress     string
	PollInterval        time.Duration // default: 15 seconds
	Confirmations       uint64        // default: 6 blocks
	PointConversionRate int64         // default: 1000 points per 1 0G
}

// NewDepositListener creates a new DepositListener instance.
func NewDepositListener(ctx context.Context, cfg *DepositListenerConfig, db *database.DB) (*DepositListener, error) {
	client, err := ethclient.DialContext(ctx, cfg.RPCEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	// Compute Deposit event signature hash
	// event Deposit(address indexed user, string userId, uint256 amount)
	depositEventSignature := []byte("Deposit(address,string,uint256)")
	depositEventHash := crypto.Keccak256Hash(depositEventSignature)

	pollInterval := cfg.PollInterval
	if pollInterval == 0 {
		pollInterval = 15 * time.Second
	}

	confirmations := cfg.Confirmations
	if confirmations == 0 {
		confirmations = 6
	}

	pointConversionRate := cfg.PointConversionRate
	if pointConversionRate == 0 {
		pointConversionRate = 1000
	}

	return &DepositListener{
		client:              client,
		db:                  db,
		contractAddress:     common.HexToAddress(cfg.ContractAddress),
		depositEventHash:    depositEventHash,
		pollInterval:        pollInterval,
		confirmations:       confirmations,
		pointConversionRate: pointConversionRate,
		stopChan:            make(chan struct{}),
		doneChan:            make(chan struct{}),
	}, nil
}

// Start begins polling for deposit events.
func (dl *DepositListener) Start(ctx context.Context) {
	log.Println("[DepositListener] Starting deposit event listener...")
	go dl.pollLoop(ctx)
}

// Stop gracefully stops the listener.
func (dl *DepositListener) Stop() {
	log.Println("[DepositListener] Stopping deposit event listener...")
	close(dl.stopChan)
	<-dl.doneChan
	dl.client.Close()
	log.Println("[DepositListener] Deposit event listener stopped.")
}

// pollLoop is the main polling loop.
func (dl *DepositListener) pollLoop(ctx context.Context) {
	defer close(dl.doneChan)

	ticker := time.NewTicker(dl.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dl.stopChan:
			return
		case <-ticker.C:
			if err := dl.pollOnce(ctx); err != nil {
				log.Printf("[DepositListener] Poll error: %v", err)
			}
		}
	}
}

// pollOnce performs a single poll for deposit events.
func (dl *DepositListener) pollOnce(ctx context.Context) error {
	// Get the last processed block from the database
	lastProcessedBlock, err := dl.db.GetLastProcessedBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last processed block: %w", err)
	}

	// Get the latest block number
	latestBlock, err := dl.client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest block number: %w", err)
	}

	// Calculate the confirmed block (latestBlock - confirmations)
	if latestBlock < dl.confirmations {
		// Not enough blocks yet, skip
		return nil
	}
	confirmedBlock := latestBlock - dl.confirmations

	// If lastProcessedBlock is 0 (fresh start), start from confirmed block
	if lastProcessedBlock == 0 {
		lastProcessedBlock = confirmedBlock
		if err := dl.db.UpdateLastProcessedBlock(ctx, lastProcessedBlock); err != nil {
			return fmt.Errorf("failed to initialize last processed block: %w", err)
		}
		log.Printf("[DepositListener] Initialized at block %d", lastProcessedBlock)
		return nil
	}

	// If there are no new confirmed blocks, skip
	if confirmedBlock <= lastProcessedBlock {
		return nil
	}

	// Query logs from lastProcessedBlock+1 to confirmedBlock
	fromBlock := lastProcessedBlock + 1
	toBlock := confirmedBlock

	log.Printf("[DepositListener] Scanning blocks %d to %d for Deposit events...", fromBlock, toBlock)

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: []common.Address{dl.contractAddress},
		Topics:    [][]common.Hash{{dl.depositEventHash}},
	}

	logs, err := dl.client.FilterLogs(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to filter logs: %w", err)
	}

	log.Printf("[DepositListener] Found %d Deposit events", len(logs))

	// Process each deposit event
	for _, vlog := range logs {
		if err := dl.processDepositEvent(ctx, vlog); err != nil {
			log.Printf("[DepositListener] Error processing deposit event (tx=%s): %v", vlog.TxHash.Hex(), err)
			// Continue processing other events even if one fails
		}
	}

	// Update last processed block
	if err := dl.db.UpdateLastProcessedBlock(ctx, toBlock); err != nil {
		return fmt.Errorf("failed to update last processed block: %w", err)
	}

	log.Printf("[DepositListener] Updated last processed block to %d", toBlock)
	return nil
}

// processDepositEvent processes a single Deposit event.
func (dl *DepositListener) processDepositEvent(ctx context.Context, vlog types.Log) error {
	// Idempotency check: skip if already processed
	txHash := vlog.TxHash.Hex()
	processed, err := dl.db.IsDepositProcessed(ctx, txHash)
	if err != nil {
		return fmt.Errorf("failed to check if deposit processed: %w", err)
	}
	if processed {
		log.Printf("[DepositListener] Deposit already processed (tx=%s), skipping", txHash)
		return nil
	}

	// Decode event data
	// event Deposit(address indexed user, string userId, uint256 amount)
	// Topics: [eventHash, userAddress]
	// Data: [userId (string), amount (uint256)]
	if len(vlog.Topics) < 2 {
		return fmt.Errorf("invalid Deposit event: not enough topics")
	}

	userAddress := common.BytesToAddress(vlog.Topics[1].Bytes())

	// Parse the data field (userId + amount)
	// The data contains: userId (dynamic string) + amount (uint256)
	// ABI encoding: offset to string, amount, then string length + string data
	if len(vlog.Data) < 64 {
		return fmt.Errorf("invalid Deposit event: data too short")
	}

	// Parse userId and amount from ABI-encoded data
	userId, amount, err := dl.parseDepositEventData(vlog.Data)
	if err != nil {
		return fmt.Errorf("failed to parse deposit event data: %w", err)
	}

	// Calculate points: amountWei / 1e18 * pointConversionRate
	amountFloat := new(big.Float).SetInt(amount)
	oneEther := new(big.Float).SetInt(big.NewInt(1e18))
	amountOG := new(big.Float).Quo(amountFloat, oneEther)

	pointsFloat := new(big.Float).Mul(amountOG, big.NewFloat(float64(dl.pointConversionRate)))
	pointsInt, _ := pointsFloat.Int64()

	if pointsInt <= 0 {
		log.Printf("[DepositListener] Deposit amount too small, no points credited (tx=%s, amount=%s wei)", txHash, amount.String())
		return nil
	}

	log.Printf("[DepositListener] Processing deposit: user=%s, userId=%s, amount=%s wei, points=%d",
		userAddress.Hex(), userId, amount.String(), pointsInt)

	// Credit user points in the database (atomic transaction)
	if err := dl.db.CreditUserPoints(ctx, userId, pointsInt, txHash, amountOG); err != nil {
		return fmt.Errorf("failed to credit user points: %w", err)
	}

	// Record deposit event for idempotency
	if err := dl.db.RecordDepositEvent(ctx, txHash, vlog.BlockNumber, int(vlog.Index), userAddress.Hex(), userId, amount, pointsInt); err != nil {
		// If this fails, it's not critical since CreditUserPoints is already done
		// and we check IsDepositProcessed first
		log.Printf("[DepositListener] Warning: failed to record deposit event (tx=%s): %v", txHash, err)
	}

	log.Printf("[DepositListener] Successfully credited %d points to user %s (tx=%s)", pointsInt, userId, txHash)
	return nil
}

// parseDepositEventData parses the ABI-encoded event data.
// Data format: [offset_to_userId (32 bytes), amount (32 bytes), userId_length (32 bytes), userId_data...]
func (dl *DepositListener) parseDepositEventData(data []byte) (string, *big.Int, error) {
	if len(data) < 64 {
		return "", nil, fmt.Errorf("data too short")
	}

	// First 32 bytes: offset to userId (should be 0x40 = 64)
	// Next 32 bytes: amount
	amount := new(big.Int).SetBytes(data[32:64])

	// Remaining data contains the userId string
	if len(data) < 96 {
		return "", nil, fmt.Errorf("data too short for string")
	}

	// Next 32 bytes: userId string length
	userIdLength := new(big.Int).SetBytes(data[64:96])
	userIdLen := int(userIdLength.Int64())

	// Ensure we have enough data
	if len(data) < 96+userIdLen {
		return "", nil, fmt.Errorf("data too short for userId string")
	}

	// Extract userId bytes
	userIdBytes := data[96 : 96+userIdLen]
	userId := string(userIdBytes)

	return userId, amount, nil
}
