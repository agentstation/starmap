package runtime

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/fleet"
	"github.com/agentstation/starmap/pkg/errors"
)

// TestStartupSpreadDefaultsToFifteenMinutes proves the fleet startup spread.
// Every instance waits inside a fifteen-minute window before its first run, so
// a fleet restart never sends one burst of requests upstream.
func TestStartupSpreadDefaultsToFifteenMinutes(t *testing.T) {
	t.Parallel()

	const want = 15 * time.Minute
	if fleet.DefaultStartupSpread != want {
		t.Fatalf("fleet.DefaultStartupSpread = %s, want %s", fleet.DefaultStartupSpread, want)
	}
	if got := defaults().startupSpread; got != want {
		t.Fatalf("default startup spread = %s, want %s", got, want)
	}

	runtime, err := Open(context.Background(),
		WithStateDirectory(privateRuntimeDirectory(t)),
		WithCatalogSource("embedded"),
		WithSourcePollInterval(0),
		WithAcquisitionEnabled(false),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	offsets := map[string]time.Duration{
		controllerSource:      runtime.schedule.sourceOffset,
		controllerAcquisition: runtime.schedule.acquisitionOffset,
	}
	for controller, offset := range offsets {
		if offset < 0 || offset >= want {
			t.Errorf("%s startup offset = %s, want a value inside [0, %s)", controller, offset, want)
		}
	}
	if runtime.schedule.sourceOffset == runtime.schedule.acquisitionOffset {
		t.Error("the two controllers must not share one startup offset")
	}
}

// TestStablePhaseSurvivesRestart proves that one instance keeps its place
// inside the refresh interval across a restart. The phase rests on the durable
// instance seed, so a fleet keeps its spread after a deployment.
func TestStablePhaseSurvivesRestart(t *testing.T) {
	t.Parallel()

	directory := privateRuntimeDirectory(t)
	const interval = time.Hour

	open := func() (identity string, sourcePhase, acquisitionPhase time.Duration) {
		t.Helper()
		runtime, err := Open(context.Background(),
			WithStateDirectory(directory),
			WithCatalogSource("embedded"),
			WithListenAddress("127.0.0.1:8080"),
			WithStartupSpread(0),
			WithSourcePollInterval(interval),
			WithAcquisitionEnabled(true),
			WithAcquisitionInterval(interval),
		)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer func() {
			if err := runtime.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		}()
		return runtime.schedule.identity.Instance,
			runtime.schedule.sourcePhase,
			runtime.schedule.acquisitionPhase
	}

	firstIdentity, firstSource, firstAcquisition := open()
	secondIdentity, secondSource, secondAcquisition := open()

	if firstIdentity == "" {
		t.Fatal("the runtime derived no instance identity")
	}
	if firstIdentity != secondIdentity {
		t.Errorf("instance identity moved across a restart: %q then %q", firstIdentity, secondIdentity)
	}
	if firstSource != secondSource {
		t.Errorf("source phase moved across a restart: %s then %s", firstSource, secondSource)
	}
	if firstAcquisition != secondAcquisition {
		t.Errorf("acquisition phase moved across a restart: %s then %s",
			firstAcquisition, secondAcquisition)
	}
	if firstSource < 0 || firstSource >= interval {
		t.Errorf("source phase = %s, want a value inside [0, %s)", firstSource, interval)
	}
	if firstSource == firstAcquisition {
		t.Error("the two controllers must not share one phase")
	}

	// The seed is the durable part of the identity.
	seed := filepath.Join(directory, instanceSeedFileName)
	if _, err := os.Stat(seed); err != nil {
		t.Fatalf("the runtime retained no instance seed: %v", err)
	}
}

// TestSchedulerRejectsClonedStateWithoutOwner refuses a copied seed without an owner record.
func TestSchedulerRejectsClonedStateWithoutOwner(t *testing.T) {
	t.Parallel()
	origin := privateRuntimeDirectory(t)
	first := openTestRuntime(t, WithStateDirectory(origin))
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	seed, err := os.ReadFile(filepath.Join(origin, instanceSeedFileName))
	if err != nil {
		t.Fatal(err)
	}
	clone := privateRuntimeDirectory(t)
	target := filepath.Join(clone, instanceSeedFileName)
	if err := os.WriteFile(target, seed, constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	connected, err := Open(t.Context(), WithStateDirectory(clone), WithCatalogSource("embedded"), WithListenAddress("127.0.0.1:9090"))
	if connected != nil {
		_ = connected.Close()
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("unowned clone = %v, want migration conflict", err)
	}
	after, err := os.ReadFile(target)
	if err != nil || string(after) != string(seed) {
		t.Fatal("clone refusal changed the seed")
	}
	if _, err := os.Stat(filepath.Join(clone, ownerRecordName)); !os.IsNotExist(err) {
		t.Fatal("clone refusal claimed ownership")
	}
}

// TestRequireSourceFailsOpenWhenTheSourceFails proves that require_source
// blocks inside the Open context and reads the source once. A deployment that
// needs upstream state fails instead of serving the embedded baseline.
func TestRequireSourceFailsOpenWhenTheSourceFails(t *testing.T) {
	t.Parallel()

	source := newStubSource("required-source")
	source.errs = []error{stderrors.New("the channel refused the request")}

	runtime, err := Open(context.Background(),
		WithStateDirectory(privateRuntimeDirectory(t)),
		WithSource(source),
		WithSourceStartupPolicy("require_source"),
		WithSourcePollInterval(time.Hour),
		WithStartupSpread(0),
		WithAcquisitionEnabled(false),
		WithRefreshTimeout(200*time.Millisecond),
		WithRandom(func() float64 { return 0 }),
	)
	if err == nil {
		if closeErr := runtime.Close(); closeErr != nil {
			t.Errorf("Close: %v", closeErr)
		}
		t.Fatal("Open served the embedded baseline under require_source")
	}
	if runtime != nil {
		t.Error("a failed Open returned a runtime")
	}
	var resource *errors.ResourceError
	if !stderrors.As(err, &resource) {
		t.Errorf("Open error = %T (%v), want *errors.ResourceError", err, err)
	}
	if source.readCount() == 0 {
		t.Error("require_source read no source before it failed Open")
	}
}

// TestPreferSourceReadsOnceOnAColdStart proves that a runtime without retained
// source evidence reads once right after its startup offset. It then waits for
// its stable full-interval phase.
func TestPreferSourceReadsOnceOnAColdStart(t *testing.T) {
	t.Parallel()

	source := newStubSource("cold-source")
	source.replies = []SourceRead{{Health: HealthOK}}
	timer := newStubScheduleTimer()

	runtime := openTestRuntime(t,
		WithSource(source),
		WithSourcePollInterval(time.Hour),
		withScheduleTimer(timer.after),
	)

	eventually(t, 5*time.Second, "the cold runtime never read its source", func() bool {
		return source.readCount() == 1
	})
	if wait := timer.waited(t, 5*time.Second); wait <= 0 || wait > time.Hour {
		t.Errorf("next source run = %s, want a wait inside the poll interval", wait)
	}
	if got := runtime.Status().SourceHealth; got != HealthOK {
		t.Errorf("source health = %q, want %q", got, HealthOK)
	}
}

// TestFreshDurableEvidenceWaitsForItsPhase proves that a restart with fresh
// retained evidence sends no immediate upstream request. It waits for its
// stable full-interval phase instead.
func TestFreshDurableEvidenceWaitsForItsPhase(t *testing.T) {
	t.Parallel()

	directory := privateRuntimeDirectory(t)
	payload := testCatalogPayload(t, "phase-provider", "phase-model", "Phase Model")

	// The first runtime retains one fresh source layer.
	first := newStubSource("phase-source")
	first.replies = []SourceRead{testSourceRead(t, "generation-phase", payload, time.Now().UTC())}
	warm, err := Open(context.Background(),
		WithStateDirectory(directory),
		WithSource(first),
		WithSourcePollInterval(0),
		WithStartupSpread(0),
		WithAcquisitionEnabled(false),
	)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := warm.RefreshSource(context.Background()); err != nil {
		t.Fatalf("RefreshSource: %v", err)
	}
	if err := warm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The restart finds fresh evidence, so its worker waits for the phase.
	second := newStubSource("phase-source")
	second.replies = []SourceRead{{Health: HealthOK}}
	timer := newStubScheduleTimer()
	restarted := openTestRuntime(t,
		WithStateDirectory(directory),
		WithSource(second),
		WithSourcePollInterval(time.Hour),
		withScheduleTimer(timer.after),
	)
	if wait := timer.waited(t, 5*time.Second); wait <= 0 || wait > time.Hour {
		t.Errorf("next source run = %s, want a wait inside the poll interval", wait)
	}
	if got := second.readCount(); got != 0 {
		t.Errorf("source reads = %d, want none before the phase", got)
	}
	if _, found := restarted.Catalog().Providers().Get("phase-provider"); !found {
		t.Error("the restart lost its retained source layer")
	}
}

// TestZeroAcquisitionIntervalRunsOneStartupPass proves the canonical contract
// row of an enabled policy with a zero interval. The worker observes providers
// one time and runs no periodic work.
func TestZeroAcquisitionIntervalRunsOneStartupPass(t *testing.T) {
	t.Parallel()

	acquirer := &stubAcquirer{}
	timer := newStubScheduleTimer()
	openTestRuntime(t,
		WithAcquirer(acquirer),
		WithAcquisitionEnabled(true),
		WithAcquisitionInterval(0),
		withScheduleTimer(timer.after),
	)

	eventually(t, 5*time.Second, "the startup pass never observed providers", func() bool {
		return acquirer.callCount() >= 1
	})
	time.Sleep(100 * time.Millisecond)
	if got := acquirer.callCount(); got != 1 {
		t.Errorf("acquisition runs = %d, want one startup pass", got)
	}
	select {
	case wait := <-timer.waits:
		t.Errorf("the worker waited %s, want no periodic work", wait)
	default:
	}
}

// TestRequireSourceOpensAsANonOwner proves that require_source names the
// evidence the runtime needs, not the lease. A replica that another instance
// owns opens without a read and consumes the state that the owner publishes.
func TestRequireSourceOpensAsANonOwner(t *testing.T) {
	t.Parallel()

	leases := &stubLeaseStore{}
	leases.refuseEvery()
	source := newStubSource("required-source")
	source.errs = []error{stderrors.New("the channel refused the request")}

	runtime, err := Open(context.Background(),
		WithStateDirectory(privateRuntimeDirectory(t)),
		WithSource(source),
		WithLeaseStore(leases),
		WithSourceStartupPolicy("require_source"),
		WithSourcePollInterval(0),
		WithStartupSpread(0),
		WithAcquisitionEnabled(false),
		WithRandom(func() float64 { return 0 }),
	)
	if err != nil {
		t.Fatalf("Open as a non-owner: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := runtime.Close(); closeErr != nil {
			t.Errorf("Close: %v", closeErr)
		}
	})
	if got := source.readCount(); got != 0 {
		t.Errorf("source reads = %d, want no read from a non-owner", got)
	}
	if got := runtime.Status().Lease; got != string(leaseLost) {
		t.Errorf("status lease = %q, want %q", got, leaseLost)
	}
	if runtime.Catalog() == nil {
		t.Error("a non-owner replica served no catalog")
	}
}
