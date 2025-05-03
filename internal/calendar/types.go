package calendar

import (
	"time"
)

// LocationStatus represents the interpreted status from an event title.
type LocationStatus string

const (
	StatusHome     LocationStatus = "Home"
	StatusOffice   LocationStatus = "Office"
	StatusLibrary  LocationStatus = "Library"
	StatusVacation LocationStatus = "Vacation"
	StatusUnknown  LocationStatus = "Unknown"
)

// ManagedEventInfo holds extracted information about a managed event.
type ManagedEventInfo struct {
	EventID           string
	Date              time.Time
	LocationCode      string
	Status            LocationStatus
	Description       string
	UpdatedTs         time.Time
	IsManagedProperty bool // True if identified via private property
	IsManagedDescTag  bool // True if identified via description tag
	ColorID           string
	RecurringEventID  *string    // Pointer to handle null
	OriginalStartTime *time.Time // Pointer to handle null
	NeedsProperty     bool       // Flag if property needs to be added
	NeedsDescUpdate   bool       // Flag if description tag needs removal
	NeedsColorUpdate  bool       // Flag if color needs correction
}
