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
