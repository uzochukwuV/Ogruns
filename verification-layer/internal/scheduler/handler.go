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
	EventType    string  `json:"event_type"`
	SignalID     string  `json:"signal_id"`
	NodeID       string  `json:"node_id"`
	TokenPair    string  `json:"token_pair"`
	Direction    string  `json:"direction"`
	EntryPrice   float64 `json:"entry_price"`
	TargetPrice  float64 `json:"target_price"`
	StopLoss     float64 `json:"stop_loss"`
	PriceAtExpiry float64 `json:"price_at_expiry"`
	Outcome      string  `json:"outcome"`
	PnLPercent   float64 `json:"pnl_percent"`
	RiskReward   float64 `json:"risk_reward"`
	// Node's updated reputation after this signal
	NodeTrustScore float64 `json:"node_trust_score"`
	NodeTier       string  `json:"node_tier"`
	NodeWinRate    float64 `json:"node_win_rate"`
	NodeSharpe     float64 `json:"node_sharpe_ratio"`
	AnalyzedAt     int64   `json:"analyzed_at"`
}

// OnChainBroadcaster interface for pushing scores on-chain
type OnChainBroadcaster interface {
	UpdateNodeScore(ctx context.Context, nodeID string, score float64, tier uint8) error
}

// AnalysisHandler processes analysis results using the sophisticated Scorer
type AnalysisHandler struct {
	mu          sync.RWMutex
	scorer      *scorer.Scorer
	webhooks    map[string][]string // nodeID -> webhook URLs
	httpClient  *http.Client
	broadcaster OnChainBroadcaster // Optional on-chain broadcaster
}

func NewAnalysisHandler() *AnalysisHandler {
	return &AnalysisHandler{
		scorer:     scorer.NewScorer(),
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

// HandleResult processes an analysis result using the sophisticated scorer
func (h *AnalysisHandler) HandleResult(result AnalysisResult) {
	nodeID := result.Signal.Envelope.NodeID

	// Update the signal with final price before scoring
	result.Signal.ClosedPrice = result.PriceAtExpiry
	result.Signal.ClosedAt = result.AnalyzedAt

	// Use the sophisticated scorer to update node reputation
	stats := h.scorer.RecordClosed(result.Signal)

	// Fire webhooks with updated reputation
	h.fireWebhooks(nodeID, result, stats)

	// Push score on-chain if broadcaster is configured
	if h.broadcaster != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tierIndex := scorer.TierIndex[stats.Tier]
			err := h.broadcaster.UpdateNodeScore(ctx, nodeID, stats.TrustScore, tierIndex)
			if err != nil {
				log.Printf("Handler: ❌ Failed to push score on-chain for %s: %v", nodeID[:16]+"...", err)
			} else {
				log.Printf("Handler: ✅ Score pushed on-chain for %s (%.1f, %s)", nodeID[:16]+"...", stats.TrustScore, stats.Tier)
			}
		}()
	}

	// Log the result with full scoring details
	log.Printf("Handler: Node %s updated -> Trust: %.1f, Tier: %s, WinRate: %.1f%%, Sharpe: %.2f, EV: %.2f",
		nodeID[:16]+"...",
		stats.TrustScore,
		stats.Tier,
		stats.WinRate*100,
		stats.SharpeRatio,
		stats.AvgEV,
	)
}

func (h *AnalysisHandler) fireWebhooks(nodeID string, result AnalysisResult, stats scorer.NodeStats) {
	h.mu.RLock()
	urls, exists := h.webhooks[nodeID]
	h.mu.RUnlock()

	if !exists || len(urls) == 0 {
		return
	}

	payload := result.Signal.Envelope.Payload
	rr := calculateRiskReward(payload)

	webhookPayload := WebhookPayload{
		EventType:      "SIGNAL_ANALYZED",
		SignalID:       result.Signal.ID,
		NodeID:         nodeID,
		TokenPair:      payload.TokenPair,
		Direction:      payload.Direction,
		EntryPrice:     payload.EntryPrice,
		TargetPrice:    payload.TakeProfit,
		StopLoss:       payload.StopLoss,
		PriceAtExpiry:  result.PriceAtExpiry,
		Outcome:        string(result.Outcome),
		PnLPercent:     result.PnLPercent,
		RiskReward:     rr,
		NodeTrustScore: stats.TrustScore,
		NodeTier:       string(stats.Tier),
		NodeWinRate:    stats.WinRate,
		NodeSharpe:     stats.SharpeRatio,
		AnalyzedAt:     result.AnalyzedAt,
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
