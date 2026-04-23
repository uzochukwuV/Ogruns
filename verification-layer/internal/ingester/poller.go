// Package ingester (poller.go) watches the NodeRegistry smart contract for
// SignalPublished(address indexed nodeId, bytes32 rootHash, uint256 timestamp)
// events, downloads the corresponding blob from 0G Storage, verifies its
// cryptographic signature, and injects valid signals into the ResolutionEngine.
//
// Discovery pattern (why events, not random polling):
//   - Nodes call NodeRegistry.publishSignal(rootHash) on-chain after uploading
//     their signal blob to 0G Storage.
//   - The verifier polls FilterLogs for that event topic, keyed to the registry
//     contract address, so it only downloads blobs that a registered node
//     explicitly announced.
//   - This is gas-cheap for nodes (~21k + 1 event) and gives the verifier an
//     ordered, replayable log of all published signals.
package ingester

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/0xprotocol/verification-layer/internal/engine"
	"github.com/0xprotocol/verification-layer/internal/scorer"
	"github.com/0xprotocol/verification-layer/pkg/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// SignalPublished event signature: keccak256("SignalPublished(address,bytes32,uint256)")
var signalPublishedTopic = crypto.Keccak256Hash(
	[]byte("SignalPublished(address,bytes32,uint256)"),
)

// VerifyFn is satisfied by crypto.VerifySignal — kept as a function type so
// the poller can be unit-tested with a stub.
type VerifyFn func(env types.SignalEnvelope) (bool, error)

// ScorerInterface is the subset of scorer.Scorer the poller needs.
type ScorerInterface interface {
	RecordClosed(sig *types.ActiveSignal) scorer.NodeStats
}

// Poller watches the NodeRegistry contract for new signal announcements.
type Poller struct {
	storage      *StorageClient
	evmClient    *ethclient.Client
	registryAddr common.Address
	pollInterval time.Duration
	// lastBlock tracks the highest processed block to avoid re-processing events.
	lastBlock uint64
}

// NewPoller creates a Poller.
//   - storage: initialised StorageClient for blob downloads.
//   - rpcURL:  0G EVM RPC (e.g. https://evmrpc-testnet.0g.ai).
//   - registryAddr: deployed NodeRegistry contract address.
func NewPoller(storage *StorageClient, rpcURL, registryAddr string) (*Poller, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("poller: dial rpc: %w", err)
	}
	return &Poller{
		storage:      storage,
		evmClient:    client,
		registryAddr: common.HexToAddress(registryAddr),
		pollInterval: 15 * time.Second,
	}, nil
}

// Start launches the polling loop. It is blocking and should be run in a goroutine.
//
//   - eng:    the ResolutionEngine that receives verified PENDING signals.
//   - sc:     the Scorer that should record closed signals when the engine emits them.
//   - verify: cryptographic verification function (crypto.VerifySignal).
func (p *Poller) Start(eng *engine.ResolutionEngine, sc ScorerInterface, verify VerifyFn) {
	// Also wire engine event stream to scorer in a separate goroutine.
	go p.drainEventStream(eng, sc)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	log.Printf("Poller: watching NodeRegistry %s every %s", p.registryAddr.Hex(), p.pollInterval)

	for range ticker.C {
		if err := p.poll(context.Background(), eng, verify); err != nil {
			log.Printf("Poller: poll error: %v", err)
		}
	}
}

// poll fetches SignalPublished events since lastBlock, downloads and verifies
// each blob, and passes valid signals to the engine.
func (p *Poller) poll(ctx context.Context, eng *engine.ResolutionEngine, verify VerifyFn) error {
	latest, err := p.evmClient.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("get latest block: %w", err)
	}
	if latest <= p.lastBlock {
		return nil
	}

	from := p.lastBlock + 1
	if p.lastBlock == 0 {
		// On first run, look back ~1 hour (≈240 blocks at 15s) to catch
		// any signals published before the verifier started.
		if latest > 240 {
			from = latest - 240
		} else {
			from = 0
		}
	}

	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(latest),
		Addresses: []common.Address{p.registryAddr},
		Topics:    [][]common.Hash{{signalPublishedTopic}},
	}

	logs, err := p.evmClient.FilterLogs(ctx, query)
	if err != nil {
		return fmt.Errorf("filter logs [%d–%d]: %w", from, latest, err)
	}

	log.Printf("Poller: scanned blocks %d–%d, found %d SignalPublished events", from, latest, len(logs))

	for _, l := range logs {
		// Topic[1] = indexed nodeId (address padded to 32 bytes)
		if len(l.Topics) < 2 {
			continue
		}
		nodeAddr := common.BytesToAddress(l.Topics[1].Bytes())

		// Data layout: bytes32 rootHash || uint256 timestamp  (ABI-encoded, non-indexed)
		if len(l.Data) < 64 {
			continue
		}
		rootHash := fmt.Sprintf("0x%x", l.Data[:32])

		if err := p.ingestBlob(ctx, eng, verify, nodeAddr.Hex(), rootHash); err != nil {
			log.Printf("Poller: ingest error for node=%s hash=%s: %v", nodeAddr.Hex(), rootHash, err)
		}
	}

	p.lastBlock = latest
	return nil
}

// ingestBlob downloads a blob from 0G Storage, parses and verifies it, then
// adds it to the resolution engine.
func (p *Poller) ingestBlob(
	ctx context.Context,
	eng *engine.ResolutionEngine,
	verify VerifyFn,
	nodeID, rootHash string,
) error {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("signal_%s.json", rootHash[2:10]))
	defer os.Remove(tmpFile)

	if err := p.storage.DownloadFile(ctx, rootHash, tmpFile); err != nil {
		return fmt.Errorf("download %s: %w", rootHash, err)
	}

	raw, err := os.ReadFile(tmpFile)
	if err != nil {
		return fmt.Errorf("read tmp file: %w", err)
	}

	var env types.SignalEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("parse envelope: %w", err)
	}

	// Safety: the node_id in the envelope must match the on-chain emitter.
	if common.HexToAddress(env.NodeID) != common.HexToAddress(nodeID) {
		return fmt.Errorf("node_id mismatch: envelope=%s event=%s", env.NodeID, nodeID)
	}

	ok, err := verify(env)
	if err != nil || !ok {
		return fmt.Errorf("signature invalid: %w", err)
	}

	eng.AddSignal(env)
	log.Printf("Poller: registered signal node=%s pair=%s dir=%s",
		env.NodeID, env.Payload.TokenPair, env.Payload.Direction)
	return nil
}

// drainEventStream reads the engine's event stream and forwards closed signals
// to the scorer so reputation scores stay up-to-date in real time.
func (p *Poller) drainEventStream(eng *engine.ResolutionEngine, sc ScorerInterface) {
	for sig := range eng.EventStream() {
		switch sig.State {
		case types.StateClosedWin, types.StateClosedLoss, types.StateExpired:
			stats := sc.RecordClosed(sig)
			log.Printf("Scorer: node=%s trust=%.2f tier=%s wins=%d losses=%d",
				stats.NodeID, stats.TrustScore, stats.Tier, stats.WinCount, stats.LossCount)
		}
	}
}
