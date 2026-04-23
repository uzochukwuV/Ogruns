// Package api exposes the Verifier API Gate used by downstream execution bots
// and front-end consumers. It provides:
//
//	GET  /api/v1/nodes            – paginated marketplace listing with live stats
//	GET  /api/v1/nodes/{nodeId}   – single-node detail
//	WS   /api/v1/stream           – authenticated live signal stream per node
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/internal/dispatcher"
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
	hub         *streamHub
	dispatcher  *dispatcher.WebhookDispatcher
	ctx         context.Context
	cancel      context.CancelFunc
	signalQueue chan types.SubmitRawSignalRequest // L2 Ingestion Queue
}

// NewServer creates a Server but does not start listening.
// eventStream is the *types.ActiveSignal channel emitted by the ResolutionEngine.
func NewServer(sc *scorer.Scorer, eventStream <-chan *types.ActiveSignal, dispatch *dispatcher.WebhookDispatcher) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	hub := newStreamHub(eventStream)
	go hub.run(ctx)
	return &Server{
		scorer:      sc,
		hub:         hub,
		dispatcher:  dispatch,
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
	mux.HandleFunc("/api/v1/signals", s.handleSubmitSignal) // New L2 Ingestion endpoint
	mux.HandleFunc("/api/v1/subscribers/webhook", s.handleRegisterWebhook) // AI Agent Webhook Registration
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

// WS /api/v1/stream?node_id=0xABC...
//
// Upgrades to WebSocket and streams every state-change event for the requested
// node as a JSON object. The connection stays open until the client disconnects.
//
// Future: add ?subscriber_address=0x...&token=<sig> and verify on-chain
// subscription via SubscriptionManager.isSubscribed before streaming.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		http.Error(w, "node_id query param required", http.StatusBadRequest)
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

	log.Printf("WS: client %s subscribed to node %s", clientID, nodeID)

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

	// Push to internal queue for the Verification Engine and Batcher to consume
	select {
	case s.signalQueue <- req:
		writeJSON(w, http.StatusAccepted, map[string]string{
			"status":  "queued",
			"message": "Signal accepted for verification and batching",
		})
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "server queue full"})
	}
}

// handleRegisterWebhook allows an AI Agent to register a URL to be awoken when a signal arrives.
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

	// In production: verify `req.Signature` and check the SubscriptionManager
	// smart contract to ensure `req.SubscriberAddress` actually paid for `req.NodeID`.
	// For MVP, we trust the registration.

	s.dispatcher.Register(req.NodeID, req.TargetURL)

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Webhook registered. Your AI Agent will be awoken on new signals.",
	})
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
