package pipeline

import (
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func guardObservationHealth(
	baseline *catalogs.Catalog,
	observations []sources.Observation,
) ([]sources.Observation, error) {
	if baseline == nil || baseline.Providers().Len() == 0 {
		return observations, nil
	}
	guarded := make([]sources.Observation, len(observations))
	for index, observation := range observations {
		updated, err := guardObservationVolume(baseline, observation)
		if err != nil {
			return nil, err
		}
		guarded[index] = updated
	}
	return guarded, nil
}

func guardObservationVolume(
	baseline *catalogs.Catalog,
	observation sources.Observation,
) (sources.Observation, error) {
	if observation.SourceID == sources.LocalCatalogID || observation.Catalog == nil {
		return observation, nil
	}
	issues := append([]sources.ObservationIssue(nil), observation.Issues...)
	for _, provider := range baseline.Providers().List() {
		historical := sourceAttributedModelIDs(baseline, observation.SourceID, provider)
		if len(historical) == 0 {
			continue
		}
		current := observationProviderModelIDs(observation.Catalog, provider)
		missing := 0
		for modelID := range historical {
			if _, exists := current[modelID]; !exists {
				missing++
			}
		}
		if missing == 0 || hasVolumeIssue(issues, provider.ID) {
			continue
		}
		issues = append(issues, sources.ObservationIssue{
			Scope:   sources.ObservationIssueScopeProvider,
			Code:    sources.ObservationIssueCodeVolumeCollapse,
			Subject: provider.ID.String(),
			Message: fmt.Sprintf(
				"source omitted %d of %d previously attributed models; absence is retained as last-known-good",
				missing,
				len(historical),
			),
		})
	}
	if len(issues) == len(observation.Issues) {
		return observation, nil
	}
	return sources.NewObservation(observation.SourceID, observation.Catalog, sources.ObservationMetadata{
		ObservedAt:   observation.ObservedAt,
		Revision:     observation.Revision,
		Completeness: sources.ObservationCompletenessPartial,
		Status:       sources.ObservationStatusDegraded,
		Records:      observation.Records,
		Issues:       issues,
	})
}

func sourceAttributedModelIDs(
	baseline *catalogs.Catalog,
	source sources.ID,
	provider catalogs.Provider,
) map[string]struct{} {
	models := make(map[string]struct{})
	for modelID := range provider.Models {
		for _, entries := range baseline.Provenance().FindModel(provider.ID, modelID) {
			if currentProvenanceSource(entries) == source {
				models[modelID] = struct{}{}
				break
			}
		}
	}
	return models
}

func currentProvenanceSource(entries []provenance.Provenance) sources.ID {
	if len(entries) == 0 {
		return ""
	}
	current := entries[0]
	for _, entry := range entries[1:] {
		if entry.Timestamp.After(current.Timestamp) {
			current = entry
		}
	}
	return current.Source
}

func observationProviderModelIDs(
	catalog *catalogs.Catalog,
	baseline catalogs.Provider,
) map[string]struct{} {
	provider, err := catalog.Provider(baseline.ID)
	if err != nil {
		for _, alias := range baseline.Aliases {
			provider, err = catalog.Provider(alias)
			if err == nil {
				break
			}
		}
	}
	models := make(map[string]struct{})
	if err != nil {
		return models
	}
	for modelID := range provider.Models {
		models[modelID] = struct{}{}
	}
	return models
}

func hasVolumeIssue(issues []sources.ObservationIssue, providerID catalogs.ProviderID) bool {
	for _, issue := range issues {
		if issue.Code == sources.ObservationIssueCodeVolumeCollapse &&
			issue.Subject == providerID.String() {
			return true
		}
	}
	return false
}
