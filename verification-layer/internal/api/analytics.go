package api

import (
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0xprotocol/verification-layer/internal/scorer"
	"github.com/0xprotocol/verification-layer/pkg/types"
)

// ─── Analytics Types ─────────────────────────────────────────────────────────

// SignalHistoryItem represents a single historical signal for frontend display
type SignalHistoryItem struct {
	ID          string  `json:"id"`
	TokenPair   string  `json:"token_pair"`
	Direction   string  `json:"direction"`
	EntryPrice  float64 `json:"entry_price"`
	TakeProfit  float64 `json:"take_profit"`
	StopLoss    float64 `json:"stop_loss"`
	ClosedPrice float64 `json:"closed_price"`
	PnLPercent  float64 `json:"pnl_percent"`
	Outcome     string  `json:"outcome"`      // "WIN", "LOSS", "EXPIRED"
	OutcomeType string  `json:"outcome_type"` // "full_win", "partial_win", "partial_loss", "full_loss"
	Confidence  float64 `json:"confidence"`   // weight_pct
	CreatedAt   int64   `json:"created_at"`
	ClosedAt    int64   `json:"closed_at"`
	Duration    int64   `json:"duration_sec"` // seconds from creation to close
}

// NodeAnalytics contains detailed analytics for a single node
type NodeAnalytics struct {
	NodeID         string              `json:"node_id"`
	Stats          scorer.NodeStats    `json:"stats"`
	SignalHistory  []SignalHistoryItem `json:"signal_history"`
	PerformanceByToken map[string]TokenPerformance `json:"performance_by_token"`
	MonthlyReturns []MonthlyReturn     `json:"monthly_returns"`
	Streaks        StreakInfo          `json:"streaks"`
}

