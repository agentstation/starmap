package catalogscheduler

import (
	"context"
	stderrors "errors"
	"sync/atomic"
	"testing"
	"time"

	starmaperrors "github.com/agentstation/starmap/pkg/errors"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type initialRunSyncer struct {
	calls    atomic.Int32
	baseline *atomic.Bool
	entered  chan struct{}
	release  chan struct{}
	err      error
}

func (s *initialRunSyncer) Sync(ctx context.Context, _ ...pkgsync.Option) (*pkgsync.Result, error) {
	s.calls.Add(1)
	if s.entered != nil {
		select {
		case s.entered <- struct{}{}:
		default:
		}
	}
	if s.release != nil {
		select {
		case <-s.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.baseline != nil {
		s.baseline.Store(true)
	}
	return &pkgsync.Result{}, nil
}

func TestInitialRunStartupBlockingAttemptsOnceAndGatesReadiness(t *testing.T) {
	var baseline atomic.Bool
	syncer := &initialRunSyncer{baseline: &baseline}
	runner := newTestRunner(t, syncer, NewMemoryLease(), "blocking-replica")
	controller := newTestInitialRunController(t, runner, InitialRunPolicy{
		Mode: InitialRunStartupBlocking, CoalesceWindow: time.Minute,
	}, func() BaselineReadiness { return BaselineReadiness{Ready: baseline.Load()} })

	if readiness := controller.Readiness(); readiness.Ready || readiness.State != InitialRunStatePending {
		t.Fatalf("pre-start readiness = %#v", readiness)
	}
	if err := controller.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if syncer.calls.Load() != 1 {
		t.Fatalf("startup calls = %d, want 1", syncer.calls.Load())
	}
	readiness := controller.Readiness()
	if !readiness.Ready || readiness.Degraded || readiness.State != InitialRunStateSucceeded {
		t.Fatalf("completed readiness = %#v", readiness)
	}
	if err := controller.Start(context.Background()); !stderrors.Is(err, starmaperrors.ErrConflict) {
		t.Fatalf("second Start error = %v", err)
	}
	if syncer.calls.Load() != 1 {
		t.Fatalf("second Start made %d calls", syncer.calls.Load())
	}
}

func TestInitialRunStartupBackgroundCoalescesFirstTickWithoutDuplicate(t *testing.T) {
	var baseline atomic.Bool
	baseline.Store(true)
	syncer := &initialRunSyncer{
		baseline: &baseline, entered: make(chan struct{}, 2), release: make(chan struct{}),
	}
	ledger := NewMemoryRunLedger()
	runner, err := NewRunner(syncer, NewMemoryLease(), LeaseRequest{
		Key: DefaultLeaseKey, Owner: "background-replica", TTL: DefaultLeaseTTL,
	}, WithRunLedger(ledger, fixedGenerationReader("generation-base")))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	ids := []string{"startup-run", "coalesced-tick", "later-tick"}
	runner.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	startedAt := time.Date(2026, time.July, 11, 1, 0, 0, 0, time.UTC)
	controller := newTestInitialRunController(t, runner, InitialRunPolicy{
		Mode: InitialRunStartupBackground, CoalesceWindow: time.Minute,
	}, func() BaselineReadiness { return BaselineReadiness{Ready: baseline.Load()} })
	controller.now = func() time.Time { return startedAt }
	if err := controller.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	select {
	case <-syncer.entered:
	case <-time.After(time.Second):
		t.Fatal("background initial run did not start")
	}
	readiness := controller.Readiness()
	if !readiness.Ready || !readiness.Degraded || readiness.State != InitialRunStateRunning || readiness.IssueCode != InitialRunIssuePending {
		t.Fatalf("background running readiness = %#v", readiness)
	}

	tickResult := make(chan RunResult, 1)
	tickErr := make(chan error, 1)
	go func() {
		result, err := controller.RunScheduledAt(context.Background(), startedAt.Add(30*time.Second), 0)
		tickResult <- result
		tickErr <- err
	}()
	select {
	case early := <-tickResult:
		t.Fatalf("first tick returned before initial attempt completed: %#v", early)
	case <-time.After(10 * time.Millisecond):
	}
	if syncer.calls.Load() != 1 {
		t.Fatalf("first tick duplicated in-flight startup, calls = %d", syncer.calls.Load())
	}
	close(syncer.release)
	if _, err := controller.Wait(context.Background()); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if err := <-tickErr; err != nil {
		t.Fatalf("coalesced tick: %v", err)
	}
	if result := <-tickResult; result.Status != RunStatusSkippedInitialRun || result.RunID != "coalesced-tick" {
		t.Fatalf("coalesced result = %#v", result)
	}
	if syncer.calls.Load() != 1 {
		t.Fatalf("coalesced tick made %d calls", syncer.calls.Load())
	}
	readiness = controller.Readiness()
	if !readiness.Ready || readiness.Degraded || readiness.State != InitialRunStateSucceeded {
		t.Fatalf("background completed readiness = %#v", readiness)
	}
	record, err := ledger.Get(context.Background(), "coalesced-tick")
	if err != nil || record.Status != RunStatusSkippedInitialRun || len(record.Attempts) != 0 || record.Trigger != TriggerScheduled {
		t.Fatalf("coalesced ledger record = %#v/%v", record, err)
	}

	later, err := controller.RunScheduledAt(context.Background(), startedAt.Add(2*time.Minute), 0)
	if err != nil || later.Status != RunStatusSucceeded || syncer.calls.Load() != 2 {
		t.Fatalf("later tick = %#v/%v, calls=%d", later, err, syncer.calls.Load())
	}
}

func TestInitialRunFailureReadinessAndRecoveryTickMatrix(t *testing.T) {
	failure := starmaperrors.ErrProviderUnavailable
	for _, test := range []struct {
		name          string
		mode          InitialRunMode
		baselineReady bool
		wantReady     bool
		wantStartErr  bool
	}{
		{name: "blocking rejects startup despite baseline", mode: InitialRunStartupBlocking, baselineReady: true, wantStartErr: true},
		{name: "background serves last known good degraded", mode: InitialRunStartupBackground, baselineReady: true, wantReady: true},
		{name: "background without baseline is unready", mode: InitialRunStartupBackground},
	} {
		t.Run(test.name, func(t *testing.T) {
			syncer := &initialRunSyncer{err: failure}
			runner := newTestRunner(t, syncer, NewMemoryLease(), "failure-replica")
			startedAt := time.Date(2026, time.July, 11, 2, 0, 0, 0, time.UTC)
			controller := newTestInitialRunController(t, runner, InitialRunPolicy{
				Mode: test.mode, CoalesceWindow: time.Minute,
			}, func() BaselineReadiness { return BaselineReadiness{Ready: test.baselineReady} })
			controller.now = func() time.Time { return startedAt }
			startErr := controller.Start(context.Background())
			if test.mode == InitialRunStartupBackground {
				_, startErr = controller.Wait(context.Background())
			}
			if test.mode == InitialRunStartupBackground && !stderrors.Is(startErr, failure) {
				t.Fatalf("background Wait error = %v", startErr)
			}
			if test.mode == InitialRunStartupBlocking && (startErr != nil) != test.wantStartErr {
				t.Fatalf("Start error = %v", startErr)
			}
			readiness := controller.Readiness()
			if readiness.Ready != test.wantReady || !readiness.Degraded || readiness.State != InitialRunStateFailed || readiness.IssueCode != InitialRunIssueFailed {
				t.Fatalf("failed readiness = %#v", readiness)
			}
			before := syncer.calls.Load()
			if _, err := controller.RunScheduledAt(context.Background(), startedAt.Add(30*time.Second), 0); !stderrors.Is(err, failure) {
				t.Fatalf("recovery tick error = %v", err)
			}
			if syncer.calls.Load() != before+1 {
				t.Fatalf("failed startup suppressed recovery tick, calls %d -> %d", before, syncer.calls.Load())
			}
		})
	}
}

func TestInitialRunScheduleOnlyMakesNoStartupAttempt(t *testing.T) {
	var baseline atomic.Bool
	baseline.Store(true)
	syncer := &initialRunSyncer{}
	runner := newTestRunner(t, syncer, NewMemoryLease(), "schedule-only-replica")
	controller := newTestInitialRunController(t, runner, InitialRunPolicy{Mode: InitialRunScheduleOnly}, func() BaselineReadiness {
		return BaselineReadiness{Ready: baseline.Load()}
	})
	if err := controller.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	result, err := controller.Wait(context.Background())
	if err != nil || result.Status != RunStatusSkippedScheduleOnly || syncer.calls.Load() != 0 {
		t.Fatalf("schedule-only startup = %#v/%v, calls=%d", result, err, syncer.calls.Load())
	}
	readiness := controller.Readiness()
	if !readiness.Ready || readiness.Degraded || readiness.State != InitialRunStateScheduleOnly {
		t.Fatalf("schedule-only readiness = %#v", readiness)
	}
	result, err = controller.RunScheduledAt(context.Background(), time.Now().UTC(), 0)
	if err != nil || result.Status != RunStatusSucceeded || syncer.calls.Load() != 1 {
		t.Fatalf("scheduled tick = %#v/%v, calls=%d", result, err, syncer.calls.Load())
	}
}

func TestInitialRunPolicyRequiresExplicitValidMode(t *testing.T) {
	tests := []InitialRunPolicy{
		{},
		{Mode: InitialRunStartupBlocking},
		{Mode: InitialRunStartupBackground, CoalesceWindow: -time.Second},
		{Mode: InitialRunScheduleOnly, CoalesceWindow: time.Second},
	}
	for _, policy := range tests {
		if err := policy.Validate(); err == nil {
			t.Fatalf("policy %#v passed validation", policy)
		}
	}
}

func newTestInitialRunController(t *testing.T, runner *Runner, policy InitialRunPolicy, baseline BaselineReadinessProbe) *InitialRunController {
	t.Helper()
	controller, err := NewInitialRunController(runner, policy, baseline)
	if err != nil {
		t.Fatalf("NewInitialRunController: %v", err)
	}
	return controller
}
