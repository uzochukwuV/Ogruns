package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
)

// ResolutionEngine is the core matching engine that compares incoming high-frequency
// WebSocket ticks against all active and pending trading signals.
type ResolutionEngine struct {
	mu           sync.RWMutex
	signals      map[string]*types.ActiveSignal
	eventStream  chan *types.ActiveSignal // Emits state changes
	tickStream   <-chan types.Tick
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewResolutionEngine(tickStream <-chan types.Tick) *ResolutionEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &ResolutionEngine{
		signals:      make(map[string]*types.ActiveSignal),
		eventStream:  make(chan *types.ActiveSignal, 1000),
		tickStream:   tickStream,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (e *ResolutionEngine) EventStream() <-chan *types.ActiveSignal {
	return e.eventStream
}

// AddSignal registers a newly ingested, verified signal from 0G Storage
func (e *ResolutionEngine) AddSignal(env types.SignalEnvelope) {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := fmt.Sprintf("%s_%d", env.NodeID, env.Timestamp)
	
	activeSig := &types.ActiveSignal{
		ID:       id,
		Envelope: env,
		State:    types.StatePending,
	}
	
	e.signals[id] = activeSig
	log.Printf("Engine: Registered new PENDING signal for %s (%s)", env.Payload.TokenPair, env.Payload.Direction)
}

func (e *ResolutionEngine) Start() {
	go e.processTicks()
	go e.cleanupExpired()
}

func (e *ResolutionEngine) Stop() {
	e.cancel()
}

// processTicks is the ultra-fast matching loop
func (e *ResolutionEngine) processTicks() {
	for {
		select {
		case <-e.ctx.Done():
			return
		case tick := <-e.tickStream:
			e.evaluateTick(tick)
		}
	}
}

// evaluateTick compares a single tick against all relevant signals
func (e *ResolutionEngine) evaluateTick(tick types.Tick) {
	e.mu.RLock()
	// In a production system with 1M signals, you would index e.signals by TokenPair
	// For this MVP, we iterate over the map.
	for id, sig := range e.signals {
		if sig.Envelope.Payload.TokenPair != tick.TokenPair {
			continue
		}
		
		// If signal is already closed, ignore (cleaned up periodically)
		if sig.State == types.StateClosedWin || sig.State == types.StateClosedLoss || sig.State == types.StateExpired || sig.State == types.StateCancelled {
			continue
		}

		// Fast path resolution logic
		e.resolveSignal(id, sig, tick)
	}
	e.mu.RUnlock()
}

func (e *ResolutionEngine) resolveSignal(id string, sig *types.ActiveSignal, tick types.Tick) {
	payload := sig.Envelope.Payload
	isLong := payload.Direction == "LONG"
	isShort := payload.Direction == "SHORT"
	now := time.Now().Unix()

	// Handle State: PENDING -> ACTIVE
	if sig.State == types.StatePending {
		// Did we cross the entry price?
		// For LONG: if tick drops below or equals entry
		// For SHORT: if tick spikes above or equals entry
		hitEntry := (isLong && tick.Price <= payload.EntryPrice) || (isShort && tick.Price >= payload.EntryPrice)
		
		if hitEntry {
			e.mu.RUnlock() // Escalate lock to write
			e.mu.Lock()
			sig.State = types.StateActive
			sig.EntryHitAt = now
			log.Printf("Engine: %s hit ENTRY at %.4f -> ACTIVE", sig.ID, tick.Price)
			e.eventStream <- sig
			e.mu.Unlock()
			e.mu.RLock() // Re-acquire read lock
		}
		return
	}

	// Handle State: ACTIVE -> CLOSED
	if sig.State == types.StateActive {
		// Check Take Profit
		hitTP := (isLong && tick.Price >= payload.TakeProfit) || (isShort && tick.Price <= payload.TakeProfit)
		
		// Check Stop Loss
		hitSL := (isLong && tick.Price <= payload.StopLoss) || (isShort && tick.Price >= payload.StopLoss)

		if hitTP || hitSL {
			e.mu.RUnlock() // Escalate lock to write
			e.mu.Lock()
			
			sig.ClosedAt = now
			sig.ClosedPrice = tick.Price
			
			if hitTP {
				sig.State = types.StateClosedWin
				log.Printf("Engine: %s hit TAKE PROFIT at %.4f -> WIN", sig.ID, tick.Price)
			} else {
				sig.State = types.StateClosedLoss
				log.Printf("Engine: %s hit STOP LOSS at %.4f -> LOSS", sig.ID, tick.Price)
			}
			
			e.eventStream <- sig
			e.mu.Unlock()
			e.mu.RLock() // Re-acquire read lock
		}
	}
}

// cleanupExpired runs every minute to sweep out EXPIRED or CANCELLED signals
func (e *ResolutionEngine) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.mu.Lock()
			now := time.Now().Unix()
			
			for id, sig := range e.signals {
				if sig.Envelope.Payload.ExpiryTime < now {
					if sig.State == types.StatePending {
						sig.State = types.StateCancelled
						log.Printf("Engine: %s EXPIRED before entry -> CANCELLED", id)
					} else if sig.State == types.StateActive {
						sig.State = types.StateExpired
						sig.ClosedAt = now
						// Note: ClosedPrice should ideally be fetched from the last tick
						log.Printf("Engine: %s EXPIRED while active -> EXPIRED", id)
					}
					
					e.eventStream <- sig
					// Remove from memory to prevent memory leaks
					delete(e.signals, id)
				}
			}
			e.mu.Unlock()
		}
	}
}
