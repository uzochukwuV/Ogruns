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
	"github.com/0xprotocol/verification-layer/internal/scheduler"
	"github.com/0xprotocol/verification-layer/pkg/types"
)

// SchedulerBatcher is the L2 sequencer that uses the new scheduled analysis system.
// It consumes raw signals from the API, schedules them for expiry-time analysis,
// and periodically batches them to 0G Storage.
type SchedulerBatcher struct {
	storage       *StorageClient
	scheduler     *scheduler.SignalScheduler
	handler       *scheduler.AnalysisHandler
	contracts     *contracts.ContractManager
	verifyFn      VerifyFn
	batchInterval time.Duration

	mu      sync.Mutex
	signals []types.SignalEnvelope

	// Event stream for WebSocket clients
	eventStream chan *types.ActiveSignal
}

// NewSchedulerBatcher creates a new batcher that uses scheduled analysis
func NewSchedulerBatcher(
	storage *StorageClient,
	priceFetcher scheduler.PriceFetcher,
	cm *contracts.ContractManager,
	verify VerifyFn,
) *SchedulerBatcher {
	eventStream := make(chan *types.ActiveSignal, 1000)

	// Create the analysis handler
	handler := scheduler.NewAnalysisHandler()

	// If we have a contract manager, set it as the on-chain broadcaster
	if cm != nil {
		handler.SetBroadcaster(cm)
	}

	// Create result handler that emits to event stream and delegates to handler
	resultHandler := func(result scheduler.AnalysisResult) {
		// Send to event stream for WebSocket clients
		select {
		case eventStream <- result.Signal:
		default:
			log.Printf("SchedulerBatcher: Event stream full, dropping event")
		}

		// Process with the analysis handler (scoring, webhooks, on-chain)
		handler.HandleResult(result)
	}

	// Create the scheduler with our result handler
	sched := scheduler.NewSignalScheduler(priceFetcher, resultHandler)

	return &SchedulerBatcher{
		storage:       storage,
		scheduler:     sched,
		handler:       handler,
		contracts:     cm,
		verifyFn:      verify,
		batchInterval: 24 * time.Hour,
		signals:       make([]types.SignalEnvelope, 0),
		eventStream:   eventStream,
	}
}

// Start listens to the API queue for incoming zero-gas signals
func (b *SchedulerBatcher) Start(ctx context.Context, apiQueue <-chan types.SubmitRawSignalRequest) {
	log.Printf("SchedulerBatcher: L2 Sequencer started with scheduled analysis. Batch interval: %s", b.batchInterval)

	ticker := time.NewTicker(b.batchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.flushTo0G(context.Background()) // Try to flush before shutdown
			b.scheduler.Stop()
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

func (b *SchedulerBatcher) processIncomingSignal(req types.SubmitRawSignalRequest) {
	// 1. Cryptographic Verification
	ok, err := b.verifyFn(req.Envelope)
	if err != nil || !ok {
		log.Printf("SchedulerBatcher: ❌ Rejected invalid signal from %s: %v", req.NodeID, err)
		return
	}

	// 2. Schedule for expiry-time analysis (replaces real-time engine)
	if err := b.scheduler.AddSignal(req.Envelope); err != nil {
		log.Printf("SchedulerBatcher: ⚠️ Failed to schedule signal: %v", err)
		return
	}

	// 3. Queue for Batching
	b.mu.Lock()
	b.signals = append(b.signals, req.Envelope)
	b.mu.Unlock()

	log.Printf("SchedulerBatcher: ✅ Signal from %s scheduled for analysis at expiry", req.NodeID[:16]+"...")
}

// flushTo0G creates a single JSON bundle of all signals and uploads it to 0G Storage
func (b *SchedulerBatcher) flushTo0G(ctx context.Context) {
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
		log.Printf("SchedulerBatcher: ❌ Failed to serialize batch: %v", err)
		return
	}

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		log.Printf("SchedulerBatcher: ❌ Failed to write batch file: %v", err)
		return
	}

	// Upload to 0G Storage (1 Gas Fee for 1,000s of signals)
	rootHash, err := b.storage.UploadFile(ctx, tmpFile)
	if err != nil {
		log.Printf("SchedulerBatcher: ❌ 0G Upload failed: %v", err)
		// Re-queue the bundle so we don't lose data
		b.mu.Lock()
		b.signals = append(bundle, b.signals...)
		b.mu.Unlock()
		return
	}

	log.Printf("SchedulerBatcher: ✅ Successfully batched %d signals to 0G Storage. RootHash: %s", len(bundle), rootHash)

	// Write the 0G Root Hash to the NodeRegistry.sol smart contract
	if b.contracts != nil {
		err = b.contracts.PublishBatchHash(ctx, rootHash)
		if err != nil {
			log.Printf("SchedulerBatcher: ⚠️ Failed to anchor batch hash on-chain: %v", err)
		} else {
			log.Printf("SchedulerBatcher: 🔗 Batch anchored on-chain successfully.")
		}
	}
}

// GetEventStream returns the channel for WebSocket event streaming
func (b *SchedulerBatcher) GetEventStream() <-chan *types.ActiveSignal {
	return b.eventStream
}

// GetHandler returns the analysis handler for external access to scoring
func (b *SchedulerBatcher) GetHandler() *scheduler.AnalysisHandler {
	return b.handler
}

// GetScheduler returns the signal scheduler for external access
func (b *SchedulerBatcher) GetScheduler() *scheduler.SignalScheduler {
	return b.scheduler
}

// RegisterWebhook registers a webhook URL for a specific node
func (b *SchedulerBatcher) RegisterWebhook(nodeID, webhookURL string) {
	b.handler.RegisterWebhook(nodeID, webhookURL)
}

// GetPendingSignalCount returns the number of signals waiting to be analyzed
func (b *SchedulerBatcher) GetPendingSignalCount() int {
	return b.scheduler.GetSignalCount()
}
