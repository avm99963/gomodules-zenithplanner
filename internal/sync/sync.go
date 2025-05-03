package sync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	"gomodules.avm99963.com/zenithplanner/internal/calendar"
	"gomodules.avm99963.com/zenithplanner/internal/config"
	"gomodules.avm99963.com/zenithplanner/internal/database"

	gcal "google.golang.org/api/calendar/v3"
)

// Syncer handles the synchronization logic.
// TODO: Separate this into several pieces. It's too large. E.g. move
// code to a cache package with methods which interact with the DB,
// calendar code should be moved to calendar, ...
type Syncer struct {
	dbRepo          *database.Repository
	calendarService *gcal.Service
	cfg             *config.Config
	colorMap        map[calendar.LocationStatus]string // Precomputed color map
	// Mutex shared between sync and other tasks to perform work.
	mutex sync.Mutex
	// Queue used to perform sync. At most 1 sync will be queued.
	syncQueue chan struct{}
}

// NewSyncer creates a new Syncer instance.
func NewSyncer(dbRepo *database.Repository, calendarService *gcal.Service, cfg *config.Config) *Syncer {
	colorMap := map[calendar.LocationStatus]string{
		calendar.StatusHome:     "3",  // Mauve/Grape
		calendar.StatusVacation: "10", // Green/Basil
		calendar.StatusOffice:   "5",  // Yellow/Banana
		calendar.StatusLibrary:  "2",  // Pale Green/Sage
		calendar.StatusUnknown:  "8",  // Gray
	}

	return &Syncer{
		dbRepo:          dbRepo,
		calendarService: calendarService,
		cfg:             cfg,
		colorMap:        colorMap,
		syncQueue:       make(chan struct{}, 1),
	}
}

// StartSyncWorker launches a background goroutine to process queued incremental syncs.
func (s *Syncer) StartSyncWorker(ctx context.Context) {
	log.Println("Starting sync worker goroutine...")
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Sync worker stopping due to context cancellation.")
				return
			case <-s.syncQueue:
				s.mutex.Lock()
				syncCtx := context.Background()
				err := s.runSync(syncCtx)
				if err != nil {
					log.Printf("Error during worker-driven sync: %v", err)
				}
				s.mutex.Unlock()
			}
		}
	}()
}

// RequestIncrementalSync attempts to queue a sync.
func (s *Syncer) RequestSync() {
	select {
	case s.syncQueue <- struct{}{}:
		log.Println("Sync requested and queued.")
	default:
		log.Println("Sync requested but queue is full (sync likely already running or queued). Request ignored.")
	}
}

// runSync synchronizes location data between Google Calendar and the
// DB. It will attempt to perform an incremental sync if possible,
// falling back to a full sync.
func (s *Syncer) runSync(ctx context.Context) error {
	log.Println("Starting sync...")

	syncToken, err := s.dbRepo.GetSyncState(ctx, "syncToken")
	if err != nil {
		log.Printf("No sync token found or error retrieving it: %v. Triggering full sync.", err)
		return s.RunFullSync(ctx)
	}
	if syncToken == "" {
		log.Println("Empty sync token found. Triggering full sync.")
		return s.RunFullSync(ctx)
	}

	log.Println("Performing an incremental sync since syncToken is available...")
	err, performFullSync := s.RunIncrementalSync(ctx, syncToken)
	if err != nil && performFullSync {
		log.Printf("Failed incremental sync: %v. Falling back to a full sync...", err)
		return s.RunFullSync(ctx)
	}
	return err
}

// getDatesToConciliate retrieves a slice of dates which should be
// conciliated, since they are affected by the received events.
func (s *Syncer) getDatesToConciliate(ctx context.Context, events []*gcal.Event) []time.Time {
	affectedDatesMap := make(map[string]struct{}) // Use a map as a set for unique dates
	for _, event := range events {
		if shouldStart, date := s.shouldStartConciliation(ctx, event); shouldStart {
			affectedDatesMap[date] = struct{}{}
		}
	}

	return dateStrMapToTimeSlice(affectedDatesMap)
}

