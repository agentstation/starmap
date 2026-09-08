package acquisition

import (
	"bytes"
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

// providerAttemptTarget identifies one independently observed provider scope.
// An empty binding retains the legacy unscoped observation contract.
type providerAttemptTarget struct {
	providerID catalogs.ProviderID
	binding    sources.ProviderAcquisitionBinding
}

type providerAnswer struct {
	index       int
	observation ProviderObservation
}

// acquireTargets shares the coalescing and cancellation rules for both selection modes.
// Answers use target positions because multiple bindings can name the same provider.
func (a *Acquirer) acquireTargets(ctx context.Context, request runtime.AcquisitionRequest, targets []providerAttemptTarget) (runtime.AcquisitionResult, error) {
	if err := ctx.Err(); err != nil {
		return runtime.AcquisitionResult{}, err
	}
	if len(targets) == 0 {
		return runtime.AcquisitionResult{}, nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	answers := make(chan providerAnswer, len(targets))
	for i, target := range targets {
		go func() {
			answers <- providerAnswer{index: i, observation: a.observeTarget(runCtx, request.Current, target)}
		}()
	}
	observed := make([]ProviderObservation, len(targets))
	answered := make([]bool, len(targets))
	count := 0
	var pending []runtime.ProviderLayer
	var window <-chan time.Time
	for count < len(targets) {
		if err := runCtx.Err(); err != nil {
			return a.closeTargets(targets, observed, answered), err
		}
		select {
		case answer := <-answers:
			answered[answer.index] = true
			observed[answer.index] = answer.observation
			count++
			if len(answer.observation.Layer.Payload) == 0 {
				continue
			}
			pending = append(pending, answer.observation.Layer)
			if window == nil {
				window = a.after(a.coalesceWindow(request))
			}
		case <-window:
			window = nil
			if err := a.emitTargets(runCtx, request, len(targets), count, pending); err != nil {
				return a.closeTargets(targets, observed, answered), err
			}
			pending = nil
		case <-runCtx.Done():
			return a.closeTargets(targets, observed, answered), runCtx.Err()
		}
	}
	return a.closeTargets(targets, observed, answered), runCtx.Err()
}

func (a *Acquirer) observeTarget(ctx context.Context, current *catalogs.Catalog, target providerAttemptTarget) ProviderObservation {
	if target.binding.ID == "" {
		return a.observe(ctx, current, target.providerID)
	}
	started := a.now()
	observation, err := a.ObserveProviderBinding(ctx, current, target.binding)
	if err != nil {
		observation = ProviderObservation{Attempt: sources.ProviderAttempt{
			Outcome: sources.ProviderOutcomeFailed, Reason: sources.ClassifyProviderReason(err),
			Requested: observation.Attempt.Requested,
		}}
	}
	observation.Attempt.ProviderID = target.providerID
	observation.Attempt.BindingID = target.binding.ID
	observation.Attempt.BindingRevision = target.binding.Revision
	if observation.Attempt.StartedAt.IsZero() {
		observation.Attempt.StartedAt = started
	}
	if observation.Attempt.CompletedAt.IsZero() {
		observation.Attempt.CompletedAt = a.now()
	}
	return observation
}

func (a *Acquirer) emitTargets(ctx context.Context, request runtime.AcquisitionRequest, eligible, answered int, layers []runtime.ProviderLayer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(layers) == 0 || request.Publish == nil {
		return nil
	}
	owned := make([]runtime.ProviderLayer, len(layers))
	for i, layer := range layers {
		layer.Payload = bytes.Clone(layer.Payload)
		layer.Receipt = layer.Receipt.Clone()
		owned[i] = layer
	}
	logging.Debug().Str("run_id", request.RunID).Int("answered", answered).Int("eligible", eligible).Int("layers", len(layers)).Msg("Acquisition published a coalescing window before every target answered")
	return request.Publish(ctx, owned)
}

func (a *Acquirer) closeTargets(targets []providerAttemptTarget, observed []ProviderObservation, answered []bool) runtime.AcquisitionResult {
	result := runtime.AcquisitionResult{Eligible: len(targets)}
	now := a.now()
	for i, target := range targets {
		if !answered[i] {
			result.Attempts = append(result.Attempts, sources.ProviderAttempt{
				ProviderID: target.providerID, BindingID: target.binding.ID, BindingRevision: target.binding.Revision,
				Outcome: sources.ProviderOutcomeFailed, Reason: sources.ProviderReasonRequestTimeout,
				Requested: target.binding.ID == "", CompletedAt: now,
			})
			continue
		}
		result.Attempts = append(result.Attempts, observed[i].Attempt)
		if len(observed[i].Layer.Payload) > 0 {
			result.Layers = append(result.Layers, observed[i].Layer)
		}
	}
	return result
}
