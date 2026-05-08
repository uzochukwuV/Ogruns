"""
EIP-191 Signer for Trading Signals
Signs signal envelopes for cryptographic verification
"""
import json
import time
from typing import Dict, Any
from dataclasses import asdict

from eth_account import Account
from eth_account.messages import encode_defunct

from generator import SignalPayload


def _go_compatible_value(v: Any) -> Any:
    """Convert value to match Go's json.Marshal output.

    Go outputs whole number floats without decimal: 65000 not 65000.0
    """
    if isinstance(v, float) and v == int(v):
        return int(v)
    elif isinstance(v, dict):
        return {k: _go_compatible_value(val) for k, val in v.items()}
    elif isinstance(v, list):
        return [_go_compatible_value(item) for item in v]
    return v


def create_envelope(
    private_key: str,
    payload: SignalPayload,
    timestamp: int = None,
) -> Dict:
    """
    Create a signed signal envelope.

    The envelope contains:
    - node_id: Agent's Ethereum address
    - timestamp: Unix timestamp
    - payload: Signal details
    - signature: EIP-191 signature

    Args:
        private_key: Hex-encoded private key (with or without 0x prefix)
        payload: SignalPayload object
        timestamp: Unix timestamp (default: current time)

    Returns:
        Signed envelope dict ready for API submission
    """
    # Normalize private key
    if not private_key.startswith("0x"):
        private_key = "0x" + private_key

    account = Account.from_key(private_key)
    node_id = account.address

    if timestamp is None:
        timestamp = int(time.time())

    # Convert payload to dict with correct field ordering
    # IMPORTANT: Field order must match Go's json.Marshal for signature verification
    payload_dict = {
        "token_pair": payload.token_pair,
    }

    # Add optional fields only if they have values (omitempty behavior)
    if payload.exchange:
        payload_dict["exchange"] = payload.exchange

    payload_dict["direction"] = payload.direction
    payload_dict["entry_price"] = payload.entry_price
    payload_dict["take_profit"] = payload.take_profit
    payload_dict["stop_loss"] = payload.stop_loss
    payload_dict["expiry_time"] = payload.expiry_time
    payload_dict["weight_pct"] = payload.weight_pct

    # Optional fields (omitempty)
    if payload.trade_type and payload.trade_type != "spot":
        payload_dict["trade_type"] = payload.trade_type
    if payload.leverage and payload.leverage > 1:
        payload_dict["leverage"] = payload.leverage

    # Convert floats to ints where appropriate (Go outputs 65000 not 65000.0)
    payload_dict = _go_compatible_value(payload_dict)

    # Create the message to sign (must match Go's verify.go format)
    # Go format: "<timestamp>:<payload_json>"
    payload_json = json.dumps(payload_dict, separators=(",", ":"), sort_keys=False)
    message = f"{timestamp}:{payload_json}"

    # Sign with EIP-191
    message_hash = encode_defunct(text=message)
    signed = account.sign_message(message_hash)
    signature = "0x" + signed.signature.hex()

    return {
        "node_id": node_id,
        "timestamp": timestamp,
        "payload": payload_dict,
        "signature": signature,
    }


def verify_envelope(envelope: Dict) -> bool:
    """
    Verify an envelope's signature.

    Returns True if the signature is valid, False otherwise.
    """
    try:
        node_id = envelope["node_id"]
        timestamp = envelope["timestamp"]
        payload = envelope["payload"]
        signature = envelope["signature"]

        # Reconstruct the message (must match Go's verify.go format)
        payload = _go_compatible_value(payload)
        payload_json = json.dumps(payload, separators=(",", ":"), sort_keys=False)
        message = f"{timestamp}:{payload_json}"

        # Verify signature (strip 0x prefix if present)
        sig_hex = signature[2:] if signature.startswith("0x") else signature
        message_hash = encode_defunct(text=message)
        recovered = Account.recover_message(message_hash, signature=bytes.fromhex(sig_hex))

        return recovered.lower() == node_id.lower()

    except Exception as e:
        print(f"Verification failed: {e}")
        return False


if __name__ == "__main__":
    # Test signing and verification
    from eth_account import Account as Acc

    # Generate a test key
    test_account = Acc.create()
    test_key = test_account.key.hex()
    print(f"Test Address: {test_account.address}")

    # Create a test payload
    test_payload = SignalPayload(
        token_pair="BTC/USDT",
        exchange="coingecko",
        direction="long",
        entry_price=65000.0,
        take_profit=68000.0,
        stop_loss=63000.0,
        expiry_time=int(time.time()) + 600,
        weight_pct=75,
    )

    # Sign it
    envelope = create_envelope(test_key, test_payload)
    print(f"\nEnvelope created:")
    print(json.dumps(envelope, indent=2))

    # Verify it
    valid = verify_envelope(envelope)
    print(f"\nSignature valid: {valid}")
