package scorer

import (
	"math"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
)

// Tier represents a node's reputation bracket, which controls subscription price
// and the creator/platform fee split.
type Tier string

const (
	TierBronze  Tier = "BRONZE"  // Score  0–40  | $10/mo  | 60% creator
	TierSilver  Tier = "SILVER"  // Score 41–70  | $25/mo  | 70% creator
	TierGold    Tier = "GOLD"    // Score 71–90  | $50/mo  | 80% creator
	TierDiamond Tier = "DIAMOND" // Score 91–100 | $100/mo | 85% creator
)

// TierIndex maps Tier → uint8 for on-chain encoding (matches Solidity enum).
var TierIndex = map[Tier]uint8{
	TierBronze:  0,
	TierSilver:  1,
	TierGold:    2,
	TierDiamond: 3,
}

// TierCreatorBps is the creator's share of each subscription payment, in basis points.
// Platform receives (10000 - creatorBps).
var TierCreatorBps = map[Tier]uint16{
	TierBronze:  6000, // 60% creator / 40% platform
	TierSilver:  7000, // 70% creator / 30% platform
	TierGold:    8000, // 80% creator / 20% platform
	TierDiamond: 8500, // 85% creator / 15% platform
}

// TierPriceUSDCents is the monthly subscription price per tier in USD cents.
var TierPriceUSDCents = map[Tier]uint64{
	TierBronze:  1000,  // $10
	TierSilver:  2500,  // $25
	TierGold:    5000,  // $50
	TierDiamond: 10000, // $100
}

// NodeStats is the full computed reputation snapshot for a single AI Trading Signal Agent.
// Note: Named "NodeStats" for backward compatibility, but represents "Agent" stats.
type NodeStats struct {
	NodeID       string  `json:"node_id"`
	TotalSignals int     `json:"total_signals"`
	WinCount     int     `json:"win_count"`
	LossCount    int     `json:"loss_count"`
	ExpiredCount int     `json:"expired_count"`
	WinRate      float64 `json:"win_rate"`     // 0.0–1.0
	AvgEV        float64 `json:"avg_ev"`       // Time-decay-weighted expected value
	SharpeRatio  float64 `json:"sharpe_ratio"` // Risk-adjusted consistency score
	TrustScore   float64 `json:"trust_score"`  // 0–100, the canonical reputation number
	Tier         Tier    `json:"tier"`
	UpdatedAt    int64   `json:"updated_at"` // Unix timestamp

	// Agentic ID fields (ERC-7857 integration for verified AI agents)
	IsVerified bool   `json:"is_verified"`          // True if linked to Agentic ID NFT
	AgenticId  uint64 `json:"agentic_id,omitempty"` // ERC-7857 token ID (0 if not verified)
}

// Scorer computes NodeStats from a slice of resolved ActiveSignals.
//
// Scoring formula (out of 100):
//   - EV Score  (max 60 pts): time-decay-weighted expected value, normalised against
//     an EV of 2.0 (excellent). A node with EV ≥ 2.0 gets the full 60 pts.
//   - Win Rate  (max 25 pts): raw wins / (wins + losses).
//   - Sharpe    (max 15 pts): mean(returns)/stddev(returns), normalised against 3.0.
//
// Time decay: each signal is weighted by exp(-λ·days_since_close).
// Default λ = 0.05 → half-life ≈ 14 days. Recent performance dominates.
//
// Weight penalty: signals with high weight_pct that lose are naturally penalised
// more heavily because their return contribution is scaled by (weight_pct / 100).
type Scorer struct {
	DecayLambda float64 // default 0.05

	mu        sync.RWMutex
	nodeStats map[string]*NodeStats

	// closedSignals stores all resolved signals per node for re-scoring.
	closedSignals map[string][]*types.ActiveSignal

	// aiProofs stores AI evaluation proofs for 0G Storage daily batch upload
	aiProofs []AIProof
}

func NewScorer() *Scorer {
	return &Scorer{
		DecayLambda:   0.05,
		nodeStats:     make(map[string]*NodeStats),
		closedSignals: make(map[string][]*types.ActiveSignal),
		aiProofs:      make([]AIProof, 0),
	}
}

// AddAIProof adds an AI evaluation proof to be included in daily 0G Storage batch
func (s *Scorer) AddAIProof(proof AIProof) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiProofs = append(s.aiProofs, proof)
}

// GetAIProofs returns all pending AI proofs (for batch upload)
func (s *Scorer) GetAIProofs() []AIProof {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]AIProof, len(s.aiProofs))
	copy(result, s.aiProofs)
	return result
}

// ClearAIProofs clears the proofs after successful upload
func (s *Scorer) ClearAIProofs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiProofs = make([]AIProof, 0)
}

// RecordClosed ingests a newly closed signal and immediately recomputes the node score.
func (s *Scorer) RecordClosed(sig *types.ActiveSignal) NodeStats {
	nodeID := sig.Envelope.NodeID

	s.mu.Lock()
	s.closedSignals[nodeID] = append(s.closedSignals[nodeID], sig)
	signals := s.closedSignals[nodeID]
	s.mu.Unlock()

	stats := s.compute(nodeID, signals)

	s.mu.Lock()
	s.nodeStats[nodeID] = &stats
	s.mu.Unlock()

	return stats
}

// GetStats returns the cached stats for a node (thread-safe, non-blocking).
func (s *Scorer) GetStats(nodeID string) (NodeStats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats, ok := s.nodeStats[nodeID]
	if !ok {
		return NodeStats{}, false
	}
	return *stats, true
}

// AllStats returns a snapshot of all tracked nodes.
func (s *Scorer) AllStats() []NodeStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]NodeStats, 0, len(s.nodeStats))
	for _, s := range s.nodeStats {
		out = append(out, *s)
	}
	return out
}

