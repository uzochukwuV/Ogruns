package scorer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
)

// AIScorer uses 0G AI Router to evaluate signal quality
type AIScorer struct {
	apiKey     string
	apiURL     string
	model      string
	httpClient *http.Client
	enabled    bool
}

// AIScoreResult contains the AI evaluation of a signal
type AIScoreResult struct {
	Score            float64  `json:"score"`              // 0-100 final adjusted score
	OriginalScore    float64  `json:"original_score"`     // Original algorithmic score
	Adjustment       float64  `json:"adjustment"`         // Score adjustment applied
	Confidence       float64  `json:"confidence"`         // AI's confidence in the evaluation
	RiskRating       string   `json:"risk_rating"`        // LOW, MEDIUM, HIGH, EXTREME
	Reasoning        string   `json:"reasoning"`          // Human-readable explanation
	Suggestions      []string `json:"suggestions"`        // Improvement suggestions
	ManipulationFlag bool     `json:"manipulation_flag"`  // True if manipulation detected
	ManipulationType string   `json:"manipulation_type"`  // Type of manipulation detected
	Verified         bool     `json:"tee_verified"`       // TEE verification status
}

// SignalHistory tracks recent signals for manipulation detection
type SignalHistory struct {
	TokenPair   string  `json:"token_pair"`
	Direction   string  `json:"direction"`
	EntryPrice  float64 `json:"entry_price"`
	Timestamp   int64   `json:"timestamp"`
	Outcome     string  `json:"outcome"`
	PnLPercent  float64 `json:"pnl_percent"`
}

// ChatMessage represents a message in the chat format
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request format for 0G AI Router
type ChatRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	Stream    bool          `json:"stream"`
	VerifyTEE bool          `json:"verify_tee,omitempty"`
}

// ChatResponse is the response format from 0G AI Router
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	TEEVerified bool `json:"tee_verified,omitempty"`
}

// NewAIScorer creates a new AI scorer instance
func NewAIScorer() *AIScorer {
	apiKey := os.Getenv("ZG_AI_API_KEY")
	if apiKey == "" {
		log.Println("AIScorer: ZG_AI_API_KEY not set, AI scoring disabled")
		return &AIScorer{enabled: false}
	}

	apiURL := os.Getenv("ZG_AI_API_URL")
	if apiURL == "" {
		apiURL = "https://router-api-testnet.integratenetwork.work/v1/chat/completions"
	}

	model := os.Getenv("ZG_AI_MODEL")
	if model == "" {
		model = "qwen/qwen-2.5-7b-instruct"
	}

	log.Printf("AIScorer: Initialized with model %s (TEE verified)", model)

	return &AIScorer{
		apiKey:     apiKey,
		apiURL:     apiURL,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		enabled:    true,
	}
}

// IsEnabled returns whether AI scoring is enabled
func (s *AIScorer) IsEnabled() bool {
	return s.enabled
}

