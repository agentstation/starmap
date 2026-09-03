package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// TestOpenReturnsEmbeddedStateBeforeSourceReply proves that Open returns a
// usable runtime while the source read still blocks. A consumer serves the
// verified embedded catalog without a network wait.
func TestOpenReturnsEmbeddedStateBeforeSourceReply(t *testing.T) {
	source := newStubSource("blocking-source")
	source.release = make(chan struct{})
	source.replies = []SourceRead{{Health: HealthOK}}

	runtime := openTestRuntime(t,
		WithSource(source),
		WithSourcePollInterval(time.Millisecond),
	)

	// The scheduled read is in flight and cannot answer yet.
	select {
	case <-source.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the scheduled source read never started")
	}

	catalog := runtime.Catalog()
	if catalog == nil {
		t.Fatal("Catalog returned nil while the source read was in flight")
	}
	if _, err := catalog.FindModel("gpt-4o"); err != nil {
		t.Fatalf("the embedded catalog is not served before the source reply: %v", err)
	}
	state := runtime.State()
	if state.GenerationID == "" {
		t.Fatal("State returned no generation identity before the source reply")
	}
	if state.Catalog != catalog {
		t.Fatal("Catalog and State returned different generations")
	}
	if !runtime.Status().Usable {
		t.Fatal("the runtime is not usable before the source reply")
	}
	if runtime.Status().FallbackReason != FallbackAwaitingSource {
		t.Fatalf("fallback reason = %q, want %q",
			runtime.Status().FallbackReason, FallbackAwaitingSource)
	}

	close(source.release)
}

// TestRuntimeReadsReachNoExternalSystem proves that Catalog, State, and Status
// answer from retained state alone.
func TestRuntimeReadsReachNoExternalSystem(t *testing.T) {
	source := newStubSource("counted-source")
	acquirer := &stubAcquirer{}

	runtime := openTestRuntime(t, WithSource(source), WithAcquirer(acquirer))

	for range 64 {
		if runtime.Catalog() == nil {
			t.Fatal("Catalog returned nil")
		}
		if runtime.State().Catalog == nil {
			t.Fatal("State returned no catalog")
		}
		if !runtime.Status().Usable {
			t.Fatal("Status reported an unusable runtime")
		}
	}
	if reads := source.readCount(); reads != 0 {
		t.Fatalf("source reads = %d, want 0 for read-only calls", reads)
	}
	if calls := acquirer.callCount(); calls != 0 {
		t.Fatalf("acquisition runs = %d, want 0 for read-only calls", calls)
	}
}

// TestRuntimeRefreshMethodsChangeDistinctLayers proves that RefreshSource
// changes the source layer only, Sync changes the provider layers only, and
// Refresh changes both.
func TestRuntimeRefreshMethodsChangeDistinctLayers(t *testing.T) {
	published := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	sourcePayload := testCatalogPayload(t, "source-provider", "source-model", "Source Model")
	source := newStubSource("layer-source")
	source.replies = []SourceRead{testSourceRead(t, "generation-1", sourcePayload, published)}

	layer := testProviderLayer(t, "acquired-provider", "acquired-model", "Acquired Model", published)
	acquirer := &stubAcquirer{result: AcquisitionResult{
		Eligible: 1,
		Attempts: []sources.ProviderAttempt{
			testAttempt("acquired-provider", sources.ProviderOutcomeSucceeded, ""),
		},
		Layers: []ProviderLayer{layer},
	}}

	runtime := openTestRuntime(t, WithSource(source), WithAcquirer(acquirer))

	if hasSourceLayer(runtime) || providerLayerCount(runtime) != 0 {
		t.Fatal("a new runtime already retains a layer")
	}

	if _, err := runtime.RefreshSource(context.Background()); err != nil {
		t.Fatalf("RefreshSource: %v", err)
	}
	if !hasSourceLayer(runtime) {
		t.Fatal("RefreshSource left the source layer empty")
	}
	if count := providerLayerCount(runtime); count != 0 {
		t.Fatalf("provider layers after RefreshSource = %d, want 0", count)
	}
	if _, found := runtime.Catalog().Providers().Get("source-provider"); !found {
		t.Fatal("the effective catalog lost the source layer")
	}

	if _, err := runtime.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if count := providerLayerCount(runtime); count != 1 {
		t.Fatalf("provider layers after Sync = %d, want 1", count)
	}
	if reads := source.readCount(); reads != 1 {
		t.Fatalf("source reads after Sync = %d, want 1", reads)
	}
	for _, id := range []catalogs.ProviderID{"source-provider", "acquired-provider"} {
		if _, found := runtime.Catalog().Providers().Get(id); !found {
			t.Fatalf("the effective catalog lost provider %s", id)
		}
	}

	report, err := runtime.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if report.Source.RunID != report.RunID || report.Acquisition.RunID != report.RunID {
		t.Fatal("Refresh reported different run identities for its two layers")
	}
	if reads := source.readCount(); reads != 2 {
		t.Fatalf("source reads after Refresh = %d, want 2", reads)
	}
	if calls := acquirer.callCount(); calls != 2 {
		t.Fatalf("acquisition runs after Refresh = %d, want 2", calls)
	}
}

