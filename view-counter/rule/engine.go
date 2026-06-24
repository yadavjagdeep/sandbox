package rule

import "view-counter/models"

// Validate takes a batch of view events and returns only the valid ones.
// This is a black box — in production, it checks for bots, duplicate views,
// suspicious patterns, etc.
func Validate(events []models.ViewEvent) []models.ViewEvent {
	// Stub: pass through all events as valid
	// In production: call an external rule engine service
	var valid []models.ViewEvent
	for _, e := range events {
		// Example rule: watch_duration must be >= 30 seconds
		if e.WatchDuration >= 30 {
			valid = append(valid, e)
		}
	}
	return valid
}
