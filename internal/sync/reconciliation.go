package sync

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"
	"gomodules.avm99963.com/zenithplanner/internal/calendar"
	"gomodules.avm99963.com/zenithplanner/internal/database"
	"gomodules.avm99963.com/zenithplanner/internal/email"

	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

// RunReconciliation performs the cleanup and core reconciliation logic for a set of dates.
func (s *Syncer) RunReconciliation(ctx context.Context, datesToReconcile []time.Time, triggeredByIncremental bool) error {
	log.Printf("Starting reconciliation for %d dates...", len(datesToReconcile))
	changesForEmail := make(map[string]string) // (date_str, "previous -> new")
	var emailClient *email.Client

	if s.cfg.App.EnableEmailConfirmations {
		emailClient = email.NewClient(s.cfg.SMTP)
	}

	for _, date := range datesToReconcile {
		dateStr := date.Format("2006-01-02")
		locationDiff, err := s.runSingleReconciliation(ctx, date)
		if err != nil {
			log.Printf("Error reconcialiating date %s: %v", dateStr, err)
		}
		if locationDiff != "" {
			changesForEmail[dateStr] = locationDiff
		}
	}

	if emailClient != nil && len(changesForEmail) > 0 {
		log.Printf("Sending confirmation email for %d changed dates.", len(changesForEmail))
		emailErr := emailClient.SendConfirmation(changesForEmail)
		if emailErr != nil {
			log.Printf("Error sending confirmation email: %v", emailErr)
		}
	}

	log.Printf("Reconciliation finished for %d dates.", len(datesToReconcile))
	return nil
}

// runSingleReconciliation runs reconciliation, and returns a string
// with the change performed to the location (in order to be included in
// the email) and an error.
func (s *Syncer) runSingleReconciliation(ctx context.Context, date time.Time) (string, error) {
	dateStr := date.Format("2006-01-02")
	log.Printf("Reconciling date: %s", dateStr)

	cachedEvents, err := s.dbRepo.GetCachedEventsByDate(ctx, date)
	if err != nil {
		return "", fmt.Errorf("error querying cache for date %s: %w", dateStr, err)
	}

	authoritativeEvent := s.cleanUpDuplicates(dateStr, cachedEvents)

	currentDbEntry, err := s.dbRepo.GetScheduleEntry(ctx, date)
	if err != nil {
		return "", fmt.Errorf("error fetching current schedule_entries for %s: %w", dateStr, err)
	}

	previousLocation := s.cfg.App.DefaultLocationCode
	if currentDbEntry != nil {
		previousLocation = currentDbEntry.LocationCode
	}

	dbChanged, newLocationCode, err := s.coreReconciliationLogic(ctx, date, authoritativeEvent, currentDbEntry)
	if err != nil {
		return "", fmt.Errorf("Error during core reconciliation for %s: %w", dateStr, err)
	}

	locationDiff := ""
	if dbChanged && previousLocation != newLocationCode && (authoritativeEvent != nil || newLocationCode != s.cfg.App.DefaultLocationCode) {
		formattedPreviousLocation := previousLocation
		if authoritativeEvent == nil {
			formattedPreviousLocation = "<none>"
		}
		locationDiff = fmt.Sprintf("%s → %s", formattedPreviousLocation, newLocationCode)
	}

	return locationDiff, nil
}

func (s *Syncer) cleanUpDuplicates(dateStr string, cachedEvents []database.CachedEvent) *database.CachedEvent {
	authoritativeEvent, duplicatesToDelete := identifyAuthoritativeCachedEvent(cachedEvents)
	s.deleteEventsFromCalendar(dateStr, duplicatesToDelete)
	return authoritativeEvent
}

// identifyAuthoritativeCachedEvent returns the authorative cached event
// (the most recent one) and a list of duplicates to be deleted.
func identifyAuthoritativeCachedEvent(cachedEvents []database.CachedEvent) (*database.CachedEvent, []string) {
	var authoritativeEvent *database.CachedEvent = nil
	duplicates := []string{}
	managedEvents := make([]database.CachedEvent, 0, len(cachedEvents))

	for i := range cachedEvents {
		event := cachedEvents[i]
		if event.IsManagedProperty || event.IsManagedDescription {
			managedEvents = append(managedEvents, event)
		}
	}

	if len(managedEvents) > 0 {
		sort.Slice(managedEvents, func(i, j int) bool {
			return managedEvents[j].UpdatedTs.Before(managedEvents[i].UpdatedTs)
		})
		authoritativeEvent = &managedEvents[0]
		for i := 1; i < len(managedEvents); i++ {
			duplicates = append(duplicates, managedEvents[i].EventID)
		}
	}

	return authoritativeEvent, duplicates
}

