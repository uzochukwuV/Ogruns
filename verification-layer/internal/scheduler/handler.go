package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/internal/scorer"
	"github.com/0xprotocol/verification-layer/pkg/types"
)

// WebhookPayload is sent to subscribers when a signal is analyzed
type WebhookPayload struct {
	EventType     string  `json:"event_type"`
	SignalID      string  `json:"signal_id"`
	NodeID        string  `json:"node_id"`
	TokenPair     string  `json:"token_pair"`
	Direction     string  `json:"direction"`
	TradeType     string  `json:"trade_type"`            // "spot", "perpetual", "futures"
	Leverage      float64 `json:"leverage"`              // 1x for spot, higher for perps
	EntryPrice    float64 `json:"entry_price"`
	TargetPrice   float64 `json:"target_price"`
	StopLoss      float64 `json:"stop_loss"`
	PriceAtExpiry float64 `json:"price_at_expiry"`
	Outcome       string  `json:"outcome"`
	PnLPercent    float64 `json:"pnl_percent"`
	LeveragedPnL  float64 `json:"leveraged_pnl_percent"` // PnL with leverage applied
	RiskReward    float64 `json:"risk_reward"`
	// Node's updated reputation after this signal
	NodeTrustScore float64 `json:"node_trust_score"`
	NodeTier       string  `json:"node_tier"`
	NodeWinRate    float64 `json:"node_win_rate"`
	NodeSharpe     float64 `json:"node_sharpe_ratio"`
	AnalyzedAt     int64   `json:"analyzed_at"`
	// 0G AI evaluation (if available)
	AIScore       float64  `json:"ai_score,omitempty"`        // 0-100 quality score from 0G AI
	AIRiskRating  string   `json:"ai_risk_rating,omitempty"`  // LOW, MEDIUM, HIGH, EXTREME
	AIReasoning   string   `json:"ai_reasoning,omitempty"`    // AI's explanation
	AITEEVerified bool     `json:"ai_tee_verified,omitempty"` // TEE verification status
}

// OnChainBroadcaster interface for pushing scores on-chain
type OnChainBroadcaster interface {
	UpdateNodeScore(ctx context.Context, nodeID string, score float64, tier uint8) error
}

// AnalysisHandler processes analysis results using the sophisticated Scorer
type AnalysisHandler struct {
	mu          sync.RWMutex
	scorer      *scorer.Scorer
	aiScorer    *scorer.AIScorer    // 0G AI-powered signal evaluator
	webhooks    map[string][]string // nodeID -> webhook URLs
	httpClient  *http.Client
	broadcaster OnChainBroadcaster // Optional on-chain broadcaster
}