// EvaluateAndAdjustScore is the PRIMARY scoring method - AI evaluates the algorithmic score
// and adjusts it based on signal quality, manipulation detection, and historical patterns.
// This is the score that should be posted on-chain.
func (s *AIScorer) EvaluateAndAdjustScore(ctx context.Context, env types.SignalEnvelope, algorithmicScore float64, history []SignalHistory, outcome string, pnlPercent float64) (*AIScoreResult, error) {
	if !s.enabled {
		// Fallback: return algorithmic score with basic manipulation checks
		return s.heuristicAdjustment(env.Payload, algorithmicScore, history, outcome, pnlPercent), nil
	}

	payload := env.Payload

	// Build history summary for AI
	var historyStr string
	duplicateCount := 0
	recentWins := 0
	recentLosses := 0
	lastHourSignals := 0
	now := time.Now().Unix()

	for _, h := range history {
		if now-h.Timestamp < 3600 { // Last hour
			lastHourSignals++
		}
		if h.TokenPair == payload.TokenPair && h.Direction == payload.Direction {
			if now-h.Timestamp < 7200 { // Same signal within 2 hours
				duplicateCount++
			}
		}
		if h.Outcome == "WIN" {
			recentWins++
		} else if h.Outcome == "LOSS" {
			recentLosses++
		}
	}

	if len(history) > 0 {
		historyStr = fmt.Sprintf(`
RECENT SIGNAL HISTORY (last %d signals):
- Wins: %d, Losses: %d
- Signals in last hour: %d
- Duplicate signals (same token+direction in 2h): %d
- Overall pattern: %s`, len(history), recentWins, recentLosses, lastHourSignals, duplicateCount, describePattern(history))
	} else {
		historyStr = "No prior signal history (new agent)"
	}

	// Calculate metrics
	var tpDistance, slDistance, riskReward float64
	if payload.Direction == "long" {
		tpDistance = ((payload.TakeProfit - payload.EntryPrice) / payload.EntryPrice) * 100
		slDistance = ((payload.EntryPrice - payload.StopLoss) / payload.EntryPrice) * 100
	} else {
		tpDistance = ((payload.EntryPrice - payload.TakeProfit) / payload.EntryPrice) * 100
		slDistance = ((payload.StopLoss - payload.EntryPrice) / payload.EntryPrice) * 100
	}
	if slDistance > 0 {
		riskReward = tpDistance / slDistance
	}

	prompt := fmt.Sprintf(`You are a FRAUD DETECTION AI for a trading signal platform. Your job is to evaluate signal quality AND detect manipulation.

CURRENT SIGNAL:
- Token: %s | Direction: %s
- Entry: $%.8f | TP: $%.8f (%.2f%%) | SL: $%.8f (%.2f%%)
- Risk/Reward: %.2f:1 | Weight: %.0f%%
- Outcome: %s | PnL: %.2f%%

ALGORITHMIC SCORE: %.1f/100
%s

MANIPULATION PATTERNS TO DETECT:
1. SIGNAL_SPAM: >5 signals in 1 hour, or >3 duplicate signals
2. DUPLICATE_ABUSE: Same token+direction within 2 hours repeatedly
3. SAFE_BET_FARMING: Very tight TP/SL (<1%%) on stable assets for easy wins
4. WASH_TRADING: Pattern of wins that seem too consistent/artificial
5. SCORE_PADDING: Low-quality signals designed to boost win count

YOUR TASK:
1. Evaluate if this signal's outcome is legitimate
2. Check for manipulation patterns in the history
3. Adjust the algorithmic score if needed (can increase for quality, decrease for manipulation)
4. If manipulation detected, apply penalty (up to -50 points)

RESPOND IN THIS EXACT JSON FORMAT:
{
  "final_score": <0-100>,
  "adjustment": <-50 to +20>,
  "manipulation_detected": <true|false>,
  "manipulation_type": "<NONE|SIGNAL_SPAM|DUPLICATE_ABUSE|SAFE_BET_FARMING|WASH_TRADING|SCORE_PADDING>",
  "reasoning": "<2-3 sentences explaining your decision>",
  "confidence": <0-100>
}`,
		payload.TokenPair, payload.Direction,
		payload.EntryPrice, payload.TakeProfit, tpDistance, payload.StopLoss, slDistance,
		riskReward, payload.WeightPct,
		outcome, pnlPercent,
		algorithmicScore, historyStr)

	// Make AI request
	reqBody := ChatRequest{
		Model: s.model,
		Messages: []ChatMessage{
			{Role: "system", Content: "You are a trading signal fraud detection AI. Be strict about manipulation. Legitimate traders don't spam signals."},
			{Role: "user", Content: prompt},
		},
		Stream:    false,
		VerifyTEE: true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return s.heuristicAdjustment(payload, algorithmicScore, history, outcome, pnlPercent), nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return s.heuristicAdjustment(payload, algorithmicScore, history, outcome, pnlPercent), nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return s.heuristicAdjustment(payload, algorithmicScore, history, outcome, pnlPercent), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.heuristicAdjustment(payload, algorithmicScore, history, outcome, pnlPercent), nil
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return s.heuristicAdjustment(payload, algorithmicScore, history, outcome, pnlPercent), nil
	}

	if len(chatResp.Choices) == 0 {
		return s.heuristicAdjustment(payload, algorithmicScore, history, outcome, pnlPercent), nil
	}

	// Parse AI response
	content := chatResp.Choices[0].Message.Content
	result, err := parseAdjustmentResponse(content, algorithmicScore)
	if err != nil {
		log.Printf("AIScorer: Failed to parse adjustment response: %v", err)
		return s.heuristicAdjustment(payload, algorithmicScore, history, outcome, pnlPercent), nil
	}

	result.Verified = chatResp.TEEVerified
	result.OriginalScore = algorithmicScore

	if result.ManipulationFlag {
		log.Printf("AIScorer: ⚠️ MANIPULATION DETECTED for %s - Type: %s, Penalty: %.0f",
			env.NodeID[:16]+"...", result.ManipulationType, result.Adjustment)
	}

	log.Printf("AIScorer: %s scored %.0f -> %.0f (adj: %+.0f) | %s | TEE: %v",
		payload.TokenPair, algorithmicScore, result.Score, result.Adjustment,
		result.ManipulationType, result.Verified)

	return result, nil
}

