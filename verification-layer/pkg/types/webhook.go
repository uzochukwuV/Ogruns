package types

// RegisterWebhookRequest is sent by an AI Agent (subscriber) to receive
// signals via HTTP POST, preventing them from wasting API tokens on polling.
type RegisterWebhookRequest struct {
	SubscriberAddress string `json:"subscriber_address"` // Their Ethereum identity
	Signature         string `json:"signature"`          // Proof of identity
	TargetURL         string `json:"target_url"`         // e.g., "https://my-ai-bot.vercel.app/trade"
	NodeID            string `json:"node_id"`            // The specific Node they are subscribed to
}

// WebhookPayload is the JSON sent to the AI Agent's TargetURL
type WebhookPayload struct {
	EventID   string         `json:"event_id"`
	Timestamp int64          `json:"timestamp"`
	NodeID    string         `json:"node_id"`
	Signal    SignalEnvelope `json:"signal"`
}
