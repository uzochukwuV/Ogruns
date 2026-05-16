package database

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ─── User Methods ─────────────────────────────────────────────────────────────

// CreditUserPoints atomically credits points to a user and logs the transaction
func (db *DB) CreditUserPoints(ctx context.Context, userID string, points int64, txHash string, amountOG *big.Float) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update user balance
	_, err = tx.Exec(ctx, `
		UPDATE users
		SET points_balance = points_balance + $1,
		    points_total = points_total + $1
		WHERE user_id = $2
	`, points, userID)
	if err != nil {
		return fmt.Errorf("failed to update user balance: %w", err)
	}

	// Insert transaction log
	amount0gStr := amountOG.Text('f', 18)
	_, err = tx.Exec(ctx, `
		INSERT INTO point_transactions
		(user_id, transaction_type, points, tx_hash, amount_0g, description)
		VALUES ($1, 'DEPOSIT', $2, $3, $4, 'Deposit from vault')
	`, userID, points, txHash, amount0gStr)
	if err != nil {
		return fmt.Errorf("failed to log transaction: %w", err)
	}

	return tx.Commit(ctx)
}

// DebitUserPoints deducts points from a user (with balance check)
func (db *DB) DebitUserPoints(ctx context.Context, userID string, points int64, reason string) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check balance
	var balance int64
	err = tx.QueryRow(ctx, `SELECT points_balance FROM users WHERE user_id = $1`, userID).Scan(&balance)
	if err != nil {
		return fmt.Errorf("failed to get user balance: %w", err)
	}

	if balance < points {
		return fmt.Errorf("insufficient points: have %d, need %d", balance, points)
	}

	// Deduct points
	_, err = tx.Exec(ctx, `
		UPDATE users
		SET points_balance = points_balance - $1,
		    points_spent = points_spent + $1
		WHERE user_id = $2
	`, points, userID)
	if err != nil {
		return fmt.Errorf("failed to debit points: %w", err)
	}

	// Log transaction
	_, err = tx.Exec(ctx, `
		INSERT INTO point_transactions
		(user_id, transaction_type, points, description)
		VALUES ($1, 'SPEND', $2, $3)
	`, userID, points, reason)
	if err != nil {
		return fmt.Errorf("failed to log transaction: %w", err)
	}

	return tx.Commit(ctx)
}

// GetUserPointBalance returns a user's current point balance
func (db *DB) GetUserPointBalance(ctx context.Context, userID string) (int64, error) {
	var balance int64
	err := db.pool.QueryRow(ctx, `
		SELECT points_balance FROM users WHERE user_id = $1
	`, userID).Scan(&balance)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("user not found")
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}
	return balance, nil
}

// GetUserByAPIKey looks up a user by their API key
func (db *DB) GetUserByAPIKey(ctx context.Context, apiKey string) (string, error) {
	var userID string
	err := db.pool.QueryRow(ctx, `
		SELECT user_id FROM users WHERE api_key = $1
	`, apiKey).Scan(&userID)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("invalid API key")
	}
	if err != nil {
		return "", fmt.Errorf("failed to lookup user: %w", err)
	}
	return userID, nil
}

// ─── Subscription Methods ─────────────────────────────────────────────────────

