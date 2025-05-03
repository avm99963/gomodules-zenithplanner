package calendar

import (
	"strings"

	gcal "google.golang.org/api/calendar/v3"
)

const (
	ManagedPropertyKey = "zenithplanner_managed"
	descriptionTag     = "Add-To-ZenithPlanner: true"
)

// HasManagedProperty checks if the event has the ZenithPlanner private property.
func HasManagedProperty(event *gcal.Event) bool {
	if event.ExtendedProperties == nil || event.ExtendedProperties.Private == nil {
		return false
	}
	val, exists := event.ExtendedProperties.Private[ManagedPropertyKey]
	return exists && val == "true"
}

// HasDescriptionTag checks if the event description contains the specific tag.
func HasDescriptionTag(event *gcal.Event) bool {
	return strings.Contains(event.Description, descriptionTag)
}

// AddManagedProperty prepares an Event object patch to add the private property.
// Returns nil if property already exists.
func AddManagedProperty(event *gcal.Event) *gcal.Event {
	if HasManagedProperty(event) {
		return nil
	}
	patch := &gcal.Event{
		ExtendedProperties: &gcal.EventExtendedProperties{
			Private: map[string]string{
				ManagedPropertyKey: "true",
			},
		},
	}
	// Ensure we don't overwrite other private properties if they exist
	if event.ExtendedProperties != nil && event.ExtendedProperties.Private != nil {
		for k, v := range event.ExtendedProperties.Private {
			if _, exists := patch.ExtendedProperties.Private[k]; !exists {
				patch.ExtendedProperties.Private[k] = v
			}
		}
	}
	return patch
}

// RemoveDescriptionTag prepares an Event object patch to remove the tag from the description.
// Returns nil if the tag is not present.
func RemoveDescriptionTag(event *gcal.Event) *gcal.Event {
	if !HasDescriptionTag(event) {
		return nil
	}
	newDesc := strings.ReplaceAll(event.Description, descriptionTag+"\n", "")
	newDesc = strings.ReplaceAll(newDesc, descriptionTag, "") // Remove if it's the only line
	patch := &gcal.Event{
		Description: strings.TrimSpace(newDesc),
		ForceSendFields: []string{"Description"},
	}
	return patch
}

// SetColor prepares an Event object patch to set the color ID.
// Returns nil if the color is already correct.
func SetColor(event *gcal.Event, targetColorId string) *gcal.Event {
	if event.ColorId == targetColorId {
		return nil
	}
	return &gcal.Event{
		ColorId: targetColorId,
		ForceSendFields: []string{"ColorId"},
	}
}
