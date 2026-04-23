package crypto

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestVerifySignal(t *testing.T) {
	// 1. Generate a fresh private key (acts as our Node Developer)
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// 2. Derive the Node ID (Ethereum Address)
	pubKey := privateKey.PublicKey
	nodeID := crypto.PubkeyToAddress(pubKey).Hex()

	// 3. Create a dummy signal payload
	payload := types.SignalPayload{
		TokenPair:  "WIF/USDT",
		Direction:  "LONG",
		EntryPrice: 3.05,
		TakeProfit: 3.50,
		StopLoss:   2.80,
		ExpiryTime: time.Now().Add(1 * time.Hour).Unix(),
		WeightPct:  85.5,
	}

	// 4. Construct the message exactly as the verifier expects it
	timestamp := time.Now().Unix()
	payloadBytes, _ := json.Marshal(payload)
	message := fmt.Sprintf("%d:%s", timestamp, string(payloadBytes))

	// 5. Hash and sign the message (EIP-191 format)
	prefixedHash := crypto.Keccak256Hash(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)),
	)
	
	sigBytes, err := crypto.Sign(prefixedHash.Bytes(), privateKey)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}

	// Adjust V for Ethereum (add 27)
	sigBytes[64] += 27
	signatureHex := hexutil.Encode(sigBytes)

	// 6. Create the final envelope
	envelope := types.SignalEnvelope{
		NodeID:    nodeID,
		Timestamp: timestamp,
		Payload:   payload,
		Signature: signatureHex,
	}

	// 7. Test the verification function
	valid, err := VerifySignal(envelope)
	if err != nil {
		t.Fatalf("VerifySignal returned error: %v", err)
	}
	if !valid {
		t.Errorf("Expected signature to be valid, but it was rejected")
	}

	// 8. Test tampering (changing the price)
	envelope.Payload.EntryPrice = 1.00 // Attacker changes the price
	valid, err = VerifySignal(envelope)
	if err == nil || valid {
		t.Errorf("Expected tampered signature to fail, but it passed")
	}
}
