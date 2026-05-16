package api

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ─── V2 API Handlers: Off-Chain Subscription System ──────────────────────────

// Subscription pricing tiers (points per month)
const (
	PriceBronze  = 10_000  // 10k points/month
	PriceSilver  = 25_000  // 25k points/month
	PriceGold    = 50_000  // 50k points/month
	PriceDiamond = 100_000 // 100k points/month
)

// SubscribeRequest is the request body for POST /api/v2/subscribe
type SubscribeRequest struct {
	ProviderAddress string `json:"provider_address"`
	Tier            string `json:"tier"` // BRONZE, SILVER, GOLD, DIAMOND
}

// UnsubscribeRequest is the request body for DELETE /api/v2/subscriptions/{provider}
// (provider is in URL path)

// WithdrawRequest is the request body for POST /api/v2/providers/withdraw
type WithdrawRequest struct {
	ProviderAddress string `json:"provider_address"`
	Amount          string `json:"amount"`      // Amount in 0G (e.g., "10.5")
	Signature       string `json:"signature"`   // EIP-191 signature
	Timestamp       string `json:"timestamp"`   // Request timestamp
}

// ─── User Endpoints ───────────────────────────────────────────────────────────

// handleV2Subscribe creates an off-chain subscription to a provider.
// POST /api/v2/subscribe
func (s *Server) handleV2Subscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract API key from Authorization header
	userID, err := s.extractAPIKey(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}
	defer r.Body.Close()

	// Validate inputs
	if req.ProviderAddress == "" || req.Tier == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing provider_address or tier"})
		return
	}

	// Normalize tier to uppercase
	tier := strings.ToUpper(req.Tier)

	// Get tier price
	pointsPerMonth, err := getTierPrice(tier)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Check if user has enough points
	balance, err := s.db.GetUserPointBalance(r.Context(), userID)
	if err != nil {
		log.Printf("Failed to get user balance: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get balance"})
		return
	}

	if balance < pointsPerMonth {
		writeJSON(w, http.StatusPaymentRequired, map[string]interface{}{
			"error":            "insufficient_points",
			"balance":          balance,
			"required":         pointsPerMonth,
			"shortfall":        pointsPerMonth - balance,
		})
		return
	}

	// Create subscription (deducts points atomically)
	if err := s.db.CreateSubscription(r.Context(), userID, req.ProviderAddress, tier, pointsPerMonth); err != nil {
		log.Printf("Failed to create subscription: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create subscription"})
		return
	}

	log.Printf("User %s subscribed to provider %s (tier: %s, cost: %d points)", userID, req.ProviderAddress, tier, pointsPerMonth)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Subscription created",
		"tier":    tier,
		"cost":    pointsPerMonth,
		"provider": req.ProviderAddress,
	})
}

// handleV2Subscriptions lists all active subscriptions for the authenticated user.
// GET /api/v2/subscriptions
func (s *Server) handleV2Subscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract API key from Authorization header
	userID, err := s.extractAPIKey(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	// Get all active subscriptions
	subs, err := s.db.GetUserSubscriptions(r.Context(), userID)
	if err != nil {
		log.Printf("Failed to get user subscriptions: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get subscriptions"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"subscriptions": subs,
		"count":         len(subs),
	})
}

// handleV2Unsubscribe cancels a subscription to a provider.
// DELETE /api/v2/subscriptions/{provider}
func (s *Server) handleV2Unsubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract API key from Authorization header
	userID, err := s.extractAPIKey(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	// Extract provider address from URL path
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v2/subscriptions/"), "/")
	providerAddress := parts[0]
	if providerAddress == "" {
		http.Error(w, "missing provider address", http.StatusBadRequest)
		return
	}

	// Delete subscription
	if err := s.db.DeleteSubscription(r.Context(), userID, providerAddress); err != nil {
		log.Printf("Failed to delete subscription: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete subscription"})
		return
	}

	log.Printf("User %s unsubscribed from provider %s", userID, providerAddress)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Subscription cancelled",
		"provider": providerAddress,
	})
}

// handleV2Balance returns the user's point balance.
// GET /api/v2/balance
func (s *Server) handleV2Balance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract API key from Authorization header
	userID, err := s.extractAPIKey(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	// Get user point balance
	balance, err := s.db.GetUserPointBalance(r.Context(), userID)
	if err != nil {
		log.Printf("Failed to get user balance: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get balance"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": userID,
		"balance": balance,
	})
}

