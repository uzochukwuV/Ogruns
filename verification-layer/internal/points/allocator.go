package points

import (
	"context"
	"log"
	"math"
)

// Database interface for point operations.
type Database interface {
	CreditProviderPoints(ctx context.Context, providerAddress string, points int64, signalID string) error
	CountSubscribers(ctx context.Context, providerAddress string) (int, error)
	GetSubscriberTier(ctx context.Context, userID, providerAddress string) (string, error)
	GetAllSubscribersForProvider(ctx context.Context, providerAddress string) ([]struct{ UserID string; Tier string }, error)
}

// Allocator calculates and credits performance-based points to signal providers.
type Allocator struct {
	db Database
}

// NewAllocator creates a new point allocator.
func NewAllocator(db Database) *Allocator {
	return &Allocator{db: db}
}

// AllocatePoints calculates and credits points to a provider based on signal performance.
// Formula: POINTS = BASE_POINTS × OUTCOME_MULTIPLIER × SUBSCRIBER_MULTIPLIER
func (a *Allocator) AllocatePoints(ctx context.Context, providerAddress, signalID string, pnlPercent float64) error {
	// Get all subscribers for this provider to calculate tier distribution and count
	subscribers, err := a.db.GetAllSubscribersForProvider(ctx, providerAddress)
	if err != nil {
		return err
	}

	if len(subscribers) == 0 {
		log.Printf("[PointAllocator] No subscribers for provider %s, skipping allocation", providerAddress)
		return nil
	}

	// Calculate average base points across all subscriber tiers
	totalBasePoints := 0
	for _, sub := range subscribers {
		basePoints := getBasePoints(sub.Tier)
		totalBasePoints += basePoints
	}
	avgBasePoints := float64(totalBasePoints) / float64(len(subscribers))

	// Calculate outcome multiplier based on PnL
	outcomeMultiplier := getOutcomeMultiplier(pnlPercent)

	// Calculate subscriber multiplier based on total subscriber count
	subscriberMultiplier := getSubscriberMultiplier(len(subscribers))

	// Calculate total points
	pointsFloat := avgBasePoints * outcomeMultiplier * subscriberMultiplier
	points := int64(math.Round(pointsFloat))

	// Don't credit points for terrible performance (0x multiplier)
	if points <= 0 {
		log.Printf("[PointAllocator] Signal %s from %s: PnL=%.2f%%, points=0 (poor performance)",
			signalID, providerAddress, pnlPercent)
		return nil
	}

	// Credit points to provider
	if err := a.db.CreditProviderPoints(ctx, providerAddress, points, signalID); err != nil {
		return err
	}

	log.Printf("[PointAllocator] Signal %s from %s: PnL=%.2f%%, subscribers=%d, points=%d (base=%.1f, outcome=%.1fx, subscriber=%.1fx)",
		signalID, providerAddress, pnlPercent, len(subscribers), points, avgBasePoints, outcomeMultiplier, subscriberMultiplier)

	return nil
}

// getBasePoints returns the base points for a given subscription tier.
func getBasePoints(tier string) int {
	switch tier {
	case "BRONZE":
		return 10
	case "SILVER":
		return 15
	case "GOLD":
		return 20
	case "DIAMOND":
		return 25
	default:
		return 10 // default to BRONZE
	}
}

// getOutcomeMultiplier returns the multiplier based on signal PnL performance.
func getOutcomeMultiplier(pnlPercent float64) float64 {
	switch {
	case pnlPercent > 10.0:
		return 5.0 // Excellent: > +10%
	case pnlPercent >= 5.0:
		return 3.0 // Very good: +5% to +10%
	case pnlPercent >= 2.0:
		return 2.0 // Good: +2% to +5%
	case pnlPercent >= 0.5:
		return 1.5 // Okay: +0.5% to +2%
	case pnlPercent >= -0.5:
		return 0.5 // Break-even: -0.5% to +0.5%
	case pnlPercent >= -2.0:
		return 0.2 // Poor: -2% to -0.5%
	default:
		return 0.0 // Terrible: < -2%
	}
}

// getSubscriberMultiplier returns the multiplier based on total subscriber count.
func getSubscriberMultiplier(subscriberCount int) float64 {
	switch {
	case subscriberCount >= 500:
		return 3.0 // Mega influencer: 500+
	case subscriberCount >= 101:
		return 2.0 // Major influencer: 101-500
	case subscriberCount >= 51:
		return 1.5 // Medium influencer: 51-100
	case subscriberCount >= 11:
		return 1.2 // Small influencer: 11-50
	default:
		return 1.0 // Starting out: 1-10
	}
}