func NewAnalysisHandler() *AnalysisHandler {
	return &AnalysisHandler{
		scorer:     scorer.NewScorer(),
		aiScorer:   scorer.NewAIScorer(),
		webhooks:   make(map[string][]string),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetBroadcaster sets the on-chain broadcaster for pushing scores
func (h *AnalysisHandler) SetBroadcaster(b OnChainBroadcaster) {
	h.broadcaster = b
}

// RegisterWebhook registers a webhook URL for a node
func (h *AnalysisHandler) RegisterWebhook(nodeID, webhookURL string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.webhooks[nodeID]; !exists {
		h.webhooks[nodeID] = []string{}
	}
	h.webhooks[nodeID] = append(h.webhooks[nodeID], webhookURL)
	log.Printf("Handler: Registered webhook for node %s: %s", nodeID, webhookURL)
}


// HandleResult processes an analysis result using AI-adjusted scoring
func (h *AnalysisHandler) HandleResult(result AnalysisResult) {
	nodeID := result.Signal.Envelope.NodeID

	// Update the signal with final price before scoring
	result.Signal.ClosedPrice = result.PriceAtExpiry
	result.Signal.ClosedAt = result.AnalyzedAt

	// Step 1: Get algorithmic score
	algorithmicStats := h.scorer.RecordClosed(result.Signal)

	// Step 2: Build signal history for manipulation detection
	history := h.buildSignalHistory(nodeID)

	// Step 3: Determine outcome string
	outcome := "EXPIRED"
	if result.Outcome == types.StateClosedWin {
		outcome = "WIN"
	} else if result.Outcome == types.StateClosedLoss {
		outcome = "LOSS"
	}

	// Step 4: Get AI-adjusted score (this is the FINAL score for on-chain)
	var finalScore float64
	var aiResult *scorer.AIScoreResult

	if h.aiScorer != nil && h.aiScorer.IsEnabled() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		aiResult, _ = h.aiScorer.EvaluateAndAdjustScore(
			ctx,
			result.Signal.Envelope,
			algorithmicStats.TrustScore,
			history,
			outcome,
			result.PnLPercent,
		)
		cancel()

		if aiResult != nil {
			finalScore = aiResult.Score
		} else {
			finalScore = algorithmicStats.TrustScore
		}
	} else {
		finalScore = algorithmicStats.TrustScore
	}

	// Step 5: Update stats with AI-adjusted score
	finalStats := algorithmicStats
	finalStats.TrustScore = finalScore
	finalStats.Tier = tierFromScore(finalScore)

	// Step 6: Store AI proof locally (will be included in daily 0G Storage batch)
	if aiResult != nil {
		proof := scorer.AIProof{
			NodeID:           nodeID,
			SignalID:         result.Signal.ID,
			Timestamp:        time.Now().Unix(),
			AlgorithmicScore: algorithmicStats.TrustScore,
			AIAdjustedScore:  finalScore,
			Adjustment:       aiResult.Adjustment,
			ManipulationFlag: aiResult.ManipulationFlag,
			ManipulationType: aiResult.ManipulationType,
			AIReasoning:      aiResult.Reasoning,
			TEEVerified:      aiResult.Verified,
			Outcome:          outcome,
			PnLPercent:       result.PnLPercent,
			FinalTier:        string(finalStats.Tier),
		}
		h.scorer.AddAIProof(proof)
		log.Printf("Handler: 📝 AI proof stored locally for %s (will upload in daily batch)", nodeID[:16]+"...")
	}

	// Step 7: Fire webhooks with AI-adjusted reputation
	h.fireWebhooksWithAI(nodeID, result, finalStats, aiResult)

	// Step 8: Push AI-ADJUSTED score on-chain (this is the authoritative score)
	if h.broadcaster != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tierIndex := scorer.TierIndex[finalStats.Tier]
			err := h.broadcaster.UpdateNodeScore(ctx, nodeID, finalScore, tierIndex)
			if err != nil {
				log.Printf("Handler: ❌ Failed to push score on-chain for %s: %v", nodeID[:16]+"...", err)
			} else {
				log.Printf("Handler: ✅ AI-adjusted score pushed on-chain for %s (%.1f, %s)", nodeID[:16]+"...", finalScore, finalStats.Tier)
			}
		}()
	}

	// Log with AI adjustment details
	if aiResult != nil && aiResult.Adjustment != 0 {
		log.Printf("Handler: Node %s -> Algo: %.1f, AI: %.1f (adj: %+.1f), Tier: %s, Manipulation: %v",
			nodeID[:16]+"...",
			algorithmicStats.TrustScore,
			finalScore,
			aiResult.Adjustment,
			finalStats.Tier,
			aiResult.ManipulationFlag,
		)
	} else {
		log.Printf("Handler: Node %s updated -> Trust: %.1f, Tier: %s, WinRate: %.1f%%, Sharpe: %.2f",
			nodeID[:16]+"...",
			finalScore,
			finalStats.Tier,
			finalStats.WinRate*100,
			finalStats.SharpeRatio,
		)
	}
}

// buildSignalHistory builds history from closed signals for manipulation detection
func (h *AnalysisHandler) buildSignalHistory(nodeID string) []scorer.SignalHistory {
	signals := h.scorer.GetSignalHistory(nodeID)
	if signals == nil {
		return nil
	}

	history := make([]scorer.SignalHistory, 0, len(signals))
	for _, sig := range signals {
		outcome := "EXPIRED"
		if sig.State == types.StateClosedWin {
			outcome = "WIN"
		} else if sig.State == types.StateClosedLoss {
			outcome = "LOSS"
		}

		pnl := 0.0
		if sig.ClosedPrice > 0 && sig.Envelope.Payload.EntryPrice > 0 {
			if sig.Envelope.Payload.Direction == "long" {
				pnl = ((sig.ClosedPrice - sig.Envelope.Payload.EntryPrice) / sig.Envelope.Payload.EntryPrice) * 100
			} else {
				pnl = ((sig.Envelope.Payload.EntryPrice - sig.ClosedPrice) / sig.Envelope.Payload.EntryPrice) * 100
			}
		}

		history = append(history, scorer.SignalHistory{
			TokenPair:  sig.Envelope.Payload.TokenPair,
			Direction:  sig.Envelope.Payload.Direction,
			EntryPrice: sig.Envelope.Payload.EntryPrice,
			Timestamp:  sig.ClosedAt,
			Outcome:    outcome,
			PnLPercent: pnl,
		})
	}

	return history
}

// tierFromScore determines tier from score (mirrors scorer.go)
func tierFromScore(score float64) scorer.Tier {
	switch {
	case score >= 91:
		return scorer.TierDiamond
	case score >= 71:
		return scorer.TierGold
	case score >= 41:
		return scorer.TierSilver
	default:
		return scorer.TierBronze
	}
}