// heuristicAdjustment provides fallback manipulation detection without AI
func (s *AIScorer) heuristicAdjustment(payload types.SignalPayload, algorithmicScore float64, history []SignalHistory, outcome string, pnlPercent float64) *AIScoreResult {
	adjustment := 0.0
	manipulationFlag := false
	manipulationType := "NONE"
	var reasons []string

	now := time.Now().Unix()
	lastHourSignals := 0
	duplicateCount := 0

	for _, h := range history {
		if now-h.Timestamp < 3600 {
			lastHourSignals++
		}
		if h.TokenPair == payload.TokenPair && h.Direction == payload.Direction {
			if now-h.Timestamp < 7200 {
				duplicateCount++
			}
		}
	}

	// Check for signal spam (>5 signals/hour)
	if lastHourSignals > 5 {
		adjustment -= 20
		manipulationFlag = true
		manipulationType = "SIGNAL_SPAM"
		reasons = append(reasons, fmt.Sprintf("%d signals in last hour (spam detected)", lastHourSignals))
	}

	// Check for duplicate abuse
	if duplicateCount > 2 {
		adjustment -= 30
		manipulationFlag = true
		manipulationType = "DUPLICATE_ABUSE"
		reasons = append(reasons, fmt.Sprintf("%d duplicate signals on same token+direction", duplicateCount))
	}

	// Check for safe bet farming (tiny TP/SL)
	var tpDist, slDist float64
	if payload.Direction == "long" {
		tpDist = ((payload.TakeProfit - payload.EntryPrice) / payload.EntryPrice) * 100
		slDist = ((payload.EntryPrice - payload.StopLoss) / payload.EntryPrice) * 100
	} else {
		tpDist = ((payload.EntryPrice - payload.TakeProfit) / payload.EntryPrice) * 100
		slDist = ((payload.StopLoss - payload.EntryPrice) / payload.EntryPrice) * 100
	}

	if tpDist < 1.0 && slDist < 1.0 {
		adjustment -= 15
		manipulationFlag = true
		manipulationType = "SAFE_BET_FARMING"
		reasons = append(reasons, "Very tight TP/SL suggests safe bet farming")
	}

	// Quality bonus for good R:R and meaningful move
	if tpDist > 2.0 && slDist > 1.0 {
		rr := tpDist / slDist
		if rr >= 2.0 && outcome == "WIN" {
			adjustment += 10
			reasons = append(reasons, "Quality signal with good R:R")
		}
	}

	finalScore := algorithmicScore + adjustment
	if finalScore < 0 {
		finalScore = 0
	}
	if finalScore > 100 {
		finalScore = 100
	}

	reasoning := "Heuristic evaluation"
	if len(reasons) > 0 {
		reasoning = strings.Join(reasons, "; ")
	}

	return &AIScoreResult{
		Score:            finalScore,
		OriginalScore:    algorithmicScore,
		Adjustment:       adjustment,
		Confidence:       70,
		RiskRating:       getRiskRatingFromScore(finalScore),
		Reasoning:        reasoning,
		ManipulationFlag: manipulationFlag,
		ManipulationType: manipulationType,
		Verified:         false,
	}
}

