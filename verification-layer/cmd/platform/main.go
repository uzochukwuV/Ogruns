// cmd/platform is the production entry point for the Ogruns Verification Layer.
// It wires together all six subsystems and runs them concurrently with graceful
// shutdown on SIGINT / SIGTERM.
//
// Startup order:
//  1. CEX Aggregator  — Binance WebSocket tick feed
//  2. Resolution Engine — in-memory state machine
//  3. Scorer — reputation calculator (receives closed signals from engine)
//  4. API Server — REST + WebSocket gateway for downstream bots
//  5. Broadcaster — periodic on-chain score writer (optional, needs contract addr)
//  6. Ingester Poller — watches NodeRegistry for new signal blobs (optional)
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xprotocol/verification-layer/internal/aggregator"
	"github.com/0xprotocol/verification-layer/internal/api"
	"github.com/0xprotocol/verification-layer/internal/broadcaster"
	"github.com/0xprotocol/verification-layer/internal/config"
	"github.com/0xprotocol/verification-layer/internal/crypto"
	"github.com/0xprotocol/verification-layer/internal/dispatcher"
	"github.com/0xprotocol/verification-layer/internal/engine"
	"github.com/0xprotocol/verification-layer/internal/ingester"
	"github.com/0xprotocol/verification-layer/internal/scorer"
	"github.com/0xprotocol/verification-layer/pkg/types"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	log.Printf("Starting Ogruns Verification Layer | address=%s rpc=%s api=%s",
		cfg.Address, cfg.RPCURL, cfg.APIAddr)

	// ── 1. CEX Aggregator ────────────────────────────────────────────────────
	cex := aggregator.NewCEXAggregator()
	cex.Start()
	log.Println("[1/6] CEX Aggregator started (Binance mini-ticker stream)")

	// ── 2. Resolution Engine ─────────────────────────────────────────────────
	eng := engine.NewResolutionEngine(cex.TickStream)
	eng.Start()
	log.Println("[2/6] Resolution Engine started")

	// ── 3. Scorer ────────────────────────────────────────────────────────────
	sc := scorer.NewScorer()
	// Wire: engine event stream → scorer (and log every state change).
	go drainEvents(eng.EventStream(), sc)
	log.Println("[3/6] Scorer started")

	// ── 4. API Server ────────────────────────────────────────────────────────
	dispatch := dispatcher.NewWebhookDispatcher()
	srv := api.NewServer(sc, eng.EventStream(), dispatch, nil) // nil = no subscription gating
	go func() {
		if err := srv.Start(cfg.APIAddr); err != nil {
			log.Fatalf("API server error: %v", err)
		}
	}()
	log.Printf("[4/6] API Server started on %s", cfg.APIAddr)

	// ── 5. On-Chain Broadcaster ──────────────────────────────────────────────
	if cfg.ReputationContractAddr != "" {
		bc, err := broadcaster.New(cfg.RPCURL, cfg.PrivateKey, cfg.ReputationContractAddr)
		if err != nil {
			log.Printf("[5/6] Broadcaster init failed (continuing without): %v", err)
		} else {
			interval := time.Duration(cfg.BroadcastIntervalSec) * time.Second
			go bc.RunPeriodicBroadcast(sc, interval)
			log.Printf("[5/6] Broadcaster started — syncing every %s", interval)
		}
	} else {
		log.Println("[5/6] Broadcaster skipped — REPUTATION_CONTRACT_ADDR not set")
	}

	// ── 6. Ingester Poller ───────────────────────────────────────────────────
	if cfg.RegistryContractAddr != "" {
		storageClient, err := ingester.NewStorageClient(
			cfg.RPCURL, cfg.PrivateKey, cfg.IndexerTurboURL,
		)
		if err != nil {
			log.Printf("[6/6] Storage client init failed (continuing without): %v", err)
		} else {
			poller, err := ingester.NewPoller(storageClient, cfg.RPCURL, cfg.RegistryContractAddr)
			if err != nil {
				log.Printf("[6/6] Poller init failed (continuing without): %v", err)
			} else {
				go poller.Start(eng, sc, crypto.VerifySignal)
				log.Printf("[6/6] Ingester Poller started — watching NodeRegistry %s",
					cfg.RegistryContractAddr)
			}
		}
	} else {
		log.Println("[6/6] Ingester Poller skipped — REGISTRY_CONTRACT_ADDR not set")
	}

	log.Println("All systems running. Waiting for signals...")

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutdown signal received. Stopping subsystems...")
	cex.Stop()
	eng.Stop()
	srv.Stop()
	log.Println("Shutdown complete.")
}

// drainEvents reads every state-change event from the resolution engine,
// feeds closed signals to the scorer, and logs the outcome.
func drainEvents(stream <-chan *types.ActiveSignal, sc *scorer.Scorer) {
	for sig := range stream {
		switch sig.State {
		case types.StateClosedWin, types.StateClosedLoss, types.StateExpired:
			stats := sc.RecordClosed(sig)
			log.Printf("Signal closed: node=%s pair=%s state=%s | trust=%.1f tier=%s W:%d L:%d",
				sig.Envelope.NodeID[:10], sig.Envelope.Payload.TokenPair,
				sig.State, stats.TrustScore, stats.Tier, stats.WinCount, stats.LossCount)
		case types.StateActive:
			log.Printf("Signal active: node=%s pair=%s entry=%.4f",
				sig.Envelope.NodeID[:10], sig.Envelope.Payload.TokenPair,
				sig.Envelope.Payload.EntryPrice)
		}
	}
}
