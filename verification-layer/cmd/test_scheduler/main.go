package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/0xprotocol/verification-layer/internal/aggregator"
	"github.com/0xprotocol/verification-layer/internal/scheduler"
	"github.com/0xprotocol/verification-layer/pkg/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("📊 Scheduled Signal Analysis Test")
	fmt.Println("   With Sophisticated Scoring (EV + Sharpe + Decay)")
	fmt.Println("==================================================")

	// Load env
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, using defaults")
	}

	// 1. Create price fetcher (CoinGecko)
	fmt.Println("\n[1] Initializing CoinGecko price fetcher...")
	tickStream := make(chan types.Tick, 1000)
	priceFetcher := aggregator.NewCoinGeckoPriceFetcher(tickStream, os.Getenv("COINGECKO_API_KEY"))
	priceFetcher.Start()

	// 2. Create analysis handler (uses sophisticated Scorer)
	fmt.Println("[2] Creating analysis handler with Scorer...")
	handler := scheduler.NewAnalysisHandler()

	// 3. Create scheduler
	fmt.Println("[3] Creating signal scheduler...")
	sched := scheduler.NewSignalScheduler(priceFetcher, handler.HandleResult)

	// 4. Create test node identity
	fmt.Println("\n[4] Creating test node identity...")
	privateKey, err := getPrivateKey()
	if err != nil {
		log.Fatalf("Failed to get private key: %v", err)
	}
	nodeAddress := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	fmt.Printf("    Node Address: %s\n", nodeAddress)

	// 5. Fetch current prices
	fmt.Println("\n[5] Fetching current prices...")
	btcPrice, err := priceFetcher.FetchPriceOnce("BTCUSDT")
	if err != nil {
		log.Fatalf("Failed to fetch BTC price: %v", err)
	}
	ethPrice, err := priceFetcher.FetchPriceOnce("ETHUSDT")
	if err != nil {
		log.Fatalf("Failed to fetch ETH price: %v", err)
	}
	fmt.Printf("    BTC: $%.2f | ETH: $%.2f\n", btcPrice, ethPrice)

	// 6. Create test signals with different risk/reward profiles
	fmt.Println("\n[6] Creating test signals with varying R:R ratios...")

	signals := []types.SignalEnvelope{
		// Signal 1: Tight stop, 3:1 R:R (aggressive)
		{
			NodeID:    nodeAddress,
			Timestamp: time.Now().Unix(),
			Payload: types.SignalPayload{
				TokenPair:  "BTCUSDT",
				Direction:  "LONG",
				EntryPrice: btcPrice,
				TakeProfit: btcPrice * 1.03,  // +3% TP
				StopLoss:   btcPrice * 0.99,  // -1% SL (3:1 R:R)
				ExpiryTime: time.Now().Add(20 * time.Second).Unix(),
				WeightPct:  50, // 50% position size
			},
		},
		// Signal 2: Conservative 1:1 R:R
		{
			NodeID:    nodeAddress,
			Timestamp: time.Now().Unix() + 1,
			Payload: types.SignalPayload{
				TokenPair:  "ETHUSDT",
				Direction:  "LONG",
				EntryPrice: ethPrice,
				TakeProfit: ethPrice * 1.02, // +2% TP
				StopLoss:   ethPrice * 0.98, // -2% SL (1:1 R:R)
				ExpiryTime: time.Now().Add(25 * time.Second).Unix(),
				WeightPct:  30,
			},
		},
		// Signal 3: SHORT with 2:1 R:R
		{
			NodeID:    nodeAddress,
			Timestamp: time.Now().Unix() + 2,
			Payload: types.SignalPayload{
				TokenPair:  "BTCUSDT",
				Direction:  "SHORT",
				EntryPrice: btcPrice,
				TakeProfit: btcPrice * 0.96, // -4% TP
				StopLoss:   btcPrice * 1.02, // +2% SL (2:1 R:R)
				ExpiryTime: time.Now().Add(30 * time.Second).Unix(),
				WeightPct:  40,
			},
		},
		// Signal 4: High conviction LONG
		{
			NodeID:    nodeAddress,
			Timestamp: time.Now().Unix() + 3,
			Payload: types.SignalPayload{
				TokenPair:  "ETHUSDT",
				Direction:  "LONG",
				EntryPrice: ethPrice * 0.95, // Below current (already in profit)
				TakeProfit: ethPrice * 1.05,
				StopLoss:   ethPrice * 0.90,
				ExpiryTime: time.Now().Add(35 * time.Second).Unix(),
				WeightPct:  80, // High conviction
			},
		},
		// Signal 5: Another BTC signal
		{
			NodeID:    nodeAddress,
			Timestamp: time.Now().Unix() + 4,
			Payload: types.SignalPayload{
				TokenPair:  "BTCUSDT",
				Direction:  "LONG",
				EntryPrice: btcPrice * 0.98,
				TakeProfit: btcPrice * 1.02,
				StopLoss:   btcPrice * 0.96,
				ExpiryTime: time.Now().Add(40 * time.Second).Unix(),
				WeightPct:  25,
			},
		},
	}

	// 7. Register signals
	fmt.Println("\n[7] Registering signals with scheduler...")
	for i, sig := range signals {
		if err := sched.AddSignal(sig); err != nil {
			log.Printf("Failed to add signal %d: %v", i+1, err)
		} else {
			rr := calculateRR(sig.Payload)
			fmt.Printf("    ✅ Signal %d: %s %s | Entry $%.0f | R:R %.1f:1 | Weight %d%%\n",
				i+1,
				sig.Payload.TokenPair,
				sig.Payload.Direction,
				sig.Payload.EntryPrice,
				rr,
				int(sig.Payload.WeightPct),
			)
		}
	}

	fmt.Printf("\n    📋 Total pending signals: %d\n", sched.GetSignalCount())

	// 8. Wait for analysis to complete
	fmt.Println("\n[8] Waiting for scheduled analysis...")
	fmt.Println("    (Signals analyzed at expiry with EV + Sharpe + Time Decay)")
	fmt.Println()

	for i := 0; i < 50; i++ {
		remaining := sched.GetSignalCount()
		fmt.Printf("\r    ⏳ Time: %ds | Pending: %d", i, remaining)

		if remaining == 0 {
			fmt.Println()
			break
		}
		time.Sleep(1 * time.Second)
	}

	// 9. Show detailed results
	fmt.Println("\n[9] Analysis Complete!")
	fmt.Println("══════════════════════════════════════════════════")

	stats, ok := handler.GetNodeStats(nodeAddress)
	if !ok {
		fmt.Println("No stats available")
	} else {
		tierInfo := handler.GetTierInfo(stats.Tier)

		fmt.Printf("📊 Node: %s\n", nodeAddress[:20]+"...")
		fmt.Println("──────────────────────────────────────────────────")
		fmt.Printf("   Trust Score:    %.1f / 100\n", stats.TrustScore)
		fmt.Printf("   Tier:           %s ($%.0f/month)\n", stats.Tier, tierInfo.PriceUSD)
		fmt.Printf("   Creator Split:  %.0f%%\n", tierInfo.CreatorSplit)
		fmt.Println("──────────────────────────────────────────────────")
		fmt.Printf("   Win Rate:       %.1f%% (%d/%d)\n", stats.WinRate*100, stats.WinCount, stats.WinCount+stats.LossCount)
		fmt.Printf("   Expected Value: %.3f\n", stats.AvgEV)
		fmt.Printf("   Sharpe Ratio:   %.3f\n", stats.SharpeRatio)
		fmt.Println("──────────────────────────────────────────────────")
		fmt.Printf("   Total Signals:  %d\n", stats.TotalSignals)
		fmt.Printf("   Wins:           %d\n", stats.WinCount)
		fmt.Printf("   Losses:         %d\n", stats.LossCount)
		fmt.Printf("   Expired:        %d\n", stats.ExpiredCount)
		fmt.Println("══════════════════════════════════════════════════")

		// Explain the scoring breakdown
		fmt.Println("\n📈 Score Breakdown:")
		evScore := min(max(stats.AvgEV, 0)/2.0, 1.0) * 60.0
		winScore := stats.WinRate * 25.0
		sharpeScore := min(max(stats.SharpeRatio, 0)/3.0, 1.0) * 15.0

		fmt.Printf("   EV Score:      %.1f / 60 pts (EV=%.2f, target=2.0)\n", evScore, stats.AvgEV)
		fmt.Printf("   WinRate Score: %.1f / 25 pts (%.0f%% wins)\n", winScore, stats.WinRate*100)
		fmt.Printf("   Sharpe Score:  %.1f / 15 pts (ratio=%.2f, target=3.0)\n", sharpeScore, stats.SharpeRatio)
		fmt.Printf("   ────────────────────────\n")
		fmt.Printf("   TOTAL:         %.1f / 100 pts\n", stats.TrustScore)
	}

	fmt.Println("\n✅ Test Complete!")

	// Cleanup
	priceFetcher.Stop()
	sched.Stop()
}

func getPrivateKey() (*ecdsa.PrivateKey, error) {
	keyHex := os.Getenv("VERIFIER_PRIVATE_KEY")
	if keyHex == "" {
		return crypto.GenerateKey()
	}

	if len(keyHex) >= 2 && keyHex[:2] == "0x" {
		keyHex = keyHex[2:]
	}

	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, err
	}

	return crypto.ToECDSA(keyBytes)
}

func calculateRR(p types.SignalPayload) float64 {
	if p.Direction == "LONG" {
		sl := p.EntryPrice - p.StopLoss
		if sl <= 0 {
			return 1.0
		}
		return (p.TakeProfit - p.EntryPrice) / sl
	}
	sl := p.StopLoss - p.EntryPrice
	if sl <= 0 {
		return 1.0
	}
	return (p.EntryPrice - p.TakeProfit) / sl
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
