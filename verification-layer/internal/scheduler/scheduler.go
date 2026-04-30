package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
)

// PriceFetcher interface for fetching prices at analysis time
type PriceFetcher interface {
	FetchPriceOnce(tokenPair string) (float64, error)
}

// AnalysisResult contains the outcome of signal analysis
type AnalysisResult struct {
	Signal       *types.ActiveSignal
	PriceAtExpiry float64
	Outcome      types.SignalState
	PnLPercent   float64 // Profit/Loss percentage
	Score        float64 // 0-100 score for this signal
	AnalyzedAt   int64
}

// ResultHandler is called when a signal analysis completes
type ResultHandler func(result AnalysisResult)

// SignalScheduler manages signals and schedules analysis at expiry time
type SignalScheduler struct {
	mu            sync.RWMutex
	signals       map[string]*scheduledSignal
	priceFetcher  PriceFetcher
	resultHandler ResultHandler
	ctx           context.Context
	cancel        context.CancelFunc
}

type scheduledSignal struct {
	signal *types.ActiveSignal
	timer  *time.Timer
}

func NewSignalScheduler(priceFetcher PriceFetcher, handler ResultHandler) *SignalScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &SignalScheduler{
		signals:       make(map[string]*scheduledSignal),
		priceFetcher:  priceFetcher,
		resultHandler: handler,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (s *SignalScheduler) Stop() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel all pending timers
	for _, ss := range s.signals {
		if ss.timer != nil {
			ss.timer.Stop()
		}
	}
}

// AddSignal registers a new signal and schedules analysis at expiry time
func (s *SignalScheduler) AddSignal(env types.SignalEnvelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("%s_%d", env.NodeID, env.Timestamp)

	// Check if already exists
	if _, exists := s.signals[id]; exists {
		return fmt.Errorf("signal %s already registered", id)
	}

	now := time.Now().Unix()
	expiryTime := env.Payload.ExpiryTime

	// Validate expiry time
	if expiryTime <= now {
		return fmt.Errorf("signal %s has already expired", id)
	}

	// Create active signal
	activeSig := &types.ActiveSignal{
		ID:       id,
		Envelope: env,
		State:    types.StatePending,
	}

	// Calculate duration until expiry
	duration := time.Duration(expiryTime-now) * time.Second

	// Schedule analysis timer
	timer := time.AfterFunc(duration, func() {
		s.analyzeSignal(id)
	})

	s.signals[id] = &scheduledSignal{
		signal: activeSig,
		timer:  timer,
	}

	log.Printf("Scheduler: Registered signal %s for %s (%s), analysis in %v",
		id, env.Payload.TokenPair, env.Payload.Direction, duration.Round(time.Second))

	return nil
}

// GetSignal returns a signal by ID
func (s *SignalScheduler) GetSignal(id string) (*types.ActiveSignal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if ss, ok := s.signals[id]; ok {
		return ss.signal, true
	}
	return nil, false
}

// GetPendingSignals returns all pending signals
func (s *SignalScheduler) GetPendingSignals() []*types.ActiveSignal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var signals []*types.ActiveSignal
	for _, ss := range s.signals {
		if ss.signal.State == types.StatePending || ss.signal.State == types.StateActive {
			signals = append(signals, ss.signal)
		}
	}
	return signals
}

// GetSignalCount returns the number of pending signals
func (s *SignalScheduler) GetSignalCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.signals)
}

// analyzeSignal runs at expiry time to evaluate the signal
func (s *SignalScheduler) analyzeSignal(id string) {
	s.mu.Lock()
	ss, exists := s.signals[id]
	if !exists {
		s.mu.Unlock()
		log.Printf("Scheduler: Signal %s not found for analysis", id)
		return
	}

	signal := ss.signal
	payload := signal.Envelope.Payload
	s.mu.Unlock()

	log.Printf("Scheduler: Analyzing signal %s for %s at expiry", id, payload.TokenPair)

	// Fetch current price
	currentPrice, err := s.priceFetcher.FetchPriceOnce(payload.TokenPair)
	if err != nil {
		log.Printf("Scheduler: Failed to fetch price for %s: %v", payload.TokenPair, err)
		// Mark as expired without price data
		s.finalizeSignal(id, 0, types.StateExpired, 0, 0)
		return
	}

	log.Printf("Scheduler: %s price at expiry: $%.2f (entry: $%.2f, TP: $%.2f, SL: $%.2f)",
		payload.TokenPair, currentPrice, payload.EntryPrice, payload.TakeProfit, payload.StopLoss)

	// Evaluate signal outcome
	outcome, pnlPercent, score := s.evaluateSignal(payload, currentPrice)

	// Finalize the signal
	s.finalizeSignal(id, currentPrice, outcome, pnlPercent, score)
}

