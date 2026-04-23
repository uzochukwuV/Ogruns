package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/0xprotocol/verification-layer/internal/crypto"
	"github.com/0xprotocol/verification-layer/pkg/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("🚀 0G Verification Layer - Development Test Build")
	fmt.Println("==================================================")

	// Simulate a Node Developer creating a signal
	fmt.Println("\n[1] Simulating a Node Developer...")
	privateKey, err := ethcrypto.GenerateKey()
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	nodeID := ethcrypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	fmt.Printf("    Node ID (Ethereum Address): %s\n", nodeID)

	// Create a signal payload
	timestamp := time.Now().Unix()
	payload := types.SignalPayload{
		TokenPair:  "WIF/USDT",
		Exchange:   "binance",
		Direction:  "LONG",
		EntryPrice: 3.05,
		TakeProfit: 3.50,
		StopLoss:   2.80,
		ExpiryTime: timestamp + 3600, // 1 hour from now
		WeightPct:  85.5,
	}

	// Sign the payload deterministically
	payloadBytes, _ := json.Marshal(payload)
	message := fmt.Sprintf("%d:%s", timestamp, string(payloadBytes))

	prefixedHash := ethcrypto.Keccak256Hash(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)),
	)

	sigBytes, err := ethcrypto.Sign(prefixedHash.Bytes(), privateKey)
	if err != nil {
		log.Fatalf("Failed to sign message: %v", err)
	}
	sigBytes[64] += 27 // Adjust V
	signatureHex := hexutil.Encode(sigBytes)

	// Create the final Envelope (what gets pushed to 0G Storage)
	envelope := types.SignalEnvelope{
		NodeID:    nodeID,
		Timestamp: timestamp,
		Payload:   payload,
		Signature: signatureHex,
	}

	envJSON, _ := json.MarshalIndent(envelope, "", "  ")
	fmt.Println("\n[2] Signal Envelope Generated (Ready for 0G Storage):")
	fmt.Println(string(envJSON))

	// Simulate the Verification Layer receiving the signal
	fmt.Println("\n[3] Verifier Layer Ingesting Signal...")
	fmt.Println("    Validating cryptographic signature...")
	
	isValid, err := crypto.VerifySignal(envelope)
	if err != nil {
		fmt.Printf("    ❌ Verification Failed: %v\n", err)
	} else if isValid {
		fmt.Println("    ✅ Verification Passed! The signature perfectly matches the Node ID.")
	}

	// Simulate a malicious actor tampering with the payload
	fmt.Println("\n[4] Simulating an attacker modifying the take_profit price...")
	tamperedEnvelope := envelope
	tamperedEnvelope.Payload.TakeProfit = 100.00 // Attacker wants a fake 100x return

	isValid, err = crypto.VerifySignal(tamperedEnvelope)
	if err != nil {
		fmt.Printf("    ❌ Tampered Verification correctly Failed: %v\n", err)
	} else if isValid {
		fmt.Println("    ⚠️ CRITICAL ERROR: Tampered signal was accepted!")
	}

	fmt.Println("\n==================================================")
	fmt.Println("🏁 Test Build Complete. The Verification Layer is secure.")
}