// parseAdjustmentResponse parses the AI's adjustment response
func parseAdjustmentResponse(content string, algorithmicScore float64) (*AIScoreResult, error) {
	content = strings.TrimSpace(content)

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON found")
	}

	jsonStr := content[start : end+1]

	var parsed struct {
		FinalScore           float64 `json:"final_score"`
		Adjustment           float64 `json:"adjustment"`
		ManipulationDetected bool    `json:"manipulation_detected"`
		ManipulationType     string  `json:"manipulation_type"`
		Reasoning            string  `json:"reasoning"`
		Confidence           float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, err
	}

	// Validate and clamp
	if parsed.FinalScore < 0 {
		parsed.FinalScore = 0
	}
	if parsed.FinalScore > 100 {
		parsed.FinalScore = 100
	}
	if parsed.Adjustment < -50 {
		parsed.Adjustment = -50
	}
	if parsed.Adjustment > 20 {
		parsed.Adjustment = 20
	}

	return &AIScoreResult{
		Score:            parsed.FinalScore,
		OriginalScore:    algorithmicScore,
		Adjustment:       parsed.Adjustment,
		Confidence:       parsed.Confidence,
		RiskRating:       getRiskRatingFromScore(parsed.FinalScore),
		Reasoning:        parsed.Reasoning,
		ManipulationFlag: parsed.ManipulationDetected,
		ManipulationType: parsed.ManipulationType,
	}, nil
}

// describePattern analyzes history for pattern description
func describePattern(history []SignalHistory) string {
	if len(history) < 3 {
		return "Insufficient data"
	}

	wins := 0
	losses := 0
	for _, h := range history {
		if h.Outcome == "WIN" {
			wins++
		} else {
			losses++
		}
	}

	winRate := float64(wins) / float64(len(history)) * 100
	if winRate > 90 {
		return fmt.Sprintf("Suspiciously high win rate (%.0f%%)", winRate)
	} else if winRate > 70 {
		return fmt.Sprintf("Strong performance (%.0f%% wins)", winRate)
	} else if winRate > 50 {
		return fmt.Sprintf("Moderate performance (%.0f%% wins)", winRate)
	}
	return fmt.Sprintf("Weak performance (%.0f%% wins)", winRate)
}

// getRiskRatingFromScore returns risk rating based on score
func getRiskRatingFromScore(score float64) string {
	switch {
	case score >= 70:
		return "LOW"
	case score >= 50:
		return "MEDIUM"
	case score >= 30:
		return "HIGH"
	default:
		return "EXTREME"
	}
}