// TestRuntimeSyncReturnsAcquisitionReport proves that Sync reports every
// terminal provider outcome and the publication it produced.
func TestRuntimeSyncReturnsAcquisitionReport(t *testing.T) {
	observed := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	layer := testProviderLayer(t, "answered", "answered-model", "Answered Model", observed)
	acquirer := &stubAcquirer{result: AcquisitionResult{
		Eligible: 3,
		Attempts: []sources.ProviderAttempt{
			testAttempt("answered", sources.ProviderOutcomeSucceeded, ""),
			testAttempt("unconfigured", sources.ProviderOutcomeSkippedNotConfigured,
				sources.ProviderReasonCredentialUnavailable),
			testAttempt("refused", sources.ProviderOutcomeFailed,
				sources.ProviderReasonCredentialRejected),
		},
		Layers: []ProviderLayer{layer},
	}}

	runtime := openTestRuntime(t, WithSource(newStubSource("idle")), WithAcquirer(acquirer))

	report, err := runtime.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if report.RunID == "" {
		t.Fatal("the acquisition report carries no run identity")
	}
	if report.Eligible != 3 {
		t.Fatalf("eligible = %d, want 3", report.Eligible)
	}
	if report.Succeeded != 1 || report.Skipped != 1 || report.Failed != 1 {
		t.Fatalf("outcome counts = %d/%d/%d, want 1/1/1",
			report.Succeeded, report.Skipped, report.Failed)
	}
	if !report.Published || report.GenerationID == "" {
		t.Fatal("the acquisition report says nothing was published")
	}
	if report.Health != HealthDegraded {
		t.Fatalf("health = %q, want %q", report.Health, HealthDegraded)
	}
	if report.CompletedAt.Before(report.StartedAt) {
		t.Fatal("the acquisition report completed before it started")
	}
	if len(report.Attempts) != 3 {
		t.Fatalf("attempts = %d, want 3", len(report.Attempts))
	}
}