// CreateSubscription creates or renews a subscription (deducts points)
func (db *DB) CreateSubscription(ctx context.Context, userID, providerAddress, tier string, pointsPerMonth int64) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Deduct points from user
	var balance int64
	err = tx.QueryRow(ctx, `SELECT points_balance FROM users WHERE user_id = $1`, userID).Scan(&balance)
	if err != nil {
		return fmt.Errorf("failed to get user balance: %w", err)
	}

	if balance < pointsPerMonth {
		return fmt.Errorf("insufficient points: have %d, need %d", balance, pointsPerMonth)
	}

	_, err = tx.Exec(ctx, `
		UPDATE users
		SET points_balance = points_balance - $1,
		    points_spent = points_spent + $1
		WHERE user_id = $2
	`, pointsPerMonth, userID)
	if err != nil {
		return fmt.Errorf("failed to debit points: %w", err)
	}

	// Insert or update subscription
	_, err = tx.Exec(ctx, `
		INSERT INTO user_subscriptions (user_id, provider_address, tier, points_per_month, start_time, end_time)
		VALUES ($1, $2, $3, $4, NOW(), NOW() + INTERVAL '30 days')
		ON CONFLICT (user_id, provider_address)
		DO UPDATE SET
			tier = EXCLUDED.tier,
			points_per_month = EXCLUDED.points_per_month,
			end_time = NOW() + INTERVAL '30 days',
			start_time = NOW()
	`, userID, strings.ToLower(providerAddress), tier, pointsPerMonth)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	// Log transaction
	_, err = tx.Exec(ctx, `
		INSERT INTO point_transactions
		(user_id, transaction_type, points, description)
		VALUES ($1, 'SPEND', $2, $3)
	`, userID, pointsPerMonth, fmt.Sprintf("Subscription to %s (%s tier)", providerAddress[:10], tier))
	if err != nil {
		return fmt.Errorf("failed to log transaction: %w", err)
	}

	return tx.Commit(ctx)
}

// IsSubscribedOffChain checks if a user is subscribed to a provider
func (db *DB) IsSubscribedOffChain(ctx context.Context, userID, providerAddress string) (bool, error) {
	var subscribed bool
	err := db.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_subscriptions
			WHERE user_id = $1 AND provider_address = $2 AND end_time > NOW()
		)
	`, userID, strings.ToLower(providerAddress)).Scan(&subscribed)
	return subscribed, err
}

// GetAllActiveProviderAddresses returns all provider addresses a user is subscribed to
func (db *DB) GetAllActiveProviderAddresses(ctx context.Context, userID string) ([]string, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT provider_address FROM user_subscriptions
		WHERE user_id = $1 AND end_time > NOW()
		ORDER BY provider_address
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions: %w", err)
	}
	defer rows.Close()

	var providers []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, fmt.Errorf("failed to scan provider: %w", err)
		}
		providers = append(providers, provider)
	}

	return providers, rows.Err()
}

// DeleteSubscription removes a user's subscription to a provider
func (db *DB) DeleteSubscription(ctx context.Context, userID, providerAddress string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE user_subscriptions
		SET end_time = NOW()
		WHERE user_id = $1 AND provider_address = $2
	`, userID, strings.ToLower(providerAddress))
	return err
}

// ─── Provider Methods ─────────────────────────────────────────────────────────

// CreditProviderPoints credits points to a provider for a signal
func (db *DB) CreditProviderPoints(ctx context.Context, providerAddress string, points int64, signalID string) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	provider := strings.ToLower(providerAddress)

	// Upsert provider record
	_, err = tx.Exec(ctx, `
		INSERT INTO provider_points (provider_address, points_balance, points_earned_total)
		VALUES ($1, $2, $2)
		ON CONFLICT (provider_address)
		DO UPDATE SET
			points_balance = provider_points.points_balance + EXCLUDED.points_balance,
			points_earned_total = provider_points.points_earned_total + EXCLUDED.points_earned_total
	`, provider, points)
	if err != nil {
		return fmt.Errorf("failed to credit provider: %w", err)
	}

	// Log transaction
	_, err = tx.Exec(ctx, `
		INSERT INTO point_transactions
		(provider_address, transaction_type, points, signal_id, description)
		VALUES ($1, 'EARN', $2, $3, 'Signal performance reward')
	`, provider, points, signalID)
	if err != nil {
		return fmt.Errorf("failed to log transaction: %w", err)
	}

	return tx.Commit(ctx)
}

