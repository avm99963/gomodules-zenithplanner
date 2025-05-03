package sync

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	gcal "google.golang.org/api/calendar/v3"
)

const (
	dbKeyChannelID              = "channelId"
	dbKeyResourceID             = "resourceId"
	dbKeyChannelExpiration      = "channelExpiration"
	channelRenewalThresholdDays = 3
)

// RunHorizonMaintenanceTask performs the daily check for default events within the future horizon.
func (s *Syncer) RunHorizonMaintenanceTask(ctx context.Context) error {
	const logPrefix = "Horizon Maintenance Task:"
	log.Println(logPrefix, "Waiting for lock...")
	s.mutex.Lock()
	log.Println(logPrefix, "Starting...")
	defer s.mutex.Unlock()
	defer log.Println(logPrefix, "Finished.")

	startDate := time.Now()
	endDate := startDate.AddDate(0, 0, s.cfg.App.FutureHorizonDays)
	datesToCheck := generateDateRange(startDate, endDate)

	log.Printf("%s Checking %d dates from %s to %s",
		logPrefix,
		len(datesToCheck),
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"))

	log.Printf("%s Triggering reconciliation for %d dates...", logPrefix, len(datesToCheck))
	err := s.RunReconciliation(ctx, datesToCheck, false)
	if err != nil {
		return fmt.Errorf("%s Error during reconciliation: %w", logPrefix, err)
	}

	return nil
}

// RunChannelRenewalTask performs the daily webhook channel renewal check.
func (s *Syncer) RunChannelRenewalTask(ctx context.Context) error {
	const logPrefix = "Channel Renewal Task:"
	log.Println(logPrefix, "Starting...")
	defer log.Println(logPrefix, "Finished.") // Use defer for guaranteed finish log

	channelID, resourceID, expirationStr, err := s.getCurrentChannelInfo(ctx, logPrefix)
	if err != nil {
		log.Printf("%s Error retrieving existing channel info: %v. Ensuring channel exists.", logPrefix, err)
		// Fallback to ensure logic if retrieval failed or info is missing
		return s.EnsureWebhookChannelExists(ctx)
	}

	expirationTime, err := time.Parse(time.RFC3339Nano, expirationStr)
	if err != nil {
		log.Printf("%s Cannot parse stored expiration time '%s': %v. Ensuring channel exists.", logPrefix, expirationStr, err)
		// If parsing fails, assume the stored data is bad, treat as missing
		return s.EnsureWebhookChannelExists(ctx)
	}

	renewalThreshold := time.Now().AddDate(0, 0, channelRenewalThresholdDays)
	if !expirationTime.Before(renewalThreshold) {
		log.Printf("%s Channel %s expiration (%s) is not within %d-day renewal threshold. No action needed.",
			logPrefix, channelID, expirationTime.Format(time.RFC1123), channelRenewalThresholdDays)
		return nil
	}

	log.Printf("%s Channel %s expires on %s (within %d-day threshold). Renewing...",
		logPrefix, channelID, expirationTime.Format(time.RFC1123), channelRenewalThresholdDays)

	// Create the new channel
	newChannel, err := s.createCalendarWebhookChannel(ctx, logPrefix+" Renewal:")
	if err != nil {
		return fmt.Errorf("failed to create renewal channel: %w", err)
	}

	err = s.storeChannelInfo(ctx, newChannel, logPrefix+" Renewal:")
	if err != nil {
		return fmt.Errorf("failed to store renewed channel info: %w", err)
	}

	// Only stop the old one *after* successfully storing the new one
	s.stopCalendarWebhookChannel(ctx, channelID, resourceID, logPrefix+" Old Channel Cleanup:")

	return nil // Renewal process successful
}

// EnsureWebhookChannelExists creates and stores info for a new webhook channel.
func (s *Syncer) EnsureWebhookChannelExists(ctx context.Context) error {
	logPrefix := "Ensure Webhook Channel Exists:"
	newChannel, err := s.createCalendarWebhookChannel(ctx, logPrefix)
	if err != nil {
		return err
	}

	err = s.storeChannelInfo(ctx, newChannel, logPrefix)
	if err != nil {
		return err
	}

	return nil
}

// --- Helper Functions ---

// getCurrentChannelInfo retrieves the stored webhook channel details from the database.
func (s *Syncer) getCurrentChannelInfo(ctx context.Context, logPrefix string) (id, resourceID, expStr string, err error) {
	log.Printf("%s Retrieving current channel info from DB...", logPrefix)
	id, errChan := s.dbRepo.GetSyncState(ctx, dbKeyChannelID)
	resID, errRes := s.dbRepo.GetSyncState(ctx, dbKeyResourceID)
	exp, errExp := s.dbRepo.GetSyncState(ctx, dbKeyChannelExpiration)

	// Combine potential errors for a clearer single failure point if any occurred
	if errChan != nil || errRes != nil || errExp != nil {
		// You could potentially return a more structured error here
		return "", "", "", fmt.Errorf("failed to get one or more channel state parts: channelIdErr=%v, resourceIdErr=%v, expirationErr=%v", errChan, errRes, errExp)
	}

	log.Printf("%s Found existing channel info: ID=%s, ResourceID=%s", logPrefix, id, resID)
	return id, resID, exp, nil
}

// createCalendarWebhookChannel sends a Watch request to Google Calendar API.
func (s *Syncer) createCalendarWebhookChannel(ctx context.Context, logPrefix string) (*gcal.Channel, error) {
	log.Printf("%s Creating new webhook channel via API...", logPrefix)
	newChannelID := uuid.New().String()
	// Ensure trailing slash is removed before appending path
	webhookURL := strings.TrimSuffix(s.cfg.App.BaseURL, "/") + "/webhook/calendar"

	watchCall := s.calendarService.Events.Watch(s.cfg.Google.CalendarID, &gcal.Channel{
		Id:      newChannelID,
		Type:    "web_hook",
		Address: webhookURL,
		Token:   s.cfg.Google.WebhookVerificationToken,
		// Params: // Add params if needed
	})

	newChannel, err := watchCall.Do()
	if err != nil {
		log.Printf("%s Failed API call to create webhook channel: %v", logPrefix, err)
		return nil, fmt.Errorf("calendar API watch request failed: %w", err)
	}

	log.Printf("%s API call successful. Created channel ID: %s, ResourceID: %s", logPrefix, newChannel.Id, newChannel.ResourceId)
	return newChannel, nil
}

// storeChannelInfo saves the channel details to the database.
// If storing fails, it attempts to stop the (newly created) channel.
func (s *Syncer) storeChannelInfo(ctx context.Context, channel *gcal.Channel, logPrefix string) error {
	log.Printf("%s Storing info for channel ID: %s in DB...", logPrefix, channel.Id)
	expTime := time.UnixMilli(channel.Expiration)
	expTimeStr := expTime.Format(time.RFC3339Nano) // Consistent storage format

	// Store sequentially, checking errors after each step
	var err error
	if err = s.dbRepo.SetSyncState(ctx, dbKeyChannelID, channel.Id); err != nil {
		// Error handled below
	} else if err = s.dbRepo.SetSyncState(ctx, dbKeyResourceID, channel.ResourceId); err != nil {
		// Error handled below
	} else if err = s.dbRepo.SetSyncState(ctx, dbKeyChannelExpiration, expTimeStr); err != nil {
		// Error handled below
	}

	// Centralized error check after all attempts
	if err != nil {
		log.Printf("%s Failed to store channel info in DB: %v", logPrefix, err)
		// Attempt cleanup: Stop the channel we just created but failed to save properly
		log.Printf("%s Attempting cleanup: Stopping channel %s due to storage failure.", logPrefix, channel.Id)
		s.stopCalendarWebhookChannel(ctx, channel.Id, channel.ResourceId, logPrefix+" Cleanup:")
		return fmt.Errorf("failed to store new channel info [%s]: %w", channel.Id, err)
	}

	log.Printf("%s Successfully stored channel info. ID: %s, ResourceID: %s, Expires: %s",
		logPrefix, channel.Id, channel.ResourceId, expTime.Format(time.RFC1123))
	return nil
}

// stopCalendarWebhookChannel sends a Stop request to Google Calendar API for a given channel.
// It logs errors but handles 'Not Found' gracefully.
func (s *Syncer) stopCalendarWebhookChannel(ctx context.Context, channelID, resourceID, logPrefix string) {
	// Avoid panic if IDs are somehow empty, though this shouldn't happen in normal flow
	if channelID == "" || resourceID == "" {
		log.Printf("%s Skipping stop channel request due to empty ID/ResourceID.", logPrefix)
		return
	}

	log.Printf("%s Attempting to stop channel via API: ID=%s, ResourceID=%s", logPrefix, channelID, resourceID)
	stopCall := s.calendarService.Channels.Stop(&gcal.Channel{Id: channelID, ResourceId: resourceID})

	err := stopCall.Do()

	if err == nil {
		log.Printf("%s Successfully stopped channel %s.", logPrefix, channelID)
		return
	}

	// Handle specific errors (like 404 Not Found) gracefully
	if isNotFoundError(err) {
		log.Printf("%s Channel %s already stopped or invalid (Not Found).", logPrefix, channelID)
	} else {
		// Log other errors more prominently as they might indicate an issue
		log.Printf("%s Error stopping channel %s: %v", logPrefix, channelID, err)
	}
}