// TestRuntimeCloseJoinsWithinFiveSeconds proves that Close stops runtime-owned
// work inside its bound and that a second call changes nothing.
func TestRuntimeCloseJoinsWithinFiveSeconds(t *testing.T) {
	source := newStubSource("scheduled")
	acquirer := &stubAcquirer{}
	runtime, err := Open(context.Background(),
		WithStateDirectory(t.TempDir()),
		WithStartupSpread(0),
		WithSource(source),
		WithAcquirer(acquirer),
		WithSourcePollInterval(time.Hour),
		WithAcquisitionInterval(time.Hour),
		WithLeaseStore(&stubLeaseStore{}),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	started := time.Now()
	if err := runtime.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	elapsed := time.Since(started)
	if elapsed >= closeJoinTimeout {
		t.Fatalf("Close joined in %s, want less than %s", elapsed, closeJoinTimeout)
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, ok := <-runtime.Updates(); ok {
		t.Fatal("Updates stayed open after Close")
	}
}

// TestRuntimeRetainsLayersUnderRace proves that concurrent refresh and read
// calls keep every retained layer and publish one whole generation at a time.
func TestRuntimeRetainsLayersUnderRace(t *testing.T) {
	published := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	sourcePayload := testCatalogPayload(t, "race-source", "race-source-model", "Race Source")
	source := newStubSource("race-source")
	source.replies = []SourceRead{testSourceRead(t, "generation-race", sourcePayload, published)}

	layer := testProviderLayer(t, "race-provider", "race-provider-model", "Race Provider", published)
	acquirer := &stubAcquirer{result: AcquisitionResult{
		Eligible: 1,
		Attempts: []sources.ProviderAttempt{
			testAttempt("race-provider", sources.ProviderOutcomeSucceeded, ""),
		},
		Layers: []ProviderLayer{layer},
	}}

	runtime := openTestRuntime(t, WithSource(source), WithAcquirer(acquirer))

	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			for range 16 {
				if _, err := runtime.RefreshSource(context.Background()); err != nil {
					t.Errorf("RefreshSource: %v", err)
					return
				}
				if _, err := runtime.Sync(context.Background()); err != nil {
					t.Errorf("Sync: %v", err)
					return
				}
			}
		})
		group.Go(func() {
			for range 64 {
				state := runtime.State()
				if state.Catalog == nil {
					t.Error("State returned no catalog during a refresh")
					return
				}
				if state.GenerationID == "" {
					t.Error("State returned a generation without an identity")
					return
				}
			}
		})
	}
	group.Wait()

	for _, id := range []catalogs.ProviderID{"race-source", "race-provider"} {
		if _, found := runtime.Catalog().Providers().Get(id); !found {
			t.Fatalf("the effective catalog lost provider %s under race", id)
		}
	}
	if !hasSourceLayer(runtime) {
		t.Fatal("the source layer did not survive the race")
	}
	if count := providerLayerCount(runtime); count != 1 {
		t.Fatalf("provider layers = %d, want 1", count)
	}
}

// TestRefreshJoinsActiveRunAndCancels proves that a second refresh joins the
// run in flight and that cancellation stops the shared run.
func TestRefreshJoinsActiveRunAndCancels(t *testing.T) {
	source := newStubSource("joined")
	source.release = make(chan struct{})

	runtime := openTestRuntime(t, WithSource(source))

	first := make(chan RefreshReport, 1)
	go func() {
		report, err := runtime.RefreshSource(context.Background())
		if err != nil {
			t.Errorf("owner RefreshSource: %v", err)
		}
		first <- RefreshReport{RunID: report.RunID}
	}()

	select {
	case <-source.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the owning run never reached the source")
	}

	// A second caller of the same kind joins the run in flight. The run group
	// reports the join, so the assertion needs no timing guess.
	shared, owns, err := runtime.runs.start(
		context.Background(), runtime.ctx, runKindSource, "second-caller", 0)
	if err != nil {
		t.Fatalf("join the active run: %v", err)
	}
	if owns {
		t.Fatal("the second caller started a second run")
	}

	second := make(chan RefreshReport, 1)
	go func() {
		report, joinErr := shared.join(context.Background())
		if joinErr != nil {
			t.Errorf("join the active run: %v", joinErr)
		}
		second <- report
	}()

	close(source.release)
	owner := <-first
	joiner := <-second
	if owner.RunID == "" {
		t.Fatal("the owning run carries no identity")
	}
	if owner.RunID != joiner.RunID {
		t.Fatalf("run identities = %q and %q, want one shared run", owner.RunID, joiner.RunID)
	}
	if reads := source.readCount(); reads != 1 {
		t.Fatalf("source reads = %d, want 1 for one joined run", reads)
	}

	// A cancelled caller stops the shared run instead of leaking it.
	blocking := newStubSource("cancelled")
	blocking.release = make(chan struct{})
	cancellable := openTestRuntime(t, WithSource(blocking))
	ctx, cancel := context.WithCancel(context.Background())
	failed := make(chan error, 1)
	go func() {
		_, err := cancellable.RefreshSource(ctx)
		failed <- err
	}()
	select {
	case <-blocking.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the cancellable run never reached the source")
	}
	cancel()
	select {
	case err := <-failed:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled RefreshSource error = %v, want context cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not stop the run")
	}
	close(blocking.release)
}

