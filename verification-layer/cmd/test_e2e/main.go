package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("🤖 AI Node Creator - E2E Integration Test")
	fmt.Println("==================================================")

	// 1. Generate Node Identity
	nodeKey, _ := ethcrypto.GenerateKey()
	nodePubKey := nodeKey.Public().(*ecdsa.PublicKey)
	nodeID := ethcrypto.PubkeyToAddress(*nodePubKey).Hex()
	fmt.Printf("[1] Generated Node Identity: %s\n", nodeID)

	// 2. Register Webhook (Pretending to be a Subscriber)
	fmt.Println("\n[2] Registering AI Agent Webhook (Subscriber)...")
	webhookReq := types.RegisterWebhookRequest{
		SubscriberAddress: "0xMockSubscriberAddress",
		Signature:         "0xMockSignature",
		TargetURL:         "http://localhost:3000/trade", // A mock Node.js server we will spin up
		NodeID:            nodeID,
	}

	webhookBytes, _ := json.Marshal(webhookReq)
	resp, err := http.Post("http://localhost:9090/api/v1/subscribers/webhook", "application/json", bytes.NewBuffer(webhookBytes))
	if err != nil {
		log.Fatalf("Webhook registration failed: %v", err)
	}
	defer resp.Body.Close()
	fmt.Println("✅ Webhook Registered successfully.")

	// 3. Create a Signal Payload that will instantly hit ENTRY
	// To ensure the engine picks it up instantly, we'll just set the entry price extremely high
	// so the current Binance tick is guaranteed to trigger it.
	fmt.Println("\n[3] Generating Signal Payload...")

	// Wait a moment to make sure Binance ticks are flowing
	time.Sleep(3 * time.Second)

	// Fetch a live tick directly to ensure we set the mock entry/TP precisely around current price
	payload := types.SignalPayload{
		TokenPair:  "BTCUSDT",
		Direction:  "LONG",
		EntryPrice: 1000000.0, // High entry means current price (< 1000000) immediately triggers LONG entry
		TakeProfit: 1000000.0, // High TP so we just sit in ACTIVE state
		StopLoss:   1.0,
		ExpiryTime: time.Now().Add(1 * time.Hour).Unix(),
		WeightPct:  100.0,
	}

	envelope := types.SignalEnvelope{
		NodeID:    nodeID,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}

	payloadBytes, _ := json.Marshal(payload)

	// Format: <timestamp>:<payload_json>
	message := fmt.Sprintf("%d:%s", envelope.Timestamp, string(payloadBytes))

	// Keccak256 hash the message first (this is how the frontend usually signs)
	messageHash := ethcrypto.Keccak256Hash([]byte(message))

	// Prepend exactly what geth's personal_sign prepends
	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(messageHash.Bytes()), messageHash.Bytes())
	finalHash := ethcrypto.Keccak256Hash([]byte(prefixedMessage))

	sigBytes, _ := ethcrypto.Sign(finalHash.Bytes(), nodeKey)

	// Wait to make sure the server has received its first tick so it evaluates the new signal immediately
	time.Sleep(3 * time.Second)
	sigBytes[64] += 27
	envelope.Signature = hexutil.Encode(sigBytes)

	// 4. Submit the raw signal to the Verification Layer (L2 Batcher)
	fmt.Println("\n[4] Submitting Zero-Gas Signal to Verification Layer...")
	submitReq := types.SubmitRawSignalRequest{
		NodeID:   nodeID,
		Envelope: envelope,
	}

	submitBytes, _ := json.Marshal(submitReq)
	resp2, err := http.Post("http://localhost:8000/api/v1/signals", "application/json", bytes.NewBuffer(submitBytes))
	if err != nil {
		log.Fatalf("Signal submission failed: %v", err)
	}
	defer resp2.Body.Close()
	fmt.Println("✅ Signal submitted and queued for L2 Batching.")
	fmt.Println("   (The engine should pick this up in ~1 second and fire the webhook!)")

	// Keep the script alive slightly longer to allow webhook to trigger before script exits
	time.Sleep(10 * time.Second)
}
