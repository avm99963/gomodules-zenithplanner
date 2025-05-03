package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GetSyncState retrieves a value from the sync_state table.
func (r *Repository) GetSyncState(ctx context.Context, key string) (string, error) {
	var value string
	query := "SELECT value FROM sync_state WHERE key = $1"
	err := r.pool.QueryRow(ctx, query, key).Scan(&value)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("sync state key '%s' not found", key)
		}
		return "", fmt.Errorf("failed to get sync state for key '%s': %w", key, err)
	}
	return value, nil
}

// SetSyncState inserts or updates a value in the sync_state table.
func (r *Repository) SetSyncState(ctx context.Context, key, value string) error {
	query := `
        INSERT INTO sync_state (key, value)
        VALUES ($1, $2)
        ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
    `
	_, err := r.pool.Exec(ctx, query, key, value)
	if err != nil {
		return fmt.Errorf("failed to set sync state for key '%s': %w", key, err)
	}
	return nil
}
