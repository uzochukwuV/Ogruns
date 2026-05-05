package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/0xprotocol/verification-layer/internal/aggregator"
	"github.com/0xprotocol/verification-layer/internal/api"
	"github.com/0xprotocol/verification-layer/internal/contracts"
	"github.com/0xprotocol/verification-layer/internal/engine"
	"github.com/0xprotocol/verification-layer/internal/scorer"
	"github.com/0xprotocol/verification-layer/pkg/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("📈 Verification Layer - CEX Engine Integration Test")
	fmt.Println("==================================================")

	// Load configuration from .env so PRICE_SOURCE and private-key settings are available.
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found, falling back to environment variables")
	}

	// 1. Initialize the Aggregator
	agg := aggregator.NewCEXAggregator()
	agg.Start()
	fmt.Println("[1] Started price aggregator using PRICE_SOURCE=" + os.Getenv("PRICE_SOURCE"))

	// Track BTCUSDT so CoinGecko will emit a tick for this token pair
	agg.TrackToken("BTCUSDT")

	// 2. Initialize the Resolution Engine, passing it the Aggregator's stream
	eng := engine.NewResolutionEngine(agg.TickStream)
	eng.Start()
	fmt.Println("[2] Started In-Memory Resolution Engine")

	// 3. Initialize the API Server (receives signals via HTTP POST)
	// Pass nil for ContractManager to disable subscription gating in test mode
	sc := scorer.NewScorer()
	apiServer := api.NewServer(sc, eng.EventStream(), nil, nil)
	go func() {
		if err := apiServer.Start(":8000"); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()
	fmt.Println("[3] Started API Server on :8000")

	// Give the server time to bind
	time.Sleep(500 * time.Millisecond)

	// 4. Derive Node Identity from the private key in .env
	verifierPrivKey := os.Getenv("VERIFIER_PRIVATE_KEY")
	if verifierPrivKey == "" {
		log.Fatal("VERIFIER_PRIVATE_KEY is required in .env")
	}
	privKeyBytes, err := hex.DecodeString(verifierPrivKey)
	if err != nil {
		log.Fatalf("invalid VERIFIER_PRIVATE_KEY: %v", err)
	}
	nodeKey, err := ethcrypto.ToECDSA(privKeyBytes)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}
	nodePubKey := nodeKey.Public().(*ecdsa.PublicKey)
	nodeID := ethcrypto.PubkeyToAddress(*nodePubKey).Hex()
	fmt.Printf("\n[4] Derived Node Identity from env private key: %s\n", nodeID)

	// 5. Register the node on-chain (NodeRegistry contract)
	fmt.Println("\n[5] Registering node on-chain...")
	rpcURL := os.Getenv("ZG_RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://evmrpc-testnet.0g.ai"
	}
	registryAddr := os.Getenv("REGISTRY_CONTRACT_ADDR")
	if registryAddr == "" {
		log.Println("⚠️  Warning: REGISTRY_CONTRACT_ADDR not set, skipping on-chain registration")
		fmt.Println("    Set REGISTRY_CONTRACT_ADDR and ZG_RPC_URL to enable contract interaction")
	} else {
		nodePrivKeyHex := os.Getenv("VERIFIER_PRIVATE_KEY")
		cm, err := contracts.NewContractManager(rpcURL, nodePrivKeyHex, "", registryAddr)
		if err != nil {
			log.Printf("⚠️  Contract manager error: %v (continuing with local test)", err)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			name, tokenFocus, _, _, active, _, err := cm.GetNodeInfo(ctx, nodeID)
			cancel()
			if err == nil && active {
				fmt.Printf("    ✅ Node already registered on-chain: %s (name=%s, tokenFocus=%s)\n", nodeID, name, tokenFocus)
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
				err = cm.RegisterNode(ctx, "test-node", "BTC,ETH", "Integration test node")
				cancel()
				if err != nil {
					log.Printf("⚠️  Registration error: %v (continuing with local test)", err)
				} else {
					fmt.Printf("    ✅ Node registered on-chain at: %s\n", nodeID)
				}
			}
		}
	}

	// 6. Fetch the current BTC price directly from CoinGecko (avoid consuming the engine tick stream)
	fmt.Println("\n[6] Fetching current BTC price from CoinGecko...")
	currentBTCPrice, err := agg.FetchPriceOnce("BTCUSDT")
	if err != nil {
		log.Printf("⚠️  Failed to fetch BTC price from CoinGecko: %v", err)
		agg.Stop()
		eng.Stop()
		apiServer.Stop()
		return
	}
	fmt.Printf("    Current BTC Price: %.2f\n", currentBTCPrice)
	// 7. Create and Sign a Signal Payload
	fmt.Println("\n[7] Creating and signing signal payload...")

	payload := types.SignalPayload{
		TokenPair:  "BTCUSDT",
		Direction:  "LONG",
		EntryPrice: currentBTCPrice + 1.00, // Slightly above current to trigger on next tick
		TakeProfit: currentBTCPrice + 5.00, // $5 profit target
		StopLoss:   currentBTCPrice - 5.00, // $5 stop loss
		ExpiryTime: time.Now().Add(1 * time.Hour).Unix(),
		WeightPct:  100.0,
	}

	envelope := types.SignalEnvelope{
		NodeID:    nodeID,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}

	// Sign the envelope using ECDSA (matching the verify.go format)
	payloadBytes, _ := json.Marshal(payload)
	message := fmt.Sprintf("%d:%s", envelope.Timestamp, string(payloadBytes))
	messageHash := ethcrypto.Keccak256Hash([]byte(message))
	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(messageHash.Bytes()), messageHash.Bytes())
	finalHash := ethcrypto.Keccak256Hash([]byte(prefixedMessage))
	sigBytes, _ := ethcrypto.Sign(finalHash.Bytes(), nodeKey)
	sigBytes[64] += 27 // Add the recovery ID offset
	envelope.Signature = hexutil.Encode(sigBytes)

	fmt.Printf("    Signature: %s\n", envelope.Signature[:20]+"...")

	// 8. Submit the Signal to the Verification Layer API (L2 Ingestion)
	fmt.Println("\n[8] Posting signed signal to verification layer API...")

	submitReq := types.SubmitRawSignalRequest{
		NodeID:   nodeID,
		Envelope: envelope,
	}

	submitBytes, _ := json.Marshal(submitReq)
	resp, err := http.Post(
		"http://localhost:8000/api/v1/signals",
		"application/json",
		bytes.NewBuffer(submitBytes),
	)
	if err != nil {
		log.Fatalf("Signal submission failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		log.Fatalf("API returned %d, expected 202", resp.StatusCode)
	}
	fmt.Println("    ✅ Signal accepted by verification layer (HTTP 202)")

	// Give the engine time to process the signal
	time.Sleep(1 * time.Second)

	// 9. Wait for the engine to resolve the signal
	fmt.Println("\n[9] Listening for State Transitions from the Engine...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			log.Println("Test Timed Out waiting for BTC to move $5.")
			agg.Stop()
			eng.Stop()
			apiServer.Stop()
			return
		case event := <-eng.EventStream():
			fmt.Printf("    🔥 STATE CHANGE: Signal %s transitioned to %s at Price: %.2f\n", event.ID, event.State, event.ClosedPrice)
			if event.State == types.StateClosedWin || event.State == types.StateClosedLoss {
				fmt.Println("\n==================================================")
				fmt.Println("🏁 Engine Integration Test Complete!")
				fmt.Println("   Signal flow: Register node → Generate → Sign → POST → Batch → Resolve")
				agg.Stop()
				eng.Stop()
				apiServer.Stop()
				return
			}
		}
	}
}
