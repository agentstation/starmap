package catalogscheduler

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/types"
)

// CatalogIdentity is the atomic immutable-catalog identity supplied by the
// deployment composition root when operational state is evaluated.
type CatalogIdentity struct {
	GenerationID string `json:"generation_id"`
	Sequence     uint64 `json:"sequence"`
}

// OperationalRun is the secret-safe endpoint projection of the latest actual
// synchronization attempt.
type OperationalRun struct {
	ID                    string    `json:"id"`
	Trigger               Trigger   `json:"trigger"`
	Status                RunStatus `json:"status"`
	StartedAt             time.Time `json:"started_at"`
	CompletedAt           time.Time `json:"completed_at"`
	DurationSeconds       float64   `json:"duration_seconds"`
	Attempts              int       `json:"attempts"`
	BaseGenerationID      string    `json:"base_generation_id,omitempty"`
	PublishedGenerationID string    `json:"published_generation_id,omitempty"`
	SyncRunID             string    `json:"sync_run_id,omitempty"`
	FailureType           string    `json:"failure_type,omitempty"`
}

// SchedulerOperationalState reports whether deployment scheduling is wired
// and, when configured, the explicit startup lifecycle.
type SchedulerOperationalState struct {
	Configured bool                 `json:"configured"`
	InitialRun *InitialRunReadiness `json:"initial_run,omitempty"`
}

// OperationalState is one internally consistent operator view of catalog
// identity, source freshness, the last actual sync, and scheduler lifecycle.
type OperationalState struct {
	EvaluatedAt     time.Time                 `json:"evaluated_at"`
	Catalog         CatalogIdentity           `json:"catalog"`
	Freshness       *FreshnessReport          `json:"freshness,omitempty"`
	LastSync        *OperationalRun           `json:"last_sync,omitempty"`
	DegradedSources []types.SourceID          `json:"degraded_sources"`
	Scheduler       SchedulerOperationalState `json:"scheduler"`
}

// Operations composes deployment-owned scheduler telemetry without owning a
// ticker, cadence, or synchronization lifecycle.
type Operations struct {
	ledger    RunLedger
	freshness *FreshnessMonitor
	initial   *InitialRunController
	now       func() time.Time
}

// OperationsOption configures optional scheduler telemetry inputs.
type OperationsOption func(*Operations) error

// NewOperations creates an operational-state composer. With no options it
// explicitly reports an unconfigured scheduler rather than inventing state.
func NewOperations(options ...OperationsOption) (*Operations, error) {
	operations := &Operations{now: time.Now}
	for _, option := range options {
		if option == nil {
			return nil, &errors.ValidationError{Field: "catalog_scheduler.operations.option", Message: "must not be nil"}
		}
		if err := option(operations); err != nil {
			return nil, err
		}
	}
	return operations, nil
}

// WithOperationsRunLedger exposes durable scheduler history.
func WithOperationsRunLedger(ledger RunLedger) OperationsOption {
	return func(operations *Operations) error {
		if isNilInterface(ledger) {
			return &errors.ValidationError{Field: "catalog_scheduler.operations.run_ledger", Message: "is required"}
		}
		operations.ledger = ledger
		return nil
	}
}

// WithOperationsFreshness exposes the configured source SLA report.
func WithOperationsFreshness(monitor *FreshnessMonitor) OperationsOption {
	return func(operations *Operations) error {
		if monitor == nil {
			return &errors.ValidationError{Field: "catalog_scheduler.operations.freshness", Message: "is required"}
		}
		operations.freshness = monitor
		return nil
	}
}

// WithOperationsInitialRun exposes the explicit startup lifecycle.
func WithOperationsInitialRun(controller *InitialRunController) OperationsOption {
	return func(operations *Operations) error {
		if controller == nil {
			return &errors.ValidationError{Field: "catalog_scheduler.operations.initial_run", Message: "is required"}
		}
		operations.initial = controller
		return nil
	}
}

// WithOperationsClock supplies the evaluation clock used by operational and
// freshness reports. Deployments normally omit it; deterministic compositions
// and tests may share their scheduler clock.
func WithOperationsClock(now func() time.Time) OperationsOption {
	return func(operations *Operations) error {
		if now == nil {
			return &errors.ValidationError{Field: "catalog_scheduler.operations.clock", Message: "is required"}
		}
		operations.now = now
		return nil
	}
}

// State evaluates all configured operational inputs at one UTC instant.
func (o *Operations) State(ctx context.Context, identity CatalogIdentity) (OperationalState, error) {
	if o == nil {
		return OperationalState{}, &errors.ValidationError{Field: "catalog_scheduler.operations", Message: "is required"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	state := OperationalState{
		EvaluatedAt: o.now().UTC(), Catalog: identity,
		DegradedSources: make([]types.SourceID, 0),
		Scheduler:       SchedulerOperationalState{Configured: o.ledger != nil || o.freshness != nil || o.initial != nil},
	}
	if o.freshness != nil {
		report, err := o.freshness.Report(state.EvaluatedAt)
		if err != nil {
			return OperationalState{}, err
		}
		state.Freshness = &report
		for _, source := range report.Sources {
			if source.State != FreshnessStateFresh {
				state.DegradedSources = append(state.DegradedSources, source.Source)
			}
		}
	}
	if o.ledger != nil {
		last, err := o.lastSync(ctx)
		if err != nil {
			return OperationalState{}, err
		}
		state.LastSync = last
	}
	if o.initial != nil {
		initial := o.initial.Readiness()
		state.Scheduler.InitialRun = &initial
	}
	return state, nil
}

func (o *Operations) lastSync(ctx context.Context) (*OperationalRun, error) {
	var latest *RunRecord
	for _, status := range []RunStatus{RunStatusSucceeded, RunStatusFailed} {
		records, err := o.ledger.List(ctx, RunQuery{Status: status, Limit: 1})
		if err != nil {
			return nil, err
		}
		if len(records) > 0 && (latest == nil || records[0].StartedAt.After(latest.StartedAt)) {
			record := records[0]
			latest = &record
		}
	}
	if latest == nil {
		return nil, nil
	}
	return &OperationalRun{
		ID: latest.ID, Trigger: latest.Trigger, Status: latest.Status,
		StartedAt: latest.StartedAt, CompletedAt: latest.CompletedAt,
		DurationSeconds: latest.Duration.Seconds(), Attempts: len(latest.Attempts),
		BaseGenerationID:      latest.BaseGenerationID,
		PublishedGenerationID: latest.PublishedGenerationID, SyncRunID: latest.SyncRunID,
		FailureType: latest.FailureType,
	}, nil
}
