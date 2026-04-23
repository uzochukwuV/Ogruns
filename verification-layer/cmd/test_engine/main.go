package main

import (
	"fmt"
	"log"
	"time"

	"github.com/0xprotocol/verification-layer/internal/aggregator"
	"github.com/0xprotocol/verification-layer/internal/engine"
	"github.com/0xprotocol/verification-layer/pkg/types"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("📈 Verification Layer - CEX Engine Integration Test")
	fmt.Println("==================================================")

	// 1. Initialize the Aggregator
	agg := aggregator.NewCEXAggregator()
	agg.Start()
	fmt.Println("[1] Started Binance WebSocket Aggregator (!miniTicker@arr)")

	// 2. Initialize the Resolution Engine, passing it the Aggregator's stream
	eng := engine.NewResolutionEngine(agg.TickStream)
	eng.Start()
	fmt.Println("[2] Started In-Memory Resolution Engine")

	// 3. Inject a Mock Signal
	// Let's create a very tight signal on BTC to guarantee it triggers immediately
	fmt.Println("\n[3] Injecting a mock PENDING signal for BTC/USDT...")
	
	// We wait for the first tick to get the current price so we can set a realistic target
	var currentBTCPrice float64
	for tick := range agg.TickStream {
		if tick.TokenPair == "BTC/USDT" {
			currentBTCPrice = tick.Price
			break
		}
	}

	fmt.Printf("    Current BTC Price: %.2f\n", currentBTCPrice)
	
	// Create a LONG signal just below current price to force an entry
	mockEnvelope := types.SignalEnvelope{
		NodeID:    "0xMOCK_NODE_DIAMOND",
		Timestamp: time.Now().Unix(),
		Payload: types.SignalPayload{
			TokenPair:  "BTC/USDT",
			Direction:  "LONG",
			EntryPrice: currentBTCPrice + 1.00, // Slightly higher to guarantee it hits instantly on next tick
			TakeProfit: currentBTCPrice + 5.00, // $5 profit target
			StopLoss:   currentBTCPrice - 5.00, // $5 stop loss
			ExpiryTime: time.Now().Add(1 * time.Hour).Unix(),
			WeightPct:  100.0,
		},
		Signature: "0xMOCK_SIGNATURE",
	}

	eng.AddSignal(mockEnvelope)

	// 4. Wait for the engine to resolve the signal
	fmt.Println("\n[4] Listening for State Transitions from the Engine...")
	
	timeout := time.After(30 * time.Second)
	
	for {
		select {
		case <-timeout:
			log.Println("Test Timed Out waiting for BTC to move $5.")
			return
		case event := <-eng.EventStream():
			fmt.Printf("    🔥 STATE CHANGE: Signal %s transitioned to %s at Price: %.2f\n", event.ID, event.State, event.ClosedPrice)
			if event.State == types.StateClosedWin || event.State == types.StateClosedLoss {
				fmt.Println("\n==================================================")
				fmt.Println("🏁 Engine Integration Test Complete!")
				agg.Stop()
				eng.Stop()
				return
			}
		}
	}
}
