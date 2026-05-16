-- 001_initial.sql
-- Database schema for 0G Signal Intelligence Network v2 (Off-Chain Points System)

-- ─── Users ────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    user_id        VARCHAR(64) PRIMARY KEY,
    wallet_address VARCHAR(42) UNIQUE,
    api_key        VARCHAR(64) UNIQUE NOT NULL,
    api_key_hash   VARCHAR(128) NOT NULL,
    points_balance BIGINT DEFAULT 0 CHECK (points_balance >= 0),
    points_total   BIGINT DEFAULT 0,
    points_spent   BIGINT DEFAULT 0,
    created_at     TIMESTAMPTZ DEFAULT NOW(),
    updated_at     TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_wallet ON users(wallet_address);
CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key);

-- ─── User Subscriptions (Off-Chain) ──────────────────────────────────────────

CREATE TABLE IF NOT EXISTS user_subscriptions (
    id               SERIAL PRIMARY KEY,
    user_id          VARCHAR(64) REFERENCES users(user_id) ON DELETE CASCADE,
    provider_address VARCHAR(42) NOT NULL,
    tier             VARCHAR(10) NOT NULL DEFAULT 'BRONZE',
    points_per_month BIGINT NOT NULL,
    start_time       TIMESTAMPTZ DEFAULT NOW(),
    end_time         TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, provider_address)
);

CREATE INDEX IF NOT EXISTS idx_subs_user ON user_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subs_provider ON user_subscriptions(provider_address);
CREATE INDEX IF NOT EXISTS idx_subs_end_time ON user_subscriptions(end_time);

-- ─── Provider Points ──────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS provider_points (
    provider_address       VARCHAR(42) PRIMARY KEY,
    points_balance         BIGINT DEFAULT 0 CHECK (points_balance >= 0),
    points_earned_total    BIGINT DEFAULT 0,
    points_withdrawn_total BIGINT DEFAULT 0,
    last_withdrawal_at     TIMESTAMPTZ,
    created_at             TIMESTAMPTZ DEFAULT NOW(),
    updated_at             TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_provider_balance ON provider_points(points_balance) WHERE points_balance > 0;

-- ─── Point Transactions (Audit Log) ──────────────────────────────────────────

CREATE TABLE IF NOT EXISTS point_transactions (
    id               SERIAL PRIMARY KEY,
    user_id          VARCHAR(64),
    provider_address VARCHAR(42),
    transaction_type VARCHAR(20) NOT NULL CHECK (transaction_type IN ('DEPOSIT', 'SPEND', 'EARN', 'WITHDRAW')),
    points           BIGINT NOT NULL,
    tx_hash          VARCHAR(66) UNIQUE,
    amount_0g        NUMERIC(30, 18),
    signal_id        VARCHAR(128),
    description      TEXT,
    created_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tx_user ON point_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_tx_provider ON point_transactions(provider_address);
CREATE INDEX IF NOT EXISTS idx_tx_type ON point_transactions(transaction_type);
CREATE INDEX IF NOT EXISTS idx_tx_created ON point_transactions(created_at DESC);

-- ─── Withdrawal Requests ──────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS withdrawal_requests (
    id               SERIAL PRIMARY KEY,
    provider_address VARCHAR(42) NOT NULL,
    points_amount    BIGINT NOT NULL,
    og_amount        NUMERIC(30, 18) NOT NULL,
    status           VARCHAR(20) DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'QUEUED', 'EXECUTED', 'FAILED', 'CANCELLED')),
    on_chain_id      BIGINT,
    tx_hash          VARCHAR(66),
    error_message    TEXT,
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    executed_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_withdrawal_provider ON withdrawal_requests(provider_address);
CREATE INDEX IF NOT EXISTS idx_withdrawal_status ON withdrawal_requests(status);
CREATE INDEX IF NOT EXISTS idx_withdrawal_created ON withdrawal_requests(created_at DESC);

-- ─── Solvency Snapshots ───────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS solvency_snapshots (
    id                    SERIAL PRIMARY KEY,
    merkle_root           VARCHAR(66) NOT NULL,
    total_user_points     BIGINT NOT NULL,
    total_provider_points BIGINT NOT NULL,
    total_outstanding_og  NUMERIC(30, 18) NOT NULL,
    vault_balance_og      NUMERIC(30, 18) NOT NULL,
    is_solvent            BOOLEAN NOT NULL,
    published_tx_hash     VARCHAR(66),
    storage_root_hash     VARCHAR(66),
    created_at            TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_solvency_created ON solvency_snapshots(created_at DESC);

-- ─── Deposit Events (Idempotency) ────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS deposit_events (
    id              SERIAL PRIMARY KEY,
    tx_hash         VARCHAR(66) UNIQUE NOT NULL,
    block_number    BIGINT NOT NULL,
    log_index       INT NOT NULL,
    user_address    VARCHAR(42) NOT NULL,
    user_id         VARCHAR(64) NOT NULL,
    amount_wei      NUMERIC(30, 0) NOT NULL,
    points_credited BIGINT NOT NULL,
    processed_at    TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tx_hash, log_index)
);

CREATE INDEX IF NOT EXISTS idx_deposit_tx ON deposit_events(tx_hash);
CREATE INDEX IF NOT EXISTS idx_deposit_block ON deposit_events(block_number DESC);
CREATE INDEX IF NOT EXISTS idx_deposit_user ON deposit_events(user_id);

-- ─── Vault Balance Cache ──────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS vault_balance_cache (
    id         SERIAL PRIMARY KEY,
    balance    NUMERIC(30, 18) NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert initial cache row
INSERT INTO vault_balance_cache (id, balance) VALUES (1, 0)
ON CONFLICT (id) DO NOTHING;

-- ─── Last Processed Block ─────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS deposit_listener_state (
    id                  SERIAL PRIMARY KEY,
    last_processed_block BIGINT NOT NULL DEFAULT 0,
    updated_at          TIMESTAMPTZ DEFAULT NOW()
);

-- Insert initial state row
INSERT INTO deposit_listener_state (id, last_processed_block) VALUES (1, 0)
ON CONFLICT (id) DO NOTHING;

-- ─── Triggers ─────────────────────────────────────────────────────────────────

-- Update updated_at timestamp on row update
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_provider_points_updated_at
BEFORE UPDATE ON provider_points
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();
