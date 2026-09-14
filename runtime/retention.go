package runtime

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

// startRetentionSchedule owns automatic cleanup independently of source update scheduling.
// The first pass follows startup spread. Shutdown waits for its complete lifetime.
func (r *Runtime) startRetentionSchedule() {
	if !r.config.retention.Enabled {
		return
	}
	r.work.Go(func() {
		r.runSchedule("retention", r.config.retention.Interval, r.schedule.retentionOffset, r.schedule.retentionPhase, true, nil, func(ctx context.Context) {
			if err := r.collectRetainedState(ctx); err != nil {
				r.logScheduledFailure("retention", err)
			}
		})
	})
}

func (r *Runtime) retentionStatus(last RetentionStatus) RetentionStatus {
	policy := r.config.retention
	last.Enabled, last.Interval = policy.Enabled, policy.Interval
	last.MaxGenerations, last.MaxBytes = policy.MaxGenerations, policy.MaxBytes
	last.ScanEntries, last.InputMaxBytes = policy.ScanEntries, policy.InputMaxBytes
	last.Health = orUnknown(last.Health)
	if last.GenerationCollection == "" {
		last.GenerationCollection = "unsupported"
		if r.config.leaseStore != nil {
			last.GenerationCollection = "shared_coordination_required"
		} else if r.client.CanCollectGenerations() {
			last.GenerationCollection = "supported"
		}
	}
	return last
}

// collectRetainedState serializes generation and input retirement with runtime publication.
// Pending recovery refuses generation deletion. Local input references remain independently verifiable.
func (r *Runtime) collectRetainedState(ctx context.Context) (resultErr error) {
	r.publicationMu.Lock()
	defer r.publicationMu.Unlock()
	r.providerRetentionMu.Lock()
	defer r.providerRetentionMu.Unlock()
	r.mu.RLock()
	lastSuccess := r.report.retention.SucceededAt
	served := r.effective.GenerationID
	r.mu.RUnlock()
	report := r.retentionStatus(RetentionStatus{AttemptedAt: r.config.now(), SucceededAt: lastSuccess})
	report.Health = HealthOK
	defer func() {
		if resultErr != nil {
			report.Health = HealthDegraded
		} else if report.GenerationCollection != "supported" {
			report.Health, report.Reason = HealthDegraded, report.GenerationCollection
		} else if report.OverLimit {
			report.Health, report.Reason = HealthDegraded, "required_content_exceeds_limit"
			report.SucceededAt = r.config.now()
		} else {
			report.SucceededAt = r.config.now()
		}
		r.mu.Lock()
		r.report.retention = report
		r.mu.Unlock()
	}()
	if err := ctx.Err(); err != nil {
		report.Reason = "cancelled"
		return err
	}
	policy := r.config.retention
	pending, err := r.store.loadInputPublication()
	if err != nil {
		report.Reason = "publication_recovery_unreadable"
		return err
	}
	if pending != nil {
		report.Reason = "publication_recovery_pending"
		return invalidInputPublication("complete pending retention recovery before automatic collection")
	}
	inputs, err := r.collectRetainedInputsLocked(ctx, InputCollectionRequest{MaxEntries: policy.ScanEntries, MaxBytes: policy.InputMaxBytes})
	report.ScannedInputs, report.InputBytes, report.RemovedInputs = inputs.Scanned, inputs.SnapshotBytes, len(inputs.Removed)
	if err != nil {
		report.Reason = "input_collection_failed"
		return err
	}
	if report.GenerationCollection != "supported" {
		return nil
	}
	state := r.client.CurrentCatalogState()
	request := storage.RetentionRequest{ExpectedGenerationID: state.GenerationID, MaxGenerations: policy.MaxGenerations,
		MaxBytes: policy.MaxBytes, ScanEntries: policy.ScanEntries}
	if served != "" && served != state.GenerationID {
		request.RequiredGenerationIDs = append(request.RequiredGenerationIDs, served)
	}
	if r.config.generationPin != "" {
		request.RequiredGenerationIDs = append(request.RequiredGenerationIDs, r.config.generationPin)
	}
	generations, err := r.client.CollectGenerations(ctx, request)
	report.Generations, report.GenerationBytes = generations.After.Generations, generations.After.Bytes
	report.ProtectedGenerations, report.ProtectedBytes = generations.Protected.Generations, generations.Protected.Bytes
	report.RemovedGenerations, report.OverLimit = len(generations.Removed), generations.OverLimit
	if err != nil {
		report.Reason = "generation_collection_failed"
		return err
	}
	return nil
}

// RetentionSnapshot returns the configured policy and last collection result without storage reads.
func (r *Runtime) RetentionSnapshot() RetentionStatus {
	if r == nil {
		return RetentionStatus{}
	}
	r.mu.RLock()
	last := r.report.retention
	r.mu.RUnlock()
	return r.retentionStatus(last)
}