// TestRefreshAddsNoDeadlineByDefault proves that a refresh carries no deadline
// unless the deployment configures one.
func TestRefreshAddsNoDeadlineByDefault(t *testing.T) {
	var deadlines []bool
	var mu sync.Mutex
	source := newStubSource("deadline")
	source.observe = func(ctx context.Context) {
		_, ok := ctx.Deadline()
		mu.Lock()
		deadlines = append(deadlines, ok)
		mu.Unlock()
	}

	runtime := openTestRuntime(t, WithSource(source))
	if _, err := runtime.RefreshSource(context.Background()); err != nil {
		t.Fatalf("RefreshSource: %v", err)
	}
	mu.Lock()
	first := deadlines[0]
	mu.Unlock()
	if first {
		t.Fatal("the default refresh carried a deadline")
	}

	bounded := newStubSource("bounded")
	var boundedDeadline bool
	bounded.observe = func(ctx context.Context) {
		_, boundedDeadline = ctx.Deadline()
	}
	limited := openTestRuntime(t, WithSource(bounded), WithRefreshTimeout(time.Minute))
	if _, err := limited.RefreshSource(context.Background()); err != nil {
		t.Fatalf("bounded RefreshSource: %v", err)
	}
	if !boundedDeadline {
		t.Fatal("a configured refresh timeout added no deadline")
	}
	if DefaultRefreshTimeout != 0 {
		t.Fatalf("DefaultRefreshTimeout = %s, want zero", DefaultRefreshTimeout)
	}
}

// hasSourceLayer reports whether the runtime retains an upstream layer.
func hasSourceLayer(runtime *Runtime) bool {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.layers.source != nil
}

// providerLayerCount returns how many provider layers the runtime retains.
func providerLayerCount(runtime *Runtime) int {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return len(runtime.layers.providers)
}

// TestCloseNeverRacesACallerOwnedRun proves that Close never closes the
// publication channel under a run that a caller owns. The acquirer ignores its
// context and answers after Close started, exactly as a slow provider client
// can, and the run still returns a typed result.
func TestCloseNeverRacesACallerOwnedRun(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	acquirer := &stubAcquirer{entered: make(chan struct{})}
	acquirer.observe = func(context.Context) { <-release }
	acquirer.result = AcquisitionResult{
		Eligible: 1,
		Attempts: []sources.ProviderAttempt{
			testAttempt("race-provider", sources.ProviderOutcomeSucceeded, ""),
		},
		Layers: []ProviderLayer{
			testProviderLayer(t, "race-provider", "race-model", "Race Model", time.Now().UTC()),
		},
	}

	runtime := openTestRuntime(t, WithAcquirer(acquirer))

	type outcome struct {
		report AcquisitionReport
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		report, err := runtime.Sync(context.Background())
		done <- outcome{report: report, err: err}
	}()

	select {
	case <-acquirer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the caller-owned run never reached the acquirer")
	}

	// Close runs while the caller-owned run still publishes.
	closed := make(chan error, 1)
	go func() { closed <- runtime.Close() }()
	close(release)

	select {
	case err := <-closed:
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close never returned")
	}

	select {
	case result := <-done:
		if result.report.RunID == "" {
			t.Error("the run returned no report identity")
		}
		if result.err != nil && !typedRuntimeError(result.err) {
			t.Errorf("Sync error = %T (%v), want a typed result", result.err, result.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the caller-owned run never returned")
	}
}

// typedRuntimeError reports whether one runtime failure carries a defined type.
// A cancelled run reports the standard cancellation instead.
func typedRuntimeError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var validation *pkgerrors.ValidationError
	var conflict *pkgerrors.ConflictError
	var resource *pkgerrors.ResourceError
	var config *pkgerrors.ConfigError
	return errors.As(err, &validation) ||
		errors.As(err, &conflict) ||
		errors.As(err, &resource) ||
		errors.As(err, &config)
}
