// Package api exposes the Verifier API Gate used by downstream execution bots
// and front-end consumers. It provides:
//
//	GET  /api/v1/nodes            – paginated marketplace listing with live stats
//	GET  /api/v1/nodes/{nodeId}   – single-node detail
//	WS   /api/v1/stream           – authenticated live signal stream per node
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/internal/contracts"
	"github.com/0xprotocol/verification-layer/internal/dispatcher"
	"github.com/0xprotocol/verification-layer/internal/ratelimit"
	"github.com/0xprotocol/verification-layer/internal/scorer"
	"github.com/0xprotocol/verification-layer/pkg/types"
	"github.com/gorilla/websocket"
)

// ─── Hub ─────────────────────────────────────────────────────────────────────

// streamHub fans out engine events to every connected WebSocket subscriber.
// A subscriber registers interest in one specific node_id; only events for
// that node are forwarded to its channel.
type streamHub struct {
	// nodeID → set of per-client channels
	subs sync.Map

	eventSource <-chan *types.ActiveSignal
}

func newStreamHub(src <-chan *types.ActiveSignal) *streamHub {
	return &streamHub{eventSource: src}
}

func (h *streamHub) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-h.eventSource:
			if !ok {
				return
			}
			nodeID := ev.Envelope.NodeID
			if raw, loaded := h.subs.Load(nodeID); loaded {
				clients := raw.(*sync.Map)
				clients.Range(func(_, chRaw any) bool {
					ch := chRaw.(chan *types.ActiveSignal)
					select {
					case ch <- ev:
					default: // drop if client is slow
					}
					return true
				})
			}
		}
	}
}

// subscribe registers a client channel for events from nodeID.
// Returns an unsubscribe function the caller must invoke on disconnect.
func (h *streamHub) subscribe(nodeID, clientID string) (chan *types.ActiveSignal, func()) {
	ch := make(chan *types.ActiveSignal, 64)
	actual, _ := h.subs.LoadOrStore(nodeID, &sync.Map{})
	clients := actual.(*sync.Map)
	clients.Store(clientID, ch)

	unsub := func() {
		clients.Delete(clientID)
		close(ch)
	}
	return ch, unsub
}

// ─── Server ───────────────────────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	CheckOrigin:      func(r *http.Request) bool { return true }, // tighten in prod
}

// Server wires scorer and the engine event stream into HTTP handlers.
type Server struct {
	scorer      *scorer.Scorer
	aiScorer    *scorer.AIScorer // 0G AI-powered signal evaluator
	hub         *streamHub
	dispatcher  *dispatcher.WebhookDispatcher
	rateLimiter *ratelimit.RateLimiter
	subGate     *SubscriptionGate // Subscription verification for premium endpoints
	ctx         context.Context
	cancel      context.CancelFunc
	signalQueue chan types.SubmitRawSignalRequest // L2 Ingestion Queue
}

// NewServer creates a Server but does not start listening.
// eventStream is the *types.ActiveSignal channel emitted by the ResolutionEngine.
// cm is optional - if nil, subscription gating is disabled (dev mode).
func NewServer(sc *scorer.Scorer, eventStream <-chan *types.ActiveSignal, dispatch *dispatcher.WebhookDispatcher, cm *contracts.ContractManager) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	hub := newStreamHub(eventStream)
	go hub.run(ctx)

	// Initialize rate limiter with default config
	rl := ratelimit.NewRateLimiter(ratelimit.DefaultConfig())
	log.Printf("Rate limiter initialized: %d signals/min per node, %d signals/min global",
		ratelimit.DefaultConfig().NodeMaxSignals,
		ratelimit.DefaultConfig().GlobalMaxSignals)

	// Initialize subscription gate
	subGate := NewSubscriptionGate(cm)
	if cm != nil && cm.HasSubscriptionContract() {
		log.Println("✅ Subscription gating ENABLED - premium endpoints require on-chain subscription")
	} else {
		log.Println("⚠️  Subscription gating DISABLED - all endpoints are publicly accessible")
	}

	// Start subscription cache cleanup
	subGate.StartCacheCleanup(ctx)

	// Initialize AI scorer for 0G AI evaluation
	aiScorer := scorer.NewAIScorer()

	return &Server{
		scorer:      sc,
		aiScorer:    aiScorer,
		hub:         hub,
		dispatcher:  dispatch,
		rateLimiter: rl,
		subGate:     subGate,
		ctx:         ctx,
		cancel:      cancel,
		signalQueue: make(chan types.SubmitRawSignalRequest, 10000), // High capacity for MVP
	}
}

