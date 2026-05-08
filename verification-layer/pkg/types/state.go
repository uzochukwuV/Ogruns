package types

// Tick represents a normalized, high-frequency price update from an exchange
type Tick struct {
	Exchange  string
	TokenPair string
	Price     float64
	Timestamp int64
}

// SignalState tracks the lifecycle of a verified signal
type SignalState string

const (
	StatePending    SignalState = "PENDING"   // Waiting for Entry Price
	StateActive     SignalState = "ACTIVE"    // Entry Price hit, waiting for TP/SL
	StateClosedWin  SignalState = "WIN"       // Take Profit hit
	StateClosedLoss SignalState = "LOSS"      // Stop Loss hit
	StateExpired    SignalState = "EXPIRED"   // Expiry Time reached before TP/SL
	StateCancelled  SignalState = "CANCELLED" // Expiry Time reached before Entry
)

// ActiveSignal wraps a SignalEnvelope with its current state and metadata
type ActiveSignal struct {
	ID          string         `json:"id"`           // Unique identifier (e.g., node_id + timestamp)
	Envelope    SignalEnvelope `json:"envelope"`     // The original signal envelope
	State       SignalState    `json:"state"`        // Current state of the signal
	EntryHitAt  int64          `json:"entry_hit_at"` // Timestamp when State changed from PENDING to ACTIVE
	ClosedAt    int64          `json:"closed_at"`    // Timestamp when State changed to CLOSED
	ClosedPrice float64        `json:"closed_price"` // The price at the time of closing
}