// fireWebhooksWithAI sends webhooks with AI evaluation data
func (h *AnalysisHandler) fireWebhooksWithAI(nodeID string, result AnalysisResult, stats scorer.NodeStats, aiResult *scorer.AIScoreResult) {
	h.mu.RLock()
	urls, exists := h.webhooks[nodeID]
	h.mu.RUnlock()

	if !exists || len(urls) == 0 {
		return
	}

	payload := result.Signal.Envelope.Payload
	rr := calculateRiskReward(payload)
	leverage := payload.GetLeverage()
	leveragedPnL := result.PnLPercent * leverage

	webhookPayload := WebhookPayload{
		EventType:      "SIGNAL_ANALYZED",
		SignalID:       result.Signal.ID,
		NodeID:         nodeID,
		TokenPair:      payload.TokenPair,
		Direction:      payload.Direction,
		TradeType:      payload.GetTradeType(),
		Leverage:       leverage,
		EntryPrice:     payload.EntryPrice,
		TargetPrice:    payload.TakeProfit,
		StopLoss:       payload.StopLoss,
		PriceAtExpiry:  result.PriceAtExpiry,
		Outcome:        string(result.Outcome),
		PnLPercent:     result.PnLPercent,
		LeveragedPnL:   leveragedPnL,
		RiskReward:     rr,
		NodeTrustScore: stats.TrustScore,
		NodeTier:       string(stats.Tier),
		NodeWinRate:    stats.WinRate,
		NodeSharpe:     stats.SharpeRatio,
		AnalyzedAt:     result.AnalyzedAt,
	}

	// Add AI evaluation data
	if aiResult != nil {
		webhookPayload.AIScore = aiResult.Score
		webhookPayload.AIRiskRating = aiResult.RiskRating
		webhookPayload.AIReasoning = aiResult.Reasoning
		webhookPayload.AITEEVerified = aiResult.Verified
	}

	jsonPayload, err := json.Marshal(webhookPayload)
	if err != nil {
		log.Printf("Handler: Failed to marshal webhook payload: %v", err)
		return
	}

	for _, url := range urls {
		go h.sendWebhook(url, jsonPayload)
	}
}

func (h *AnalysisHandler) sendWebhook(url string, payload []byte) {
	resp, err := h.httpClient.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("Handler: Webhook failed to %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("Handler: Webhook sent successfully to %s", url)
	} else {
		log.Printf("Handler: Webhook returned %d from %s", resp.StatusCode, url)
	}
}

// GetNodeStats returns the sophisticated stats for a node
func (h *AnalysisHandler) GetNodeStats(nodeID string) (scorer.NodeStats, bool) {
	return h.scorer.GetStats(nodeID)
}

// GetAllNodeStats returns all node statistics
func (h *AnalysisHandler) GetAllNodeStats() []scorer.NodeStats {
	return h.scorer.AllStats()
}

// GetScorer returns the underlying scorer for direct access
func (h *AnalysisHandler) GetScorer() *scorer.Scorer {
	return h.scorer
}

// GetTierInfo returns pricing and fee split info for a tier
func (h *AnalysisHandler) GetTierInfo(tier scorer.Tier) TierInfo {
	return TierInfo{
		Tier:         string(tier),
		PriceUSD:     float64(scorer.TierPriceUSDCents[tier]) / 100.0,
		CreatorSplit: float64(scorer.TierCreatorBps[tier]) / 100.0,
	}
}

// TierInfo contains pricing and revenue split information
type TierInfo struct {
	Tier         string  `json:"tier"`
	PriceUSD     float64 `json:"price_usd"`      // Monthly subscription price
	CreatorSplit float64 `json:"creator_split"`  // Percentage to creator (e.g., 85.0)
}

// calculateRiskReward computes the risk/reward ratio
func calculateRiskReward(p types.SignalPayload) float64 {
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

// Summary returns a summary of all nodes for logging
func (h *AnalysisHandler) Summary() string {
	stats := h.scorer.AllStats()
	if len(stats) == 0 {
		return "No nodes scored yet"
	}

	summary := "=== Node Reputation Summary ===\n"
	for _, s := range stats {
		tierInfo := h.GetTierInfo(s.Tier)
		summary += "─────────────────────────────────────\n"
		summary += "Node: " + s.NodeID[:20] + "...\n"
		summary += "  Trust Score: " + formatFloat(s.TrustScore) + "/100\n"
		summary += "  Tier: " + string(s.Tier) + " ($" + formatFloat(tierInfo.PriceUSD) + "/mo)\n"
		summary += "  Win Rate: " + formatFloat(s.WinRate*100) + "%\n"
		summary += "  Sharpe Ratio: " + formatFloat(s.SharpeRatio) + "\n"
		summary += "  Expected Value: " + formatFloat(s.AvgEV) + "\n"
		summary += "  Signals: " + formatInt(s.TotalSignals) + " (W:" + formatInt(s.WinCount) + " L:" + formatInt(s.LossCount) + " E:" + formatInt(s.ExpiredCount) + ")\n"
	}
	return summary
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

func formatInt(i int) string {
	return fmt.Sprintf("%d", i)
}
