package crypto

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/0xprotocol/verification-layer/pkg/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// VerifySignal validates that the ECDSA signature on the SignalEnvelope
// was produced by the private key corresponding to the NodeID.
func VerifySignal(env types.SignalEnvelope) (bool, error) {
	// 1. Serialize the payload + timestamp deterministically
	// We use a strict JSON representation for the payload
	payloadBytes, err := json.Marshal(env.Payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 2. Construct the exact message string that was signed
	// Format: <timestamp>:<payload_json>
	message := fmt.Sprintf("%d:%s", env.Timestamp, string(payloadBytes))

	// 3. Hash the message using standard Ethereum signed message format
	// "\x19Ethereum Signed Message:\n" + len(message) + message
	
	// If the node signed via personal_sign, the actual hash is prefixed
	// We'll support standard EIP-191 personal sign format
	prefixedHash := crypto.Keccak256Hash(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)),
	)

	// 4. Decode the signature from hex
	sigBytes, err := hexutil.Decode(env.Signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}

	// 5. Handle Ethereum's signature recovery ID offset (V)
	// Ethereum adds 27 to V. Ecrecover expects 0 or 1.
	if len(sigBytes) == 65 {
		if sigBytes[64] == 27 || sigBytes[64] == 28 {
			sigBytes[64] -= 27
		}
	} else {
		return false, fmt.Errorf("invalid signature length: expected 65, got %d", len(sigBytes))
	}

	// 6. Recover the public key from the signature and hash
	pubKeyBytes, err := crypto.Ecrecover(prefixedHash.Bytes(), sigBytes)
	if err != nil {
		return false, fmt.Errorf("failed to recover public key: %w", err)
	}

	// 7. Convert recovered public key to Ethereum address
	recoveredPubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal public key: %w", err)
	}
	
	recoveredAddress := crypto.PubkeyToAddress(*recoveredPubKey).Hex()

	// 8. Compare the recovered address with the claimed NodeID
	if strings.EqualFold(recoveredAddress, env.NodeID) {
		return true, nil
	}

	return false, fmt.Errorf("signature mismatch: recovered %s, expected %s", recoveredAddress, env.NodeID)
}
