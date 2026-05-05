package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/internal/contracts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// SubscriptionGate handles on-chain subscription verification for premium endpoints.
// It caches subscription status to avoid hitting the blockchain on every request.
type SubscriptionGate struct {
	cm    *contracts.ContractManager
	cache sync.Map // "user:node" -> cachedSub
}

type cachedSub struct {
	subscribed bool
	checkedAt  time.Time
}

const (
	subCacheTTL       = 5 * time.Minute  // Cache subscription status for 5 minutes
	signatureMaxAge   = 5 * time.Minute  // Signatures older than this are rejected (replay protection)
	cacheCleanupEvery = 10 * time.Minute // How often to clean expired cache entries
)

// NewSubscriptionGate creates a new subscription gate
func NewSubscriptionGate(cm *contracts.ContractManager) *SubscriptionGate {
	sg := &SubscriptionGate{cm: cm}
	return sg
}

// StartCacheCleanup starts a background goroutine that periodically removes expired cache entries.
// Call this once after creating the gate. The goroutine stops when ctx is cancelled.
func (sg *SubscriptionGate) StartCacheCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(cacheCleanupEvery)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sg.cleanupExpiredCache()
			}
		}
	}()
	log.Printf("Subscription cache cleanup started (every %v)", cacheCleanupEvery)
}

// cleanupExpiredCache removes all cache entries older than the TTL
func (sg *SubscriptionGate) cleanupExpiredCache() {
	now := time.Now()
	expired := 0

	sg.cache.Range(func(key, value any) bool {
		c := value.(cachedSub)
		if now.Sub(c.checkedAt) > subCacheTTL {
			sg.cache.Delete(key)
			expired++
		}
		return true
	})

	if expired > 0 {
		log.Printf("Subscription cache cleanup: removed %d expired entries", expired)
	}
}

// SubscriptionError represents a subscription check failure
type SubscriptionError struct {
	Code    string `json:"error"`
	Message string `json:"message"`
	NodeID  string `json:"node_id,omitempty"`
	Price   string `json:"price_wei,omitempty"`
}

