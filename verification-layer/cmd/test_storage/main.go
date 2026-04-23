package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/0xprotocol/verification-layer/internal/config"
	"github.com/0xprotocol/verification-layer/internal/ingester"
	"github.com/0xprotocol/verification-layer/pkg/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("📡 0G Storage - Testnet Upload & Download Test")
	fmt.Println("==================================================")

	// Load configuration from .env
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	fmt.Printf("Using RPC: %s\n", cfg.RPCURL)
	fmt.Printf("Using Indexer: %s\n", cfg.IndexerStandardURL)
	fmt.Printf("Using Address: %s\n", cfg.Address)

	// 1. Create a simulated Signal Envelope
	fmt.Println("\n[1] Creating a test Signal Envelope...")
	privateKeyBytes, err := hexutil.Decode("0x" + cfg.PrivateKey)
	if err != nil {
		log.Fatalf("Invalid private key: %v", err)
	}
	privateKey, err := ethcrypto.ToECDSA(privateKeyBytes)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	timestamp := time.Now().Unix()
	payload := types.SignalPayload{
		TokenPair:  "SOL/USDT",
		Direction:  "LONG",
		EntryPrice: 150.50,
		TakeProfit: 165.00,
		StopLoss:   140.00,
		ExpiryTime: timestamp + 3600,
		WeightPct:  90.0,
	}

	payloadBytes, _ := json.Marshal(payload)
	message := fmt.Sprintf("%d:%s", timestamp, string(payloadBytes))

	prefixedHash := ethcrypto.Keccak256Hash(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)),
	)
	sigBytes, _ := ethcrypto.Sign(prefixedHash.Bytes(), privateKey)
	sigBytes[64] += 27
	signatureHex := hexutil.Encode(sigBytes)

	envelope := types.SignalEnvelope{
		NodeID:    cfg.Address,
		Timestamp: timestamp,
		Payload:   payload,
		Signature: signatureHex,
	}

	// Write to temporary file for uploading
	tmpFile := "/tmp/test_signal.json"
	envBytes, _ := json.Marshal(envelope)
	os.WriteFile(tmpFile, envBytes, 0644)

	// 2. Initialize the Storage Client
	fmt.Println("\n[2] Connecting to 0G Storage...")
	client, err := ingester.NewStorageClient(cfg.RPCURL, cfg.PrivateKey, cfg.IndexerTurboURL)
	if err != nil {
		log.Fatalf("Failed to initialize Storage Client: %v", err)
	}

	// 3. Upload the file
	fmt.Println("\n[3] Uploading file to 0G Galileo Testnet... (this may take a moment)")
	ctx := context.Background()
	rootHash, err := client.UploadFile(ctx, tmpFile)
	if err != nil {
		log.Fatalf("❌ Upload failed: %v", err)
	}
	fmt.Printf("✅ Upload Successful! Root Hash: %s\n", rootHash)

	// Wait for a few seconds to let indexer catch up (optional but good practice)
	fmt.Println("    Waiting 5 seconds for indexer sync...")
	time.Sleep(5 * time.Second)

	// 4. Download the file
	fmt.Println("\n[4] Downloading file from 0G Galileo Testnet...")
	outFile := "/tmp/downloaded_signal.json"
	err = client.DownloadFile(ctx, rootHash, outFile)
	if err != nil {
		log.Fatalf("❌ Download failed: %v", err)
	}

	downloadedBytes, err := os.ReadFile(outFile)
	if err != nil {
		log.Fatalf("Failed to read downloaded file: %v", err)
	}

	fmt.Println("✅ Download Successful! Contents:")
	fmt.Println(string(downloadedBytes))
	
	fmt.Println("\n==================================================")
	fmt.Println("🏁 End-to-End Testnet Storage Test Complete")
}
