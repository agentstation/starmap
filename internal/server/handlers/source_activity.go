package handlers

import "github.com/agentstation/starmap/pkg/sources"

// acquisitionActivityDetail copies validated activity into the operation summary.
func acquisitionActivityDetail(detail map[string]any, activities []sources.SourceActivity, attempts []sources.ProviderAttempt) map[string]any {
	var safeActivities []sources.SourceActivity
	for _, activity := range activities {
		if activity.Valid() {
			safeActivities = append(safeActivities, activity)
		}
	}
	var safeAttempts []sources.ProviderAttempt
	for _, attempt := range attempts {
		if attempt.Validate() == nil {
			safeAttempts = append(safeAttempts, attempt)
		}
	}
	if len(safeActivities) == 0 && len(safeAttempts) == 0 {
		return detail
	}
	if detail == nil {
		detail = make(map[string]any)
	}
	detail["source_activities"] = safeActivities
	detail["provider_attempts"] = safeAttempts
	return detail
}
