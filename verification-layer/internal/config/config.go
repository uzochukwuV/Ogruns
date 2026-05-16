package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the Verification Layer platform.
// Values are read from environment variables (or a .env file in the working directory).
type Config struct {
	// Verifier identity — private key used for signing on-chain transactions.
	PrivateKey string
	Address    string

	// 0G Network EVM RPC endpoint.
	RPCURL string

	// 0G Storage indexer URLs. Standard uses HTTP; Turbo uses P2P.
	IndexerStandardURL string
	IndexerTurboURL    string

	// Smart contract addresses on 0G Network (set after deployment).
	RegistryContractAddr     string // AgentRegistry.sol
	ReputationContractAddr   string // ReputationOracle.sol
	SubscriptionContractAddr string // SubscriptionManager.sol (V1 on-chain)
	PointVaultAddr           string // PointVault.sol (V2 off-chain hybrid)

	// REST/WebSocket API bind address.
	APIAddr string

	// How often the broadcaster syncs scores on-chain (seconds).
	BroadcastIntervalSec int

	// V2 Off-Chain Subscription System Configuration
	DatabaseURL             string // Neon PostgreSQL connection string
	PointConversionRate     int64  // Points per 1 0G (default: 1000)
	SolvencyIntervalHours   int    // Hours between solvency proofs (default: 6)
	DepositConfirmations    uint64 // Block confirmations before crediting deposits (default: 6)
}

func LoadConfig() (*Config, error) {
	// Load .env if present — env vars already set in the process take precedence.
	if err := godotenv.Load(".env"); err != nil {
		log.Println("config: no .env file found, reading from environment")
	}

	isMainnet := getEnvBool("IS_MAINNET", false)

	cfg := &Config{
		PrivateKey:               getEnv("VERIFIER_PRIVATE_KEY", ""),
		Address:                  getEnv("VERIFIER_ADDRESS", ""),
		RPCURL:                   getEnvWithMode("ZG_RPC_URL", "https://evmrpc-testnet.0g.ai", isMainnet),
		IndexerStandardURL:       getEnvWithMode("ZG_INDEXER_STANDARD_URL", "https://indexer-storage-testnet-standard.0g.ai", isMainnet),
		IndexerTurboURL:          getEnvWithMode("ZG_INDEXER_TURBO_URL", "https://indexer-storage-testnet-turbo.0g.ai", isMainnet),
		RegistryContractAddr:     getEnvWithMode("REGISTRY_CONTRACT_ADDR", "", isMainnet),
		ReputationContractAddr:   getEnvWithMode("REPUTATION_CONTRACT_ADDR", "", isMainnet),
		SubscriptionContractAddr: getEnvWithMode("SUBSCRIPTION_CONTRACT_ADDR", "", isMainnet),
		PointVaultAddr:           getEnvWithMode("POINT_VAULT_ADDR", "", isMainnet),
		APIAddr:                  getEnv("API_ADDR", ":8080"),
		BroadcastIntervalSec:     getEnvInt("BROADCAST_INTERVAL_SEC", 300),
		DatabaseURL:              getEnv("DATABASE_URL", ""),
		PointConversionRate:      int64(getEnvInt("POINT_CONVERSION_RATE", 1000)),
		SolvencyIntervalHours:    getEnvInt("SOLVENCY_INTERVAL_HOURS", 6),
		DepositConfirmations:     uint64(getEnvInt("DEPOSIT_CONFIRMATIONS", 6)),
	}

	if cfg.PrivateKey == "" {
		return nil, fmt.Errorf("VERIFIER_PRIVATE_KEY is required — run 'go run ./cmd/keygen' to generate one")
	}
	return cfg, nil
}

func getEnvWithMode(key, fallback string, mainnet bool) string {
	if mainnet {
		if v, exists := os.LookupEnv(key + "_MAINNET"); exists {
			return v
		}
	}
	return getEnv(key, fallback)
}

func getEnvBool(key string, def bool) bool {
	v, exists := os.LookupEnv(key)
	if !exists || v == "" {
		return def
	}
	switch v {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes":
		return true
	case "0", "false", "FALSE", "False", "no", "NO", "No":
		return false
	default:
		return def
	}
}

func getEnv(key, fallback string) string {
	if v, exists := os.LookupEnv(key); exists {
		return v
	}
	return fallback
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def
	}
	return n
}
