package acquisition

import (
	"slices"

	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// preflightActivityError reports selection when no collector supplied activity.
func preflightActivityError(options *pkgsync.Options, err error) error {
	if err == nil || len(sources.ActivityFromError(err)) != 0 {
		return err
	}
	return &sources.ActivityError{
		Activities:       pipeline.SourceConfiguration(options),
		ProviderAttempts: sources.ProviderAttemptsFromError(err),
		AcceptedSources:  sources.AcceptedSourcesFromError(err),
		Err:              err,
	}
}

// sourceSelectionActivityError retains the runtime policy when it refuses a request.
func sourceSelectionActivityError(selected []sources.ID, err error) error {
	activity := pipeline.SourceConfiguration(nil)
	for i := range activity {
		activity[i].SelectionUnknown = false
		activity[i].Enabled = slices.Contains(selected, activity[i].Source)
	}
	return &sources.ActivityError{Activities: activity, Err: err}
}

// preparedActivityError retains completed acquisition when publication fails.
func preparedActivityError(prepared *pipeline.Prepared, err error) error {
	if prepared == nil || prepared.Result == nil {
		return err
	}
	return &sources.ActivityError{
		Activities:       slices.Clone(prepared.Result.SourceActivities),
		ProviderAttempts: slices.Clone(prepared.Result.ProviderAttempts),
		Err:              err,
	}
}
