package runtime

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
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
	if r == nil {
		return ObservationInputs{}, &errors.ValidationError{Field: "runtime", Message: "is required"}
	}
	if ctx == nil {
		return ObservationInputs{}, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return ObservationInputs{}, err
	}
	r.mu.RLock()
	layers, current := r.layers, r.effective
	r.mu.RUnlock()
	baseline, err := layers.selectedBaseline(layers.embedded)
	if err != nil {
		return ObservationInputs{}, err
	}
	if err := ctx.Err(); err != nil {
		return ObservationInputs{}, err
	}
	return ObservationInputs{Current: current, Baseline: baseline}, nil
}

// UpdateObservations prepares and publishes original observations under runtime ownership.
// The callback may read sources. Runtime shutdown and caller cancellation stop its context.
// An error or empty observation list preserves accepted state. The callback must not
// call another mutation on this runtime. Use ObservationInputs for a read-only preview.
func (r *Runtime) UpdateObservations(ctx context.Context, prepare func(context.Context, ObservationInputs) ([]sources.Observation, error)) (starmap.CatalogState, error) {
	if prepare == nil {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "observations.prepare", Message: "is required"}
	}
	var state starmap.CatalogState
	_, err := r.execute(ctx, runKindManual, func(runCtx context.Context, _ *RefreshReport, epoch uint64) error {
		inputs, err := r.ObservationInputs(runCtx)
		if err != nil {
			return err
		}
		observations, err := prepare(runCtx, inputs)
		if err != nil {
			return err
		}
		if err := runCtx.Err(); err != nil {
			return err
		}
		if len(observations) == 0 {
			state = inputs.Current
			return nil
		}
		prepared, err := prepareManualObservations(runCtx, observations)
		if err != nil {
			return err
		}
		state, err = r.publishInputs(runCtx, nil, nil, prepared, epoch)
		return err
	})
	return state, err
}

func (l *layerSet) selectedBaseline(embedded starmap.CatalogState) (starmap.CatalogState, error) {
	baseline := embedded
	if l.source != nil {
		decoded, err := catalogs.DecodeCatalogPayload(l.source.Payload)
		if err != nil {
			return starmap.CatalogState{}, errors.WrapResource("decode", "retained source layer", l.source.GenerationID, err)
		}
		baseline = starmap.CatalogState{Catalog: decoded, GenerationID: l.source.GenerationID, PayloadChecksum: l.source.Checksum, GeneratedAt: l.source.PublishedAt}
	}
	if baseline.Catalog == nil {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "effective catalog", Message: "has no baseline"}
	}
	return baseline, nil
}
