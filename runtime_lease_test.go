package starmap

import (
	"context"
	stderrors "errors"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// TestRuntimeLeaseRejectsStaleEpochAtCommit proves the durable commit fence. A
// run that starts under one lease epoch cannot publish after another instance
// takes the lease, so two instances never overwrite one another.
func TestRuntimeLeaseRejectsStaleEpochAtCommit(t *testing.T) {
	t.Parallel()

	leases := &stubLeaseStore{}
	source := newStubSource("lease-source")
	source.release = make(chan struct{})
	payload := testCatalogPayload(t, "lease-provider", "lease-model", "Lease Model")
	source.replies = []SourceRead{
		testSourceRead(t, "generation-lease", payload, time.Now().UTC()),
	}

	runtime := openTestRuntime(t,
		WithCatalogStore(storage.NewMemory()),
		WithSource(source),
		WithLeaseStore(leases),
	)
	if runtime.lease.status() != leaseHeld {
		t.Fatalf("lease state = %q, want %q", runtime.lease.status(), leaseHeld)
	}
	before := runtime.State()

	ctx := context.Background()
	failed := make(chan error, 1)
	go func() {
		_, err := runtime.RefreshSource(ctx)
		failed <- err
	}()

	// The run holds the epoch it started under. Another instance now takes the
	// lease, and this instance observes the new epoch on its next renewal.
	<-source.entered
	leases.bumpEpoch()
	if err := runtime.lease.renewOnce(ctx); err != nil {
		t.Fatalf("renewOnce: %v", err)
	}
	close(source.release)

	err := <-failed
	if err == nil {
		t.Fatal("RefreshSource published under a stale lease epoch")
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("error = %T (%v), want *errors.ConflictError", err, err)
	}
	if after := runtime.State(); after.GenerationID != before.GenerationID {
		t.Errorf("generation moved to %q under a stale epoch", after.GenerationID)
	}

	// The next run starts under the current epoch and publishes.
	source.release = nil
	report, err := runtime.RefreshSource(ctx)
	if err != nil {
		t.Fatalf("RefreshSource under the current epoch: %v", err)
	}
	if !report.Published {
		t.Fatalf("report = %+v, want a published generation", report)
	}
	if _, found := runtime.Catalog().Providers().Get("lease-provider"); !found {
		t.Error("the published catalog lost the source provider")
	}
}

// TestRuntimeLeaseFenceRejectsALostLease proves that a lost lease stops every
// commit, even one that carries the epoch of the acquisition it started under.
func TestRuntimeLeaseFenceRejectsALostLease(t *testing.T) {
	t.Parallel()

	leases := &stubLeaseStore{}
	keeper := newLeaseKeeper(leases, "holder", time.Now)
	var work sync.WaitGroup
	if err := keeper.start(context.Background(), &work, nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		keeper.stop()
		work.Wait()
	})

	epoch := keeper.epoch()
	if epoch == 0 {
		t.Fatal("a held lease must carry a nonzero epoch")
	}
	if err := keeper.fence(epoch); err != nil {
		t.Fatalf("fence under the held epoch: %v", err)
	}

	leases.failLater(stderrors.New("another holder owns the lease"))
	if err := keeper.renewOnce(context.Background()); err == nil {
		t.Fatal("renewOnce succeeded after the lease moved")
	}
	if state := keeper.status(); state != leaseLost {
		t.Fatalf("lease state = %q, want %q", state, leaseLost)
	}
	err := keeper.fence(epoch)
	if err == nil {
		t.Fatal("fence accepted a commit after the lease moved")
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("error = %T, want *errors.ConflictError", err)
	}
}

// TestRefusedLeaseOpensAsANonOwner proves that another holder does not fail
// Open. The replica serves its retained catalog and records the lost lease.
func TestRefusedLeaseOpensAsANonOwner(t *testing.T) {
	t.Parallel()

	leases := &stubLeaseStore{}
	leases.refuseEvery()

	runtime := openTestRuntime(t, WithLeaseStore(leases))
	if state := runtime.lease.status(); state != leaseLost {
		t.Errorf("lease state = %q, want %q", state, leaseLost)
	}
	if got := runtime.Status().Lease; got != string(leaseLost) {
		t.Errorf("status lease = %q, want %q", got, leaseLost)
	}
	if runtime.Catalog() == nil {
		t.Error("a non-owner replica served no catalog")
	}
	if got := leases.acquireCount(); got != 1 {
		t.Errorf("lease acquisitions = %d, want one attempt at Open", got)
	}
}

// TestScheduledRunSkipsWhileAnotherInstanceOwnsTheLease proves that a refused
// replica sends no upstream request and observes no provider. It retries the
// lease on every scheduled run.
func TestScheduledRunSkipsWhileAnotherInstanceOwnsTheLease(t *testing.T) {
	t.Parallel()

	leases := &stubLeaseStore{}
	leases.refuseEvery()
	source := newStubSource("refused-source")
	acquirer := &stubAcquirer{}
	timer := newStubScheduleTimer()

	openTestRuntime(t,
		WithLeaseStore(leases),
		WithSource(source),
		WithSourcePollInterval(time.Hour),
		WithAcquirer(acquirer),
		WithAcquisitionEnabled(true),
		WithAcquisitionInterval(time.Hour),
		withScheduleTimer(timer.after),
	)

	// Open takes the lease once. Each scheduled startup pass retries it.
	eventually(t, 5*time.Second, "no scheduled run retried the refused lease", func() bool {
		return leases.acquireCount() >= 3
	})
	if got := source.readCount(); got != 0 {
		t.Errorf("source reads = %d, want no upstream request from a non-owner", got)
	}
	if got := acquirer.callCount(); got != 0 {
		t.Errorf("acquisition runs = %d, want no provider request from a non-owner", got)
	}
}

// TestManualRunByANonOwnerReturnsAConflict proves that an explicit refresh by a
// refused replica reports the conflict instead of writing. A later run takes
// the lease again and publishes.
func TestManualRunByANonOwnerReturnsAConflict(t *testing.T) {
	t.Parallel()

	leases := &stubLeaseStore{}
	// The first refusal answers Open. The second answers the manual run.
	leases.refuse(2)
	source := newStubSource("owner-source")
	payload := testCatalogPayload(t, "owner-provider", "owner-model", "Owner Model")
	source.replies = []SourceRead{
		testSourceRead(t, "generation-owner", payload, time.Now().UTC()),
	}

	runtime := openTestRuntime(t,
		WithCatalogStore(storage.NewMemory()),
		WithSource(source),
		WithLeaseStore(leases),
	)

	_, err := runtime.RefreshSource(context.Background())
	if err == nil {
		t.Fatal("a non-owner refresh published a generation")
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("error = %T (%v), want *errors.ConflictError", err, err)
	}
	if got := source.readCount(); got != 0 {
		t.Errorf("source reads = %d, want no upstream request from a non-owner", got)
	}

	// The other holder released the lease, so the next run owns it again.
	report, err := runtime.RefreshSource(context.Background())
	if err != nil {
		t.Fatalf("RefreshSource after the lease returned: %v", err)
	}
	if !report.Published {
		t.Fatalf("report = %+v, want a published generation", report)
	}
	if state := runtime.lease.status(); state != leaseHeld {
		t.Errorf("lease state = %q, want %q", state, leaseHeld)
	}
	if runtime.lease.epoch() == 0 {
		t.Error("the reacquired lease carries no epoch")
	}
	if _, found := runtime.Catalog().Providers().Get("owner-provider"); !found {
		t.Error("the published catalog lost the source provider")
	}
}