// shouldStartConciliation returns whether an event affects the current
// sync state and should thus start conciliation, and the date affected.
func (s *Syncer) shouldStartConciliation(ctx context.Context, event *gcal.Event) (bool, string) {
	// Determine the date(s) affected by this change
	// For recurring master changes, this might involve querying instances or estimating range
	// For single instances/non-recurring, it's just the event's date
	eventDateStr := ""
	if event.Start != nil {
		eventDateStr = event.Start.Date
		if eventDateStr == "" && event.Start.DateTime != "" {
			log.Printf("Skipping non-all-day event change: %s (%s)", event.Id, event.Summary)
			return false, ""
		}
	} else if event.Status == "cancelled" {
		// Need a way to find the date for deleted events if Start is nil
		// Fetch from cache BEFORE deleting to get the date
		cached, _ := s.dbRepo.GetCachedEventByID(ctx, event.Id) // Need this method
		if cached != nil {
			eventDateStr = cached.Date.Format("2006-01-02")
			log.Printf("Identified date %s for deleted event %s from cache.", eventDateStr, event.Id)
		} else {
			log.Printf("Could not determine date for deleted event %s. Reconciliation might miss this date.", event.Id)
		}
	} else {
		log.Printf("Warning: cannot obtain start date for event %s. Reconciliation might miss this date.", event.Id)
	}

	return eventDateStr != "", eventDateStr
}

func dateStrMapToTimeSlice(dateStrings map[string]struct{}) []time.Time {
	dates := make([]time.Time, 0, len(dateStrings))
	for dateStr := range dateStrings {
		t, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			dates = append(dates, t)
		} else {
			log.Printf("Error parsing date to reconciliate %s: %v", dateStr, err)
		}
	}
	return dates
}

// updateCache updates the events cache from a list of changedEvents,
// which can include added, modified and deleted events.
func (s *Syncer) updateDBCache(ctx context.Context, changedEvents []*gcal.Event) error {
	err := s.deleteRecurringEventIDsFromDBCache(ctx, changedEvents)
	if err != nil {
		return fmt.Errorf("failed to delete recurring event IDs from DB cache: %w", err)
	}

	for _, event := range changedEvents {
		if event.Status == "cancelled" {
			log.Printf("Deleting event %s from cache.", event.Id)
			if err := s.dbRepo.DeleteCachedEvent(ctx, event.Id); err != nil {
				return fmt.Errorf("failed to delete event %s from cache: %w", event.Id, err)
			}
		} else {
			parsedInfo, _ := calendar.ParseEvent(event, s.colorMap)
			if parsedInfo != nil {
				cachedEvent := database.CachedEvent{
					EventID:              parsedInfo.EventID,
					Date:                 parsedInfo.Date,
					Title:                &parsedInfo.LocationCode,
					Description:          &parsedInfo.Description,
					UpdatedTs:            parsedInfo.UpdatedTs,
					IsManagedProperty:    calendar.HasManagedProperty(event),
					IsManagedDescription: calendar.HasDescriptionTag(event),
					ColorID:              &parsedInfo.ColorID,
					RecurringEventID:     parsedInfo.RecurringEventID,
					OriginalStartTime:    parsedInfo.OriginalStartTime,
				}
				if parsedInfo.LocationCode == "" {
					cachedEvent.Title = nil
				}
				if parsedInfo.Description == "" {
					cachedEvent.Description = nil
				}
				if parsedInfo.ColorID == "" {
					cachedEvent.ColorID = nil
				}

				log.Printf("Upserting event %s into cache.", event.Id)
				if err := s.dbRepo.UpsertCachedEvent(ctx, cachedEvent); err != nil {
					return fmt.Errorf("failed to upsert event %s into cache: %w", event.Id, err)
				}
			}
		}
	}
	return nil
}

// deleteRecurringEventIDsFromDBCache makes sure that the cache doesn't
// contain the original event when we receive recurring events.
func (s *Syncer) deleteRecurringEventIDsFromDBCache(ctx context.Context, changedEvents []*gcal.Event) error {
	recurringEventIDs := make(map[string]struct{})

	for _, event := range changedEvents {
		if event.RecurringEventId != "" {
			recurringEventIDs[event.RecurringEventId] = struct{}{}
		}
	}

	log.Printf("Possibly deleting %d events which might have been promoted to recurring events.")
	for id := range recurringEventIDs {
		err := s.dbRepo.DeleteCachedEvent(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to delete cached event which might have been promoted to recurring event: %v")
		}
	}

	return nil
}
