package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

// CachedEvent represents a row in the calendar_event_cache table.
type CachedEvent struct {
	EventID              string     `db:"event_id"`
	Date                 time.Time  `db:"date"`
	Title                *string    `db:"title"`       // Use pointer for nullable text
	Description          *string    `db:"description"` // Added description field
	UpdatedTs            time.Time  `db:"updated_ts"`
	IsManagedProperty    bool       `db:"is_managed_property"`
	IsManagedDescription bool       `db:"is_managed_description"`
	ColorID              *string    `db:"color_id"`            // Use pointer for nullable text
	RecurringEventID     *string    `db:"recurring_event_id"`  // Use pointer for nullable text
	OriginalStartTime    *time.Time `db:"original_start_time"` // Use pointer for nullable timestamp
}

// UpsertCachedEvent inserts or updates an event in the cache.
func (r *Repository) UpsertCachedEvent(ctx context.Context, event CachedEvent) error {
	query := `
        INSERT INTO calendar_event_cache (
            event_id, date, title, description, updated_ts, is_managed_property,
            is_managed_description, color_id, recurring_event_id, original_start_time
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        ON CONFLICT (event_id) DO UPDATE SET
            date = EXCLUDED.date,
            title = EXCLUDED.title,
            description = EXCLUDED.description,
            updated_ts = EXCLUDED.updated_ts,
            is_managed_property = EXCLUDED.is_managed_property,
            is_managed_description = EXCLUDED.is_managed_description,
            color_id = EXCLUDED.color_id,
            recurring_event_id = EXCLUDED.recurring_event_id,
            original_start_time = EXCLUDED.original_start_time;
    `
	normalizedDate := normalizeDate(event.Date)

	_, err := r.pool.Exec(ctx, query,
		event.EventID, normalizedDate, event.Title, event.Description, event.UpdatedTs, event.IsManagedProperty,
		event.IsManagedDescription, event.ColorID, event.RecurringEventID, event.OriginalStartTime,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert cached event %s: %w", event.EventID, err)
	}
	return nil
}

// DeleteCachedEvent removes an event from the cache by its ID.
func (r *Repository) DeleteCachedEvent(ctx context.Context, eventID string) error {
	query := "DELETE FROM calendar_event_cache WHERE event_id = $1"
	cmdTag, err := r.pool.Exec(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("failed to delete cached event %s: %w", eventID, err)
	}
	if cmdTag.RowsAffected() == 0 {
		log.Printf("Info: No rows deleted from cache for event ID %s (might have been already deleted).", eventID)
	}
	return nil
}

// ClearCachedEvents removes all events from the cache (used before full sync rebuild).
func (r *Repository) ClearCachedEvents(ctx context.Context) error {
	query := "DELETE FROM calendar_event_cache"
	_, err := r.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to clear calendar event cache: %w", err)
	}
	return nil
}

// GetCachedEventsByDate retrieves all cached events for a specific date.
func (r *Repository) GetCachedEventsByDate(ctx context.Context, date time.Time) ([]CachedEvent, error) {
	events := []CachedEvent{}
	query := `
        SELECT event_id, date, title, description, updated_ts, is_managed_property,
               is_managed_description, color_id, recurring_event_id, original_start_time
        FROM calendar_event_cache
        WHERE date = $1
        ORDER BY updated_ts DESC -- Order by updated time might be useful
    `
	normalizedDate := normalizeDate(date)
	rows, err := r.pool.Query(ctx, query, normalizedDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query cached events for date %s: %w", date.Format("2006-01-02"), err)
	}
	defer rows.Close()

	for rows.Next() {
		var event CachedEvent
		err := rows.Scan(
			&event.EventID, &event.Date, &event.Title, &event.Description, &event.UpdatedTs, &event.IsManagedProperty,
			&event.IsManagedDescription, &event.ColorID, &event.RecurringEventID, &event.OriginalStartTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan cached event row: %w", err)
		}
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating cached event rows: %w", err)
	}

	return events, nil
}

// GetCachedEventByID retrieves a single cached event by its Google Calendar Event ID.
// Returns nil, nil if the event is not found in the cache.
func (r *Repository) GetCachedEventByID(ctx context.Context, eventID string) (*CachedEvent, error) {
	event := &CachedEvent{}
	query := `
        SELECT event_id, date, title, description, updated_ts, is_managed_property,
               is_managed_description, color_id, recurring_event_id, original_start_time
        FROM calendar_event_cache
        WHERE event_id = $1
    `
	err := r.pool.QueryRow(ctx, query, eventID).Scan(
		&event.EventID, &event.Date, &event.Title, &event.Description, &event.UpdatedTs, &event.IsManagedProperty,
		&event.IsManagedDescription, &event.ColorID, &event.RecurringEventID, &event.OriginalStartTime,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found is not an error in this context
		}
		return nil, fmt.Errorf("failed to get cached event by ID %s: %w", eventID, err)
	}
	return event, nil
}
