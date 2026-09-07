package runtime

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/sources"
)

func TestCloseRejectsLateProviderPublication(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	acquirer := &stubAcquirer{entered: make(chan struct{})}
	acquirer.observe = func(context.Context) { <-release }
	acquirer.result = AcquisitionResult{
		Eligible: 1,
		Attempts: []sources.ProviderAttempt{testAttempt("late-provider", sources.ProviderOutcomeSucceeded, "")},
		Layers:   []ProviderLayer{testProviderLayer(t, "late-provider", "late-model", "Late Model", time.Now().UTC())},
	}
	connected := openTestRuntime(t, WithAcquirer(acquirer))
	before := connected.State().GenerationID
	finished := make(chan error, 1)
	go func() { _, err := connected.Sync(context.Background()); finished <- err }()
	select {
	case <-acquirer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not start")
	}
	closed := make(chan error, 1)
	go func() { closed <- connected.Close() }()
	<-connected.ctx.Done()
	close(release)
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Errorf("late provider result = %v, want cancellation", err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if connected.State().GenerationID != before || providerLayerCount(connected) != 0 {
		t.Fatal("shutdown retained or published a late provider result")
	}
	connected.mu.RLock()
	succeeded := !connected.report.acquisitionSucceededAt.IsZero()
	connected.mu.RUnlock()
	if succeeded {
		t.Fatal("canceled publication advanced successful acquisition freshness")
	}
}

func TestSourceRetentionRejectsCancellationAfterRead(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var readFinished atomic.Bool
	source := newStubSource("cancel-after-read")
	source.observe = func(context.Context) { readFinished.Store(true) }
	now := time.Now().UTC()
	source.replies = []SourceRead{testSourceRead(t, "late-generation", testCatalogPayload(t, "late-provider", "late-model", "Late Model"), now)}
	connected := openTestRuntime(t, WithSource(source), WithClock(func() time.Time {
		if readFinished.Swap(false) {
			cancel()
		}
		return now
	}))
	before := connected.State().GenerationID
	var report RefreshReport
	err := connected.readSource(ctx, &report, connected.lease.epoch())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("source retention = %v, want cancellation", err)
	}
	retained, err := connected.store.loadSource()
	if err != nil {
		t.Fatal(err)
	}
	if retained != nil || connected.layers.source != nil || connected.State().GenerationID != before {
		t.Fatal("canceled source read changed durable or active catalog state")
	}
}

func TestCloseRejectsLateSourcePublication(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	source := newStubSource("late-source")
	source.observe = func(context.Context) { <-release }
	source.replies = []SourceRead{testSourceRead(t, "late-generation", testCatalogPayload(t, "late-provider", "late-model", "Late Model"), time.Now().UTC())}
	connected := openTestRuntime(t, WithSource(source))
	before := connected.State().GenerationID
	finished := make(chan error, 1)
	go func() { _, err := connected.RefreshSource(context.Background()); finished <- err }()
	select {
	case <-source.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("source did not start")
	}
	closed := make(chan error, 1)
	go func() { closed <- connected.Close() }()
	<-connected.ctx.Done()
	close(release)
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Errorf("late source result = %v, want cancellation", err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	connected.mu.RLock()
	retained := connected.layers.source != nil
	connected.mu.RUnlock()
	if connected.State().GenerationID != before || retained {
		t.Fatal("shutdown retained or published a late source result")
	}
}
