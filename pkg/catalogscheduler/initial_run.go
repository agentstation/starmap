package catalogscheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// InitialRunMode selects the deployment's explicit startup behavior.
type InitialRunMode string

const (
	// InitialRunStartupBlocking runs Sync before startup can complete.
	InitialRunStartupBlocking InitialRunMode = "startup_blocking"
	// InitialRunStartupBackground runs Sync asynchronously while an existing
	// ready baseline may continue serving in degraded startup state.
	InitialRunStartupBackground InitialRunMode = "startup_background"
	// InitialRunScheduleOnly performs no startup Sync and waits for cadence.
	InitialRunScheduleOnly InitialRunMode = "schedule_only"
)

// InitialRunPolicy defines startup mode and the bounded interval in which a
// successful/running startup attempt replaces the first scheduled tick.
type InitialRunPolicy struct {
	Mode           InitialRunMode
	CoalesceWindow time.Duration
}

// Validate verifies an explicit mode and bounded coalescing policy.
func (p InitialRunPolicy) Validate() error {
	switch p.Mode {
	case InitialRunStartupBlocking, InitialRunStartupBackground:
		if p.CoalesceWindow <= 0 {
			return &errors.ValidationError{Field: "catalog_scheduler.initial_run.coalesce_window", Value: p.CoalesceWindow, Message: "must be positive for a startup attempt"}
		}
	case InitialRunScheduleOnly:
		if p.CoalesceWindow != 0 {
			return &errors.ValidationError{Field: "catalog_scheduler.initial_run.coalesce_window", Value: p.CoalesceWindow, Message: "must be zero for schedule-only mode"}
		}
	default:
		return &errors.ValidationError{Field: "catalog_scheduler.initial_run.mode", Value: p.Mode, Message: "is not supported"}
	}
	return nil
}

// BaselineReadiness is the non-startup catalog/freshness readiness supplied by
// the deployment composition root.
type BaselineReadiness struct {
	Ready    bool
	Degraded bool
}

// BaselineReadinessProbe evaluates the current catalog/freshness baseline.
type BaselineReadinessProbe func() BaselineReadiness

// InitialRunState is the lifecycle of the one startup decision.
type InitialRunState string

// Initial run lifecycle states.
const (
	InitialRunStatePending      InitialRunState = "pending"
	InitialRunStateRunning      InitialRunState = "running"
	InitialRunStateSucceeded    InitialRunState = "succeeded"
	InitialRunStateFailed       InitialRunState = "failed"
	InitialRunStateLeaseHeld    InitialRunState = "lease_held"
	InitialRunStateScheduleOnly InitialRunState = "schedule_only"
)

// Stable startup-readiness issue codes.
const (
	InitialRunIssuePending   = "initial_run_pending"
	InitialRunIssueFailed    = "initial_run_failed"
	InitialRunIssueLeaseHeld = "initial_run_lease_held"
	InitialRunIssueBaseline  = "initial_run_baseline_unready"
)

// InitialRunReadiness combines explicit startup state with the supplied
// catalog/freshness baseline.
type InitialRunReadiness struct {
	Mode        InitialRunMode  `json:"mode"`
	State       InitialRunState `json:"state"`
	Ready       bool            `json:"ready"`
	Degraded    bool            `json:"degraded"`
	IssueCode   string          `json:"issue_code,omitempty"`
	RunID       string          `json:"run_id,omitempty"`
	FailureType string          `json:"failure_type,omitempty"`
}

// InitialRunController executes exactly one startup policy decision and owns
// coalescing with the first scheduled tick. It owns no ticker or cadence.
type InitialRunController struct {
	mu       sync.RWMutex
	runner   *Runner
	policy   InitialRunPolicy
	baseline BaselineReadinessProbe
	now      func() time.Time

	startedAt time.Time
	state     InitialRunState
	result    RunResult
	err       error
	started   bool
	done      chan struct{}
}

// NewInitialRunController creates a passive startup policy controller.
func NewInitialRunController(runner *Runner, policy InitialRunPolicy, baseline BaselineReadinessProbe) (*InitialRunController, error) {
	if runner == nil {
		return nil, &errors.ValidationError{Field: "catalog_scheduler.initial_run.runner", Message: "is required"}
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if baseline == nil {
		return nil, &errors.ValidationError{Field: "catalog_scheduler.initial_run.baseline", Message: "is required"}
	}
	return &InitialRunController{
		runner: runner, policy: policy, baseline: baseline, now: time.Now,
		state: InitialRunStatePending, done: make(chan struct{}),
	}, nil
}

// Start applies the configured startup decision once. Blocking mode returns the
// Sync outcome; background mode returns after launching it; schedule-only
// closes immediately without source work.
func (c *InitialRunController) Start(ctx context.Context, options ...pkgsync.Option) error {
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return &errors.ConflictError{Resource: "catalog scheduler initial run", Expected: "not started", Actual: "started"}
	}
	c.started = true
	c.startedAt = c.now().UTC()
	mode := c.policy.Mode
	if mode == InitialRunScheduleOnly {
		c.state = InitialRunStateScheduleOnly
		c.result = RunResult{Status: RunStatusSkippedScheduleOnly, LeaseOwner: c.runner.request.Owner}
		close(c.done)
		c.mu.Unlock()
		return nil
	}
	c.state = InitialRunStateRunning
	c.mu.Unlock()

	if mode == InitialRunStartupBackground {
		go c.execute(ctx, options...)
		return nil
	}
	c.execute(ctx, options...)
	c.mu.RLock()
	err := c.err
	c.mu.RUnlock()
	return err
}

