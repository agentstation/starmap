package runtime

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ObservationUpdate contains original acquisition results and explicit reset scopes.
// Acquisition derives scopes from the sources and bindings that actually completed.
type ObservationUpdate struct {
	Observations []sources.Observation
	Resets       []ObservationReset
}

// UpdateAcquisition prepares and commits one acquisition under runtime ownership.
// Failed preparation preserves accepted state. The callback must not mutate this runtime.
func (r *Runtime) UpdateAcquisition(ctx context.Context, prepare func(context.Context, ObservationInputs) (ObservationUpdate, error)) (starmap.CatalogState, error) {
	if prepare == nil {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "acquisition.prepare", Message: "is required"}
	}
	var state starmap.CatalogState
	_, err := r.execute(ctx, runKindManual, func(runCtx context.Context, _ *RefreshReport, epoch uint64) error {
		inputs, err := r.ObservationInputs(runCtx)
		if err != nil {
			return err
		}
		update, err := prepare(runCtx, inputs)
		if err != nil {
			return err
		}
		if err := runCtx.Err(); err != nil {
			return err
		}
		resets, err := prepareObservationResets(update.Resets)
		if err != nil {
			return err
		}
		if len(update.Observations) == 0 && len(resets) == 0 {
			state = inputs.Current
			return nil
		}
		observations, err := prepareManualObservations(runCtx, update.Observations)
		if err != nil {
			return err
		}
		state, err = r.publishInputs(runCtx, nil, nil, observations, epoch, resets)
		return err
	})
	return state, err
}

// PreviewAcquisition computes an acquisition against one captured runtime snapshot.
// It writes no catalog, workspace, or runtime state. The callback owns source reads.
func (r *Runtime) PreviewAcquisition(ctx context.Context, prepare func(context.Context, ObservationInputs) (ObservationUpdate, error)) (starmap.CatalogState, error) {
	if prepare == nil {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "acquisition.prepare", Message: "is required"}
	}
	inputs, candidate, err := r.observationSnapshot(ctx)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	update, err := prepare(ctx, inputs)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	if err := ctx.Err(); err != nil {
		return starmap.CatalogState{}, err
	}
	resets, err := prepareObservationResets(update.Resets)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	if len(update.Observations) == 0 && len(resets) == 0 {
		return inputs.Current, nil
	}
	observations, err := prepareManualObservations(ctx, update.Observations)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	if err := candidate.acquisitionSources.validateManual(observations); err != nil {
		return starmap.CatalogState{}, err
	}
	if err := candidate.providerBindings.validateManual(observations, true); err != nil {
		return starmap.CatalogState{}, err
	}
	if err := validateObservationReplacement(ctx, resets, observations); err != nil {
		return starmap.CatalogState{}, err
	}
	observations, err = candidate.prepareManualInputs(ctx, observations, nil, resets)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	if len(observations) == 0 {
		return inputs.Current, nil
	}
	candidate.manual = &manualBatch{parent: candidate.manual, observations: observations, resets: resets}
	return candidate.build(ctx, candidate.embedded)
}
