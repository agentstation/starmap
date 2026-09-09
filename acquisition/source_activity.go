package acquisition

import (
	"context"
	"slices"

	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

// AcquireSourceReport returns observations and activity owned by this call.
// Existing callers can use AcquireSources when they need only observations.
func (a *SourceAcquirer) AcquireSourceReport(ctx context.Context, request runtime.SourceAcquisitionRequest) ([]sources.Observation, []sources.SourceActivity, error) {
	report := a.SourceConfiguration()
	if request.Sources != nil {
		for i := range report {
			report[i].Enabled = slices.Contains(request.Sources, report[i].Source)
		}
	}
	seen := make(map[sources.ID]bool)
	record := func(activities []sources.SourceActivity) {
		for _, activity := range activities {
			if !activity.Valid() {
				continue
			}
			for i := range report {
				if report[i].Source != activity.Source {
					continue
				}
				current := &report[i]
				current.Attempted = current.Attempted || activity.Attempted
				if !seen[activity.Source] {
					current.Eligibility = activity.Eligibility
				} else if current.Eligibility == sources.EligibilityEligible || activity.Eligibility == sources.EligibilityEligible {
					current.Eligibility = sources.EligibilityEligible
				} else if current.Eligibility == sources.EligibilityUnknown || activity.Eligibility == sources.EligibilityUnknown {
					current.Eligibility = sources.EligibilityUnknown
				} else {
					current.Eligibility = sources.EligibilityIneligible
				}
				seen[activity.Source] = true
			}
		}
	}
	observations, err := a.acquireSources(ctx, request, record)
	return observations, report, err
}

// SourceConfiguration reports built-in metadata support and constructor defaults.
// It reads no workspace, tool, network endpoint, or credential material.
func (a *SourceAcquirer) SourceConfiguration() []sources.SourceActivity {
	var selected []sources.ID
	if a != nil {
		selected = a.options.Sources
	}
	result := make([]sources.SourceActivity, 0, 3)
	for _, id := range []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ModelsDevGitID} {
		result = append(result, sources.SourceActivity{Source: id, Supported: a != nil && a.pipeline != nil, Enabled: slices.Contains(selected, id), Eligibility: sources.EligibilityUnknown})
	}
	return result
}
