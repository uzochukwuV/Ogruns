package ingester

import (
	"context"
	"log"
	"time"

	"github.com/0xprotocol/verification-layer/internal/contracts"
	"github.com/0xprotocol/verification-layer/internal/scorer"
)

// Broadcaster reads the finalized scores from the Scorer and writes them
// to the ReputationOracle.sol smart contract once per interval.
type Broadcaster struct {
	sc           *scorer.Scorer
	contracts    *contracts.ContractManager
	syncInterval time.Duration
}

func NewBroadcaster(sc *scorer.Scorer, cm *contracts.ContractManager, intervalSec int) *Broadcaster {
	return &Broadcaster{
		sc:           sc,
		contracts:    cm,
		syncInterval: time.Duration(intervalSec) * time.Second,
	}
}

func (b *Broadcaster) Start(ctx context.Context) {
	if b.contracts == nil {
		log.Println("Broadcaster: ⚠️ No Contract Manager provided. Running in simulation mode.")
		return
	}

	log.Printf("Broadcaster: Started. Syncing Reputation Scores every %s", b.syncInterval)
	ticker := time.NewTicker(b.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.syncScores(ctx)
		}
	}
}

func (b *Broadcaster) syncScores(ctx context.Context) {
	// In production, we'd only sync nodes whose scores changed recently
	stats := b.sc.AllStats()
	if len(stats) == 0 {
		return
	}

	log.Printf("Broadcaster: 📡 Syncing %d scores to ReputationOracle...", len(stats))
	for _, stat := range stats {
		err := b.contracts.UpdateNodeScore(ctx, stat.NodeID, stat.TrustScore, scorer.TierIndex[stat.Tier])
		if err != nil {
			log.Printf("Broadcaster: ❌ Failed to update score for %s: %v", stat.NodeID, err)
		}
	}
	log.Println("Broadcaster: ✅ On-chain sync complete.")
}
