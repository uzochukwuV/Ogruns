package main

import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	// Generate a new private key
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	// Extract the private key as bytes and format as hex string
	privateKeyBytes := crypto.FromECDSA(privateKey)
	privateKeyHex := hexutil.Encode(privateKeyBytes)[2:] // remove 0x prefix for .env

	// Derive the public key
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("Error casting public key to ECDSA")
	}

	// Derive the Ethereum address
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	// Write to .env file
	envContent := fmt.Sprintf("VERIFIER_PRIVATE_KEY=%s\nVERIFIER_ADDRESS=%s\n", privateKeyHex, address)

	err = os.WriteFile("/workspace/verification-layer/.env", []byte(envContent), 0600)
	if err != nil {
		log.Fatalf("Failed to write .env file: %v", err)
	}

	fmt.Println("==================================================")
	fmt.Println("🔐 Verification Layer Identity Generated")
	fmt.Println("==================================================")
	fmt.Printf("Address (Fund this on 0G Testnet): %s\n", address)
	fmt.Println("Private Key saved securely to /workspace/verification-layer/.env")
}