// handleV2DepositInfo returns vault deposit information.
// GET /api/v2/deposit-info
func (s *Server) handleV2DepositInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Note: Vault address should be obtained from config
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"conversion_rate":  "1000", // 1 0G = 1000 points
		"deposit_instructions": "Send 0G to the vault address with your userId in the transaction data",
		"note": "Contact admin for vault address",
	})
}

// ─── Provider Endpoints ───────────────────────────────────────────────────────

// handleV2ProviderBalance returns the provider's point balance.
// GET /api/v2/providers/balance?provider_address=0x...&signature=0x...&timestamp=...
func (s *Server) handleV2ProviderBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract and verify provider signature
	providerAddress, err := s.verifyProviderSignature(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	// Get provider point balance
	balance, err := s.db.GetProviderPointBalance(r.Context(), providerAddress)
	if err != nil {
		log.Printf("Failed to get provider balance: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get balance"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider_address": providerAddress,
		"points_balance":   balance,
		"og_equivalent":    float64(balance) / 1000.0, // Convert to 0G
	})
}

// handleV2ProviderWithdraw processes a provider withdrawal request.
// POST /api/v2/providers/withdraw
func (s *Server) handleV2ProviderWithdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}
	defer r.Body.Close()

	// Validate inputs
	if req.ProviderAddress == "" || req.Amount == "" || req.Signature == "" || req.Timestamp == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	// Verify EIP-191 signature
	message := fmt.Sprintf("Withdraw %s 0G from %s at %s", req.Amount, req.ProviderAddress, req.Timestamp)
	recoveredAddress, err := recoverAddress(message, req.Signature)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
		return
	}

	if !strings.EqualFold(recoveredAddress, req.ProviderAddress) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "signature does not match provider address"})
		return
	}

	// Parse amount in 0G
	amountOG, ok := new(big.Float).SetString(req.Amount)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount format"})
		return
	}

	// Convert to wei
	oneEther := new(big.Float).SetInt(big.NewInt(1e18))
	amountWeiFloat := new(big.Float).Mul(amountOG, oneEther)
	amountWei, _ := amountWeiFloat.Int(nil)

	// Calculate points to redeem
	pointsFloat := new(big.Float).Mul(amountOG, big.NewFloat(1000.0))
	pointsToRedeem, _ := pointsFloat.Int64()

	// Check provider balance
	providerBalance, err := s.db.GetProviderPointBalance(r.Context(), req.ProviderAddress)
	if err != nil {
		log.Printf("Failed to get provider balance: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get balance"})
		return
	}

	if providerBalance < pointsToRedeem {
		writeJSON(w, http.StatusPaymentRequired, map[string]interface{}{
			"error":     "insufficient_points",
			"balance":   providerBalance,
			"required":  pointsToRedeem,
		})
		return
	}

	// Get auto threshold from contract
	autoThreshold, err := s.vaultContract.GetAutoThreshold(r.Context())
	if err != nil {
		log.Printf("Failed to get auto threshold: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get threshold"})
		return
	}

	// Debit provider points
	if err := s.db.DebitProviderPoints(r.Context(), req.ProviderAddress, pointsToRedeem); err != nil {
		log.Printf("Failed to debit provider points: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to debit points"})
		return
	}

	providerAddr := common.HexToAddress(req.ProviderAddress)

	// Execute withdrawal based on amount
	if amountWei.Cmp(autoThreshold) <= 0 {
		// Immediate withdrawal (amount <= autoThreshold)
		txHash, err := s.vaultContract.Withdraw(r.Context(), providerAddr, amountWei, big.NewInt(pointsToRedeem))
		if err != nil {
			log.Printf("Withdrawal failed: %v", err)
			// Refund points on failure
			_ = s.db.CreditProviderPoints(r.Context(), req.ProviderAddress, pointsToRedeem, "withdrawal_refund_"+txHash)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "withdrawal failed: " + err.Error()})
			return
		}

		// Record withdrawal
		if err := s.db.RecordWithdrawalRequest(r.Context(), req.ProviderAddress, pointsToRedeem, amountOG, "COMPLETED", 0, txHash); err != nil {
			log.Printf("Failed to record withdrawal: %v", err)
		}

		log.Printf("Provider %s withdrew %s 0G (immediate, tx=%s)", req.ProviderAddress, req.Amount, txHash)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":         "completed",
			"message":        "Withdrawal completed immediately",
			"tx_hash":        txHash,
			"amount_og":      req.Amount,
			"points_redeemed": pointsToRedeem,
		})
	} else {
		// Queued withdrawal (amount > autoThreshold)
		withdrawalID, txHash, err := s.vaultContract.QueueWithdrawal(r.Context(), providerAddr, amountWei, big.NewInt(pointsToRedeem))
		if err != nil {
			log.Printf("Queue withdrawal failed: %v", err)
			// Refund points on failure
			_ = s.db.CreditProviderPoints(r.Context(), req.ProviderAddress, pointsToRedeem, "queue_refund_"+txHash)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "queue withdrawal failed: " + err.Error()})
			return
		}

		// Record queued withdrawal
		if err := s.db.RecordWithdrawalRequest(r.Context(), req.ProviderAddress, pointsToRedeem, amountOG, "QUEUED", withdrawalID, txHash); err != nil {
			log.Printf("Failed to record withdrawal: %v", err)
		}

		log.Printf("Provider %s queued withdrawal of %s 0G (id=%d, tx=%s)", req.ProviderAddress, req.Amount, withdrawalID, txHash)

		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"status":          "queued",
			"message":         "Withdrawal queued for timelock (24 hours)",
			"withdrawal_id":   withdrawalID,
			"tx_hash":         txHash,
			"amount_og":       req.Amount,
			"points_redeemed": pointsToRedeem,
			"execute_after":   time.Now().Add(24 * time.Hour).Unix(),
		})
	}
}

