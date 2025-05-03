package sync

import (
	"context"
	"fmt"
	"log"
	"time"

	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

// RunIncrementalSync processes changes fetched using a sync token. Returns an error and whether full sync should be attempted.
func (s *Syncer) RunIncrementalSync(ctx context.Context, syncToken string) (error, bool) {
	log.Printf("Fetching changes using sync token: %s...", syncToken[:min(10, len(syncToken))])
	changedEvents, nextSyncToken, err := s.fetchIncrementalChanges(ctx, syncToken)
	if err != nil {
		err = fmt.Errorf("Failed to fetch incremental changes: %w. syncToken has been cleared.")
		// Clear the invalid token so an incremental sync isn't attempted again
		_ = s.dbRepo.SetSyncState(ctx, "syncToken", "")
		return err, true
	}

	log.Printf("Fetched %d changed events.", len(changedEvents))

	if len(changedEvents) == 0 && nextSyncToken != "" {
		log.Println("No changes detected. Updating sync token.")
		err = s.dbRepo.SetSyncState(ctx, "syncToken", nextSyncToken)
		if err != nil {
			err = fmt.Errorf("failed to persist new sync token after no changes: %w", err)
			return err, false
		}
		return nil, false
	}

	err = s.updateDBCache(ctx, changedEvents)
	if err != nil {
		err = fmt.Errorf("failed to update cache: %w", err)
		return err, true
	}
	affectedDates := s.getDatesToConciliate(ctx, changedEvents)
	s.reconciliate(ctx, affectedDates)

	if nextSyncToken != "" {
		log.Println("Persisting new sync token after incremental sync.")
		err = s.dbRepo.SetSyncState(ctx, "syncToken", nextSyncToken)
		if err != nil {
			err = fmt.Errorf("CRITICAL: failed to persist sync token after incremental sync: %w", err)
			return err, false
		}
	} else {
		log.Println("Warning: No new sync token received after incremental sync.")
	}

	log.Println("Incremental sync processing finished.")
	return nil, false
}

// fetchIncrementalChanges retrieves changed events using a sync token.
func (s *Syncer) fetchIncrementalChanges(ctx context.Context, syncToken string) ([]*gcal.Event, string, error) {
	var changedEvents []*gcal.Event
	var pageToken string
	var nextSyncToken = syncToken

	for {
		call := s.calendarService.Events.List(s.cfg.Google.CalendarID).
			PageToken(pageToken).
			SingleEvents(true). // Important for recurrence handling
			SyncToken(syncToken).
			Fields(googleapi.Field("items(id,summary,description,start,end,updated,colorId,recurringEventId,originalStartTime,extendedProperties,status),nextPageToken,nextSyncToken"))

		resp, err := call.Do()
		if err != nil {
			return nil, "", fmt.Errorf("failed to list changed calendar events (page token: %s): %w", pageToken, err)
		}

		changedEvents = append(changedEvents, resp.Items...)

		if resp.NextPageToken == "" {
			nextSyncToken = resp.NextSyncToken
			break
		}
		pageToken = resp.NextPageToken
		log.Printf("Fetched incremental page, continuing pagination...")

	}
	return changedEvents, nextSyncToken, nil
}

func (s *Syncer) reconciliate(ctx context.Context, dates []time.Time) {
	if len(dates) > 0 {
		log.Printf("Triggering reconciliation for %d affected dates...", len(dates))
		err := s.RunReconciliation(ctx, dates, true)
		if err != nil {
			log.Printf("Error during post-incremental-sync reconciliation: %v", err)
		}
	} else {
		log.Println("No valid dates identified for reconciliation.")
	}
}