// VerifySubscription checks if the request has valid subscription credentials.
// It verifies:
//  1. subscriber_address query param or X-Subscriber-Address header
//  2. signature query param or X-Subscriber-Signature header (proves address ownership)
//  3. On-chain subscription status via SubscriptionManager.isSubscribed()
//
// Returns nil if access is granted, or a SubscriptionError explaining the denial.
func (sg *SubscriptionGate) VerifySubscription(r *http.Request, nodeID string) *SubscriptionError {
	// If no contract manager or no subscription contract configured, allow access
	if sg.cm == nil || !sg.cm.HasSubscriptionContract() {
		return nil // Dev mode: no gating
	}

	// Extract subscriber address from request
	subscriberAddr := r.URL.Query().Get("subscriber_address")
	if subscriberAddr == "" {
		subscriberAddr = r.Header.Get("X-Subscriber-Address")
	}
	if subscriberAddr == "" {
		return &SubscriptionError{
			Code:    "missing_credentials",
			Message: "Premium endpoint requires subscriber_address query param or X-Subscriber-Address header",
			NodeID:  nodeID,
		}
	}

	// Validate address format
	if !common.IsHexAddress(subscriberAddr) {
		return &SubscriptionError{
			Code:    "invalid_address",
			Message: "subscriber_address must be a valid Ethereum address",
		}
	}

	// Extract signature
	signature := r.URL.Query().Get("signature")
	if signature == "" {
		signature = r.Header.Get("X-Subscriber-Signature")
	}
	if signature == "" {
		return &SubscriptionError{
			Code:    "missing_signature",
			Message: "Premium endpoint requires signature to prove address ownership",
		}
	}

	// Extract timestamp (for replay protection)
	timestampStr := r.URL.Query().Get("timestamp")
	if timestampStr == "" {
		timestampStr = r.Header.Get("X-Subscriber-Timestamp")
	}
	if timestampStr == "" {
		return &SubscriptionError{
			Code:    "missing_timestamp",
			Message: "Premium endpoint requires timestamp for replay protection",
		}
	}

	// Verify signature proves ownership of the address
	// The message format is: "access:{nodeID}:{timestamp}"
	if err := sg.verifyAddressOwnership(subscriberAddr, nodeID, signature, timestampStr); err != nil {
		return &SubscriptionError{
			Code:    "invalid_signature",
			Message: fmt.Sprintf("Signature verification failed: %v", err),
		}
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", strings.ToLower(subscriberAddr), strings.ToLower(nodeID))
	if cached, ok := sg.cache.Load(cacheKey); ok {
		c := cached.(cachedSub)
		if time.Since(c.checkedAt) < subCacheTTL {
			if c.subscribed {
				return nil // Access granted (cached)
			}
			return &SubscriptionError{
				Code:    "not_subscribed",
				Message: "Your address is not subscribed to this node",
				NodeID:  nodeID,
			}
		}
	}

	// Query on-chain subscription status
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	subscribed, err := sg.cm.IsSubscribed(ctx, subscriberAddr, nodeID)
	if err != nil {
		log.Printf("Subscription check error for %s->%s: %v", subscriberAddr[:10], nodeID[:10], err)
		// On error, allow access but don't cache (fail-open for hackathon)
		return nil
	}

	// Cache the result
	sg.cache.Store(cacheKey, cachedSub{
		subscribed: subscribed,
		checkedAt:  time.Now(),
	})

	if !subscribed {
		// Try to get the price for helpful error message
		price, _ := sg.cm.GetSubscriptionPrice(ctx, nodeID)
		priceStr := ""
		if price != nil {
			priceStr = price.String()
		}
		return &SubscriptionError{
			Code:    "not_subscribed",
			Message: "Your address is not subscribed to this node. Subscribe via SubscriptionManager contract.",
			NodeID:  nodeID,
			Price:   priceStr,
		}
	}

	return nil // Access granted
}

// verifyAddressOwnership verifies that the signature proves ownership of the address
// Message format: "access:{nodeID}:{timestamp}" where timestamp is Unix seconds
func (sg *SubscriptionGate) verifyAddressOwnership(claimedAddr, nodeID, signature, timestampStr string) error {
	// Parse and validate timestamp
	var timestamp int64
	if _, err := fmt.Sscanf(timestampStr, "%d", &timestamp); err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}

	// Check timestamp is within acceptable range (replay protection)
	sigTime := time.Unix(timestamp, 0)
	age := time.Since(sigTime)
	if age < 0 {
		age = -age // Handle future timestamps
	}
	if age > signatureMaxAge {
		return fmt.Errorf("signature expired: signed %v ago (max %v)", age.Round(time.Second), signatureMaxAge)
	}

	// Build the message that should have been signed
	message := fmt.Sprintf("access:%s:%d", nodeID, timestamp)

	// Apply EIP-191 personal_sign prefix
	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	messageHash := crypto.Keccak256Hash([]byte(prefixedMessage))

	// Decode signature
	sigBytes, err := hexutil.Decode(signature)
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}

	if len(sigBytes) != 65 {
		return fmt.Errorf("invalid signature length: got %d, want 65", len(sigBytes))
	}

	// Normalize v value (27/28 → 0/1)
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	// Recover public key
	pubKey, err := crypto.SigToPub(messageHash.Bytes(), sigBytes)
	if err != nil {
		return fmt.Errorf("signature recovery failed: %w", err)
	}

	// Get address from public key
	recoveredAddr := crypto.PubkeyToAddress(*pubKey).Hex()

	// Compare addresses (case-insensitive)
	if !strings.EqualFold(recoveredAddr, claimedAddr) {
		return fmt.Errorf("signature is from %s, not %s", recoveredAddr[:10]+"...", claimedAddr[:10]+"...")
	}

	return nil
}

// WriteSubscriptionError writes a subscription error as JSON response
func WriteSubscriptionError(w http.ResponseWriter, err *SubscriptionError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusPaymentRequired) // 402 Payment Required
	json.NewEncoder(w).Encode(err)
}

// ClearCache removes a specific user-node subscription from cache (call after subscription events)
func (sg *SubscriptionGate) ClearCache(userAddr, nodeID string) {
	cacheKey := fmt.Sprintf("%s:%s", strings.ToLower(userAddr), strings.ToLower(nodeID))
	sg.cache.Delete(cacheKey)
}

// IsEnabled returns true if subscription gating is active (contract configured)
func (sg *SubscriptionGate) IsEnabled() bool {
	return sg.cm != nil && sg.cm.HasSubscriptionContract()
}
