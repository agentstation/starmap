package runtime

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ObservationInputs separates accepted local facts from the selected baseline.
// Both catalogs are immutable snapshots. Baseline excludes this runtime's local observations.
type ObservationInputs struct {
	// Current is the complete accepted effective generation.
	Current starmap.CatalogState
	// Baseline is the retained upstream generation, or the compiled catalog when no upstream layer exists.
	Baseline starmap.CatalogState
}

// ObservationInputs returns the current catalog and its selected baseline.
// It reads retained memory, starts no acquisition, and writes no files.
// A later update reads new snapshots under runtime operation ownership.
func (r *Runtime) ObservationInputs(ctx context.Context) (ObservationInputs, error) {
	inputs, _, err := r.observationSnapshot(ctx)
	return inputs, err
}

func (r *Runtime) observationSnapshot(ctx context.Context) (ObservationInputs, layerSet, error) {
	if r == nil {
		return ObservationInputs{}, layerSet{}, &errors.ValidationError{Field: "runtime", Message: "is required"}
	}
	if ctx == nil {
		return ObservationInputs{}, layerSet{}, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return ObservationInputs{}, layerSet{}, err
	}
	r.mu.RLock()
	layers, current := r.layers, r.effective
	r.mu.RUnlock()
	baseline, err := layers.selectedBaseline(layers.embedded)
	if err != nil {
		return ObservationInputs{}, layerSet{}, err
	}
	return ObservationInputs{Current: current, Baseline: baseline}, layers, ctx.Err()
}

// UpdateObservations prepares and publishes original observations under runtime ownership.
// The callback may read sources. Runtime shutdown and caller cancellation stop its context.
// An error or empty observation list preserves accepted state. The callback must not
// call another mutation on this runtime. Use ObservationInputs for a read-only preview.
//
// Optional resets replace prior local acquisition observations within the named scopes.
// Each scope requires complete successful replacement evidence. The baseline and
// unrelated scopes remain. Resets and replacements share the catalog publication journal.
func (r *Runtime) UpdateObservations(ctx context.Context, prepare func(context.Context, ObservationInputs) ([]sources.Observation, error), resets ...ObservationReset) (starmap.CatalogState, error) {
	if prepare == nil {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "observations.prepare", Message: "is required"}
	}
	resets, err := prepareObservationResets(resets)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	return r.UpdateAcquisition(ctx, func(runCtx context.Context, inputs ObservationInputs) (ObservationUpdate, error) {
		observations, err := prepare(runCtx, inputs)
		return ObservationUpdate{Observations: observations, Resets: resets}, err
	})
}

func (l *layerSet) selectedBaseline(embedded starmap.CatalogState) (starmap.CatalogState, error) {
	baseline := embedded
	if l.source != nil {
		decoded, err := l.source.decodeCatalog()
		if err != nil {
			return starmap.CatalogState{}, errors.WrapResource("decode", "retained source layer", l.source.GenerationID, err)
		}
		if err := l.validateUpstreamScopePublishers(decoded); err != nil {
			return starmap.CatalogState{}, err
		}
		baseline = starmap.CatalogState{Catalog: decoded, GenerationID: l.source.GenerationID, PayloadChecksum: l.source.Checksum, GeneratedAt: l.source.PublishedAt}
		if l.source.Manifest != nil {
			baseline.AuthorityHead = l.source.Manifest.AuthorityHead
		}
	}
	if baseline.Catalog == nil {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "effective catalog", Message: "has no baseline"}
	}
	return baseline, nil
}