func (s *Syncer) deleteEventsFromCalendar(dateStr string, eventIDs []string) {
	if len(eventIDs) > 0 {
		log.Printf("Found %d duplicate managed events for %s. Cleaning up...", len(eventIDs), dateStr)
		for _, eventID := range eventIDs {
			log.Printf("Deleting duplicate event %s from calendar for date %s", eventID, dateStr)
			err := s.calendarService.Events.Delete(s.cfg.Google.CalendarID, eventID).Do()
			if err != nil && !googleapi.IsNotModified(err) && !isNotFoundError(err) { // Check for acceptable errors
				log.Printf("Error deleting duplicate event %s from calendar: %v", eventID, err)
			} else if err == nil {
				log.Printf("Successfully deleted duplicate event %s from calendar.", eventID)
			} else {
				log.Printf("Event %s likely already gone (the action was a noop).", eventID)
			}
		}
	}
}

// coreReconciliationLogic ensures the schedule_entries table and the single
// authoritative Calendar event's metadata are consistent.
// Assumes Calendar cleanup (duplicate deletion) already happened for this date.
// Returns true if schedule_entries was updated, the new location code, and any error.
func (s *Syncer) coreReconciliationLogic(ctx context.Context, date time.Time, authoritativeCacheData *database.CachedEvent, currentDbEntry *database.ScheduleEntry) (dbChanged bool, finalLocationCode string, err error) {
	var targetLocationCode, targetStatus, eventId string
	var needsProperty, needsDescriptionUpdate, needsColorUpdate, needsDbUpdate, needsEventCreation bool

	dateStr := date.Format("2006-01-02") // For logging

	// 1. Determine Target State & Required Actions
	if authoritativeCacheData != nil {
		title := derefString(authoritativeCacheData.Title)
		status := calendar.DetermineStatus(title)
		targetLocationCode = title
		targetStatus = string(status)
		eventId = authoritativeCacheData.EventID

		needsProperty = !authoritativeCacheData.IsManagedProperty
		needsDescriptionUpdate = authoritativeCacheData.IsManagedDescription
		expectedColor := s.colorMap[status]
		needsColorUpdate = derefString(authoritativeCacheData.ColorID) != expectedColor
	} else {
		targetLocationCode = s.cfg.App.DefaultLocationCode
		targetStatus = string(calendar.DetermineStatus(targetLocationCode))
		eventId = ""

		needsEventCreation = true
	}

	needsDbUpdate = currentDbEntry == nil || currentDbEntry.LocationCode != targetLocationCode || currentDbEntry.Status != targetStatus

	finalLocationCode = targetLocationCode

	if needsDbUpdate {
		log.Printf("Updating schedule_entries for %s: Code=%s, Status=%s", dateStr, targetLocationCode, targetStatus)
		entry := database.ScheduleEntry{
			Date:         date,
			LocationCode: targetLocationCode,
			Status:       targetStatus,
		}
		dbErr := s.dbRepo.UpsertScheduleEntry(ctx, entry)
		if dbErr != nil {
			return false, finalLocationCode, fmt.Errorf("failed to update schedule_entries for %s: %w", dateStr, dbErr)
		}
		dbChanged = true // Mark DB as changed only on successful update
	}

	if eventId != "" {
		var patchEvent *gcal.Event
		var patchNeeded bool

		// Prepare patch object based on needed updates
		eventToPatch := &gcal.Event{
			Id:          eventId,
			Description: derefString(authoritativeCacheData.Description),
			ColorId:     derefString(authoritativeCacheData.ColorID),
		}

		if needsProperty {
			log.Printf("Adding managed property to event %s", eventId)
			propPatch := calendar.AddManagedProperty(eventToPatch)
			patchEvent = mergeEventPatches(patchEvent, propPatch)
			patchNeeded = true
		}
		if needsDescriptionUpdate {
			log.Printf("Removing description tag from event %s", eventId)
			descPatch := calendar.RemoveDescriptionTag(eventToPatch) // Use cached description via eventToPatch
			patchEvent = mergeEventPatches(patchEvent, descPatch)
			patchNeeded = patchNeeded || descPatch != nil
		}
		if needsColorUpdate {
			log.Printf("Updating color for event %s", eventId)
			colorPatch := calendar.SetColor(eventToPatch, s.colorMap[calendar.LocationStatus(targetStatus)])
			patchEvent = mergeEventPatches(patchEvent, colorPatch)
			patchNeeded = patchNeeded || colorPatch != nil
		}

		if patchNeeded && patchEvent != nil {
			_, patchErr := s.calendarService.Events.Patch(s.cfg.Google.CalendarID, eventId, patchEvent).Do()
			if patchErr != nil {
				return false, finalLocationCode, fmt.Errorf("failed patching calendar event %s metadata: %w", eventId, patchErr)
			} else {
				log.Printf("Successfully patched metadata for event %s", eventId)
			}
		}
	}

	if needsEventCreation {
		log.Printf("Creating default calendar event for %s", dateStr)
		defaultEvent := &gcal.Event{
			Summary: targetLocationCode,
			Start:   &gcal.EventDateTime{Date: date.Format("2006-01-02")},
			End:     &gcal.EventDateTime{Date: date.AddDate(0, 0, 1).Format("2006-01-02")},
			ColorId: s.colorMap[calendar.LocationStatus(targetStatus)],
			ExtendedProperties: &gcal.EventExtendedProperties{
				Private: map[string]string{calendar.ManagedPropertyKey: "true"},
			},
		}
		createdEvent, insertErr := s.calendarService.Events.Insert(s.cfg.Google.CalendarID, defaultEvent).Do()
		if insertErr != nil {
			return false, finalLocationCode, fmt.Errorf("failaed creating default calendar event for %s: %w", dateStr, insertErr)
		} else {
			log.Printf("Successfully created default event %s for %s", createdEvent.Id, dateStr)
		}
	}

	return dbChanged, finalLocationCode, nil
}

