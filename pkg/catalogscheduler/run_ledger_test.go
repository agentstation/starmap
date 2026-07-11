package catalogscheduler

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	starmaperrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sync"

	_ "modernc.org/sqlite"
)

type runLedgerFactory struct {
	name string
	new  func(*testing.T) RunLedger
}

func TestSchedulerRunLedgerConformance(t *testing.T) {
	for _, factory := range []runLedgerFactory{
		{name: "memory", new: func(*testing.T) RunLedger { return NewMemoryRunLedger() }},
		{name: "sqlite", new: newTestSQLRunLedger},
	} {
		t.Run(factory.name, func(t *testing.T) {
			runRunLedgerConformance(t, factory.new(t))
		})
	}
}

func runRunLedgerConformance(t *testing.T, ledger RunLedger) {
	t.Helper()
	ctx := context.Background()
	start := time.Date(2026, time.July, 10, 19, 0, 0, 0, time.UTC)
	running := RunRecord{
		ID: "run-1", Trigger: TriggerScheduled, LeaseOwner: "replica-a",
		BaseGenerationID: "generation-base", StartedAt: start, Status: RunStatusRunning,
	}
	if err := ledger.Begin(ctx, running); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := ledger.Begin(ctx, running); err != nil {
		t.Fatalf("idempotent Begin: %v", err)
	}
	first := AttemptRecord{
		Number: 1, StartedAt: start.Add(time.Second), CompletedAt: start.Add(2 * time.Second),
		Duration: time.Second, Status: AttemptStatusFailed, RetryClass: RetryClassTransient,
		RetryDelay: 5 * time.Second, FailureType: "*errors.APIError",
	}
	second := AttemptRecord{
		Number: 2, StartedAt: start.Add(7 * time.Second), CompletedAt: start.Add(9 * time.Second),
		Duration: 2 * time.Second, Status: AttemptStatusSucceeded,
	}
	for _, attempt := range []AttemptRecord{first, second} {
		if err := ledger.RecordAttempt(ctx, running.ID, attempt); err != nil {
			t.Fatalf("RecordAttempt %d: %v", attempt.Number, err)
		}
		if err := ledger.RecordAttempt(ctx, running.ID, attempt); err != nil {
			t.Fatalf("idempotent RecordAttempt %d: %v", attempt.Number, err)
		}
	}
	complete := running.Copy()
	complete.CompletedAt = start.Add(10 * time.Second)
	complete.Duration = 10 * time.Second
	complete.Status = RunStatusSucceeded
	complete.Attempts = []AttemptRecord{first, second}
	complete.PublishedGenerationID = "generation-published"
	complete.SyncRunID = "sync-run-1"
	if err := ledger.Complete(ctx, complete); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if err := ledger.Complete(ctx, complete); err != nil {
		t.Fatalf("idempotent Complete: %v", err)
	}
	got, err := ledger.Get(ctx, running.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if diff := cmp.Diff(complete, got); diff != "" {
		t.Fatalf("run mismatch (-want +got):\n%s", diff)
	}
	got.Attempts[0].FailureType = "mutated"
	pristine, err := ledger.Get(ctx, running.ID)
	if err != nil || pristine.Attempts[0].FailureType != first.FailureType {
		t.Fatalf("caller mutated stored attempts: %#v/%v", pristine, err)
	}
	listed, err := ledger.List(ctx, RunQuery{Trigger: TriggerScheduled, Status: RunStatusSucceeded, Limit: 1})
	if err != nil || len(listed) != 1 || listed[0].ID != running.ID {
		t.Fatalf("List = %#v/%v", listed, err)
	}
	if _, err := ledger.Get(ctx, "missing"); !stderrors.Is(err, starmaperrors.ErrNotFound) {
		t.Fatalf("missing Get error = %v", err)
	}
}

type fixedGenerationReader string

func (r fixedGenerationReader) CurrentGenerationID() string { return string(r) }

type publishingSequenceSyncer struct {
	errors []error
	calls  int
}

func (s *publishingSequenceSyncer) Sync(context.Context, ...sync.Option) (*sync.Result, error) {
	index := s.calls
	s.calls++
	if index < len(s.errors) && s.errors[index] != nil {
		return nil, s.errors[index]
	}
	return &sync.Result{GenerationID: "generation-published", SyncRunID: "sync-publication-1"}, nil
}

func TestSchedulerRunLedgerPersistsTriggerBaseAttemptsDurationAndPublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run-ledger.db")
	db := openTestSQLite(t, path)
	ledger, err := NewSQLRunLedger(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLRunLedger: %v", err)
	}
	syncer := &publishingSequenceSyncer{errors: []error{starmaperrors.ErrProviderUnavailable}}
	runner, err := NewRunner(syncer, NewMemoryLease(), LeaseRequest{
		Key: DefaultLeaseKey, Owner: "replica-ledger", TTL: DefaultLeaseTTL,
	},
		WithRetryPolicy(RetryPolicy{MaxAttempts: 2, BaseDelay: 5 * time.Second, MaxDelay: 5 * time.Second}),
		WithRunLedger(ledger, fixedGenerationReader("generation-base")),
	)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.newID = func() (string, error) { return "scheduler-run-1", nil }
	now := time.Date(2026, time.July, 10, 20, 0, 0, 0, time.UTC)
	runner.now = func() time.Time {
		value := now
		now = now.Add(time.Second)
		return value
	}
	runner.sleep = func(context.Context, time.Duration) error { return nil }
	result, err := runner.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.RunID != "scheduler-run-1" || result.Attempts != 2 || result.Status != RunStatusSucceeded {
		t.Fatalf("result = %#v", result)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close first database: %v", err)
	}

	db = openTestSQLite(t, path)
	reopened, err := NewSQLRunLedger(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLRunLedger reopened: %v", err)
	}
	record, err := reopened.Get(context.Background(), result.RunID)
	if err != nil {
		t.Fatalf("Get reopened: %v", err)
	}
	if record.Trigger != TriggerManual || record.BaseGenerationID != "generation-base" ||
		record.PublishedGenerationID != "generation-published" || record.SyncRunID != "sync-publication-1" ||
		record.Status != RunStatusSucceeded || len(record.Attempts) != 2 || record.Duration <= 0 ||
		record.Attempts[0].RetryDelay != 5*time.Second || record.Attempts[1].Status != AttemptStatusSucceeded {
		t.Fatalf("persisted record = %#v", record)
	}
}

