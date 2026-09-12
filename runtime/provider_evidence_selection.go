package runtime

import (
	"context"
	"slices"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// providerInventory records complete evidence within one retained provider scope.
type providerInventory struct {
	observedAt time.Time
	models     map[string]*catalogs.Model
}

// selectCurrentProviderEvidence excludes fully superseded receipts from a generation.
// Original observations remain in durable history. Current facts, review candidates,
// incomplete replacements, and uncovered membership keep their original receipts.
func selectCurrentProviderEvidence(ctx context.Context, builder *catalogs.Builder, observations []sources.Observation, selection *observationResetSelection, collected *starmap.CandidateEvidence) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	reviews, err := selectCurrentProviderReviews(ctx, observations, collected.ReviewCandidates)
	if err != nil {
		return err
	}
	collected.ReviewCandidates = reviews
	referenced := make(map[string]bool)
	for _, entries := range builder.Provenance().Map() {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, entry := range entries {
			referenced[entry.ObservationID] = true
		}
	}
	for _, scope := range builder.MembershipScopes() {
		if scope.Inventory != nil {
			referenced[scope.Inventory.ObservationID] = true
		}
		for _, addition := range scope.Additions {
			referenced[addition.ObservationID] = true
		}
	}
	for _, candidate := range collected.ReviewCandidates {
		referenced[candidate.SourceObservationID] = true
	}
	complete := make(map[providerEvidenceKey]providerInventory)
	for _, observation := range observations {
		if err := ctx.Err(); err != nil {
			return err
		}
		if observation.Completeness != sources.ObservationCompletenessComplete || observation.Status != sources.ObservationStatusSucceeded {
			continue
		}
		for _, provider := range observation.Catalog.Providers().List() {
			if selection.excluded[observation.ID][""] || selection.excluded[observation.ID][provider.ID] {
				continue
			}
			key := observationProviderKey(observation, provider.ID)
			if previous, exists := complete[key]; !exists || observation.ObservedAt.After(previous.observedAt) {
				complete[key] = providerInventory{observedAt: observation.ObservedAt, models: provider.Models}
			}
		}
	}
	retired := make(map[string]bool)
	for _, observation := range observations {
		if err := ctx.Err(); err != nil {
			return err
		}
		if referenced[observation.ID] {
			continue
		}
		superseded, err := providerObservationSuperseded(ctx, observation, selection, complete)
		if err != nil {
			return err
		}
		retired[observation.ID] = superseded
	}
	collected.SourceObservations = slices.DeleteFunc(collected.SourceObservations, func(link catalogs.SourceObservationLink) bool {
		return link.Source == sources.ProvidersID && retired[link.ObservationID]
	})
	return nil
}

func providerObservationSuperseded(ctx context.Context, observation sources.Observation, selection *observationResetSelection, complete map[providerEvidenceKey]providerInventory) (bool, error) {
	selected := false
	for _, provider := range observation.Catalog.Providers().List() {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if selection.excluded[observation.ID][""] || selection.excluded[observation.ID][provider.ID] {
			continue
		}
		selected = true
		current, exists := complete[observationProviderKey(observation, provider.ID)]
		if !exists || !current.observedAt.After(observation.ObservedAt) {
			return false, nil
		}
		for id := range provider.Models {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			if _, exists := current.models[id]; !exists {
				return false, nil
			}
		}
	}
	return selected, nil
}

func observationProviderKey(observation sources.Observation, provider catalogs.ProviderID) providerEvidenceKey {
	key := providerEvidenceKey{providerID: provider}
	if binding := observation.ProviderBinding; binding != nil {
		key.bindingID, key.revision = binding.ID, binding.Revision
	}
	return key
}
