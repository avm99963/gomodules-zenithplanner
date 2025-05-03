package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

// RunFullSync performs a full synchronization: fetches all events, rebuilds cache,
// stores new sync token, and triggers reconciliation.
func (s *Syncer) RunFullSync(ctx context.Context) error {
	log.Println("Starting full sync...")

	log.Println("Fetching all events from Google Calendar...")
	allEvents, nextSyncToken, err := s.fetchAllEvents(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch all calendar events: %w", err)
	}
	log.Printf("Fetched %d total events/instances from calendar.", len(allEvents))

	log.Println("Rebuilding local event cache...")
	err = s.rebuildEventCache(ctx, allEvents)
	if err != nil {
		return fmt.Errorf("failed to rebuild event cache: %w", err)
	}
	log.Println("Event cache rebuilt successfully.")

	if nextSyncToken == "" {
		log.Println("Warning: No sync token received from full sync fetch.")
	}
	log.Println("Persisting new sync token...")
	err = s.dbRepo.SetSyncState(ctx, "syncToken", nextSyncToken)
	if err != nil {
		return fmt.Errorf("failed to persist sync token: %w", err)
	}
	log.Println("Sync token persisted.")

	log.Println("Triggering reconciliation process for full sync window and ...")
	datesToReconcile := s.getDatesToConciliateWithDefaultWindow(ctx, allEvents)

	err = s.RunReconciliation(ctx, datesToReconcile, false) // Pass false for userTriggeredChange
	if err != nil {
		// Log reconciliation error but don't necessarily fail the whole sync?
		log.Printf("Error during post-full-sync reconciliation: %v", err)
	} else {
		log.Println("Reconciliation process completed.")
	}

	log.Println("Full sync completed successfully.")
	return nil
}

// fetchAllEvents retrieves all events from the configured calendar.
func (s *Syncer) fetchAllEvents(ctx context.Context) ([]*gcal.Event, string, error) {
	var allEvents []*gcal.Event
	var pageToken string
	var nextSyncToken string

	for {
		call := s.calendarService.Events.List(s.cfg.Google.CalendarID).
			PageToken(pageToken).
			SingleEvents(true).
			Fields(googleapi.Field("items(id,summary,description,start,end,updated,colorId,recurringEventId,originalStartTime,extendedProperties,status),nextPageToken,nextSyncToken"))

		resp, err := call.Do()
		if err != nil {
			return nil, "", fmt.Errorf("failed to list calendar events (page token: %s): %w", pageToken, err)
		}

		allEvents = append(allEvents, resp.Items...)
		nextSyncToken = resp.NextSyncToken

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
		log.Printf("Fetched page, continuing pagination...")
	}

	return allEvents, nextSyncToken, nil
}

// rebuildEventCache clears the existing cache and populates it with the fetched events.
func (s *Syncer) rebuildEventCache(ctx context.Context, events []*gcal.Event) error {
	log.Println("Clearing existing calendar event cache...")
	if err := s.dbRepo.ClearCachedEvents(ctx); err != nil {
		return fmt.Errorf("failed to clear cache before rebuild: %w", err)
	}
	log.Println("Cache cleared.")

	s.updateDBCache(ctx, events)
	return nil
}

func (s *Syncer) getDatesToConciliateWithDefaultWindow(ctx context.Context, allEvents []*gcal.Event) []time.Time {
	datesMap := make(map[string]struct{})

	windowStartDate := time.Now().AddDate(0, 0, -s.cfg.App.PastSyncWindowDays)
	windowEndDate := time.Now().AddDate(0, 0, s.cfg.App.FutureHorizonDays)
	windowDates := generateDateRange(windowStartDate, windowEndDate)
	for _, d := range windowDates {
		dateStr := d.Format("2006-01-02")
		datesMap[dateStr] = struct{}{}
	}

	datesToConciliate := s.getDatesToConciliate(ctx, allEvents)
	for _, d := range datesToConciliate {
		dateStr := d.Format("2006-01-02")
		datesMap[dateStr] = struct{}{}
	}

	return dateStrMapToTimeSlice(datesMap)
}
