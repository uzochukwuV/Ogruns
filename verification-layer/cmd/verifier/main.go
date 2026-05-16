package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/0xprotocol/verification-layer/internal/aggregator"
	"github.com/0xprotocol/verification-layer/internal/api"
	"github.com/0xprotocol/verification-layer/internal/config"
	"github.com/0xprotocol/verification-layer/internal/contracts"
	"github.com/0xprotocol/verification-layer/internal/crypto"
	"github.com/0xprotocol/verification-layer/internal/database"
	"github.com/0xprotocol/verification-layer/internal/dispatcher"
	"github.com/0xprotocol/verification-layer/internal/ingester"
	"github.com/0xprotocol/verification-layer/internal/points"
	"github.com/0xprotocol/verification-layer/internal/scorer"
	"github.com/0xprotocol/verification-layer/internal/solvency"
	"github.com/0xprotocol/verification-layer/internal/vault"
	"github.com/0xprotocol/verification-layer/pkg/types"
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

	// 2. Initialize Smart Contract Manager (with Subscription support)
	var cm *contracts.ContractManager
	if cfg.RegistryContractAddr != "" && cfg.ReputationContractAddr != "" {
		cm, err = contracts.NewContractManagerWithSubscription(
			cfg.RPCURL,
			cfg.PrivateKey,
			cfg.ReputationContractAddr,
			cfg.RegistryContractAddr,
			cfg.SubscriptionContractAddr,
		)
		if err != nil {
			log.Fatalf("Failed to init Contract Manager: %v", err)
		}
		fmt.Println("✅ Smart Contract Manager Connected")
		fmt.Printf("   Registry: %s\n", cfg.RegistryContractAddr)
		fmt.Printf("   Reputation: %s\n", cfg.ReputationContractAddr)
		if cfg.SubscriptionContractAddr != "" {
			fmt.Printf("   Subscription: %s\n", cfg.SubscriptionContractAddr)
		} else {
			fmt.Println("   Subscription: Not configured (premium endpoints are public)")
		}
	} else {
		fmt.Println("⚠️ Smart Contract addresses not provided. Running in off-chain simulation mode.")
	}

	// 3. Initialize 0G Storage Client
	storageClient, err := ingester.NewStorageClient(cfg.RPCURL, cfg.PrivateKey, cfg.IndexerTurboURL)
	if err != nil {
		log.Fatalf("Failed to init Storage Client: %v", err)
	}
	fmt.Println("✅ 0G Storage Client Initialized")

	// 3.5. Initialize V2 Off-Chain Subscription System (if configured)
	var db *database.DB
	var vaultContract *vault.VaultContract
	var depositListener *vault.DepositListener
	var pointAllocator *points.Allocator
	var solvencyProver *solvency.Prover

	if cfg.DatabaseURL != "" && cfg.PointVaultAddr != "" {
		fmt.Println("")
		fmt.Println("==================================================")
		fmt.Println("🔷 Initializing V2 Off-Chain Subscription System")
		fmt.Println("==================================================")

		// Initialize database connection
		dbCfg := &database.Config{
			URL:             cfg.DatabaseURL,
			MaxConns:        20,
			MinConns:        5,
			MaxConnIdleTime: 5 * time.Minute,
		}
		db, err = database.New(context.Background(), dbCfg)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		fmt.Println("✅ Database Connected")

		// Run migrations
		if err := db.RunMigrations(context.Background()); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
		fmt.Println("✅ Database Migrations Complete")

		// Initialize PointVault contract
		vaultCfg := &vault.Config{
			RPCEndpoint:     cfg.RPCURL,
			ContractAddress: cfg.PointVaultAddr,
			PrivateKey:      cfg.PrivateKey,
		}
		vaultContract, err = vault.New(context.Background(), vaultCfg)
		if err != nil {
			log.Fatalf("Failed to init PointVault contract: %v", err)
		}
		fmt.Printf("✅ PointVault Contract Initialized (%s)\n", cfg.PointVaultAddr)

		// Initialize deposit listener
		listenerCfg := &vault.DepositListenerConfig{
			RPCEndpoint:         cfg.RPCURL,
			ContractAddress:     cfg.PointVaultAddr,
			PollInterval:        15 * time.Second,
			Confirmations:       cfg.DepositConfirmations,
			PointConversionRate: cfg.PointConversionRate,
		}
		depositListener, err = vault.NewDepositListener(context.Background(), listenerCfg, db)
		if err != nil {
			log.Fatalf("Failed to init deposit listener: %v", err)
		}
		depositListener.Start(context.Background())
		fmt.Printf("✅ Deposit Listener Started (confirmations: %d, rate: %d points/0G)\n",
			cfg.DepositConfirmations, cfg.PointConversionRate)

		// Initialize point allocator
		pointAllocator = points.NewAllocator(db)
		fmt.Println("✅ Point Allocator Initialized")

		// Initialize solvency prover
		solvencyCfg := &solvency.Config{
			IntervalHours: cfg.SolvencyIntervalHours,
		}
		solvencyProver = solvency.NewProver(db, vaultContract, nil, solvencyCfg) // nil storageClient for now
		solvencyProver.Start(context.Background())
		fmt.Printf("✅ Solvency Prover Started (interval: %d hours)\n", cfg.SolvencyIntervalHours)

		fmt.Println("==================================================")
		fmt.Println("")
	} else {
		fmt.Println("⚠️  V2 Off-Chain Subscription System DISABLED")
		if cfg.DatabaseURL == "" {
			fmt.Println("   - DATABASE_URL not configured")
		}
		if cfg.PointVaultAddr == "" {
			fmt.Println("   - POINT_VAULT_ADDR not configured")
		}
		fmt.Println("")
	}

	// 4. Initialize Price Fetcher (CoinGecko - on-demand fetching)
	priceFetcher := aggregator.NewOnDemandPriceFetcher()
	fmt.Println("✅ CoinGecko Price Fetcher Initialized (on-demand)")

	// 5. Initialize the Webhook Dispatcher (Event-Driven AI)
	dispatch := dispatcher.NewWebhookDispatcher()

	// 6. Initialize the Scheduler-based Batcher with encrypted persistence
	// This replaces both the old CEXAggregator + ResolutionEngine
	// with scheduled expiry-time analysis
	batcher := ingester.NewSchedulerBatcherWithEncryption(storageClient, priceFetcher, cm, crypto.VerifySignal, cfg.PrivateKey)
	fmt.Println("✅ Scheduler Batcher Initialized")
	fmt.Println("   - Signals analyzed at expiry time (not continuous)")
	fmt.Println("   - Scores pushed on-chain automatically")

	// 7. Load persisted scorer state (restores history from previous runs)
	dataDir := scorer.GetDataDir()
	sc := batcher.GetHandler().GetScorer()
	if err := sc.LoadState(dataDir); err != nil {
		log.Printf("Warning: Failed to load scorer state: %v", err)
	}

	// 8. Start auto-save for scorer state (every 5 minutes)
	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	sc.AutoSave(dataDir, 5*time.Minute, &wg, stopCh)

	// Get the event stream for WebSocket clients
	eventStream := batcher.GetEventStream()

	// 9. Start the REST API & WebSocket Stream (with Subscription Gating + V2 Off-Chain)
	server := api.NewServer(sc, eventStream, dispatch, cm, db, vaultContract)

	// Wire point allocator into scheduler handler (if V2 is enabled)
	if pointAllocator != nil {
		batcher.GetHandler().SetPointAllocator(pointAllocator)
		fmt.Println("✅ Point Allocator Wired into Signal Handler")
	}

	// Wire up active signals fetcher for AI agents
	server.SetActiveSignalsFetcher(func() []*types.ActiveSignal {
		return batcher.GetScheduler().GetPendingSignals()
	})

	go func() {
		if err := server.Start(cfg.APIAddr); err != nil {
			log.Fatalf("API Server failed: %v", err)
		}
	}()
	fmt.Println("✅ L2 API Gateway & WS Stream Running on " + cfg.APIAddr)

	// 10. Start the Scheduler Batcher
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
	fmt.Println("  V1 (On-Chain Subscriptions):")
	fmt.Printf("    POST %s/api/v1/signals - Submit trading signals\n", cfg.APIAddr)
	fmt.Printf("    GET  %s/api/v1/nodes   - List all nodes with scores\n", cfg.APIAddr)
	fmt.Printf("    WS   %s/api/v1/stream  - Real-time signal events\n", cfg.APIAddr)
	fmt.Printf("    GET  %s/api/v1/analytics/dashboard - Platform stats\n", cfg.APIAddr)
	fmt.Printf("    GET  %s/api/v1/analytics/nodes/{id} - Node analytics\n", cfg.APIAddr)
	if db != nil {
		fmt.Println("  V2 (Off-Chain Subscriptions):")
		fmt.Printf("    POST %s/api/v2/subscribe - Subscribe to provider\n", cfg.APIAddr)
		fmt.Printf("    GET  %s/api/v2/subscriptions - List subscriptions\n", cfg.APIAddr)
		fmt.Printf("    GET  %s/api/v2/balance - User point balance\n", cfg.APIAddr)
		fmt.Printf("    GET  %s/api/v2/deposit-info - Vault deposit info\n", cfg.APIAddr)
		fmt.Printf("    POST %s/api/v2/providers/withdraw - Provider withdrawal\n", cfg.APIAddr)
		fmt.Printf("    WS   %s/api/v2/stream - Subscription-filtered stream\n", cfg.APIAddr)
	}
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
	fmt.Printf("Data persistence: %s\n", dataDir)
	fmt.Println("")

	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down gracefully...")

	// Stop auto-save and trigger final save
	close(stopCh)
	wg.Wait()

	server.Stop()
	cancel()

	// Stop V2 system components
	if depositListener != nil {
		depositListener.Stop()
		fmt.Println("✅ Deposit Listener Stopped")
	}
	if solvencyProver != nil {
		solvencyProver.Stop()
		fmt.Println("✅ Solvency Prover Stopped")
	}
	if vaultContract != nil {
		vaultContract.Close()
		fmt.Println("✅ Vault Contract Connection Closed")
	}
	if db != nil {
		db.Close()
		fmt.Println("✅ Database Connection Closed")
	}

	fmt.Println("✅ Shutdown complete")
}
