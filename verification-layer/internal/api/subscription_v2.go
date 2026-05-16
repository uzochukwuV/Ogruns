package api

import (
	"context"
	"log"
	"sync"
	"time"
)

// SubscriptionDatabase interface for subscription cache operations.
type SubscriptionDatabase interface {
	GetAllActiveProviderAddresses(ctx context.Context, userID string) ([]string, error)
}

// SubscriptionCacheV2 provides a cached view of user subscriptions for WebSocket filtering.
// Refreshes every 5 minutes to reduce database load.
type SubscriptionCacheV2 struct {
	db           SubscriptionDatabase
	cache        sync.Map // userID -> []providerAddress
	refreshInterval time.Duration
	stopChan     chan struct{}
}

// NewSubscriptionCacheV2 creates a new subscription cache.
func NewSubscriptionCacheV2(db SubscriptionDatabase) *SubscriptionCacheV2 {
	return &SubscriptionCacheV2{
		db:              db,
		refreshInterval: 5 * time.Minute,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the periodic cache refresh loop.
func (sc *SubscriptionCacheV2) Start(ctx context.Context) {
	log.Println("[SubscriptionCacheV2] Starting subscription cache refresh loop...")
	go sc.refreshLoop(ctx)
}

// Stop gracefully stops the cache refresh loop.
func (sc *SubscriptionCacheV2) Stop() {
	close(sc.stopChan)
}

// GetSubscribedProviders returns the list of provider addresses the user is subscribed to.
// Returns a cached value if available, otherwise fetches from DB.
func (sc *SubscriptionCacheV2) GetSubscribedProviders(ctx context.Context, userID string) ([]string, error) {
	// Try to get from cache first
	if val, ok := sc.cache.Load(userID); ok {
		return val.([]string), nil
	}

	// Cache miss - fetch from DB
	return sc.fetchAndCacheProviders(ctx, userID)
}

// IsSubscribed checks if a user is subscribed to a specific provider.
func (sc *SubscriptionCacheV2) IsSubscribed(ctx context.Context, userID, providerAddress string) (bool, error) {
	providers, err := sc.GetSubscribedProviders(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, p := range providers {
		if p == providerAddress {
			return true, nil
		}
	}

	return false, nil
}

// InvalidateUser removes a user's subscription cache entry, forcing a fresh DB lookup.
func (sc *SubscriptionCacheV2) InvalidateUser(userID string) {
	sc.cache.Delete(userID)
}

// refreshLoop periodically refreshes the cache for all active users.
func (sc *SubscriptionCacheV2) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(sc.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sc.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.refreshAllUsers(ctx)
		}
	}
}

// refreshAllUsers refreshes the cache for all users with active subscriptions.
func (sc *SubscriptionCacheV2) refreshAllUsers(ctx context.Context) {
	// Iterate over all cached users and refresh
	sc.cache.Range(func(key, _ interface{}) bool {
		userID := key.(string)
		_, err := sc.fetchAndCacheProviders(ctx, userID)
		if err != nil {
			log.Printf("[SubscriptionCacheV2] Failed to refresh cache for user %s: %v", userID, err)
		}
		return true
	})
}

// fetchAndCacheProviders fetches provider addresses from DB and caches them.
func (sc *SubscriptionCacheV2) fetchAndCacheProviders(ctx context.Context, userID string) ([]string, error) {
	providers, err := sc.db.GetAllActiveProviderAddresses(ctx, userID)
	if err != nil {
		return nil, err
	}

	sc.cache.Store(userID, providers)
	return providers, nil
}
