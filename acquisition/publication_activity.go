package acquisition

import (
	"slices"

	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/pkg/sources"
)

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