// Start binds to addr and serves until the context is cancelled.
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/nodes", s.handleNodes)
	mux.HandleFunc("/api/v1/nodes/", s.handleNode) // trailing slash catches /{nodeId}
	mux.HandleFunc("/api/v1/stream", s.handleStream)
	mux.HandleFunc("/api/v1/signals", s.handleSubmitSignal)                // New L2 Ingestion endpoint
	mux.HandleFunc("/api/v1/subscribers/webhook", s.handleRegisterWebhook) // AI Agent Webhook Registration
	mux.HandleFunc("/api/v1/stats/ratelimit", s.handleRateLimitStats)      // Rate limit stats

	// Analytics endpoints for frontend dashboard
	mux.HandleFunc("/api/v1/analytics/dashboard", s.handleDashboardSummary)
	mux.HandleFunc("/api/v1/analytics/nodes/", s.handleAnalyticsRouter)

	// 0G AI evaluation endpoint
	mux.HandleFunc("/api/v1/ai/evaluate", s.handleAIEvaluate)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // WebSocket connections are long-lived
	}

	log.Printf("API server listening on %s", addr)
	return srv.ListenAndServe()
}

func (s *Server) Stop() { s.cancel() }

// GetSignalQueue exposes the channel for the Ingester/Batcher to consume
func (s *Server) GetSignalQueue() <-chan types.SubmitRawSignalRequest {
	return s.signalQueue
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// GET /api/v1/nodes
// Returns the marketplace listing: every tracked node with its live stats.
func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	all := s.scorer.AllStats()
	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": all,
		"count": len(all),
	})
}

// GET /api/v1/nodes/{nodeId}
// Returns a single node's stats.
func (s *Server) handleNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Extract nodeId from path: /api/v1/nodes/{nodeId}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/nodes/"), "/")
	nodeID := parts[0]
	if nodeID == "" {
		http.Error(w, "missing node_id", http.StatusBadRequest)
		return
	}

	stats, ok := s.scorer.GetStats(nodeID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// WS /api/v1/stream?node_id=0xABC...&subscriber_address=0x...&signature=0x...
//
// Upgrades to WebSocket and streams every state-change event for the requested
// node as a JSON object. The connection stays open until the client disconnects.
//
// PREMIUM ENDPOINT: Requires on-chain subscription verification via SubscriptionManager.
// Pass subscriber_address and signature to prove you're a paying subscriber.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		http.Error(w, "node_id query param required", http.StatusBadRequest)
		return
	}

	// 🔒 SUBSCRIPTION GATE: Verify on-chain subscription before streaming
	if subErr := s.subGate.VerifySubscription(r, nodeID); subErr != nil {
		WriteSubscriptionError(w, subErr)
		log.Printf("WS: Subscription denied for node %s - %s", nodeID[:16], subErr.Code)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade error: %v", err)
		return
	}
	defer conn.Close()

	clientID := r.RemoteAddr
	ch, unsub := s.hub.subscribe(nodeID, clientID)
	defer unsub()

	subscriberAddr := r.URL.Query().Get("subscriber_address")
	log.Printf("WS: subscriber %s connected to node %s", subscriberAddr[:16]+"...", nodeID[:16]+"...")

	// Ping loop keeps the connection alive and detects dead clients.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-s.ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				log.Printf("WS write error for %s: %v", clientID, err)
				return
			}
		}
	}
}

// handleSubmitSignal is the L2 ingestion endpoint for Node Creators.
// It accepts a raw, encrypted signal payload and injects it into the matching engine queue.
func (s *Server) handleSubmitSignal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.SubmitRawSignalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}
	defer r.Body.Close()

	if req.NodeID == "" || req.Envelope.Signature == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing node_id or signature"})
		return
	}

	// Rate limit check
	signalHash := hashSignal(req.Envelope)
	result := s.rateLimiter.Check(req.NodeID, signalHash)
	if !result.Allowed {
		w.Header().Set("Retry-After", string(rune(result.RetryAfterSec)))
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"error":            "rate_limited",
			"reason":           result.Reason,
			"retry_after_sec":  result.RetryAfterSec,
			"message":          getRateLimitMessage(result.Reason),
		})
		log.Printf("Rate limit: Rejected signal from %s (%s)", req.NodeID[:16]+"...", result.Reason)
		return
	}

	// Generate signal ID upfront (same as scheduler: node_id + timestamp)
	signalID := fmt.Sprintf("%s_%d", req.Envelope.NodeID, req.Envelope.Timestamp)

	// Push to internal queue for the Verification Engine and Batcher to consume
	select {
	case s.signalQueue <- req:
		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"status":    "queued",
			"signal_id": signalID,
			"message":   "Signal accepted for verification and batching",
		})
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "server queue full"})
	}
}

// hashSignal creates a unique hash for duplicate detection
func hashSignal(env types.SignalEnvelope) string {
	data := env.NodeID + env.Signature + env.Payload.TokenPair +
		env.Payload.Direction + string(rune(env.Payload.ExpiryTime))
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// getRateLimitMessage returns a human-readable message for rate limit reasons
func getRateLimitMessage(reason string) string {
	switch reason {
	case "duplicate_signal":
		return "This exact signal was already submitted recently"
	case "node_rate_limit":
		return "Too many signals from this node. Max 10 signals per minute."
	case "global_rate_limit":
		return "Server is at capacity. Please try again shortly."
	default:
		return "Rate limit exceeded"
	}
}

// handleRateLimitStats returns the current rate limit statistics
func (s *Server) handleRateLimitStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.rateLimiter.GetStats())
}