// DebitProviderPoints deducts points from a provider (for withdrawal)
func (db *DB) DebitProviderPoints(ctx context.Context, providerAddress string, points int64) error {
	provider := strings.ToLower(providerAddress)

	// Check balance
	var balance int64
	err := db.pool.QueryRow(ctx, `
		SELECT points_balance FROM provider_points WHERE provider_address = $1
	`, provider).Scan(&balance)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("provider not found")
	}
	if err != nil {
		return fmt.Errorf("failed to get provider balance: %w", err)
	}

	if balance < points {
		return fmt.Errorf("insufficient points: have %d, need %d", balance, points)
	}

	// Deduct points
	_, err = db.pool.Exec(ctx, `
		UPDATE provider_points
		SET points_balance = points_balance - $1,
		    points_withdrawn_total = points_withdrawn_total + $1,
		    last_withdrawal_at = NOW()
		WHERE provider_address = $2
	`, points, provider)
	return err
}

// GetProviderPointBalance returns a provider's current point balance
func (db *DB) GetProviderPointBalance(ctx context.Context, providerAddress string) (int64, error) {
	var balance int64
	err := db.pool.QueryRow(ctx, `
		SELECT COALESCE(points_balance, 0) FROM provider_points
		WHERE provider_address = $1
	`, strings.ToLower(providerAddress)).Scan(&balance)
	if err == pgx.ErrNoRows {
		return 0, nil // Provider has no points yet
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}
	return balance, nil
}

// ─── Point Balance Methods (for Solvency) ────────────────────────────────────

// GetAllPointBalances returns all user and provider point balances (for Merkle tree)
func (db *DB) GetAllPointBalances(ctx context.Context) (map[string]int64, error) {
	balances := make(map[string]int64)

	// Get user balances
	userRows, err := db.pool.Query(ctx, `
		SELECT wallet_address, points_balance FROM users WHERE points_balance > 0
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query user balances: %w", err)
	}
	defer userRows.Close()

	for userRows.Next() {
		var address string
		var balance int64
		if err := userRows.Scan(&address, &balance); err != nil {
			return nil, fmt.Errorf("failed to scan user balance: %w", err)
		}
		balances[strings.ToLower(address)] = balance
	}

	// Get provider balances
	providerRows, err := db.pool.Query(ctx, `
		SELECT provider_address, points_balance FROM provider_points WHERE points_balance > 0
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query provider balances: %w", err)
	}
	defer providerRows.Close()

	for providerRows.Next() {
		var address string
		var balance int64
		if err := providerRows.Scan(&address, &balance); err != nil {
			return nil, fmt.Errorf("failed to scan provider balance: %w", err)
		}
		balances[strings.ToLower(address)] = balance
	}

	return balances, nil
}

// SumAllPoints returns total user + provider points
func (db *DB) SumAllPoints(ctx context.Context) (int64, error) {
	var userTotal, providerTotal int64

	err := db.pool.QueryRow(ctx, `SELECT COALESCE(SUM(points_balance), 0) FROM users`).Scan(&userTotal)
	if err != nil {
		return 0, fmt.Errorf("failed to sum user points: %w", err)
	}

	err = db.pool.QueryRow(ctx, `SELECT COALESCE(SUM(points_balance), 0) FROM provider_points`).Scan(&providerTotal)
	if err != nil {
		return 0, fmt.Errorf("failed to sum provider points: %w", err)
	}

	return userTotal + providerTotal, nil
}

// ─── Deposit Event Methods ───────────────────────────────────────────────────

// IsDepositProcessed checks if a deposit event has already been processed
func (db *DB) IsDepositProcessed(ctx context.Context, txHash string) (bool, error) {
	var exists bool
	err := db.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM deposit_events WHERE tx_hash = $1)
	`, txHash).Scan(&exists)
	return exists, err
}

// RecordDepositEvent records a processed deposit event (idempotency)
func (db *DB) RecordDepositEvent(ctx context.Context, txHash string, blockNumber uint64, logIndex int, userAddress, userID string, amountWei *big.Int, pointsCredited int64) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO deposit_events
		(tx_hash, block_number, log_index, user_address, user_id, amount_wei, points_credited)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tx_hash, log_index) DO NOTHING
	`, txHash, blockNumber, logIndex, userAddress, userID, amountWei.String(), pointsCredited)
	return err
}