// ─── Solvency Verification ────────────────────────────────────────────────────

// handleV2SolvencyVerify returns the Merkle proof for a given address.
// GET /api/v2/solvency/verify?address=0x...
func (s *Server) handleV2SolvencyVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "address query param required", http.StatusBadRequest)
		return
	}

	// TODO: Implement Merkle proof retrieval from solvency system
	// For now, return placeholder
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error":   "not_implemented",
		"message": "Solvency verification will be available after Phase 6 implementation",
	})
}

// ─── Helper Functions ─────────────────────────────────────────────────────────

// extractAPIKey extracts and validates the API key from the Authorization header.
// Returns the userID associated with the API key.
func (s *Server) extractAPIKey(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("missing Authorization header")
	}

	// Expect "Bearer <api_key>"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("invalid Authorization header format")
	}

	apiKey := parts[1]

	// Validate API key against database
	userID, err := s.db.GetUserIDByAPIKey(r.Context(), apiKey)
	if err != nil {
		return "", fmt.Errorf("invalid API key")
	}

	return userID, nil
}

// verifyProviderSignature verifies the EIP-191 signature from query parameters.
// Returns the provider address if valid.
func (s *Server) verifyProviderSignature(r *http.Request) (string, error) {
	providerAddress := r.URL.Query().Get("provider_address")
	signature := r.URL.Query().Get("signature")
	timestamp := r.URL.Query().Get("timestamp")

	if providerAddress == "" || signature == "" || timestamp == "" {
		return "", fmt.Errorf("missing provider_address, signature, or timestamp")
	}

	// Construct message
	message := fmt.Sprintf("Get balance for %s at %s", providerAddress, timestamp)

	// Recover address from signature
	recoveredAddress, err := recoverAddress(message, signature)
	if err != nil {
		return "", fmt.Errorf("invalid signature: %w", err)
	}

	if !strings.EqualFold(recoveredAddress, providerAddress) {
		return "", fmt.Errorf("signature does not match provider address")
	}

	return providerAddress, nil
}

// recoverAddress recovers the Ethereum address from an EIP-191 signature.
func recoverAddress(message, signatureHex string) (string, error) {
	// EIP-191 personal_sign message format
	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := crypto.Keccak256Hash([]byte(prefixedMessage))

	// Decode signature
	signature := common.FromHex(signatureHex)
	if len(signature) != 65 {
		return "", fmt.Errorf("invalid signature length")
	}

	// Adjust v value (EIP-155)
	if signature[64] >= 27 {
		signature[64] -= 27
	}

	// Recover public key
	pubKey, err := crypto.SigToPub(hash.Bytes(), signature)
	if err != nil {
		return "", fmt.Errorf("failed to recover public key: %w", err)
	}

	// Derive address
	address := crypto.PubkeyToAddress(*pubKey)
	return address.Hex(), nil
}

// getTierPrice returns the points per month for a given tier.
func getTierPrice(tier string) (int64, error) {
	switch tier {
	case "BRONZE":
		return PriceBronze, nil
	case "SILVER":
		return PriceSilver, nil
	case "GOLD":
		return PriceGold, nil
	case "DIAMOND":
		return PriceDiamond, nil
	default:
		return 0, fmt.Errorf("invalid tier: must be BRONZE, SILVER, GOLD, or DIAMOND")
	}
}