// handleAnalyticsRouter routes analytics requests to appropriate handlers
func (s *Server) handleAnalyticsRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/analytics/nodes/")
	parts := strings.Split(path, "/")

	if len(parts) >= 2 && parts[1] == "simulate" {
		s.handlePortfolioSimulation(w, r)
		return
	}

	// Default to node analytics
	s.handleNodeAnalytics(w, r)
}

// handleRegisterWebhook allows an AI Agent to register a URL to be awoken when a signal arrives.
// PREMIUM ENDPOINT: Requires on-chain subscription to the target node.
func (s *Server) handleRegisterWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.RegisterWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}
	defer r.Body.Close()

	if req.TargetURL == "" || req.NodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing target_url or node_id"})
		return
	}

	// 🔒 SUBSCRIPTION GATE: Verify on-chain subscription
	// For webhook registration, we use subscriber_address, signature, and timestamp from the JSON body
	if req.SubscriberAddress != "" && req.Signature != "" && req.Timestamp != "" {
		// Create a mock request with the credentials for verification
		mockReq := r.Clone(r.Context())
		q := mockReq.URL.Query()
		q.Set("subscriber_address", req.SubscriberAddress)
		q.Set("signature", req.Signature)
		q.Set("timestamp", req.Timestamp)
		mockReq.URL.RawQuery = q.Encode()

		if subErr := s.subGate.VerifySubscription(mockReq, req.NodeID); subErr != nil {
			WriteSubscriptionError(w, subErr)
			return
		}
	} else if s.subGate.IsEnabled() {
		// Subscription contract is configured but no credentials provided
		writeJSON(w, http.StatusPaymentRequired, map[string]string{
			"error":   "subscription_required",
			"message": "Webhook registration requires subscriber_address, signature, and timestamp fields",
		})
		return
	}

	s.dispatcher.Register(req.NodeID, req.TargetURL)

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Webhook registered. Your AI Agent will be awoken on new signals.",
	})
}

// ─── 0G AI Evaluation ─────────────────────────────────────────────────────────

// AIEvaluateRequest is the request body for AI signal evaluation
type AIEvaluateRequest struct {
	TokenPair    string  `json:"token_pair"`
	Direction    string  `json:"direction"`
	EntryPrice   float64 `json:"entry_price"`
	TakeProfit   float64 `json:"take_profit"`
	StopLoss     float64 `json:"stop_loss"`
	ExpiryTime   int64   `json:"expiry_time"`
	WeightPct    float64 `json:"weight_pct"`
	TradeType    string  `json:"trade_type,omitempty"`
	Leverage     float64 `json:"leverage,omitempty"`
	CurrentPrice float64 `json:"current_price,omitempty"` // Optional: current market price
}

// handleAIEvaluate provides on-demand AI evaluation of trading signals
// POST /api/v1/ai/evaluate
func (s *Server) handleAIEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	// Check if AI scoring is enabled
	if s.aiScorer == nil || !s.aiScorer.IsEnabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "ai_unavailable",
			"message": "AI scoring is not configured. Set ZG_AI_API_KEY environment variable.",
		})
		return
	}

	var req AIEvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	// Validate required fields
	if req.TokenPair == "" || req.Direction == "" || req.EntryPrice <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "missing required fields: token_pair, direction, entry_price",
		})
		return
	}

	// Build signal envelope for evaluation
	payload := types.SignalPayload{
		TokenPair:  req.TokenPair,
		Direction:  req.Direction,
		EntryPrice: req.EntryPrice,
		TakeProfit: req.TakeProfit,
		StopLoss:   req.StopLoss,
		ExpiryTime: req.ExpiryTime,
		WeightPct:  req.WeightPct,
		TradeType:  req.TradeType,
		Leverage:   req.Leverage,
	}

	envelope := types.SignalEnvelope{
		NodeID:    "evaluation",
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}

	// Use current price if provided, otherwise use entry price
	currentPrice := req.CurrentPrice
	if currentPrice <= 0 {
		currentPrice = req.EntryPrice
	}

	// Evaluate with AI
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	result, err := s.aiScorer.EvaluateSignal(ctx, envelope, currentPrice)
	if err != nil {
		// Fallback to quick heuristic score
		quickScore := s.aiScorer.QuickScore(payload)
		writeJSON(w, http.StatusOK, map[string]any{
			"score":        quickScore,
			"confidence":   60,
			"risk_rating":  getRiskRating(quickScore),
			"reasoning":    "Heuristic evaluation (AI unavailable)",
			"suggestions":  []string{},
			"tee_verified": false,
			"fallback":     true,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"score":        result.Score,
		"confidence":   result.Confidence,
		"risk_rating":  result.RiskRating,
		"reasoning":    result.Reasoning,
		"suggestions":  result.Suggestions,
		"tee_verified": result.Verified,
		"fallback":     false,
	})
}

// getRiskRating returns risk rating based on score
func getRiskRating(score float64) string {
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

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