// TokenPerformance shows performance breakdown by trading pair
type TokenPerformance struct {
	TokenPair    string  `json:"token_pair"`
	TotalSignals int     `json:"total_signals"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	WinRate      float64 `json:"win_rate"`
	AvgPnL       float64 `json:"avg_pnl_percent"`
	TotalPnL     float64 `json:"total_pnl_percent"`
}

// MonthlyReturn represents monthly performance
type MonthlyReturn struct {
	Month      string  `json:"month"`       // "2024-01"
	PnLPercent float64 `json:"pnl_percent"`
	Signals    int     `json:"signals"`
	WinRate    float64 `json:"win_rate"`
}

// StreakInfo tracks win/loss streaks
type StreakInfo struct {
	CurrentStreak     int    `json:"current_streak"`      // positive = wins, negative = losses
	CurrentStreakType string `json:"current_streak_type"` // "winning" or "losing"
	LongestWinStreak  int    `json:"longest_win_streak"`
	LongestLossStreak int    `json:"longest_loss_streak"`
}

// PortfolioSimulation shows hypothetical returns
type PortfolioSimulation struct {
	InitialCapital   float64                 `json:"initial_capital"`
	FinalValue       float64                 `json:"final_value"`
	TotalReturn      float64                 `json:"total_return_percent"`
	MaxDrawdown      float64                 `json:"max_drawdown_percent"`
	SharpeRatio      float64                 `json:"sharpe_ratio"`
	Trades           int                     `json:"total_trades"`
	TimeframeDays    int                     `json:"timeframe_days"`
	EquityCurve      []EquityPoint           `json:"equity_curve"`
	TradeHistory     []TradeResult           `json:"trade_history"`
}

// EquityPoint represents a point on the equity curve
type EquityPoint struct {
	Timestamp int64   `json:"timestamp"`
	Equity    float64 `json:"equity"`
	PnL       float64 `json:"pnl_percent"`
}

// TradeResult represents a single trade in the simulation
type TradeResult struct {
	SignalID    string  `json:"signal_id"`
	TokenPair   string  `json:"token_pair"`
	Direction   string  `json:"direction"`
	EntryPrice  float64 `json:"entry_price"`
	ExitPrice   float64 `json:"exit_price"`
	PnLPercent  float64 `json:"pnl_percent"`
	PnLAmount   float64 `json:"pnl_amount"`
	Outcome     string  `json:"outcome"`
	OutcomeType string  `json:"outcome_type"` // "green", "yellow", "red"
	Timestamp   int64   `json:"timestamp"`
}

// ─── Analytics Handlers ──────────────────────────────────────────────────────

// handleNodeAnalytics returns detailed analytics for a specific node
// GET /api/v1/analytics/nodes/{nodeId}?from=timestamp&to=timestamp
// PREMIUM ENDPOINT: Requires on-chain subscription to access detailed analytics
func (s *Server) handleNodeAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract nodeId from path
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/analytics/nodes/"), "/")
	nodeID := parts[0]
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing node_id"})
		return
	}

	// 🔒 SUBSCRIPTION GATE: Verify on-chain subscription for detailed analytics
	if subErr := s.subGate.VerifySubscription(r, nodeID); subErr != nil {
		WriteSubscriptionError(w, subErr)
		return
	}

	// Parse optional time range
	fromTs := int64(0)
	toTs := time.Now().Unix()
	if f := r.URL.Query().Get("from"); f != "" {
		if parsed, err := strconv.ParseInt(f, 10, 64); err == nil {
			fromTs = parsed
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := strconv.ParseInt(t, 10, 64); err == nil {
			toTs = parsed
		}
	}

	// Get stats
	stats, ok := s.scorer.GetStats(nodeID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	// Get signal history
	signals := s.scorer.GetSignalHistory(nodeID)
	history := buildSignalHistory(signals, fromTs, toTs)

	// Build token performance breakdown
	tokenPerf := buildTokenPerformance(history)

	// Build monthly returns
	monthlyReturns := buildMonthlyReturns(history)

	// Calculate streaks
	streaks := calculateStreaks(history)

	analytics := NodeAnalytics{
		NodeID:             nodeID,
		Stats:              stats,
		SignalHistory:      history,
		PerformanceByToken: tokenPerf,
		MonthlyReturns:     monthlyReturns,
		Streaks:            streaks,
	}

	writeJSON(w, http.StatusOK, analytics)
}

// handlePortfolioSimulation calculates hypothetical portfolio returns
// GET /api/v1/analytics/nodes/{nodeId}/simulate?capital=1000&from=timestamp&to=timestamp
// PREMIUM ENDPOINT: Requires on-chain subscription
func (s *Server) handlePortfolioSimulation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract nodeId
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/analytics/nodes/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "simulate" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}
	nodeID := parts[0]

	// 🔒 SUBSCRIPTION GATE: Verify on-chain subscription
	if subErr := s.subGate.VerifySubscription(r, nodeID); subErr != nil {
		WriteSubscriptionError(w, subErr)
		return
	}

	// Parse parameters
	capital := 1000.0
	if c := r.URL.Query().Get("capital"); c != "" {
		if parsed, err := strconv.ParseFloat(c, 64); err == nil && parsed > 0 {
			capital = parsed
		}
	}

	fromTs := int64(0)
	toTs := time.Now().Unix()
	if f := r.URL.Query().Get("from"); f != "" {
		if parsed, err := strconv.ParseInt(f, 10, 64); err == nil {
			fromTs = parsed
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := strconv.ParseInt(t, 10, 64); err == nil {
			toTs = parsed
		}
	}

	// Get signal history
	signals := s.scorer.GetSignalHistory(nodeID)
	if len(signals) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no signal history for node"})
		return
	}

	history := buildSignalHistory(signals, fromTs, toTs)
	simulation := runPortfolioSimulation(capital, history, fromTs, toTs)

	writeJSON(w, http.StatusOK, simulation)
}

// handleDashboardSummary returns a summary for the main dashboard
// GET /api/v1/analytics/dashboard
func (s *Server) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allStats := s.scorer.AllStats()
	log.Printf("Dashboard: Returning %d nodes", len(allStats))

	// Calculate aggregates
	var totalSignals, totalWins, totalLosses int
	tierCounts := map[string]int{
		"BRONZE": 0, "SILVER": 0, "GOLD": 0, "DIAMOND": 0,
	}

	topNodes := make([]scorer.NodeStats, 0)
	for _, stats := range allStats {
		totalSignals += stats.TotalSignals
		totalWins += stats.WinCount
		totalLosses += stats.LossCount
		tierCounts[string(stats.Tier)]++
		topNodes = append(topNodes, stats)
	}

	// Sort by trust score descending
	sort.Slice(topNodes, func(i, j int) bool {
		return topNodes[i].TrustScore > topNodes[j].TrustScore
	})

	// Keep top 10
	if len(topNodes) > 10 {
		topNodes = topNodes[:10]
	}

	summary := map[string]interface{}{
		"total_nodes":      len(allStats),
		"total_signals":    totalSignals,
		"total_wins":       totalWins,
		"total_losses":     totalLosses,
		"platform_win_rate": calculateWinRate(totalWins, totalLosses),
		"tier_distribution": tierCounts,
		"top_nodes":        topNodes,
		"updated_at":       time.Now().Unix(),
	}

	writeJSON(w, http.StatusOK, summary)
}

// ─── Helper Functions ────────────────────────────────────────────────────────

func buildSignalHistory(signals []*types.ActiveSignal, fromTs, toTs int64) []SignalHistoryItem {
	history := make([]SignalHistoryItem, 0)

	for _, sig := range signals {
		// Filter by time range
		if sig.ClosedAt < fromTs || sig.ClosedAt > toTs {
			continue
		}

		payload := sig.Envelope.Payload

		// Calculate PnL
		var pnl float64
		if payload.EntryPrice > 0 && sig.ClosedPrice > 0 {
			if payload.Direction == "LONG" {
				pnl = ((sig.ClosedPrice - payload.EntryPrice) / payload.EntryPrice) * 100
			} else {
				pnl = ((payload.EntryPrice - sig.ClosedPrice) / payload.EntryPrice) * 100
			}
		}

		// Determine outcome type (for chart colors)
		outcomeType := determineOutcomeType(sig.State, pnl, payload)

		item := SignalHistoryItem{
			ID:          sig.ID,
			TokenPair:   payload.TokenPair,
			Direction:   payload.Direction,
			EntryPrice:  payload.EntryPrice,
			TakeProfit:  payload.TakeProfit,
			StopLoss:    payload.StopLoss,
			ClosedPrice: sig.ClosedPrice,
			PnLPercent:  pnl,
			Outcome:     string(sig.State),
			OutcomeType: outcomeType,
			Confidence:  payload.WeightPct,
			CreatedAt:   sig.Envelope.Timestamp,
			ClosedAt:    sig.ClosedAt,
			Duration:    sig.ClosedAt - sig.Envelope.Timestamp,
		}
		history = append(history, item)
	}

	// Sort by closed time descending (newest first)
	sort.Slice(history, func(i, j int) bool {
		return history[i].ClosedAt > history[j].ClosedAt
	})

	return history
}

// determineOutcomeType returns the chart color category
// green = full win (hit TP), yellow = partial win (profit but expired), red = loss
func determineOutcomeType(state types.SignalState, pnl float64, payload types.SignalPayload) string {
	switch state {
	case types.StateClosedWin:
		return "green" // Full win - hit take profit
	case types.StateClosedLoss:
		return "red" // Full loss - hit stop loss
	case types.StateExpired:
		if pnl > 0 {
			return "yellow" // Partial win - profit but didn't hit TP
		}
		return "red" // Expired at a loss
	default:
		return "gray" // Unknown/pending
	}
}

func buildTokenPerformance(history []SignalHistoryItem) map[string]TokenPerformance {
	perfMap := make(map[string]*TokenPerformance)

	for _, h := range history {
		if _, exists := perfMap[h.TokenPair]; !exists {
			perfMap[h.TokenPair] = &TokenPerformance{TokenPair: h.TokenPair}
		}
		p := perfMap[h.TokenPair]
		p.TotalSignals++
		p.TotalPnL += h.PnLPercent

		if h.OutcomeType == "green" || (h.OutcomeType == "yellow" && h.PnLPercent > 0) {
			p.Wins++
		} else {
			p.Losses++
		}
	}

	// Calculate averages
	result := make(map[string]TokenPerformance)
	for token, p := range perfMap {
		if p.TotalSignals > 0 {
			p.AvgPnL = p.TotalPnL / float64(p.TotalSignals)
			p.WinRate = float64(p.Wins) / float64(p.TotalSignals)
		}
		result[token] = *p
	}

	return result
}

func buildMonthlyReturns(history []SignalHistoryItem) []MonthlyReturn {
	monthMap := make(map[string]*MonthlyReturn)

	for _, h := range history {
		month := time.Unix(h.ClosedAt, 0).Format("2006-01")
		if _, exists := monthMap[month]; !exists {
			monthMap[month] = &MonthlyReturn{Month: month}
		}
		m := monthMap[month]
		m.Signals++
		m.PnLPercent += h.PnLPercent
		if h.OutcomeType == "green" || h.OutcomeType == "yellow" {
			m.WinRate += 1
		}
	}

	// Calculate win rates and convert to slice
	months := make([]MonthlyReturn, 0, len(monthMap))
	for _, m := range monthMap {
		if m.Signals > 0 {
			m.WinRate = m.WinRate / float64(m.Signals)
		}
		months = append(months, *m)
	}

	// Sort by month
	sort.Slice(months, func(i, j int) bool {
		return months[i].Month < months[j].Month
	})

	return months
}

func calculateStreaks(history []SignalHistoryItem) StreakInfo {
	if len(history) == 0 {
		return StreakInfo{}
	}

	// Sort by closed time ascending for streak calculation
	sorted := make([]SignalHistoryItem, len(history))
	copy(sorted, history)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ClosedAt < sorted[j].ClosedAt
	})

	var currentStreak, longestWin, longestLoss, currentWin, currentLoss int
	var streakType string

	for _, h := range sorted {
		isWin := h.OutcomeType == "green" || h.OutcomeType == "yellow"
		if isWin {
			currentWin++
			currentLoss = 0
			if currentWin > longestWin {
				longestWin = currentWin
			}
		} else {
			currentLoss++
			currentWin = 0
			if currentLoss > longestLoss {
				longestLoss = currentLoss
			}
		}
	}

	// Determine current streak
	lastItem := sorted[len(sorted)-1]
	if lastItem.OutcomeType == "green" || lastItem.OutcomeType == "yellow" {
		currentStreak = currentWin
		streakType = "winning"
	} else {
		currentStreak = -currentLoss
		streakType = "losing"
	}

	return StreakInfo{
		CurrentStreak:     currentStreak,
		CurrentStreakType: streakType,
		LongestWinStreak:  longestWin,
		LongestLossStreak: longestLoss,
	}
}

func runPortfolioSimulation(capital float64, history []SignalHistoryItem, fromTs, toTs int64) PortfolioSimulation {
	if len(history) == 0 {
		return PortfolioSimulation{InitialCapital: capital, FinalValue: capital}
	}

	// Sort by closed time ascending
	sorted := make([]SignalHistoryItem, len(history))
	copy(sorted, history)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ClosedAt < sorted[j].ClosedAt
	})

	equity := capital
	maxEquity := capital
	maxDrawdown := 0.0
	equityCurve := []EquityPoint{{Timestamp: fromTs, Equity: capital, PnL: 0}}
	trades := []TradeResult{}

	for _, h := range sorted {
		// Calculate position size (using confidence as allocation %)
		positionSize := equity * (h.Confidence / 100.0)
		pnlAmount := positionSize * (h.PnLPercent / 100.0)

		equity += pnlAmount

		// Track max drawdown
		if equity > maxEquity {
			maxEquity = equity
		}
		drawdown := ((maxEquity - equity) / maxEquity) * 100
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}

		equityCurve = append(equityCurve, EquityPoint{
			Timestamp: h.ClosedAt,
			Equity:    equity,
			PnL:       ((equity - capital) / capital) * 100,
		})

		trades = append(trades, TradeResult{
			SignalID:    h.ID,
			TokenPair:   h.TokenPair,
			Direction:   h.Direction,
			EntryPrice:  h.EntryPrice,
			ExitPrice:   h.ClosedPrice,
			PnLPercent:  h.PnLPercent,
			PnLAmount:   pnlAmount,
			Outcome:     h.Outcome,
			OutcomeType: h.OutcomeType,
			Timestamp:   h.ClosedAt,
		})
	}

	// Calculate timeframe
	timeframeDays := 0
	if len(sorted) > 0 {
		first := sorted[0].ClosedAt
		last := sorted[len(sorted)-1].ClosedAt
		timeframeDays = int((last - first) / 86400)
		if timeframeDays == 0 {
			timeframeDays = 1
		}
	}

	// Get Sharpe from scorer stats if available
	sharpe := 0.0
	if len(trades) > 0 {
		// Simple Sharpe approximation
		totalReturn := ((equity - capital) / capital) * 100
		avgReturn := totalReturn / float64(len(trades))
		// Simplified - would need actual return variance for proper Sharpe
		sharpe = avgReturn / (maxDrawdown + 0.01) // Avoid div by zero
	}

	return PortfolioSimulation{
		InitialCapital: capital,
		FinalValue:     equity,
		TotalReturn:    ((equity - capital) / capital) * 100,
		MaxDrawdown:    maxDrawdown,
		SharpeRatio:    sharpe,
		Trades:         len(trades),
		TimeframeDays:  timeframeDays,
		EquityCurve:    equityCurve,
		TradeHistory:   trades,
	}
}

func calculateWinRate(wins, losses int) float64 {
	total := wins + losses
	if total == 0 {
		return 0
	}
	return float64(wins) / float64(total)
}