func TestSchedulerRunLedgerRecordsLeaseContentionWithoutAttempt(t *testing.T) {
	lease := NewMemoryLease()
	request := LeaseRequest{Key: DefaultLeaseKey, Owner: "holder", TTL: DefaultLeaseTTL}
	guard, err := lease.Acquire(context.Background(), request)
	if err != nil {
		t.Fatalf("Acquire holder: %v", err)
	}
	t.Cleanup(func() { _ = guard.Release(context.Background()) })
	ledger := NewMemoryRunLedger()
	runner, err := NewRunner(&publishingSequenceSyncer{}, lease, LeaseRequest{
		Key: DefaultLeaseKey, Owner: "contender", TTL: DefaultLeaseTTL,
	}, WithRunLedger(ledger, fixedGenerationReader("generation-base")))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.newID = func() (string, error) { return "contended-run", nil }
	result, err := runner.RunScheduledOnce(context.Background(), 0)
	if err != nil || result.Status != RunStatusSkippedLeaseHeld {
		t.Fatalf("RunScheduledOnce = %#v/%v", result, err)
	}
	record, err := ledger.Get(context.Background(), result.RunID)
	if err != nil || record.Trigger != TriggerScheduled || record.Status != RunStatusSkippedLeaseHeld || len(record.Attempts) != 0 {
		t.Fatalf("contended record = %#v/%v", record, err)
	}
}

func TestSchedulerRunLedgerRecordsPermanentFailureWithoutProviderText(t *testing.T) {
	ledger := NewMemoryRunLedger()
	syncer := &publishingSequenceSyncer{errors: []error{
		&starmaperrors.AuthenticationError{Provider: "test", Message: "secret-response-body"},
	}}
	runner, err := NewRunner(syncer, NewMemoryLease(), LeaseRequest{
		Key: DefaultLeaseKey, Owner: "failure-replica", TTL: DefaultLeaseTTL,
	}, WithRunLedger(ledger, fixedGenerationReader("generation-base")))
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.newID = func() (string, error) { return "failed-run", nil }
	result, err := runner.RunOnce(context.Background())
	if err == nil || result.Status != RunStatusFailed {
		t.Fatalf("RunOnce = %#v/%v", result, err)
	}
	record, getErr := ledger.Get(context.Background(), result.RunID)
	if getErr != nil {
		t.Fatalf("Get: %v", getErr)
	}
	if record.Status != RunStatusFailed || len(record.Attempts) != 1 ||
		record.Attempts[0].RetryClass != RetryClassPermanent ||
		strings.Contains(fmt.Sprintf("%#v", record), "secret-response-body") {
		t.Fatalf("failed record = %#v", record)
	}
}

func TestSchedulerSQLRunLedgerRetainsInterruptedRunningRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interrupted.db")
	db := openTestSQLite(t, path)
	ledger, err := NewSQLRunLedger(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLRunLedger: %v", err)
	}
	record := RunRecord{
		ID: "interrupted-run", Trigger: TriggerScheduled, LeaseOwner: "replica-crashed",
		BaseGenerationID: "generation-base", StartedAt: time.Now().UTC(), Status: RunStatusRunning,
	}
	if err := ledger.Begin(context.Background(), record); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	db = openTestSQLite(t, path)
	reopened, err := NewSQLRunLedger(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLRunLedger reopened: %v", err)
	}
	got, err := reopened.Get(context.Background(), record.ID)
	if err != nil || got.Status != RunStatusRunning || !got.CompletedAt.IsZero() {
		t.Fatalf("interrupted record = %#v/%v", got, err)
	}
}

func newTestSQLRunLedger(t *testing.T) RunLedger {
	t.Helper()
	db := openTestSQLite(t, filepath.Join(t.TempDir(), "conformance.db"))
	ledger, err := NewSQLRunLedger(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLRunLedger: %v", err)
	}
	return ledger
}

func openTestSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("Open SQLite: %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		t.Fatalf("Ping SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