// evaluateSignal determines the outcome based on price at expiry
func (s *SignalScheduler) evaluateSignal(payload types.SignalPayload, priceAtExpiry float64) (types.SignalState, float64, float64) {
	isLong := payload.Direction == "LONG"

	// Calculate PnL percentage from entry
	var pnlPercent float64
	if isLong {
		pnlPercent = ((priceAtExpiry - payload.EntryPrice) / payload.EntryPrice) * 100
	} else {
		pnlPercent = ((payload.EntryPrice - priceAtExpiry) / payload.EntryPrice) * 100
	}

	// Determine outcome
	var outcome types.SignalState
	var score float64

	// Calculate target distances
	tpDistance := abs(payload.TakeProfit - payload.EntryPrice)
	slDistance := abs(payload.StopLoss - payload.EntryPrice)

	if isLong {
		if priceAtExpiry >= payload.TakeProfit {
			// Hit take profit
			outcome = types.StateClosedWin
			score = 100
		} else if priceAtExpiry <= payload.StopLoss {
			// Hit stop loss
			outcome = types.StateClosedLoss
			score = 0
		} else if priceAtExpiry > payload.EntryPrice {
			// Profit but didn't hit TP - partial win
			outcome = types.StateExpired
			// Score based on how close to TP vs Entry
			progress := (priceAtExpiry - payload.EntryPrice) / tpDistance
			score = 50 + (progress * 50) // 50-100 range
		} else {
			// Loss but didn't hit SL - partial loss
			outcome = types.StateExpired
			// Score based on how close to SL vs Entry
			progress := (payload.EntryPrice - priceAtExpiry) / slDistance
			score = 50 - (progress * 50) // 0-50 range
		}
	} else { // SHORT
		if priceAtExpiry <= payload.TakeProfit {
			outcome = types.StateClosedWin
			score = 100
		} else if priceAtExpiry >= payload.StopLoss {
			outcome = types.StateClosedLoss
			score = 0
		} else if priceAtExpiry < payload.EntryPrice {
			// Profit but didn't hit TP
			outcome = types.StateExpired
			progress := (payload.EntryPrice - priceAtExpiry) / tpDistance
			score = 50 + (progress * 50)
		} else {
			// Loss but didn't hit SL
			outcome = types.StateExpired
			progress := (priceAtExpiry - payload.EntryPrice) / slDistance
			score = 50 - (progress * 50)
		}
	}

	// Clamp score to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return outcome, pnlPercent, score
}

func (s *SignalScheduler) finalizeSignal(id string, priceAtExpiry float64, outcome types.SignalState, pnlPercent, score float64) {
	s.mu.Lock()
	ss, exists := s.signals[id]
	if !exists {
		s.mu.Unlock()
		return
	}

	signal := ss.signal
	signal.State = outcome
	signal.ClosedAt = time.Now().Unix()
	signal.ClosedPrice = priceAtExpiry

	// Remove from pending signals
	delete(s.signals, id)
	s.mu.Unlock()

	log.Printf("Scheduler: Signal %s finalized -> %s (PnL: %.2f%%, Score: %.1f)",
		id, outcome, pnlPercent, score)

	// Call result handler
	if s.resultHandler != nil {
		result := AnalysisResult{
			Signal:        signal,
			PriceAtExpiry: priceAtExpiry,
			Outcome:       outcome,
			PnLPercent:    pnlPercent,
			Score:         score,
			AnalyzedAt:    time.Now().Unix(),
		}
		s.resultHandler(result)
	}
}

// ForceAnalyze immediately analyzes a signal (for testing)
func (s *SignalScheduler) ForceAnalyze(id string) error {
	s.mu.Lock()
	ss, exists := s.signals[id]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("signal %s not found", id)
	}

	// Stop the timer
	if ss.timer != nil {
		ss.timer.Stop()
	}
	s.mu.Unlock()

	// Run analysis now
	s.analyzeSignal(id)
	return nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
