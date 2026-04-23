package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	PrivateKey         string
	Address            string
	RPCURL             string
	IndexerStandardURL string
	IndexerTurboURL    string
}

func LoadConfig() *Config {
	err := godotenv.Load("/workspace/verification-layer/.env")
	if err != nil {
		log.Println("Warning: No .env file found, reading from environment")
	}

	return &Config{
		PrivateKey:         getEnv("VERIFIER_PRIVATE_KEY", ""),
		Address:            getEnv("VERIFIER_ADDRESS", ""),
		RPCURL:             getEnv("RPC_URL", "https://evmrpc-testnet.0g.ai"),
		IndexerStandardURL: getEnv("INDEXER_STANDARD_URL", "https://indexer-storage-testnet-standard.0g.ai"),
		IndexerTurboURL:    getEnv("INDEXER_TURBO_URL", "https://indexer-storage-testnet-turbo.0g.ai"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
