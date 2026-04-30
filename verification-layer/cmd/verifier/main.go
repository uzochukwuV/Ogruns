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
	"github.com/0xprotocol/verification-layer/internal/ingester"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("🚀 0G Verification Layer - Main Node (Scheduled Analysis)")
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
		fmt.Printf("   Registry: %s\n", cfg.RegistryContractAddr)
		fmt.Printf("   Reputation: %s\n", cfg.ReputationContractAddr)
	} else {
		fmt.Println("⚠️ Smart Contract addresses not provided. Running in off-chain simulation mode.")
	}

	// 3. Initialize 0G Storage Client
	storageClient, err := ingester.NewStorageClient(cfg.RPCURL, cfg.PrivateKey, cfg.IndexerTurboURL)
	if err != nil {
		log.Fatalf("Failed to init Storage Client: %v", err)
	}
	fmt.Println("✅ 0G Storage Client Initialized")

	// 4. Initialize Price Fetcher (CoinGecko - on-demand fetching)
	priceFetcher := aggregator.NewOnDemandPriceFetcher()
	fmt.Println("✅ CoinGecko Price Fetcher Initialized (on-demand)")

	// 5. Initialize the Webhook Dispatcher (Event-Driven AI)
	dispatch := dispatcher.NewWebhookDispatcher()

	// 6. Initialize the Scheduler-based Batcher
	// This replaces both the old CEXAggregator + ResolutionEngine
	// with scheduled expiry-time analysis
	batcher := ingester.NewSchedulerBatcher(storageClient, priceFetcher, cm, crypto.VerifySignal)
	fmt.Println("✅ Scheduler Batcher Initialized")
	fmt.Println("   - Signals analyzed at expiry time (not continuous)")
	fmt.Println("   - Scores pushed on-chain automatically")

	// Get the event stream for WebSocket clients
	eventStream := batcher.GetEventStream()

	// 7. Start the REST API & WebSocket Stream
	server := api.NewServer(batcher.GetHandler().GetScorer(), eventStream, dispatch)
	go func() {
		if err := server.Start(cfg.APIAddr); err != nil {
			log.Fatalf("API Server failed: %v", err)
		}
	}()
	fmt.Println("✅ L2 API Gateway & WS Stream Running on " + cfg.APIAddr)

	// 8. Start the Scheduler Batcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go batcher.Start(ctx, server.GetSignalQueue())
	fmt.Println("✅ L2 Scheduler Batcher Started (Zero-Gas Ingestion)")

	// Print system status
	fmt.Println("")
	fmt.Println("==================================================")
	fmt.Println("📊 System Ready - Listening for Signals")
	fmt.Println("==================================================")
	fmt.Println("")
	fmt.Println("API Endpoints:")
	fmt.Printf("  POST %s/api/v1/signals - Submit trading signals\n", cfg.APIAddr)
	fmt.Printf("  GET  %s/api/v1/nodes   - List all nodes with scores\n", cfg.APIAddr)
	fmt.Printf("  WS   %s/api/v1/stream  - Real-time signal events\n", cfg.APIAddr)
	fmt.Println("")
	fmt.Println("Scoring Formula:")
	fmt.Println("  - EV Score:      max 60 pts (time-decay weighted expected value)")
	fmt.Println("  - Win Rate:      max 25 pts (wins / total trades)")
	fmt.Println("  - Sharpe Ratio:  max 15 pts (risk-adjusted consistency)")
	fmt.Println("")
	fmt.Println("Tier Thresholds:")
	fmt.Println("  - BRONZE:  0-40   ($10/mo,  60% creator)")
	fmt.Println("  - SILVER:  41-70  ($25/mo,  70% creator)")
	fmt.Println("  - GOLD:    71-90  ($50/mo,  80% creator)")
	fmt.Println("  - DIAMOND: 91-100 ($100/mo, 85% creator)")
	fmt.Println("")

	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down gracefully...")
	server.Stop()
	cancel()
}