func (c *InitialRunController) execute(ctx context.Context, options ...pkgsync.Option) {
	result, err := c.runner.run(ctx, TriggerStartup, options...)
	c.mu.Lock()
	c.result = result
	c.err = err
	switch {
	case err != nil:
		c.state = InitialRunStateFailed
	case result.Status == RunStatusSucceeded:
		c.state = InitialRunStateSucceeded
	case result.Status == RunStatusSkippedLeaseHeld:
		c.state = InitialRunStateLeaseHeld
	default:
		c.state = InitialRunStateFailed
		c.err = &errors.ValidationError{Field: "catalog_scheduler.initial_run.result", Value: result.Status, Message: "is not a valid startup result"}
	}
	close(c.done)
	c.mu.Unlock()
}

// Wait waits for the startup decision to become terminal.
func (c *InitialRunController) Wait(ctx context.Context) (RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.RLock()
	started := c.started
	done := c.done
	c.mu.RUnlock()
	if !started {
		return RunResult{}, &errors.ConflictError{Resource: "catalog scheduler initial run", Expected: "started", Actual: "not started"}
	}
	select {
	case <-done:
		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.result, c.err
	case <-ctx.Done():
		return RunResult{}, ctx.Err()
	}
}

// Readiness combines the mode's startup gate with the deployment's current
// baseline readiness. Background pending/failure can serve only an already
// ready baseline and is explicitly degraded.
func (c *InitialRunController) Readiness() InitialRunReadiness {
	baseline := c.baseline()
	c.mu.RLock()
	mode, state, result, runErr := c.policy.Mode, c.state, c.result, c.err
	started := c.started
	c.mu.RUnlock()
	readiness := InitialRunReadiness{Mode: mode, State: state, RunID: result.RunID}
	if runErr != nil {
		readiness.FailureType = fmt.Sprintf("%T", runErr)
	}
	if !started {
		readiness.Ready = mode == InitialRunScheduleOnly && baseline.Ready
		readiness.Degraded = baseline.Degraded || mode != InitialRunScheduleOnly
		if !readiness.Ready {
			readiness.IssueCode = InitialRunIssuePending
		}
		return readiness
	}
	switch mode {
	case InitialRunStartupBlocking:
		switch state {
		case InitialRunStateSucceeded:
			readiness.Ready = baseline.Ready
			readiness.Degraded = baseline.Degraded || !baseline.Ready
		case InitialRunStateLeaseHeld:
			readiness.Ready = baseline.Ready
			readiness.Degraded = true
			readiness.IssueCode = InitialRunIssueLeaseHeld
		default:
			readiness.Degraded = true
			readiness.IssueCode = issueForInitialState(state)
		}
	case InitialRunStartupBackground:
		readiness.Ready = baseline.Ready
		readiness.Degraded = baseline.Degraded || state != InitialRunStateSucceeded
		if state != InitialRunStateSucceeded {
			readiness.IssueCode = issueForInitialState(state)
		}
	case InitialRunScheduleOnly:
		readiness.Ready = baseline.Ready
		readiness.Degraded = baseline.Degraded || !baseline.Ready
	}
	if !baseline.Ready && readiness.IssueCode == "" {
		readiness.IssueCode = InitialRunIssueBaseline
	}
	return readiness
}

// RunScheduledAt executes a deployment-supplied tick. A tick whose due time is
// covered by an in-flight or successful startup attempt is durably coalesced;
// failed startup attempts never suppress the recovery tick.
func (c *InitialRunController) RunScheduledAt(ctx context.Context, dueAt time.Time, jitterWindow time.Duration, options ...pkgsync.Option) (RunResult, error) {
	if dueAt.IsZero() {
		return RunResult{}, &errors.ValidationError{Field: "catalog_scheduler.scheduled_tick.due_at", Message: "is required"}
	}
	dueAt = dueAt.UTC()
	c.mu.RLock()
	startedAt, state, mode := c.startedAt, c.state, c.policy.Mode
	c.mu.RUnlock()
	covered := mode != InitialRunScheduleOnly && !dueAt.Before(startedAt) && dueAt.Sub(startedAt) <= c.policy.CoalesceWindow
	if covered && state == InitialRunStateRunning {
		if _, initialErr := c.Wait(ctx); initialErr != nil {
			if ctx.Err() != nil {
				return RunResult{Status: RunStatusFailed, LeaseOwner: c.runner.request.Owner}, ctx.Err()
			}
			return c.runner.RunScheduledOnce(ctx, jitterWindow, options...)
		}
		c.mu.RLock()
		state = c.state
		c.mu.RUnlock()
	}
	if covered && (state == InitialRunStateSucceeded || state == InitialRunStateLeaseHeld) {
		return c.runner.recordSkipped(ctx, TriggerScheduled, RunStatusSkippedInitialRun)
	}
	return c.runner.RunScheduledOnce(ctx, jitterWindow, options...)
}

func issueForInitialState(state InitialRunState) string {
	switch state {
	case InitialRunStateFailed:
		return InitialRunIssueFailed
	case InitialRunStateLeaseHeld:
		return InitialRunIssueLeaseHeld
	default:
		return InitialRunIssuePending
	}
}
