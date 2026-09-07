package reconciler

import (
	"context"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// orderUnscopedProviderObservations retains individual receipts from legacy provider layers.
// A single legacy observation keeps the existing direct-reconciler contract.
func orderUnscopedProviderObservations(ctx context.Context, ordered []sources.Observation) ([]sources.Observation, *scopedObservations, error) {
	var positions []int
	for index, observation := range ordered {
		if observation.SourceID == sources.ProvidersID {
			positions = append(positions, index)
		}
	}
	if len(positions) < 2 {
		return ordered, nil, nil
	}
	providers := make([]sources.Observation, 0, len(positions))
	seen := make(map[string]bool, len(positions))
	for _, index := range positions {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		observation := ordered[index]
		if err := observation.Validate(); err != nil {
			return nil, nil, err
		}
		if seen[observation.ID] {
			return nil, nil, &errors.ConflictError{Resource: "provider observations", Message: "each observation must be supplied once"}
		}
		seen[observation.ID] = true
		observation.Issues = slices.Clone(observation.Issues)
		providers = append(providers, observation)
	}
	slices.SortFunc(providers, func(left, right sources.Observation) int {
		if scopedFallback(left) != scopedFallback(right) {
			if scopedFallback(left) {
				return -1
			}
			return 1
		}
		if order := left.ObservedAt.Compare(right.ObservedAt); order != 0 {
			return order
		}
		return strings.Compare(left.ID, right.ID)
	})
	selected := &scopedObservations{models: make(map[modelIdentity]*scopedProviderRecord), providers: make(map[catalogs.ProviderID]*scopedProviderRecord)}
	for index, observation := range providers {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if err := selected.add(observation); err != nil {
			return nil, nil, err
		}
		ordered[positions[index]] = observation
	}
	return ordered, selected, nil
}