// EvaluateSignal uses AI to evaluate the quality of a trading signal (for pre-submission checks)
func (s *AIScorer) EvaluateSignal(ctx context.Context, env types.SignalEnvelope, currentPrice float64) (*AIScoreResult, error) {
	if !s.enabled {
		return nil, fmt.Errorf("AI scoring is disabled")
	}

	payload := env.Payload

	// Calculate key metrics for the AI to evaluate
	var tpDistance, slDistance, riskReward float64
	if payload.Direction == "long" {
		tpDistance = ((payload.TakeProfit - payload.EntryPrice) / payload.EntryPrice) * 100
		slDistance = ((payload.EntryPrice - payload.StopLoss) / payload.EntryPrice) * 100
		if slDistance > 0 {
			riskReward = tpDistance / slDistance
		}
	} else {
		tpDistance = ((payload.EntryPrice - payload.TakeProfit) / payload.EntryPrice) * 100
		slDistance = ((payload.StopLoss - payload.EntryPrice) / payload.EntryPrice) * 100
		if slDistance > 0 {
			riskReward = tpDistance / slDistance
		}
	}

	// Calculate time until expiry
	expiryMinutes := (payload.ExpiryTime - time.Now().Unix()) / 60

	// Current price deviation from entry
	priceDeviation := ((currentPrice - payload.EntryPrice) / payload.EntryPrice) * 100

	// Build the evaluation prompt
	prompt := fmt.Sprintf(`You are a professional trading signal analyst. Evaluate this trading signal and provide a quality score.

SIGNAL DETAILS:
- Token Pair: %s
- Direction: %s
- Entry Price: $%.8f
- Take Profit: $%.8f (%.2f%% from entry)
- Stop Loss: $%.8f (%.2f%% from entry)
- Risk/Reward Ratio: %.2f:1
- Weight/Confidence: %.0f%%
- Time to Expiry: %d minutes
- Trade Type: %s
- Leverage: %.1fx
- Current Market Price: $%.8f (%.2f%% from entry)

EVALUATION CRITERIA:
1. Risk/Reward Analysis: Is the R:R ratio reasonable? (Ideal: 2:1 or better)
2. Stop Loss Positioning: Is the SL too tight (likely to get stopped out) or too wide (excessive risk)?
3. Take Profit Realism: Is the TP achievable within the expiry timeframe?
4. Entry Timing: Based on current price vs entry, is the entry still valid?
5. Overall Signal Quality: Considering all factors, how beneficial is this signal?

RESPOND IN THIS EXACT JSON FORMAT ONLY (no other text):
{
  "score": <0-100 integer>,
  "confidence": <0-100 integer>,
  "risk_rating": "<LOW|MEDIUM|HIGH|EXTREME>",
  "reasoning": "<2-3 sentence explanation>",
  "suggestions": ["<suggestion 1>", "<suggestion 2>"]
}`,
		payload.TokenPair,
		strings.ToUpper(payload.Direction),
		payload.EntryPrice,
		payload.TakeProfit, tpDistance,
		payload.StopLoss, slDistance,
		riskReward,
		payload.WeightPct,
		expiryMinutes,
		payload.GetTradeType(),
		payload.GetLeverage(),
		currentPrice, priceDeviation,
	)

	// Make the API request
	reqBody := ChatRequest{
		Model: s.model,
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
		Stream:    false,
		VerifyTEE: true, // Request TEE verification
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI model")
	}

	// Parse the AI's response
	content := chatResp.Choices[0].Message.Content
	result, err := parseAIResponse(content)
	if err != nil {
		log.Printf("AIScorer: Failed to parse response, using fallback: %v", err)
		// Fallback to heuristic scoring
		result = s.heuristicScore(payload, currentPrice, riskReward, expiryMinutes)
	}

	result.Verified = chatResp.TEEVerified

	log.Printf("AIScorer: %s %s scored %.0f/100 (%s risk) - TEE: %v",
		payload.Direction, payload.TokenPair, result.Score, result.RiskRating, result.Verified)

	return result, nil
}

// parseAIResponse extracts the JSON from the AI's response
func parseAIResponse(content string) (*AIScoreResult, error) {
	// Try to find JSON in the response
	content = strings.TrimSpace(content)

	// Find JSON object in the response
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonStr := content[start : end+1]

	var result AIScoreResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate the result
	if result.Score < 0 || result.Score > 100 {
		result.Score = 50 // Default to neutral
	}
	if result.Confidence < 0 || result.Confidence > 100 {
		result.Confidence = 50
	}
	if result.RiskRating == "" {
		result.RiskRating = "MEDIUM"
	}

	return &result, nil
}

