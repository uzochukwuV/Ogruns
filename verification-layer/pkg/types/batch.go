package types

// SignalBatch represents a collection of signals uploaded to 0G Storage
type SignalBatch struct {
	BatchID   string           `json:"batch_id"`  // e.g. "batch_2026-04-23"
	Timestamp int64            `json:"timestamp"` // Time the batch was created
	Signals   []SignalEnvelope `json:"signals"`   // The raw signals
}

// SubmitRawSignalRequest is sent by a Node directly to the Verifier API.
// This acts as a "Layer 2" sequencer, saving the Node from paying 0G gas fees.
type SubmitRawSignalRequest struct {
	NodeID   string         `json:"node_id"`
	Envelope SignalEnvelope `json:"envelope"` // The cryptographically signed payload
}
