// Package catalogscheduler composes deployment-owned synchronization policy
// above Starmap's explicit idempotent Sync operation.
package catalogscheduler

import (
	"context"
	stderrors "errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sync"
)

const (
	// DefaultLeaseKey coordinates one catalog publisher group.
	DefaultLeaseKey = "starmap-catalog-sync"
	// DefaultLeaseTTL bounds an abandoned renewable lease implementation.
	DefaultLeaseTTL = 15 * time.Minute
)

// Syncer is the narrow algorithm input implemented by starmap.Client.
type Syncer interface {
	Sync(context.Context, ...sync.Option) (*sync.Result, error)
}

// Lease coordinates independent scheduler replicas before provider work.
type Lease interface {
	Acquire(context.Context, LeaseRequest) (LeaseGuard, error)
}

// LeaseGuard owns one acquired lease until Release.
type LeaseGuard interface {
	Release(context.Context) error
}

// LeaseRequest identifies one scheduler owner and bounded acquisition.
type LeaseRequest struct {
	Key   string
	Owner string
	TTL   time.Duration
}

// Validate verifies a complete lease request.
func (r LeaseRequest) Validate() error {
	if strings.TrimSpace(r.Key) == "" {
		return &errors.ValidationError{Field: "catalog_scheduler.lease.key", Message: "is required"}
	}
	if strings.TrimSpace(r.Owner) == "" {
		return &errors.ValidationError{Field: "catalog_scheduler.lease.owner", Message: "is required"}
	}
	if r.TTL <= 0 {
		return &errors.ValidationError{Field: "catalog_scheduler.lease.ttl", Value: r.TTL, Message: "must be positive"}
	}
	return nil
}

// RunStatus is the disposition of one deployment trigger.
type RunStatus string

const (
	// RunStatusRunning means a durable trigger has begun but is not terminal.
	RunStatusRunning RunStatus = "running"
	// RunStatusSucceeded means this replica acquired the lease and sync returned successfully.
	RunStatusSucceeded RunStatus = "succeeded"
	// RunStatusFailed means this replica acquired the lease and sync failed.
	RunStatusFailed RunStatus = "failed"
	// RunStatusSkippedLeaseHeld means another replica already owns the publisher lease.
	RunStatusSkippedLeaseHeld RunStatus = "skipped_lease_held"
	// RunStatusSkippedInitialRun means a startup attempt already covers this tick.
	RunStatusSkippedInitialRun RunStatus = "skipped_initial_run"
	// RunStatusSkippedScheduleOnly means startup policy deliberately made no attempt.
	RunStatusSkippedScheduleOnly RunStatus = "skipped_schedule_only"
)

// RunResult reports whether this replica executed provider work.
type RunResult struct {
	Status     RunStatus
	RunID      string
	LeaseOwner string
	Sync       *sync.Result
	// Attempts is the number of Sync calls made while holding the lease.
	Attempts int
	// RetryDelays is a caller-owned record of completed backoff decisions.
	RetryDelays []time.Duration
	// AttemptRecords contains caller-owned, secret-safe attempt audit metadata.
	AttemptRecords []AttemptRecord
}

// Runner serializes one explicit synchronization attempt through a deployment lease.
type Runner struct {
	syncer    Syncer
	lease     Lease
	request   LeaseRequest
	retry     RetryPolicy
	sleep     sleepFunc
	random    randomFunc
	ledger    RunLedger
	current   CurrentGenerationReader
	freshness *FreshnessMonitor
	now       func() time.Time
	newID     func() (string, error)
}

