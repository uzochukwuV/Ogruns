package database

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// RunMigrations executes all pending SQL migrations
func (db *DB) RunMigrations(ctx context.Context) error {
	// Create schema_migrations table if it doesn't exist
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version   VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Read all migration files
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migration files by name
	var migrationNames []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			migrationNames = append(migrationNames, entry.Name())
		}
	}
	sort.Strings(migrationNames)

	// Execute each migration if not already applied
	for _, name := range migrationNames {
		version := strings.TrimSuffix(name, ".sql")

		// Check if migration already applied
		var exists bool
		err := db.pool.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)
		`, version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", version, err)
		}

		if exists {
			fmt.Printf("Migration %s already applied, skipping\n", version)
			continue
		}

		// Read migration file
		content, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", name, err)
		}

		// Execute migration in a transaction
		tx, err := db.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", version, err)
		}

		// Execute migration SQL
		_, err = tx.Exec(ctx, string(content))
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to execute migration %s: %w", version, err)
		}

		// Record migration as applied
		_, err = tx.Exec(ctx, `
			INSERT INTO schema_migrations (version) VALUES ($1)
		`, version)
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", version, err)
		}

		fmt.Printf("✅ Applied migration: %s\n", version)
	}

	return nil
}

// RollbackLastMigration rolls back the most recently applied migration
func (db *DB) RollbackLastMigration(ctx context.Context) error {
	// Get last applied migration
	var version string
	err := db.pool.QueryRow(ctx, `
		SELECT version FROM schema_migrations
		ORDER BY applied_at DESC
		LIMIT 1
	`).Scan(&version)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("no migrations to rollback")
	}
	if err != nil {
		return fmt.Errorf("failed to get last migration: %w", err)
	}

	// Remove from schema_migrations
	_, err = db.pool.Exec(ctx, `
		DELETE FROM schema_migrations WHERE version = $1
	`, version)
	if err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	fmt.Printf("⚠️  Rolled back migration: %s (manual cleanup may be required)\n", version)
	return nil
}

// GetAppliedMigrations returns a list of all applied migrations
func (db *DB) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT version FROM schema_migrations ORDER BY version
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	var migrations []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration: %w", err)
		}
		migrations = append(migrations, version)
	}

	return migrations, rows.Err()
}
