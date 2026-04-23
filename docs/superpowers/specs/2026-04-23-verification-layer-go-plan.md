# Verification Layer: Go Implementation Plan

## Overview
This document outlines the step-by-step implementation plan for building the Verification Layer in Go. It leverages the official `github.com/0gfoundation/0g-storage-client` and standard Go web/concurrency patterns.

## 1. Directory Structure
We will adopt the standard Go project layout:

```
/verification-layer
├── cmd/
│   └── verifier/           # Main entry point (main.go)
├── internal/
│   ├── config/             # Env vars and config loading
│   ├── ingester/           # 0G Storage Client integration (poll & fetch)
│   ├── aggregator/         # CEX WebSocket streams (Binance, Bybit)
│   ├── engine/             # In-memory Resolution Engine (Signal matching)
│   ├── scorer/             # Reputation calculation math
│   ├── api/                # REST API (Verifier API Gate for subscribers)
│   └── crypto/             # ecrecover, encryption/decryption
├── pkg/
│   └── types/              # Shared data structs (SignalEnvelope, Tick)
├── go.mod
└── go.sum
```

## 2. Dependencies
- **0G Storage:** `github.com/0gfoundation/0g-storage-client`
- **Ethereum/Crypto:** `github.com/ethereum/go-ethereum` (for `ecrecover` and keccak256)
- **WebSockets:** `github.com/gorilla/websocket`
- **REST API:** `github.com/gin-gonic/gin`
- **Config:** `github.com/spf13/viper`
- **Logging:** `go.uber.org/zap` or `logrus`

## 3. Implementation Phases

### Phase 1: Core Types & Cryptography
1. Initialize Go module: `go mod init github.com/yourorg/verification-layer`
2. Define the `SignalEnvelope` struct in `pkg/types`.
3. Implement `internal/crypto`. Write `VerifySignature(payload []byte, signature string) (string, error)` using `go-ethereum/crypto`.
4. Write unit tests for signature recovery to ensure malicious nodes are rejected.

### Phase 2: 0G Ingester (Storage Integration)
*Reference: `0g-serving-broker/api/fine-tuning/internal/storage/client.go`*
1. Initialize the 0G Indexer client using `indexer.NewClient`.
2. Build a polling loop in `internal/ingester` that checks for new signal hashes published by known Node IDs.
3. Fetch the encrypted JSON blob using `indexerClient.Download()`.
4. Decrypt the blob, parse into `SignalEnvelope`, verify the signature, and pass valid signals to the Engine channel.

### Phase 3: CEX Aggregator & Resolution Engine
1. **Aggregator:** Connect to Binance/Bybit WebSockets (`wss://stream.binance.com:9443/ws/!miniTicker@arr`). Normalize data into a `types.Tick` struct.
2. **Engine State:** Build a thread-safe map/struct holding `PENDING` and `ACTIVE` signals.
3. **Matching Loop:** For every incoming tick, compare against relevant active signals.
4. **State Transitions:** If `entry_price` is hit -> move to `ACTIVE`. If `take_profit` or `stop_loss` is hit -> move to `CLOSED`.

### Phase 4: Scoring & Verifier API Gate
1. **Scorer:** Run a periodic job over `CLOSED` signals. Calculate Expected Value and apply the time-decay factor. Update the Node's Tier.
2. **API Gate:** Build the REST API for execution bots (subscribers).
   - `GET /api/v1/nodes` -> Returns marketplace node stats.
   - `GET /api/v1/signals/stream?node_id=123` -> WebSocket or SSE endpoint requiring a valid subscription token. Streams decrypted, verified signals in real-time.

## 4. 0G Storage Client Reference Pattern
Based on our analysis of the `0g-serving-broker`, the initialization pattern we will mirror is:
```go
w3client := blockchain.MustNewWeb3(url, privateKey, opt)
indexerClient, err := indexer.NewClient(config, indexer.IndexerClientOption{...})
err = indexerClient.Download(ctx, hash, tmpFile, true)
```
