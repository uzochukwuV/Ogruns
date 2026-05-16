package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgx connection pool with helper methods
type DB struct {
	pool *pgxpool.Pool
}

// Config holds database connection configuration
type Config struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnIdleTime time.Duration
}

// DefaultConfig returns sensible defaults for the connection pool
func DefaultConfig(url string) *Config {
	return &Config{
		URL:             url,
		MaxConns:        20,
		MinConns:        5,
		MaxConnIdleTime: 5 * time.Minute,
	}
}

// New creates a new database connection pool
func New(ctx context.Context, cfg *Config) (*DB, error) {
	// Parse connection string
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure pool
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	// Create pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// Pool returns the underlying pgx connection pool
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// Health checks database connectivity
func (db *DB) Health(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// Stats returns connection pool statistics
func (db *DB) Stats() *pgxpool.Stat {
	return db.pool.Stat()
}
