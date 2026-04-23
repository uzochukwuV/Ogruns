package ingester

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/internal/contracts"
	"github.com/0xprotocol/verification-layer/internal/engine"
	"github.com/0xprotocol/verification-layer/pkg/types"
)

// Batcher acts as the "Layer 2" sequencer for the Verification Layer.
// It consumes raw signals from the API, injects them instantly into the Engine,
// and periodically bundles them into a single file to upload to 0G Storage.
type Batcher struct {
	storage       *StorageClient
	engine        *engine.ResolutionEngine
	contracts     *contracts.ContractManager
	verifyFn      VerifyFn
	batchInterval time.Duration

	mu      sync.Mutex
	signals []types.SignalEnvelope
}

func NewBatcher(storage *StorageClient, eng *engine.ResolutionEngine, cm *contracts.ContractManager, verify VerifyFn) *Batcher {
	return &Batcher{
		storage:       storage,
		engine:        eng,
		contracts:     cm,
		verifyFn:      verify,
		batchInterval: 24 * time.Hour, // Bundle signals daily to save massive gas
		signals:       make([]types.SignalEnvelope, 0),
	}
}

// Start listens to the API queue for incoming zero-gas signals
func (b *Batcher) Start(ctx context.Context, apiQueue <-chan types.SubmitRawSignalRequest) {
	log.Printf("Batcher: L2 Sequencer started. Batch interval: %s", b.batchInterval)

	ticker := time.NewTicker(b.batchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.flushTo0G(context.Background()) // Try to flush before shutdown
			return

		case req, ok := <-apiQueue:
			if !ok {
				return
			}
			b.processIncomingSignal(req)

		case <-ticker.C:
			b.flushTo0G(ctx)
		}
	}
}

func (b *Batcher) processIncomingSignal(req types.SubmitRawSignalRequest) {
	// 1. Cryptographic Verification
	ok, err := b.verifyFn(req.Envelope)
	if err != nil || !ok {
		log.Printf("Batcher: ❌ Rejected invalid signal from %s: %v", req.NodeID, err)
		return
	}

	// 2. Instant Injection (Web2 Speeds)
	b.engine.AddSignal(req.Envelope)

	// 3. Queue for Batching
	b.mu.Lock()
	b.signals = append(b.signals, req.Envelope)
	b.mu.Unlock()
}

// flushTo0G creates a single JSON bundle of all signals and uploads it to 0G Storage
func (b *Batcher) flushTo0G(ctx context.Context) {
	b.mu.Lock()
	if len(b.signals) == 0 {
		b.mu.Unlock()
		return
	}

	// Copy and clear the slice
	bundle := make([]types.SignalEnvelope, len(b.signals))
	copy(bundle, b.signals)
	b.signals = make([]types.SignalEnvelope, 0)
	b.mu.Unlock()

	// Create the Batch Object
	now := time.Now()
	batch := types.SignalBatch{
		BatchID:   fmt.Sprintf("batch_%s", now.Format("2006-01-02_15-04")),
		Timestamp: now.Unix(),
		Signals:   bundle,
	}

	// Write to temporary file
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s.json", batch.BatchID))
	defer os.Remove(tmpFile)

	data, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		log.Printf("Batcher: ❌ Failed to serialize batch: %v", err)
		return
	}

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		log.Printf("Batcher: ❌ Failed to write batch file: %v", err)
		return
	}

	// Upload to 0G Storage (1 Gas Fee for 1,000s of signals)
	rootHash, err := b.storage.UploadFile(ctx, tmpFile)
	if err != nil {
		log.Printf("Batcher: ❌ 0G Upload failed: %v", err)
		// Re-queue the bundle so we don't lose data
		b.mu.Lock()
		b.signals = append(bundle, b.signals...)
		b.mu.Unlock()
		return
	}

	log.Printf("Batcher: ✅ Successfully batched %d signals to 0G Storage. RootHash: %s", len(bundle), rootHash)

	// Write the 0G Root Hash to the NodeRegistry.sol smart contract
	if b.contracts != nil {
		err = b.contracts.PublishBatchHash(ctx, rootHash)
		if err != nil {
			log.Printf("Batcher: ⚠️ Failed to anchor batch hash on-chain: %v", err)
		} else {
			log.Printf("Batcher: 🔗 Batch anchored on-chain successfully.")
		}
	}
}