// GetLastProcessedBlock returns the last processed block number
func (db *DB) GetLastProcessedBlock(ctx context.Context) (uint64, error) {
	var blockNumber uint64
	err := db.pool.QueryRow(ctx, `
		SELECT last_processed_block FROM deposit_listener_state WHERE id = 1
	`).Scan(&blockNumber)
	return blockNumber, err
}

// UpdateLastProcessedBlock updates the last processed block number
func (db *DB) UpdateLastProcessedBlock(ctx context.Context, blockNumber uint64) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE deposit_listener_state SET last_processed_block = $1 WHERE id = 1
	`, blockNumber)
	return err
}

// ─── Withdrawal Request Methods ───────────────────────────────────────────────

// CreateWithdrawalRequest creates a new withdrawal request
func (db *DB) CreateWithdrawalRequest(ctx context.Context, providerAddress string, pointsAmount int64, ogAmount *big.Float) (int64, error) {
	var id int64
	err := db.pool.QueryRow(ctx, `
		INSERT INTO withdrawal_requests (provider_address, points_amount, og_amount, status)
		VALUES ($1, $2, $3, 'PENDING')
		RETURNING id
	`, strings.ToLower(providerAddress), pointsAmount, ogAmount.Text('f', 18)).Scan(&id)
	return id, err
}

// UpdateWithdrawalStatus updates a withdrawal request status
func (db *DB) UpdateWithdrawalStatus(ctx context.Context, id int64, status string, txHash string, onChainID *int64, errorMsg string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE withdrawal_requests
		SET status = $1,
		    tx_hash = $2,
		    on_chain_id = $3,
		    error_message = $4,
		    executed_at = CASE WHEN $1 = 'EXECUTED' THEN NOW() ELSE executed_at END
		WHERE id = $5
	`, status, txHash, onChainID, errorMsg, id)
	return err
}

// ─── Solvency Snapshot Methods ────────────────────────────────────────────────

// RecordSolvencySnapshot records a solvency proof snapshot
func (db *DB) RecordSolvencySnapshot(ctx context.Context, merkleRoot string, totalUserPoints, totalProviderPoints int64, totalOutstandingOG, vaultBalanceOG *big.Float, isSolvent bool, publishedTxHash, storageRootHash string) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO solvency_snapshots
		(merkle_root, total_user_points, total_provider_points, total_outstanding_og, vault_balance_og, is_solvent, published_tx_hash, storage_root_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, merkleRoot, totalUserPoints, totalProviderPoints,
		totalOutstandingOG.Text('f', 18), vaultBalanceOG.Text('f', 18),
		isSolvent, publishedTxHash, storageRootHash)
	return err
}

