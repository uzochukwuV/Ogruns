package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/0xprotocol/verification-layer/internal/aggregator"
	"github.com/0xprotocol/verification-layer/internal/api"
	"github.com/0xprotocol/verification-layer/internal/config"
	"github.com/0xprotocol/verification-layer/internal/contracts"
	"github.com/0xprotocol/verification-layer/internal/crypto"
	"github.com/0xprotocol/verification-layer/internal/dispatcher"
	"github.com/0xprotocol/verification-layer/internal/engine"
	"github.com/0xprotocol/verification-layer/internal/ingester"
	"github.com/0xprotocol/verification-layer/internal/scorer"
	"github.com/0xprotocol/verification-layer/pkg/types"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("🚀 0G Verification Layer - Main Node")
	fmt.Println("==================================================")

	// 1. Load Config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Fatal config error: %v", err)
	}

	// 2. Initialize Smart Contract Manager
	var cm *contracts.ContractManager
	if cfg.RegistryContractAddr != "" && cfg.ReputationContractAddr != "" {
		cm, err = contracts.NewContractManager(cfg.RPCURL, cfg.PrivateKey, cfg.ReputationContractAddr, cfg.RegistryContractAddr)
		if err != nil {
			log.Fatalf("Failed to init Contract Manager: %v", err)
		}
		fmt.Println("✅ Smart Contract Manager Connected")
	} else {
		fmt.Println("⚠️ Smart Contract addresses not provided. Running in off-chain simulation mode.")
	}

	// 3. Initialize 0G Storage Client
	storageClient, err := ingester.NewStorageClient(cfg.RPCURL, cfg.PrivateKey, cfg.IndexerTurboURL)
	if err != nil {
		log.Fatalf("Failed to init Storage Client: %v", err)
	}
	fmt.Println("✅ 0G Storage Client Initialized")

	// 4. Initialize Scorer
	sc := scorer.NewScorer()
	fmt.Println("✅ Reputation Scorer Initialized")

	// 5. Initialize CEX Aggregator & Resolution Engine
	agg := aggregator.NewCEXAggregator()
	agg.Start()
	defer agg.Stop()

	eng := engine.NewResolutionEngine(agg.TickStream)
	eng.Start()
	defer eng.Stop()
	fmt.Println("✅ Binance WebSocket & Resolution Engine Started")

	// 6. Initialize the Webhook Dispatcher (Event-Driven AI)
	dispatch := dispatcher.NewWebhookDispatcher()

	// 7. Start the REST API & WebSocket Stream
	// Note: We create a fan-out channel to pass events to both the API and Scorer
	eventChan1 := make(chan *types.ActiveSignal, 1000)
	eventChan2 := make(chan *types.ActiveSignal, 1000)

	go func() {
		for ev := range eng.EventStream() {
			eventChan1 <- ev
			eventChan2 <- ev

			// Instantly Wake Up any subscribed AI Agents via Webhook when signal hits ENTRY
			if ev.State == types.StateActive {
				dispatch.Broadcast(ev.Envelope)
			}
		}
	}()

	// Wire Scorer
	go func() {
		for sig := range eventChan1 {
			if sig.State == types.StateClosedWin || sig.State == types.StateClosedLoss || sig.State == types.StateExpired {
				stats := sc.RecordClosed(sig)
				log.Printf("Scorer: node=%s trust=%.2f tier=%s", stats.NodeID, stats.TrustScore, stats.Tier)
			}
		}
	}()

	server := api.NewServer(sc, eventChan2, dispatch)
	go func() {
		if err := server.Start(cfg.APIAddr); err != nil {
			log.Fatalf("API Server failed: %v", err)
		}
	}()
	fmt.Println("✅ L2 API Gateway & WS Stream Running on " + cfg.APIAddr)

	// 8. Start the L2 Batcher & Broadcaster
	// The batcher reads from the API queue, validates signals, injects them into the Engine,
	// and periodically flushes the bundle to 0G Storage to save gas.
	batcher := ingester.NewBatcher(storageClient, eng, cm, crypto.VerifySignal)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go batcher.Start(ctx, server.GetSignalQueue())
	fmt.Println("✅ L2 Batcher Started (Zero-Gas Ingestion)")

	if cm != nil {
		broadcaster := ingester.NewBroadcaster(sc, cm, cfg.BroadcastIntervalSec)
		go broadcaster.Start(ctx)
		fmt.Println("✅ On-Chain Broadcaster Started")
	}

	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down gracefully...")
	server.Stop()
	cancel()
}
