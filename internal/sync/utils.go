package sync

import (
	"time"
)

// generateDateRange creates a slice of dates between start and end (inclusive).
func generateDateRange(start, end time.Time) []time.Time {
	var dates []time.Time
	current := start
	// Normalize to start of day UTC for comparison
	endNormalized := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	for !current.After(endNormalized) {
		dates = append(dates, time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC))
		current = current.AddDate(0, 0, 1)
	}
	return dates
}