// GetLatestSolvencySnapshot returns the most recent solvency snapshot
func (db *DB) GetLatestSolvencySnapshot(ctx context.Context) (*SolvencySnapshot, error) {
	var snapshot SolvencySnapshot
	var totalOutstandingOGStr, vaultBalanceOGStr string

	err := db.pool.QueryRow(ctx, `
		SELECT id, merkle_root, total_user_points, total_provider_points,
		       total_outstanding_og, vault_balance_og, is_solvent, created_at
		FROM solvency_snapshots
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&snapshot.ID, &snapshot.MerkleRoot, &snapshot.TotalUserPoints, &snapshot.TotalProviderPoints,
		&totalOutstandingOGStr, &vaultBalanceOGStr, &snapshot.IsSolvent, &snapshot.CreatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse big floats
	snapshot.TotalOutstandingOG = new(big.Float)
	snapshot.TotalOutstandingOG.SetString(totalOutstandingOGStr)
	snapshot.VaultBalanceOG = new(big.Float)
	snapshot.VaultBalanceOG.SetString(vaultBalanceOGStr)

	return &snapshot, nil
}

// ─── API Key Authentication ───────────────────────────────────────────────────

// GetUserIDByAPIKey retrieves the userID associated with an API key
func (db *DB) GetUserIDByAPIKey(ctx context.Context, apiKey string) (string, error) {
	var userID string
	err := db.pool.QueryRow(ctx, `
		SELECT user_id
		FROM users
		WHERE api_key = $1
	`, apiKey).Scan(&userID)

	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("invalid API key")
	}
	if err != nil {
		return "", fmt.Errorf("failed to get user ID: %w", err)
	}

	return userID, nil
}

// ─── Subscription Queries ─────────────────────────────────────────────────────

// Subscriber represents a user subscribed to a provider
type Subscriber struct {
	UserID string
	Tier   string
}

// GetUserSubscriptions returns all active subscriptions for a user
func (db *DB) GetUserSubscriptions(ctx context.Context, userID string) ([]interface{}, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT provider_address, tier, points_per_month, start_time, end_time
		FROM user_subscriptions
		WHERE user_id = $1 AND end_time > NOW()
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []interface{}
	for rows.Next() {
		var sub struct {
			ProviderAddress string    `json:"provider_address"`
			Tier            string    `json:"tier"`
			PointsPerMonth  int64     `json:"points_per_month"`
			StartTime       time.Time `json:"start_time"`
			EndTime         time.Time `json:"end_time"`
		}
		if err := rows.Scan(&sub.ProviderAddress, &sub.Tier, &sub.PointsPerMonth, &sub.StartTime, &sub.EndTime); err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}
		subscriptions = append(subscriptions, sub)
	}

	return subscriptions, rows.Err()
}

// CountSubscribers returns the number of active subscribers for a provider
func (db *DB) CountSubscribers(ctx context.Context, providerAddress string) (int, error) {
	var count int
	err := db.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM user_subscriptions
		WHERE provider_address = $1 AND end_time > NOW()
	`, providerAddress).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count subscribers: %w", err)
	}

	return count, nil
}

// GetSubscriberTier returns the subscription tier for a specific user-provider pair
func (db *DB) GetSubscriberTier(ctx context.Context, userID, providerAddress string) (string, error) {
	var tier string
	err := db.pool.QueryRow(ctx, `
		SELECT tier
		FROM user_subscriptions
		WHERE user_id = $1 AND provider_address = $2 AND end_time > NOW()
	`, userID, providerAddress).Scan(&tier)

	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("no active subscription found")
	}
	if err != nil {
		return "", fmt.Errorf("failed to get subscriber tier: %w", err)
	}

	return tier, nil
}

// GetAllSubscribersForProvider returns all active subscribers for a provider with their tiers
func (db *DB) GetAllSubscribersForProvider(ctx context.Context, providerAddress string) ([]struct{ UserID string; Tier string }, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT user_id, tier
		FROM user_subscriptions
		WHERE provider_address = $1 AND end_time > NOW()
	`, providerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscribers: %w", err)
	}
	defer rows.Close()

	var subscribers []struct{ UserID string; Tier string }
	for rows.Next() {
		var sub struct{ UserID string; Tier string }
		if err := rows.Scan(&sub.UserID, &sub.Tier); err != nil {
			return nil, fmt.Errorf("failed to scan subscriber: %w", err)
		}
		subscribers = append(subscribers, sub)
	}

	return subscribers, rows.Err()
}

// RecordWithdrawalRequest records a provider withdrawal request
func (db *DB) RecordWithdrawalRequest(ctx context.Context, providerAddress string, pointsAmount int64, ogAmount *big.Float, status string, onChainID uint64, txHash string) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO withdrawal_requests
		(provider_address, points_amount, og_amount, status, on_chain_id, tx_hash)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, providerAddress, pointsAmount, ogAmount.Text('f', 18), status, onChainID, txHash)

	if err != nil {
		return fmt.Errorf("failed to record withdrawal request: %w", err)
	}

	return nil
}

// ─── Types ────────────────────────────────────────────────────────────────────

type SolvencySnapshot struct {
	ID                  int64
	MerkleRoot          string
	TotalUserPoints     int64
	TotalProviderPoints int64
	TotalOutstandingOG  *big.Float
	VaultBalanceOG      *big.Float
	IsSolvent           bool
	CreatedAt           time.Time
}
