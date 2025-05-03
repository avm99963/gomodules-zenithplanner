package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ScheduleEntry represents a row in the schedule_entries table.
type ScheduleEntry struct {
	Date         time.Time `db:"date"`
	LocationCode string    `db:"location_code"`
	Status       string    `db:"status"`
}

// UpsertScheduleEntry inserts or updates a schedule entry.
func (r *Repository) UpsertScheduleEntry(ctx context.Context, entry ScheduleEntry) error {
	query := `
        INSERT INTO schedule_entries (date, location_code, status)
        VALUES ($1, $2, $3)
        ON CONFLICT (date) DO UPDATE SET
            location_code = EXCLUDED.location_code,
            status = EXCLUDED.status;
    `
	normalizedDate := normalizeDate(entry.Date)
	_, err := r.pool.Exec(ctx, query, normalizedDate, entry.LocationCode, entry.Status)
	if err != nil {
		return fmt.Errorf("failed to upsert schedule entry for date %s: %w", entry.Date.Format("2006-01-02"), err)
	}
	return nil
}

// GetScheduleEntry retrieves a schedule entry for a specific date.
func (r *Repository) GetScheduleEntry(ctx context.Context, date time.Time) (*ScheduleEntry, error) {
	entry := &ScheduleEntry{}
	query := "SELECT date, location_code, status FROM schedule_entries WHERE date = $1"
	normalizedDate := normalizeDate(date)
	err := r.pool.QueryRow(ctx, query, normalizedDate).Scan(&entry.Date, &entry.LocationCode, &entry.Status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Return nil, nil if not found is expected behavior
		}
		return nil, fmt.Errorf("failed to get schedule entry for date %s: %w", date.Format("2006-01-02"), err)
	}
	return entry, nil
}