// ForBroadcast returns (nodeID, trustScore, tierIndex) tuples for every node
// whose score has changed since the last broadcast cycle. Used by broadcaster.
func (s *Scorer) ForBroadcast() []NodeStats {
	return s.AllStats()
}

// GetSignalHistory returns all closed signals for a node (for analytics)
func (s *Scorer) GetSignalHistory(nodeID string) []*types.ActiveSignal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if signals, ok := s.closedSignals[nodeID]; ok {
		// Return a copy to avoid race conditions
		result := make([]*types.ActiveSignal, len(signals))
		copy(result, signals)
		return result
	}
	return nil
}

// GetAllSignalHistory returns all closed signals across all nodes
func (s *Scorer) GetAllSignalHistory() []*types.ActiveSignal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []*types.ActiveSignal
	for _, signals := range s.closedSignals {
		all = append(all, signals...)
	}
	return all
}

// ─── Core computation ────────────────────────────────────────────────────────

func (s *Scorer) compute(nodeID string, signals []*types.ActiveSignal) NodeStats {
	now := time.Now().Unix()

	var (
		totalTimeWeight       float64
		weightedEVSum         float64
		returns               []float64
		wins, losses, expired int
	)

	for _, sig := range signals {
		switch sig.State {
		case types.StateClosedWin, types.StateClosedLoss, types.StateExpired:
			// only scored states
		default:
			continue
		}

		payload := sig.Envelope.Payload

		// Time-decay weight: exp(-λ · days_ago). A signal from today has weight 1.0;
		// one from 14 days ago has weight ≈ 0.5 (with default λ=0.05).
		daysAgo := math.Max(float64(now-sig.ClosedAt)/86400.0, 0)
		tw := math.Exp(-s.DecayLambda * daysAgo)

		// Confidence scaling: weight_pct/100 makes high-conviction signals count more.
		weightScale := payload.WeightPct / 100.0

		var ret float64
		switch sig.State {
		case types.StateClosedWin:
			rr := riskReward(payload)
			ret = rr * weightScale
			wins++

		case types.StateClosedLoss:
			ret = -1.0 * weightScale
			losses++

		case types.StateExpired:
			// Measure actual PnL at expiry vs entry.
			expired++
			if payload.EntryPrice > 0 && sig.ClosedPrice > 0 {
				var pct float64
				if payload.Direction == "LONG" {
					pct = (sig.ClosedPrice - payload.EntryPrice) / payload.EntryPrice
				} else {
					pct = (payload.EntryPrice - sig.ClosedPrice) / payload.EntryPrice
				}
				ret = pct * weightScale
				// Only count as win/loss if we have valid price data
				if ret > 0 {
					wins++
				} else if ret < 0 {
					losses++
				}
				// ret == 0 exactly is neutral, don't count as win or loss
			}
			// If no valid price data (ClosedPrice == 0), don't count as win or loss
		}

		weightedEVSum += ret * tw
		totalTimeWeight += tw
		returns = append(returns, ret)
	}

	total := wins + losses
	if total == 0 {
		return NodeStats{NodeID: nodeID, Tier: TierBronze, UpdatedAt: now}
	}

	ev := 0.0
	if totalTimeWeight > 0 {
		ev = weightedEVSum / totalTimeWeight
	}
	winRate := float64(wins) / float64(total)
	sharpe := calcSharpe(returns)

	// EV score: EV of 2.0 = 60 pts (theoretical max for consistent 2:1 RR 100% wins).
	evScore := math.Min(math.Max(ev, 0)/2.0, 1.0) * 60.0
	// Win rate score: 100% wins = 25 pts.
	winRateScore := winRate * 25.0
	// Sharpe score: Sharpe of 3.0 = 15 pts.
	sharpeScore := math.Min(math.Max(sharpe, 0)/3.0, 1.0) * 15.0

	trust := math.Min(math.Max(evScore+winRateScore+sharpeScore, 0), 100)

	return NodeStats{
		NodeID:       nodeID,
		TotalSignals: len(signals),
		WinCount:     wins,
		LossCount:    losses,
		ExpiredCount: expired,
		WinRate:      winRate,
		AvgEV:        ev,
		SharpeRatio:  sharpe,
		TrustScore:   trust,
		Tier:         tierFromScore(trust),
		UpdatedAt:    now,
	}
}

// riskReward returns the (TP distance) / (SL distance) ratio for a payload,
// i.e. how many dollars the node wins per dollar risked if it hits TP.
func riskReward(p types.SignalPayload) float64 {
	if p.Direction == "LONG" {
		sl := p.EntryPrice - p.StopLoss
		if sl <= 0 {
			return 1.0
		}
		return (p.TakeProfit - p.EntryPrice) / sl
	}
	// SHORT
	sl := p.StopLoss - p.EntryPrice
	if sl <= 0 {
		return 1.0
	}
	return (p.EntryPrice - p.TakeProfit) / sl
}

// calcSharpe returns the Sharpe ratio of a return series (mean / stddev).
// Returns 0 when there is insufficient data or zero variance.
func calcSharpe(returns []float64) float64 {
	n := len(returns)
	if n < 2 {
		return 0
	}
	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(n)

	var varSum float64
	for _, r := range returns {
		d := r - mean
		varSum += d * d
	}
	stddev := math.Sqrt(varSum / float64(n-1))
	if stddev < 1e-9 {
		return 0
	}
	return mean / stddev
}

func tierFromScore(score float64) Tier {
	switch {
	case score >= 91:
		return TierDiamond
	case score >= 71:
		return TierGold
	case score >= 41:
		return TierSilver
	default:
		return TierBronze
	}
}