// Helper to merge patch objects, prioritizing non-nil fields from patch2
func mergeEventPatches(patch1, patch2 *gcal.Event) *gcal.Event {
	if patch1 == nil {
		return patch2
	}
	if patch2 == nil {
		return patch1
	}

	// Initialize ForceSendFields if nil
	if patch1.ForceSendFields == nil {
		patch1.ForceSendFields = []string{}
	}

	// Merge ColorId
	colorNeedsUpdate := false
	if patch2.ColorId != "" { // If patch2 has a specific color
		colorNeedsUpdate = true
	} else { // Check if patch2 explicitly wants to clear the color
		for _, field := range patch2.ForceSendFields {
			if field == "ColorId" {
				colorNeedsUpdate = true
				break
			}
		}
	}
	if colorNeedsUpdate {
		patch1.ColorId = patch2.ColorId
		patch1.ForceSendFields = appendIfMissing(patch1.ForceSendFields, "ColorId")
	}

	// Merge Description
	descNeedsUpdate := false
	// Check if Description is explicitly set in patch2 (even if empty string)
	if patch2.Description != "" { // If patch2 has a specific description
		descNeedsUpdate = true
	} else {
		for _, field := range patch2.ForceSendFields {
			if field == "Description" {
				descNeedsUpdate = true // Patch2 explicitly sets description (even if empty)
				break
			}
		}
	}
	if descNeedsUpdate {
		patch1.Description = patch2.Description
		patch1.ForceSendFields = appendIfMissing(patch1.ForceSendFields, "Description")
	}

	// Merge Extended Properties (Private only for now)
	if patch2.ExtendedProperties != nil && patch2.ExtendedProperties.Private != nil {
		if patch1.ExtendedProperties == nil {
			patch1.ExtendedProperties = &gcal.EventExtendedProperties{}
		}
		if patch1.ExtendedProperties.Private == nil {
			patch1.ExtendedProperties.Private = make(map[string]string)
		}
		for k, v := range patch2.ExtendedProperties.Private {
			patch1.ExtendedProperties.Private[k] = v
		}
		patch1.ForceSendFields = appendIfMissing(patch1.ForceSendFields, "ExtendedProperties")
	}

	return patch1
}

// Helper to dereference string pointer, returning "" if nil
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Helper to check if a string slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Helper to append a string to a slice only if it's not already present
func appendIfMissing(slice []string, item string) []string {
	if !contains(slice, item) {
		return append(slice, item)
	}
	return slice
}

// Helper to check for 404/410 errors which might be acceptable when deleting
func isNotFoundError(err error) bool {
	if gErr, ok := err.(*googleapi.Error); ok {
		return gErr.Code == http.StatusNotFound || gErr.Code == http.StatusGone
	}
	return false
}
