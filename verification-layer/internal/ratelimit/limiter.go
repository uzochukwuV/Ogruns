package ratelimit

import (
	"sync"
	"time"
)

// RateLimiter provides per-node and global rate limiting for signal submission
type RateLimiter struct {
	mu sync.RWMutex

	// Per-node limits
	nodeWindow    time.Duration
	nodeMaxCount  int
	nodeRequests  map[string][]int64 // nodeID -> timestamps

	// Global limits
	globalWindow   time.Duration
	globalMaxCount int
	globalRequests []int64

	// Duplicate detection
	recentSignals map[string]int64 // signalHash -> timestamp
	dupWindow     time.Duration
}

// Config for rate limiter
type Config struct {
	// Per-node: max signals per window (e.g., 10 signals per minute)
	NodeMaxSignals int
	NodeWindow     time.Duration

	// Global: max signals per window (e.g., 100 signals per minute)
	GlobalMaxSignals int
	GlobalWindow     time.Duration

	// Duplicate detection window
	DuplicateWindow time.Duration
}

// DefaultConfig returns sensible defaults for a hackathon demo
func DefaultConfig() Config {
	return Config{
		NodeMaxSignals:   10,              // 10 signals per minute per node
		NodeWindow:       time.Minute,
		GlobalMaxSignals: 100,             // 100 signals per minute global
		GlobalWindow:     time.Minute,
		DuplicateWindow:  5 * time.Minute, // Reject exact duplicates within 5 min
	}
}

// NewRateLimiter creates a new rate limiter with the given config
func NewRateLimiter(cfg Config) *RateLimiter {
	rl := &RateLimiter{
		nodeWindow:     cfg.NodeWindow,
		nodeMaxCount:   cfg.NodeMaxSignals,
		nodeRequests:   make(map[string][]int64),
		globalWindow:   cfg.GlobalWindow,
		globalMaxCount: cfg.GlobalMaxSignals,
		globalRequests: make([]int64, 0),
		recentSignals:  make(map[string]int64),
		dupWindow:      cfg.DuplicateWindow,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed       bool
	Reason        string
	RetryAfterSec int
}

// Check returns whether the signal is allowed
func (rl *RateLimiter) Check(nodeID string, signalHash string) RateLimitResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now().Unix()

	// 1. Check for duplicate signal
	if lastSeen, exists := rl.recentSignals[signalHash]; exists {
		age := now - lastSeen
		if age < int64(rl.dupWindow.Seconds()) {
			return RateLimitResult{
				Allowed:       false,
				Reason:        "duplicate_signal",
				RetryAfterSec: int(int64(rl.dupWindow.Seconds()) - age),
			}
		}
	}

	// 2. Check global rate limit
	windowStart := now - int64(rl.globalWindow.Seconds())
	rl.globalRequests = filterAfter(rl.globalRequests, windowStart)

	if len(rl.globalRequests) >= rl.globalMaxCount {
		oldest := rl.globalRequests[0]
		retryAfter := int(oldest + int64(rl.globalWindow.Seconds()) - now)
		return RateLimitResult{
			Allowed:       false,
			Reason:        "global_rate_limit",
			RetryAfterSec: max(retryAfter, 1),
		}
	}

	// 3. Check per-node rate limit
	nodeWindowStart := now - int64(rl.nodeWindow.Seconds())
	if reqs, exists := rl.nodeRequests[nodeID]; exists {
		rl.nodeRequests[nodeID] = filterAfter(reqs, nodeWindowStart)
	}

	nodeReqs := rl.nodeRequests[nodeID]
	if len(nodeReqs) >= rl.nodeMaxCount {
		oldest := nodeReqs[0]
		retryAfter := int(oldest + int64(rl.nodeWindow.Seconds()) - now)
		return RateLimitResult{
			Allowed:       false,
			Reason:        "node_rate_limit",
			RetryAfterSec: max(retryAfter, 1),
		}
	}

	// All checks passed - record the request
	rl.globalRequests = append(rl.globalRequests, now)
	rl.nodeRequests[nodeID] = append(rl.nodeRequests[nodeID], now)
	rl.recentSignals[signalHash] = now

	return RateLimitResult{Allowed: true}
}

// GetStats returns current rate limit statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	now := time.Now().Unix()
	globalWindowStart := now - int64(rl.globalWindow.Seconds())

	// Count active global requests
	activeGlobal := 0
	for _, ts := range rl.globalRequests {
		if ts > globalWindowStart {
			activeGlobal++
		}
	}

	return map[string]interface{}{
		"global_requests_in_window": activeGlobal,
		"global_max":                rl.globalMaxCount,
		"global_window_sec":         int(rl.globalWindow.Seconds()),
		"node_max":                  rl.nodeMaxCount,
		"node_window_sec":           int(rl.nodeWindow.Seconds()),
		"tracked_nodes":             len(rl.nodeRequests),
		"cached_signal_hashes":      len(rl.recentSignals),
	}
}

// cleanup periodically removes old entries to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()

		now := time.Now().Unix()

		// Clean global requests
		globalCutoff := now - int64(rl.globalWindow.Seconds())*2
		rl.globalRequests = filterAfter(rl.globalRequests, globalCutoff)

		// Clean per-node requests
		nodeCutoff := now - int64(rl.nodeWindow.Seconds())*2
		for nodeID, reqs := range rl.nodeRequests {
			filtered := filterAfter(reqs, nodeCutoff)
			if len(filtered) == 0 {
				delete(rl.nodeRequests, nodeID)
			} else {
				rl.nodeRequests[nodeID] = filtered
			}
		}

		// Clean duplicate cache
		dupCutoff := now - int64(rl.dupWindow.Seconds())
		for hash, ts := range rl.recentSignals {
			if ts < dupCutoff {
				delete(rl.recentSignals, hash)
			}
		}

		rl.mu.Unlock()
	}
}

// filterAfter returns only timestamps after the cutoff
func filterAfter(timestamps []int64, cutoff int64) []int64 {
	result := make([]int64, 0, len(timestamps))
	for _, ts := range timestamps {
		if ts > cutoff {
			result = append(result, ts)
		}
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
