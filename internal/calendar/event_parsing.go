package calendar

import (
	"log"
	"regexp"
	"strings"
	"time"

	gcal "google.golang.org/api/calendar/v3"
)

// Regex patterns for location codes
var (
	homeRegex     = regexp.MustCompile(`^HOM$`)
	officeRegex   = regexp.MustCompile(`^P\d{2}[A-Z]+\d{3}$`) // e.g. P12GRAN303
	libraryRegex  = regexp.MustCompile(`^LIB.*`)
	vacationRegex = regexp.MustCompile(`^V$`)
)

// ParseEvent determines if an event is managed, and extracts location info.
// It returns a ManagedEventInfo struct and a boolean indicating if it's managed.
func ParseEvent(event *gcal.Event, colorMap map[LocationStatus]string) (*ManagedEventInfo, bool) {
	isManagedProp := HasManagedProperty(event)
	isManagedDesc := HasDescriptionTag(event)

	if !isManagedProp && !isManagedDesc {
		// Not managed by ZenithPlanner
		return nil, false
	}

	if event == nil || event.Start == nil || event.Start.Date == "" {
		log.Println("Warning: event %s isn't a full-day event. Ignoring it.", event.Id)
		return nil, false
	}

	date, err := time.Parse("2006-01-02", event.Start.Date)
	if err != nil {
		log.Println("Warning: start date cannot be parsed from event with id %s", event.Id)
		return nil, false
	}

	endDate, err := time.Parse("2006-01-02", event.End.Date)
	if err != nil {
		log.Println("Warning: end date cannot be parsed from event with id %s", event.Id)
		return nil, false
	}

	if date.AddDate(0, 0, 1) != endDate {
		log.Println("Warning: event %s is a full-day event spanning multiple days. Ignoring it.", event.Id)
		return nil, false
	}

	updatedTs, err := time.Parse(time.RFC3339, event.Updated)
	if err != nil {
		log.Println("Warning: updated timestamp cannot be parsed from event with id %s", event.Id)
		return nil, false
	}

	locationCode := strings.TrimSpace(event.Summary)
	status := DetermineStatus(locationCode)
	expectedColor := colorMap[status]

	info := &ManagedEventInfo{
		EventID:           event.Id,
		Date:              date,
		LocationCode:      locationCode,
		Status:            status,
		Description:       event.Description,
		UpdatedTs:         updatedTs,
		IsManagedProperty: isManagedProp,
		IsManagedDescTag:  isManagedDesc,
		ColorID:           event.ColorId,
		RecurringEventID:  &event.RecurringEventId,
		NeedsProperty:     !isManagedProp,
		NeedsDescUpdate:   isManagedDesc,
		NeedsColorUpdate:  event.ColorId != expectedColor && expectedColor != "",
	}
	if event.RecurringEventId == "" {
		info.RecurringEventID = nil
	}
	if event.OriginalStartTime != nil {
		ost, err := time.Parse(time.RFC3339, event.OriginalStartTime.DateTime)
		if err == nil {
			info.OriginalStartTime = &ost
		} else {
			log.Println("Warning: original start time cannot be parsed from event with id %s", event.Id)
		}
	}

	return info, true
}

// DetermineStatus interprets the location code from the event title.
func DetermineStatus(locationCode string) LocationStatus {
	if homeRegex.MatchString(locationCode) {
		return StatusHome
	}
	if vacationRegex.MatchString(locationCode) {
		return StatusVacation
	}
	if officeRegex.MatchString(locationCode) {
		return StatusOffice
	}
	if libraryRegex.MatchString(locationCode) {
		return StatusLibrary
	}
	return StatusUnknown
}
