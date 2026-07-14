package catalogscheduler

import (
	"context"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Trigger identifies why a scheduler run was requested.
type Trigger string

const (
	// TriggerManual is an explicit operator or application request.
	TriggerManual Trigger = "manual"
	// TriggerScheduled is a cadence-owned deployment request.
	TriggerScheduled Trigger = "scheduled"
	// TriggerStartup is an initial-run policy request.
	TriggerStartup Trigger = "startup"
	// TriggerAPI is a request received through a remote control plane.
	TriggerAPI Trigger = "api"
)

// Validate verifies a supported trigger value.
func (t Trigger) Validate() error {
	switch t {
	case TriggerManual, TriggerScheduled, TriggerStartup, TriggerAPI:
		return nil
	default:
		return &errors.ValidationError{Field: "catalog_scheduler.run.trigger", Value: t, Message: "is not supported"}
	}
}

// AttemptStatus is the terminal result of one Sync invocation.
type AttemptStatus string

const (
	// AttemptStatusSucceeded means Sync returned without error.
	AttemptStatusSucceeded AttemptStatus = "succeeded"
	// AttemptStatusFailed means Sync returned an error.
	AttemptStatusFailed AttemptStatus = "failed"
)

// AttemptRecord is the durable, secret-safe result of one Sync invocation.
type AttemptRecord struct {
	Number      int
	StartedAt   time.Time
	CompletedAt time.Time
	Duration    time.Duration
	Status      AttemptStatus
	RetryClass  RetryClass
	RetryDelay  time.Duration
	FailureType string
}

// Validate verifies a complete terminal attempt record.
func (a AttemptRecord) Validate() error {
	if a.Number <= 0 {
		return &errors.ValidationError{Field: "catalog_scheduler.attempt.number", Value: a.Number, Message: validationPositiveMessage}
	}
	if a.StartedAt.IsZero() || a.CompletedAt.IsZero() || a.CompletedAt.Before(a.StartedAt) {
		return &errors.ValidationError{Field: "catalog_scheduler.attempt.time", Message: "must contain ordered start and completion times"}
	}
	if a.Duration < 0 || a.RetryDelay < 0 {
		return &errors.ValidationError{Field: "catalog_scheduler.attempt.duration", Message: validationNonnegativeMessage}
	}
	if a.Duration != a.CompletedAt.Sub(a.StartedAt) {
		return &errors.ValidationError{Field: "catalog_scheduler.attempt.duration", Value: a.Duration, Message: "does not match attempt timestamps"}
	}
	switch a.Status {
	case AttemptStatusSucceeded:
		if a.FailureType != "" || a.RetryClass != "" || a.RetryDelay != 0 {
			return &errors.ValidationError{Field: "catalog_scheduler.attempt.failure", Message: "successful attempt cannot contain failure metadata"}
		}
	case AttemptStatusFailed:
		if strings.TrimSpace(a.FailureType) == "" {
			return &errors.ValidationError{Field: "catalog_scheduler.attempt.failure_type", Message: "is required for failed attempts"}
		}
		if a.RetryClass != RetryClassTransient && a.RetryClass != RetryClassPermanent {
			return &errors.ValidationError{Field: "catalog_scheduler.attempt.retry_class", Value: a.RetryClass, Message: validationInvalidMessage}
		}
		if a.RetryClass == RetryClassPermanent && a.RetryDelay != 0 {
			return &errors.ValidationError{Field: "catalog_scheduler.attempt.retry_delay", Value: a.RetryDelay, Message: "permanent failure cannot schedule a retry"}
		}
	default:
		return &errors.ValidationError{Field: "catalog_scheduler.attempt.status", Value: a.Status, Message: validationInvalidMessage}
	}
	return nil
}

// RunRecord is one queryable deployment trigger and its publication result.
// FailureType records only the Go error type; provider error text is excluded
// because it may contain credentials or response data.
type RunRecord struct {
	ID                    string
	Trigger               Trigger
	LeaseOwner            string
	BaseGenerationID      string
	StartedAt             time.Time
	CompletedAt           time.Time
	Duration              time.Duration
	Status                RunStatus
	Attempts              []AttemptRecord
	SourceObservations    []catalogs.SourceObservationLink
	PublishedGenerationID string
	SyncRunID             string
	FailureType           string
}

// Copy returns a record with caller-owned collection state.
func (r RunRecord) Copy() RunRecord {
	r.Attempts = append([]AttemptRecord(nil), r.Attempts...)
	r.SourceObservations = append([]catalogs.SourceObservationLink(nil), r.SourceObservations...)
	return r
}

// ValidateBegin verifies the immutable fields of a newly triggered run.
func (r RunRecord) ValidateBegin() error {
	if strings.TrimSpace(r.ID) == "" {
		return &errors.ValidationError{Field: "catalog_scheduler.run.id", Message: validationRequiredMessage}
	}
	if err := r.Trigger.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.LeaseOwner) == "" {
		return &errors.ValidationError{Field: "catalog_scheduler.run.lease_owner", Message: validationRequiredMessage}
	}
	if r.StartedAt.IsZero() {
		return &errors.ValidationError{Field: "catalog_scheduler.run.started_at", Message: validationRequiredMessage}
	}
	if r.Status != RunStatusRunning || !r.CompletedAt.IsZero() || r.Duration != 0 ||
		len(r.Attempts) != 0 || len(r.SourceObservations) != 0 {
		return &errors.ValidationError{Field: "catalog_scheduler.run.status", Value: r.Status, Message: "new run must be running with no terminal state"}
	}
	return nil
}

