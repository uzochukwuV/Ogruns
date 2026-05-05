package types

import (
	"fmt"
	"time"
)

// Signal validation constants
const (
	MaxExpiryHoursSpot      = 168 // 7 days for spot
	MaxExpiryHoursLeveraged = 48  // 48 hours for leveraged positions
	MaxLeverage             = 125 // Maximum allowed leverage
	WarnExpiryHours         = 12  // Warn if expiry > 12 hours
)

// SignalEnvelope represents the standard JSON structure pushed to 0G Storage
// by Node Developers. It includes the payload and the cryptographic signature.
type SignalEnvelope struct {
	NodeID           string        `json:"node_id"`
	Timestamp        int64         `json:"timestamp"`
	Payload          SignalPayload `json:"payload"`
	Signature        string        `json:"signature"`
	EncryptionPubKey string        `json:"encryption_pubkey,omitempty"`
}

// SignalValidationResult contains validation outcome and warnings
type SignalValidationResult struct {
	Valid    bool
	Error    string
	Warnings []string
}

// Validate checks if the signal envelope meets all requirements
func (e *SignalEnvelope) Validate() SignalValidationResult {
	result := SignalValidationResult{Valid: true, Warnings: []string{}}

	// Basic field validation
	if e.NodeID == "" {
		result.Valid = false
		result.Error = "node_id is required"
		return result
	}
	if e.Signature == "" {
		result.Valid = false
		result.Error = "signature is required"
		return result
	}

	// Payload validation
	p := &e.Payload

	if p.TokenPair == "" {
		result.Valid = false
		result.Error = "token_pair is required"
		return result
	}
	if p.Direction != "long" && p.Direction != "short" {
		result.Valid = false
		result.Error = "direction must be 'long' or 'short'"
		return result
	}
	if p.EntryPrice <= 0 {
		result.Valid = false
		result.Error = "entry_price must be positive"
		return result
	}
	if p.ExpiryTime <= time.Now().Unix() {
		result.Valid = false
		result.Error = "expiry_time must be in the future"
		return result
	}

	// Trade type validation
	tradeType := p.GetTradeType()
	if tradeType != "spot" && tradeType != "perpetual" && tradeType != "futures" {
		result.Valid = false
		result.Error = "trade_type must be 'spot', 'perpetual', or 'futures'"
		return result
	}

	// Leverage validation
	leverage := p.GetLeverage()
	if leverage > MaxLeverage {
		result.Valid = false
		result.Error = fmt.Sprintf("leverage cannot exceed %dx", MaxLeverage)
		return result
	}
	if tradeType == "spot" && leverage > 1 {
		result.Valid = false
		result.Error = "spot trades cannot have leverage > 1x"
		return result
	}

	// Stop loss required for leveraged trades
	if p.IsLeveraged() && p.StopLoss <= 0 {
		result.Valid = false
		result.Error = "stop_loss is required for leveraged trades"
		return result
	}

	// Expiry time validation
	expiryHours := float64(p.ExpiryTime-time.Now().Unix()) / 3600
	maxExpiry := float64(MaxExpiryHoursSpot)
	if p.IsLeveraged() {
		maxExpiry = float64(MaxExpiryHoursLeveraged)
	}
	if expiryHours > maxExpiry {
		result.Valid = false
		result.Error = fmt.Sprintf("expiry_time too far: %.1f hours (max %.0f for %s)", expiryHours, maxExpiry, tradeType)
		return result
	}

	// Warnings
	if expiryHours > WarnExpiryHours {
		result.Warnings = append(result.Warnings, fmt.Sprintf("long expiry (%.1f hours) increases outcome uncertainty", expiryHours))
	}
	if p.IsLeveraged() && leverage > 20 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("high leverage (%dx) carries significant risk", int(leverage)))
	}

	// Validate TP/SL logic for direction
	if p.Direction == "long" {
		if p.TakeProfit > 0 && p.TakeProfit <= p.EntryPrice {
			result.Warnings = append(result.Warnings, "take_profit should be above entry_price for long positions")
		}
		if p.StopLoss > 0 && p.StopLoss >= p.EntryPrice {
			result.Warnings = append(result.Warnings, "stop_loss should be below entry_price for long positions")
		}
	} else { // short
		if p.TakeProfit > 0 && p.TakeProfit >= p.EntryPrice {
			result.Warnings = append(result.Warnings, "take_profit should be below entry_price for short positions")
		}
		if p.StopLoss > 0 && p.StopLoss <= p.EntryPrice {
			result.Warnings = append(result.Warnings, "stop_loss should be above entry_price for short positions")
		}
	}

	return result
}

// SignalPayload contains the actual trading instructions
type SignalPayload struct {
	TokenPair  string  `json:"token_pair"`
	Exchange   string  `json:"exchange,omitempty"`
	Direction  string  `json:"direction"` // "long" or "short"
	EntryPrice float64 `json:"entry_price"`
	TakeProfit float64 `json:"take_profit"`
	StopLoss   float64 `json:"stop_loss"`
	ExpiryTime int64   `json:"expiry_time"`
	WeightPct  float64 `json:"weight_pct"`

	// Trade type and leverage (v2 fields)
	TradeType string  `json:"trade_type,omitempty"` // "spot", "perpetual", "futures" (default: "spot")
	Leverage  float64 `json:"leverage,omitempty"`   // 1x for spot, 1-125x for perpetuals

	// Multiple take profit levels (optional, overrides TakeProfit if set)
	TakeProfits []TakeProfitLevel `json:"take_profits,omitempty"`
}

// TakeProfitLevel defines a partial exit at a specific price
type TakeProfitLevel struct {
	Price   float64 `json:"price"`   // Target price
	Percent float64 `json:"percent"` // Percentage of position to close (0-100)
}

// GetTradeType returns the trade type, defaulting to "spot" if not set
func (p *SignalPayload) GetTradeType() string {
	if p.TradeType == "" {
		return "spot"
	}
	return p.TradeType
}

// GetLeverage returns the leverage, defaulting to 1x if not set
func (p *SignalPayload) GetLeverage() float64 {
	if p.Leverage <= 0 {
		return 1.0
	}
	return p.Leverage
}

// IsLeveraged returns true if the signal uses leverage > 1x
func (p *SignalPayload) IsLeveraged() bool {
	return p.GetLeverage() > 1.0
}
