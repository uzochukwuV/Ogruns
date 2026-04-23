package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
)

// WebhookDispatcher manages AI Agent subscriptions and pushes signals
// to their registered URLs the millisecond a signal is validated.
type WebhookDispatcher struct {
	mu sync.RWMutex
	// Map of NodeID -> Slice of Target URLs subscribed to that Node
	subscribers map[string][]string
	httpClient  *http.Client
}

func NewWebhookDispatcher() *WebhookDispatcher {
	return &WebhookDispatcher{
		subscribers: make(map[string][]string),
		httpClient: &http.Client{
			Timeout: 5 * time.Second, // AI Agents must respond quickly
		},
	}
}

// Register adds a new AI Agent URL to the broadcast list for a specific Node
func (d *WebhookDispatcher) Register(nodeID string, targetURL string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Prevent duplicate URLs
	for _, url := range d.subscribers[nodeID] {
		if url == targetURL {
			return
		}
	}

	d.subscribers[nodeID] = append(d.subscribers[nodeID], targetURL)
	log.Printf("Dispatcher: Registered AI Webhook %s for Node %s", targetURL, nodeID)
}

// Broadcast is called instantly when a valid signal arrives.
// It fires off concurrent HTTP POST requests to every subscribed AI Agent.
func (d *WebhookDispatcher) Broadcast(sig types.SignalEnvelope) {
	d.mu.RLock()
	urls, exists := d.subscribers[sig.NodeID]
	d.mu.RUnlock()

	if !exists || len(urls) == 0 {
		return
	}

	payload := types.WebhookPayload{
		EventID:   fmt.Sprintf("evt_%s_%d", sig.NodeID[:8], sig.Timestamp),
		Timestamp: time.Now().Unix(),
		NodeID:    sig.NodeID,
		Signal:    sig,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Dispatcher: Failed to marshal webhook payload: %v", err)
		return
	}

	// Fire and forget: Do not block the Verification Engine waiting for AI agents to respond
	for _, url := range urls {
		go d.sendWebhook(url, payloadBytes)
	}
}

func (d *WebhookDispatcher) sendWebhook(url string, payload []byte) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "0G-Verification-Layer/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		log.Printf("Dispatcher: ❌ Webhook failed to %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("Dispatcher: ✅ Woke up AI Agent at %s", url)
	} else {
		log.Printf("Dispatcher: ⚠️ AI Agent at %s returned status %d", url, resp.StatusCode)
	}
}