// ValidateComplete verifies a terminal run and its ordered attempts.
func (r RunRecord) ValidateComplete() error {
	if strings.TrimSpace(r.ID) == "" || r.StartedAt.IsZero() || r.CompletedAt.IsZero() || r.CompletedAt.Before(r.StartedAt) {
		return &errors.ValidationError{Field: "catalog_scheduler.run.time", Message: "terminal run must contain an ID and ordered times"}
	}
	if err := r.Trigger.Validate(); err != nil {
		return err
	}
	if r.Status != RunStatusSucceeded && r.Status != RunStatusFailed &&
		r.Status != RunStatusSkippedLeaseHeld && r.Status != RunStatusSkippedInitialRun {
		return &errors.ValidationError{Field: "catalog_scheduler.run.status", Value: r.Status, Message: "must be terminal"}
	}
	if r.Duration < 0 {
		return &errors.ValidationError{Field: "catalog_scheduler.run.duration", Value: r.Duration, Message: validationNonnegativeMessage}
	}
	if r.Duration != r.CompletedAt.Sub(r.StartedAt) {
		return &errors.ValidationError{Field: "catalog_scheduler.run.duration", Value: r.Duration, Message: "does not match run timestamps"}
	}
	for index, attempt := range r.Attempts {
		if attempt.Number != index+1 {
			return &errors.ValidationError{Field: runAttemptsValidationField, Value: attempt.Number, Message: "must be contiguous and ordered"}
		}
		if err := attempt.Validate(); err != nil {
			return err
		}
	}
	seenSources := make(map[catalogmeta.SourceID]struct{}, len(r.SourceObservations))
	for _, observation := range r.SourceObservations {
		if err := observation.Validate(); err != nil {
			return err
		}
		if _, found := seenSources[observation.Source]; found {
			return &errors.ValidationError{Field: "catalog_scheduler.run.source_observations", Value: observation.Source, Message: "source is duplicated"}
		}
		seenSources[observation.Source] = struct{}{}
	}
	if r.Status == RunStatusSucceeded && len(r.Attempts) == 0 {
		return &errors.ValidationError{Field: runAttemptsValidationField, Message: "successful run requires an attempt"}
	}
	if isSkippedRunStatus(r.Status) && len(r.Attempts) != 0 {
		return &errors.ValidationError{Field: runAttemptsValidationField, Message: "lease-skipped run cannot contain attempts"}
	}
	if isSkippedRunStatus(r.Status) && len(r.SourceObservations) != 0 {
		return &errors.ValidationError{Field: "catalog_scheduler.run.source_observations", Message: "lease-skipped run cannot contain source observations"}
	}
	if r.Status == RunStatusFailed && strings.TrimSpace(r.FailureType) == "" {
		return &errors.ValidationError{Field: "catalog_scheduler.run.failure_type", Message: "is required for failed runs"}
	}
	if r.Status != RunStatusFailed && r.FailureType != "" {
		return &errors.ValidationError{Field: "catalog_scheduler.run.failure_type", Message: "is only valid for failed runs"}
	}
	if (r.PublishedGenerationID == "") != (r.SyncRunID == "") {
		return &errors.ValidationError{Field: "catalog_scheduler.run.publication", Message: "generation and sync-run IDs must be recorded together"}
	}
	return nil
}

func isSkippedRunStatus(status RunStatus) bool {
	return status == RunStatusSkippedLeaseHeld || status == RunStatusSkippedInitialRun
}

// RunQuery filters newest-first run history. A zero limit uses a bounded default.
type RunQuery struct {
	Trigger Trigger
	Status  RunStatus
	Limit   int
}

// RunLedger persists scheduler lifecycle state. Begin precedes lease
// acquisition, RecordAttempt follows each Sync call, and Complete is terminal.
type RunLedger interface {
	Begin(context.Context, RunRecord) error
	RecordAttempt(context.Context, string, AttemptRecord) error
	Complete(context.Context, RunRecord) error
	Get(context.Context, string) (RunRecord, error)
	List(context.Context, RunQuery) ([]RunRecord, error)
}

// CurrentGenerationReader supplies the base generation observed at run start.
type CurrentGenerationReader interface {
	CurrentGenerationID() string
}
