package runtime

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/sources"
)

func (l *layerSet) reconcileManualInputs(ctx context.Context, base *catalogs.Catalog, at time.Time, active []providerEvidenceKey) (*catalogs.Builder, starmap.CandidateEvidence, error) {
	collected := starmap.CandidateEvidence{}
	var providers []sources.Observation
	for _, batch := range manualBatches(l.manual) {
		observations := make([]sources.Observation, 0, len(batch.observations))
		for _, retained := range batch.observations {
			if err := ctx.Err(); err != nil {
				return nil, collected, err
			}
			if !l.providerBindings.permitsManual(retained) {
				continue
			}
			observation, err := retained.restore()
			if err != nil {
				return nil, collected, err
			}
			collected.SourceObservations = append(collected.SourceObservations, observation.Link())
			if observation.SourceID == sources.ProvidersID {
				providers = append(providers, observation)
			} else {
				observations = append(observations, observation)
			}
		}
		if len(observations) == 0 {
			continue
		}
		metadata, reviews, err := l.reconcileManualMetadata(ctx, base, at, observations)
		if err != nil {
			return nil, collected, err
		}
		base = metadata
		collected.ReviewCandidates = append(collected.ReviewCandidates, reviews...)
	}
	seen := make(map[string]bool, len(providers))
	for _, observation := range providers {
		seen[observation.ID] = true
	}
	for _, key := range active {
		retained := l.providers[key]
		if seen[retained.Receipt.Link.ObservationID] {
			continue
		}
		observation, err := (manualObservation{Payload: retained.Payload, Receipt: retained.Receipt}).restore()
		if err != nil {
			return nil, collected, err
		}
		providers = append(providers, observation)
		collected.SourceObservations = append(collected.SourceObservations, observation.Link())
	}
	slices.SortStableFunc(providers, reconciler.CompareProviderObservations)
	for len(providers) != 0 {
		end := 1
		for end < len(providers) && reconciler.CompareProviderObservations(providers[0], providers[end]) == 0 {
			end++
		}
		result, err := l.reconcileManualBatch(ctx, base, at, providers[:end])
		if err != nil {
			return nil, collected, err
		}
		base, err = result.Catalog.Build()
		if err != nil {
			return nil, collected, err
		}
		collected.ReviewCandidates = append(collected.ReviewCandidates, result.ReviewCandidates...)
		providers = providers[end:]
	}
	builder, err := catalogs.NewBuilderFrom(base)
	if err != nil {
		return nil, collected, err
	}
	collected.ReviewCandidates = slices.DeleteFunc(collected.ReviewCandidates, func(candidate evidence.ReviewCandidate) bool {
		_, err := builder.ProviderModel(catalogs.ProviderID(candidate.ProviderID), candidate.ProviderModelID)
		return err == nil
	})
	compactManualEvidence(&collected)
	return builder, collected, nil
}

// Each metadata pass contains one observation per source. Later passes keep the
// earlier reviewed definitions through the reconciler's baseline contract.
func (l *layerSet) reconcileManualMetadata(ctx context.Context, base *catalogs.Catalog, at time.Time, observations []sources.Observation) (*catalogs.Catalog, []evidence.ReviewCandidate, error) {
	var reviews []evidence.ReviewCandidate
	for len(observations) != 0 {
		seen := make(map[sources.ID]bool)
		end := 0
		for _, observation := range observations {
			if seen[observation.SourceID] {
				break
			}
			seen[observation.SourceID] = true
			end++
		}
		result, err := l.reconcileManualBatch(ctx, base, at, observations[:end])
		if err != nil {
			return nil, nil, err
		}
		base, err = result.Catalog.Build()
		if err != nil {
			return nil, nil, err
		}
		reviews = append(reviews, result.ReviewCandidates...)
		observations = observations[end:]
	}
	return base, reviews, nil
}

func (l *layerSet) reconcileManualBatch(ctx context.Context, base *catalogs.Catalog, at time.Time, observations []sources.Observation) (*reconciler.Result, error) {
	baseSource := sources.EmbeddedCatalogID
	if l.source != nil {
		baseSource = sources.ReleaseArtifactID
	}
	// A synthetic baseline carries existing field provenance but no new receipt.
	inputs := make([]sources.Observation, 0, 1+len(observations))
	inputs = append(inputs, sources.Observation{SourceID: baseSource, Catalog: base})
	inputs = append(inputs, observations...)
	for _, observation := range observations {
		if observation.ObservedAt.After(at) {
			at = observation.ObservedAt
		}
	}
	return reconciler.ReconcileObservations(ctx, base, inputs, reconciler.WithChangeTime(at))
}

func compactManualEvidence(collected *starmap.CandidateEvidence) {
	slices.SortFunc(collected.SourceObservations, func(left, right catalogs.SourceObservationLink) int {
		if order := strings.Compare(left.Source.String(), right.Source.String()); order != 0 {
			return order
		}
		return strings.Compare(left.ObservationID, right.ObservationID)
	})
	collected.SourceObservations = slices.CompactFunc(collected.SourceObservations, func(left, right catalogs.SourceObservationLink) bool {
		return left.Source == right.Source && left.ObservationID == right.ObservationID
	})
	slices.SortFunc(collected.ReviewCandidates, evidence.CompareReviewCandidates)
	collected.ReviewCandidates = slices.CompactFunc(collected.ReviewCandidates, func(left, right evidence.ReviewCandidate) bool {
		return evidence.CompareReviewCandidates(left, right) == 0
	})
}