// heuristicScore provides a fallback scoring when AI is unavailable
func (s *AIScorer) heuristicScore(payload types.SignalPayload, currentPrice, riskReward float64, expiryMinutes int64) *AIScoreResult {
	score := 50.0
	var suggestions []string

	// Risk/Reward evaluation
	if riskReward >= 3.0 {
		score += 20
	} else if riskReward >= 2.0 {
		score += 15
	} else if riskReward >= 1.5 {
		score += 10
	} else if riskReward < 1.0 {
		score -= 20
		suggestions = append(suggestions, "Consider improving risk/reward ratio to at least 1.5:1")
	}

	// Expiry evaluation
	if expiryMinutes >= 60 && expiryMinutes <= 240 {
		score += 10 // Good timeframe
	} else if expiryMinutes < 30 {
		score -= 10
		suggestions = append(suggestions, "Expiry time is very short, consider extending")
	}

	// Weight/confidence evaluation
	if payload.WeightPct >= 70 {
		score += 10
	} else if payload.WeightPct < 50 {
		score -= 5
	}

	// Current price vs entry evaluation
	priceDeviation := ((currentPrice - payload.EntryPrice) / payload.EntryPrice) * 100
	if payload.Direction == "long" {
		if priceDeviation > 2 {
			score -= 10 // Already moved up, entry less favorable
			suggestions = append(suggestions, "Price has moved above entry, consider adjusting entry")
		} else if priceDeviation < -1 {
			score += 5 // Good entry opportunity
		}
	} else {
		if priceDeviation < -2 {
			score -= 10
			suggestions = append(suggestions, "Price has moved below entry, consider adjusting entry")
		} else if priceDeviation > 1 {
			score += 5
		}
	}

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	// Determine risk rating
	var riskRating string
	switch {
	case score >= 70:
		riskRating = "LOW"
	case score >= 50:
		riskRating = "MEDIUM"
	case score >= 30:
		riskRating = "HIGH"
	default:
		riskRating = "EXTREME"
	}

	return &AIScoreResult{
		Score:       score,
		Confidence:  60, // Heuristic confidence
		RiskRating:  riskRating,
		Reasoning:   fmt.Sprintf("Heuristic evaluation: R:R %.2f:1, %d min expiry, %.0f%% weight", riskReward, expiryMinutes, payload.WeightPct),
		Suggestions: suggestions,
		Verified:    false,
	}
}

// BatchEvaluate evaluates multiple signals efficiently
func (s *AIScorer) BatchEvaluate(ctx context.Context, signals []types.SignalEnvelope, prices map[string]float64) map[string]*AIScoreResult {
	results := make(map[string]*AIScoreResult)

	for _, env := range signals {
		signalID := fmt.Sprintf("%s_%d", env.NodeID, env.Timestamp)
		price, ok := prices[env.Payload.TokenPair]
		if !ok {
			price = env.Payload.EntryPrice // Fallback to entry price
		}

		result, err := s.EvaluateSignal(ctx, env, price)
		if err != nil {
			log.Printf("AIScorer: Failed to evaluate %s: %v", signalID, err)
			continue
		}

		results[signalID] = result
	}

	return results
}

// QuickScore provides a fast heuristic score without AI (for high-volume scenarios)
func (s *AIScorer) QuickScore(payload types.SignalPayload) float64 {
	var riskReward float64
	if payload.Direction == "long" {
		tpDist := payload.TakeProfit - payload.EntryPrice
		slDist := payload.EntryPrice - payload.StopLoss
		if slDist > 0 {
			riskReward = tpDist / slDist
		}
	} else {
		tpDist := payload.EntryPrice - payload.TakeProfit
		slDist := payload.StopLoss - payload.EntryPrice
		if slDist > 0 {
			riskReward = tpDist / slDist
		}
	}

	score := 50.0

	// R:R scoring
	if riskReward >= 2.0 {
		score += 25
	} else if riskReward >= 1.5 {
		score += 15
	} else if riskReward >= 1.0 {
		score += 5
	} else {
		score -= 15
	}

	// Weight scoring
	score += (payload.WeightPct - 50) / 5

	// Clamp
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// extractNumber extracts a number from a string using regex
func extractNumber(s string, pattern string) float64 {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(s)
	if len(match) > 1 {
		val, _ := strconv.ParseFloat(match[1], 64)
		return val
	}
	return 0
}
