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
        SubscriptionContractAddr string // SubscriptionManager.sol

        // REST/WebSocket API bind address.
        APIAddr string

        // How often the broadcaster syncs scores on-chain (seconds).
        BroadcastIntervalSec int
}

func LoadConfig() (*Config, error) {
        // Load .env if present — env vars already set in the process take precedence.
        if err := godotenv.Load(".env"); err != nil {
                log.Println("config: no .env file found, reading from environment")
        }

        cfg := &Config{
                PrivateKey:               getEnv("VERIFIER_PRIVATE_KEY", ""),
                Address:                  getEnv("VERIFIER_ADDRESS", ""),
                RPCURL:                   getEnv("ZG_RPC_URL", "https://evmrpc-testnet.0g.ai"),
                IndexerStandardURL:       getEnv("ZG_INDEXER_STANDARD_URL", "https://indexer-storage-testnet-standard.0g.ai"),
                IndexerTurboURL:          getEnv("ZG_INDEXER_TURBO_URL", "https://indexer-storage-testnet-turbo.0g.ai"),
                RegistryContractAddr:     getEnv("REGISTRY_CONTRACT_ADDR", ""),
                ReputationContractAddr:   getEnv("REPUTATION_CONTRACT_ADDR", ""),
                SubscriptionContractAddr: getEnv("SUBSCRIPTION_CONTRACT_ADDR", ""),
                APIAddr:                  getEnv("API_ADDR", ":8080"),
                BroadcastIntervalSec:     getEnvInt("BROADCAST_INTERVAL_SEC", 300),
        }

        if cfg.PrivateKey == "" {
                return nil, fmt.Errorf("VERIFIER_PRIVATE_KEY is required — run 'go run ./cmd/keygen' to generate one")
        }
        return cfg, nil
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
