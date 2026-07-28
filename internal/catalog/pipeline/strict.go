package pipeline

import (
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func requireHealthyObservations(
	configured []sources.Source,
	observations []sources.Observation,
) error {
	expected := make(map[sources.ID]struct{}, len(configured))
	for _, source := range configured {
		sourceID := source.ID()
		if _, exists := expected[sourceID]; exists {
			return requiredSourceError(
				sourceID,
				"source",
				sourceID,
				"must be configured exactly once",
			)
		}
		expected[sourceID] = struct{}{}
	}

	observed := make(map[sources.ID]sources.Observation, len(observations))
	for _, observation := range observations {
		if _, exists := expected[observation.SourceID]; !exists {
			return requiredSourceError(
				observation.SourceID,
				"source",
				observation.SourceID,
				"was not configured for this synchronization",
			)
		}
		if _, exists := observed[observation.SourceID]; exists {
			return requiredSourceError(
				observation.SourceID,
				"observation",
				observation.SourceID,
				"must be returned exactly once",
			)
		}
		observed[observation.SourceID] = observation
	}

	for _, source := range configured {
		sourceID := source.ID()
		observation, exists := observed[sourceID]
		if !exists {
			return requiredSourceError(
				sourceID,
				"observation",
				sourceID,
				"required source returned no valid observation",
			)
		}
		if observation.Status != sources.ObservationStatusSucceeded {
			return requiredSourceError(
				sourceID,
				"status",
				observation.Status,
				"required source must succeed without degradation or fallback",
			)
		}
		if observation.Completeness != sources.ObservationCompletenessComplete {
			return requiredSourceError(
				sourceID,
				"completeness",
				observation.Completeness,
				"required source must be complete",
			)
		}
		if len(observation.Catalog.Definitions()) == 0 {
			return requiredSourceError(
				sourceID,
				"catalog.models",
				0,
				"required source returned no models without a degradation issue",
			)
		}
	}
	return nil
}

func requiredSourceError(sourceID sources.ID, field string, value any, message string) error {
	return &errors.SyncError{
		Provider: sourceID.String(),
		Err: &errors.ValidationError{
			Field:   "required_source." + field,
			Value:   value,
			Message: message,
		},
	}
}
