package catalogscheduler

import (
	"context"
	stderrors "errors"
	"sync/atomic"
	"testing"
	"time"

	starmaperrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sync"
)

type blockingSyncer struct {
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (s *blockingSyncer) Sync(ctx context.Context, _ ...sync.Option) (*sync.Result, error) {
	s.calls.Add(1)
	select {
	case s.entered <- struct{}{}:
	default:
	}
	select {
	case <-s.release:
		return &sync.Result{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestSchedulerLeaseSingleFlightPreventsReplicaStampede(t *testing.T) {
	lease := NewMemoryLease()
	syncer := &blockingSyncer{entered: make(chan struct{}, 2), release: make(chan struct{})}
	first := newTestRunner(t, syncer, lease, "replica-a")
	second := newTestRunner(t, syncer, lease, "replica-b")

	firstDone := make(chan RunResult, 1)
	firstErr := make(chan error, 1)
	go func() {
		result, err := first.RunOnce(context.Background())
		firstDone <- result
		firstErr <- err
	}()
	select {
	case <-syncer.entered:
	case <-time.After(time.Second):
		t.Fatal("first replica did not enter Sync")
	}

	skipped, err := second.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("contending RunOnce: %v", err)
	}
	if skipped.Status != RunStatusSkippedLeaseHeld || skipped.Sync != nil || syncer.calls.Load() != 1 {
		t.Fatalf("contending result/calls = %#v/%d", skipped, syncer.calls.Load())
	}
	close(syncer.release)
	if err := <-firstErr; err != nil {
		t.Fatalf("first RunOnce: %v", err)
	}
	if result := <-firstDone; result.Status != RunStatusSucceeded {
		t.Fatalf("first result = %#v", result)
	}

	afterRelease, err := second.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce after release: %v", err)
	}
	if afterRelease.Status != RunStatusSucceeded || syncer.calls.Load() != 2 {
		t.Fatalf("after release result/calls = %#v/%d", afterRelease, syncer.calls.Load())
	}
}

func TestSchedulerLeaseExpiryAndFencingProtectNewOwner(t *testing.T) {
	lease := NewMemoryLease()
	now := time.Date(2026, time.July, 10, 17, 0, 0, 0, time.UTC)
	lease.now = func() time.Time { return now }
	firstRequest := LeaseRequest{Key: DefaultLeaseKey, Owner: "replica-a", TTL: time.Minute}
	first, err := lease.Acquire(context.Background(), firstRequest)
	if err != nil {
		t.Fatalf("Acquire first: %v", err)
	}
	now = now.Add(time.Minute + time.Nanosecond)
	secondRequest := LeaseRequest{Key: DefaultLeaseKey, Owner: "replica-b", TTL: time.Minute}
	second, err := lease.Acquire(context.Background(), secondRequest)
	if err != nil {
		t.Fatalf("Acquire expired lease: %v", err)
	}
	if err := first.Release(context.Background()); err != nil {
		t.Fatalf("Release stale guard: %v", err)
	}
	if _, err := lease.Acquire(context.Background(), LeaseRequest{Key: DefaultLeaseKey, Owner: "replica-c", TTL: time.Minute}); !stderrors.Is(err, starmaperrors.ErrConflict) {
		t.Fatalf("stale release removed newer lease: %v", err)
	}
	if err := second.Release(context.Background()); err != nil {
		t.Fatalf("Release second: %v", err)
	}
}

func TestSchedulerFilesystemLeaseCoordinatesIndependentAdapters(t *testing.T) {
	root := t.TempDir()
	firstLease, err := NewFilesystemLease(root)
	if err != nil {
		t.Fatalf("NewFilesystemLease first: %v", err)
	}
	secondLease, err := NewFilesystemLease(root)
	if err != nil {
		t.Fatalf("NewFilesystemLease second: %v", err)
	}
	request := LeaseRequest{Key: DefaultLeaseKey, Owner: "replica-a", TTL: DefaultLeaseTTL}
	guard, err := firstLease.Acquire(context.Background(), request)
	if err != nil {
		t.Fatalf("Acquire first: %v", err)
	}
	request.Owner = "replica-b"
	if _, err := secondLease.Acquire(context.Background(), request); err == nil {
		t.Fatal("independent filesystem lease adapter acquired held publisher lease")
	}
	if err := guard.Release(context.Background()); err != nil {
		t.Fatalf("Release first: %v", err)
	}
	secondGuard, err := secondLease.Acquire(context.Background(), request)
	if err != nil {
		t.Fatalf("Acquire second after release: %v", err)
	}
	if err := secondGuard.Release(context.Background()); err != nil {
		t.Fatalf("Release second: %v", err)
	}
}

func newTestRunner(t *testing.T, syncer Syncer, lease Lease, owner string) *Runner {
	t.Helper()
	runner, err := NewRunner(syncer, lease, LeaseRequest{Key: DefaultLeaseKey, Owner: owner, TTL: DefaultLeaseTTL})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	return runner
}
