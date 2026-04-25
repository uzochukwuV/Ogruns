package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/0xprotocol/verification-layer/internal/config"
	"github.com/0xprotocol/verification-layer/internal/contracts"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("🔗 0G Galileo Testnet - EVM Contract Test")
	fmt.Println("==================================================")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	fmt.Printf("RPC:              %s\n", cfg.RPCURL)
	fmt.Printf("Verifier Address: %s\n", cfg.Address)
	fmt.Printf("NodeRegistry:     %s\n", cfg.RegistryContractAddr)
	fmt.Printf("ReputationOracle: %s\n", cfg.ReputationContractAddr)

	cm, err := contracts.NewContractManager(
		cfg.RPCURL,
		cfg.PrivateKey,
		cfg.ReputationContractAddr,
		cfg.RegistryContractAddr,
	)
	if err != nil {
		log.Fatalf("Failed to create ContractManager: %v", err)
	}
	fmt.Println("✅ ContractManager connected to 0G Galileo (ChainID 16602)")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// ── Test 1: PublishBatchHash ────────────────────────────────────────────────
	// Use the confirmed root hash from our previous storage upload as the test
	// value; any valid 32-byte hex works for this ABI call.
	testRootHash := "0x9bfa70a57621da538894795ee5753c92b4575dd65ccf49a502d66d6e4d0ebbed"

	fmt.Printf("\n[1] Publishing batch hash to NodeRegistry.sol...\n")
	fmt.Printf("    Root Hash: %s\n", testRootHash)

	t0 := time.Now()
	if err := cm.PublishBatchHash(ctx, testRootHash); err != nil {
		log.Fatalf("❌ PublishBatchHash failed: %v", err)
	}
	fmt.Printf("✅ PublishBatchHash confirmed on-chain in %s\n", time.Since(t0).Round(time.Millisecond))

	// ── Test 2: UpdateNodeScore ─────────────────────────────────────────────────
	// Use our own verifier address as the "node" whose score we update.
	// Score 85.00 → uint256 8500; Tier 2 (Gold).
	testNodeID := cfg.Address
	testScore := float64(85.00)
	testTier := uint8(2)

	fmt.Printf("\n[2] Updating node score on ReputationOracle.sol...\n")
	fmt.Printf("    NodeID: %s\n", testNodeID)
	fmt.Printf("    Score:  %.2f  (→ uint256 %d)\n", testScore, int64(testScore*100))
	fmt.Printf("    Tier:   %d\n", testTier)

	t1 := time.Now()
	if err := cm.UpdateNodeScore(ctx, testNodeID, testScore, testTier); err != nil {
		log.Fatalf("❌ UpdateNodeScore failed: %v", err)
	}
	fmt.Printf("✅ UpdateNodeScore confirmed on-chain in %s\n", time.Since(t1).Round(time.Millisecond))

	fmt.Println("\n==================================================")
	fmt.Println("🏁 All EVM contract tests passed.")
	fmt.Println("   Both transactions confirmed with status 0x1 on")
	fmt.Println("   0G Galileo Testnet (ChainID 16602).")
	fmt.Println("==================================================")
}
