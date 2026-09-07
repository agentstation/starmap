package reconciler

import (
	"context"
	"reflect"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// scopedObservations selects provider records without combining their receipts.
// Source type sets field authority. Each selected record keeps its own observation.
type scopedObservations struct {
	models    map[modelIdentity]*scopedProviderRecord
	providers map[catalogs.ProviderID]*scopedProviderRecord
}

type scopedProviderRecord struct {
	observation sources.Observation
	provider    catalogs.Provider
}

// orderScopedObservations owns the input order and validates each scoped receipt.
// Provider records select direct observations before stale fallback, then observation time.
// Conflicting records at the same time require an explicit source correction.
func orderScopedObservations(ctx context.Context, input []sources.Observation) ([]sources.Observation, *scopedObservations, error) {
	ordered := slices.Clone(input)
	var scoped []sources.Observation
	var positions []int
	unscopedProvider := false
	bindings := make(map[string]bool)
	for index, observation := range input {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if observation.ProviderBinding == nil {
			unscopedProvider = unscopedProvider || observation.SourceID == sources.ProvidersID
			continue
		}
		if err := observation.Validate(); err != nil {
			return nil, nil, err
		}
		binding := *observation.ProviderBinding
		if bindings[binding.ID] {
			return nil, nil, &errors.ConflictError{Resource: "provider binding observations", Message: "each binding must supply exactly one active observation"}
		}
		bindings[binding.ID] = true
		observation.ProviderBinding = &binding
		observation.Issues = slices.Clone(observation.Issues)
		scoped, positions = append(scoped, observation), append(positions, index)
	}
	if len(scoped) == 0 {
		return orderUnscopedProviderObservations(ctx, ordered)
	}
	if unscopedProvider {
		return nil, nil, &errors.ConflictError{Resource: "provider observation scope", Message: "scoped and unscoped provider observations cannot share one reconciliation"}
	}
	slices.SortFunc(scoped, func(left, right sources.Observation) int {
		leftFallback, rightFallback := scopedFallback(left), scopedFallback(right)
		if leftFallback != rightFallback {
			if leftFallback {
				return -1
			}
			return 1
		}
		if order := left.ObservedAt.Compare(right.ObservedAt); order != 0 {
			return order
		}
		return strings.Compare(left.ProviderBinding.ID, right.ProviderBinding.ID)
	})
	selected := &scopedObservations{models: make(map[modelIdentity]*scopedProviderRecord), providers: make(map[catalogs.ProviderID]*scopedProviderRecord)}
	for index, observation := range scoped {
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

func (s *scopedObservations) add(observation sources.Observation) error {
	for _, provider := range observation.Catalog.Providers().List() {
		if err := s.addProvider(observation, provider); err != nil {
			return err
		}
	}
	return nil
}

func (s *scopedObservations) addProvider(observation sources.Observation, provider catalogs.Provider) error {
	record := &scopedProviderRecord{observation: observation, provider: provider}
	if previous, exists := s.providers[provider.ID]; exists && sameScopedPriority(previous.observation, observation) {
		old := previous.provider
		old.Models = nil
		current := provider
		current.Models = nil
		if !reflect.DeepEqual(old, current) {
			return &errors.ConflictError{Resource: "scoped provider facts", Message: "different provider records have the same observation time"}
		}
	}
	s.providers[provider.ID] = record
	for modelID, model := range provider.Models {
		identity := modelIdentity{providerID: provider.ID, modelID: modelID}
		if previous, exists := s.models[identity]; exists && sameScopedPriority(previous.observation, observation) {
			old := previous.provider.Models[modelID]
			if !reflect.DeepEqual(old, model) {
				return &errors.ConflictError{Resource: "scoped model facts", Message: "different model records have the same observation time"}
			}
		}
		s.models[identity] = record
	}
	return nil
}

func scopedFallback(observation sources.Observation) bool {
	return observationIsStaleFallback(sourceObservationEvidence{status: observation.Status, issues: observation.Issues}, true)
}

func sameScopedPriority(left, right sources.Observation) bool {
	return left.ObservedAt.Equal(right.ObservedAt) && scopedFallback(left) == scopedFallback(right)
}

func observationEvidence(observation sources.Observation) sourceObservationEvidence {
	result := sourceObservationEvidence{
		id: observation.ID, observedAt: observation.ObservedAt, revision: observation.Revision,
		evidenceChecksum: observation.EvidenceChecksum, completeness: observation.Completeness,
		status: observation.Status, records: observation.Records, issues: slices.Clone(observation.Issues),
	}
	if observation.ProviderBinding != nil {
		result.bindingID, result.bindingRevision = observation.ProviderBinding.ID, observation.ProviderBinding.Revision
	}
	return result
}

func (merger *merger) modelObservation(source sources.ID, identity modelIdentity) (sourceObservationEvidence, bool) {
	if source == sources.ProvidersID && merger.scoped != nil {
		if record, exists := merger.scoped.models[identity]; exists {
			return observationEvidence(record.observation), true
		}
		return sourceObservationEvidence{}, false
	}
	evidence, exists := merger.observations[source]
	return evidence, exists
}

func (merger *merger) providerObservation(source sources.ID, providerID catalogs.ProviderID) (sourceObservationEvidence, bool) {
	if source == sources.ProvidersID && merger.scoped != nil {
		if record, exists := merger.scoped.providers[providerID]; exists {
			return observationEvidence(record.observation), true
		}
		return sourceObservationEvidence{}, false
	}
	evidence, exists := merger.observations[source]
	return evidence, exists
}

// scopedPrimaryCatalog combines membership only for the primary-source filter.
// It is not an observation and supplies no synthetic receipt or field authority.
func scopedPrimaryCatalog(observations []sources.Observation) (*catalogs.Catalog, error) {
	builder := catalogs.NewEmpty()
	for _, observation := range observations {
		if observation.SourceID != sources.ProvidersID {
			continue
		}
		if err := builder.MergeWith(observation.Catalog, catalogs.WithStrategy(catalogs.MergeEnrichEmpty)); err != nil {
			return nil, err
		}
	}
	return catalogs.NewObservationCatalog(builder)
}
