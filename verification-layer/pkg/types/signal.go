package types

// SignalEnvelope represents the standard JSON structure pushed to 0G Storage
// by Node Developers. It includes the payload and the cryptographic signature.
type SignalEnvelope struct {
	NodeID           string        `json:"node_id"`
	Timestamp        int64         `json:"timestamp"`
	Payload          SignalPayload `json:"payload"`
	Signature        string        `json:"signature"`
	EncryptionPubKey string        `json:"encryption_pubkey,omitempty"`
}

// SignalPayload contains the actual trading instructions
type SignalPayload struct {
	TokenPair  string  `json:"token_pair"`
	Exchange   string  `json:"exchange,omitempty"`
	Direction  string  `json:"direction"`
	EntryPrice float64 `json:"entry_price"`
	TakeProfit float64 `json:"take_profit"`
	StopLoss   float64 `json:"stop_loss"`
	ExpiryTime int64   `json:"expiry_time"`
	WeightPct  float64 `json:"weight_pct"`
}