// NewRunner creates a deployment-owned synchronization runner.
func NewRunner(syncer Syncer, lease Lease, request LeaseRequest, options ...RunnerOption) (*Runner, error) {
	if isNilInterface(syncer) {
		return nil, &errors.ValidationError{Field: "catalog_scheduler.syncer", Message: "is required"}
	}
	if isNilInterface(lease) {
		return nil, &errors.ValidationError{Field: "catalog_scheduler.lease", Message: "is required"}
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	runner := &Runner{
		syncer: syncer, lease: lease, request: request,
		retry: DefaultRetryPolicy(), sleep: sleepContext, random: randomFloat64,
		now: time.Now, newID: newRunID,
	}
	for _, option := range options {
		if option == nil {
			return nil, &errors.ValidationError{Field: "catalog_scheduler.option", Message: "must not be nil"}
		}
		if err := option(runner); err != nil {
			return nil, err
		}
	}
	return runner, nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// RunOnce acquires the publisher lease before invoking Sync. Lease contention
// is a successful skipped disposition, not a provider failure.
func (r *Runner) RunOnce(ctx context.Context, options ...sync.Option) (result RunResult, err error) {
	return r.run(ctx, TriggerManual, options...)
}

func (r *Runner) run(ctx context.Context, trigger Trigger, options ...sync.Option) (result RunResult, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	startedAt := r.now().UTC()
	record := RunRecord{
		Trigger: trigger, LeaseOwner: r.request.Owner, StartedAt: startedAt, Status: RunStatusRunning,
	}
	if r.current != nil {
		record.BaseGenerationID = r.current.CurrentGenerationID()
	}
	if r.ledger != nil {
		record.ID, err = r.newID()
		if err != nil {
			return RunResult{Status: RunStatusFailed, LeaseOwner: r.request.Owner}, errors.WrapResource("generate", "catalog scheduler run ID", "", err)
		}
		if err := r.ledger.Begin(ctx, record); err != nil {
			return RunResult{Status: RunStatusFailed, RunID: record.ID, LeaseOwner: r.request.Owner}, errors.WrapResource("begin", "catalog scheduler run", record.ID, err)
		}
		defer func() {
			record.CompletedAt = r.now().UTC()
			record.Duration = record.CompletedAt.Sub(record.StartedAt)
			record.Status = result.Status
			record.Attempts = append([]AttemptRecord(nil), result.AttemptRecords...)
			if result.Sync != nil {
				record.PublishedGenerationID = result.Sync.GenerationID
				record.SyncRunID = result.Sync.SyncRunID
				record.SourceObservations = append([]catalogs.SourceObservationLink(nil), result.Sync.SourceObservations...)
			}
			if err != nil {
				record.FailureType = fmt.Sprintf("%T", err)
			}
			if completeErr := r.ledger.Complete(context.WithoutCancel(ctx), record); completeErr != nil {
				err = stderrors.Join(err, errors.WrapResource("complete", "catalog scheduler run", record.ID, completeErr))
				result.Status = RunStatusFailed
			}
		}()
	}
	result = RunResult{Status: RunStatusFailed, RunID: record.ID, LeaseOwner: r.request.Owner}
	guard, err := r.lease.Acquire(ctx, r.request)
	if err != nil {
		if stderrors.Is(err, errors.ErrConflict) {
			result.Status = RunStatusSkippedLeaseHeld
			return result, nil
		}
		return result, err
	}
	defer func() {
		if releaseErr := guard.Release(context.WithoutCancel(ctx)); releaseErr != nil {
			err = stderrors.Join(err, releaseErr)
			result.Status = RunStatusFailed
		}
	}()
	for attempt := 1; attempt <= r.retry.MaxAttempts; attempt++ {
		result.Attempts = attempt
		attemptStartedAt := r.now().UTC()
		syncResult, syncErr := r.syncer.Sync(ctx, options...)
		attemptCompletedAt := r.now().UTC()
		attemptRecord := AttemptRecord{
			Number: attempt, StartedAt: attemptStartedAt, CompletedAt: attemptCompletedAt,
			Duration: attemptCompletedAt.Sub(attemptStartedAt),
		}
		if syncErr == nil {
			attemptRecord.Status = AttemptStatusSucceeded
			result.AttemptRecords = append(result.AttemptRecords, attemptRecord)
			result.Sync = syncResult
			if recordErr := r.recordAttempt(ctx, record.ID, attemptRecord); recordErr != nil {
				return result, recordErr
			}
			if r.freshness != nil {
				if freshnessErr := r.freshness.RecordResult(syncResult); freshnessErr != nil {
					return result, errors.WrapResource("record", "catalog source freshness", record.ID, freshnessErr)
				}
			}
			result.Status = RunStatusSucceeded
			return result, nil
		}
		class := ClassifyRetry(syncErr)
		attemptRecord.Status = AttemptStatusFailed
		attemptRecord.RetryClass = class
		attemptRecord.FailureType = fmt.Sprintf("%T", syncErr)
		if class == RetryClassTransient && attempt < r.retry.MaxAttempts {
			attemptRecord.RetryDelay = r.retry.delay(attempt, r.random())
		}
		result.AttemptRecords = append(result.AttemptRecords, attemptRecord)
		if recordErr := r.recordAttempt(ctx, record.ID, attemptRecord); recordErr != nil {
			return result, stderrors.Join(syncErr, recordErr)
		}
		if class != RetryClassTransient || attempt == r.retry.MaxAttempts {
			return result, syncErr
		}
		delay := attemptRecord.RetryDelay
		result.RetryDelays = append(result.RetryDelays, delay)
		if sleepErr := r.sleep(ctx, delay); sleepErr != nil {
			return result, sleepErr
		}
	}
	return result, &errors.ValidationError{Field: "catalog_scheduler.retry", Message: "attempt loop completed without a result"}
}

func (r *Runner) recordAttempt(ctx context.Context, runID string, attempt AttemptRecord) error {
	if r.ledger == nil {
		return nil
	}
	if err := r.ledger.RecordAttempt(context.WithoutCancel(ctx), runID, attempt); err != nil {
		return errors.WrapResource("record", "catalog scheduler attempt", runID, err)
	}
	return nil
}

func newRunID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func (r *Runner) recordSkipped(ctx context.Context, trigger Trigger, status RunStatus) (result RunResult, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	result = RunResult{Status: status, LeaseOwner: r.request.Owner}
	if r.ledger == nil {
		return result, nil
	}
	record := RunRecord{
		Trigger: trigger, LeaseOwner: r.request.Owner, StartedAt: r.now().UTC(), Status: RunStatusRunning,
	}
	if r.current != nil {
		record.BaseGenerationID = r.current.CurrentGenerationID()
	}
	record.ID, err = r.newID()
	if err != nil {
		result.Status = RunStatusFailed
		return result, errors.WrapResource("generate", "catalog scheduler run ID", "", err)
	}
	result.RunID = record.ID
	if err := r.ledger.Begin(ctx, record); err != nil {
		result.Status = RunStatusFailed
		return result, errors.WrapResource("begin", "catalog scheduler run", record.ID, err)
	}
	record.CompletedAt = r.now().UTC()
	record.Duration = record.CompletedAt.Sub(record.StartedAt)
	record.Status = status
	if err := r.ledger.Complete(context.WithoutCancel(ctx), record); err != nil {
		result.Status = RunStatusFailed
		return result, errors.WrapResource("complete", "catalog scheduler run", record.ID, err)
	}
	return result, nil
}
