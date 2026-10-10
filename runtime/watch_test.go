package runtime

import (
	"sync"
	"testing"
	"time"
)

// watchDeadline bounds the wait from a wake to the next source read. No
// durable publication runs in that window, and the read starts within
// milliseconds. A longer wait means that the wake never arrived.
const watchDeadline = 5 * time.Second

// watchSource is a stub source that also reports upstream change and adopts a
// fleet identity. It proves the two optional source roles without a network.
type watchSource struct {
	*stubSource

	changes chan struct{}

	mu        sync.Mutex
	adopted   string
	adoptions int
}

func newWatchSource(identity string) *watchSource {
	return &watchSource{
		stubSource: newStubSource(identity),
		changes:    make(chan struct{}, 1),
	}
}

// Changes reports each upstream change as one wake.
func (w *watchSource) Changes() <-chan struct{} { return w.changes }

// AdoptInstanceIdentity records the identity the runtime handed over.
func (w *watchSource) AdoptInstanceIdentity(instance string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.adopted = instance
	w.adoptions++
}

// adoptedIdentity returns the recorded identity and how often the runtime
// handed one over.
func (w *watchSource) adoptedIdentity() (string, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.adopted, w.adoptions
}

// wake reports one upstream change.
func (w *watchSource) wake() { w.changes <- struct{}{} }

// TestRuntimeRefreshesOnAnUpstreamWake proves the reactive path of a cascade.
// A source that reports its own change wakes the source worker, so a streamed
// publication reaches the runtime without waiting for the poll boundary. The
// schedule timer of the runtime never fires, so only the wake can produce the
// second read.
func TestRuntimeRefreshesOnAnUpstreamWake(t *testing.T) {
	t.Parallel()
	published := time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)
	first := testCatalogPayload(t, "wake-provider", "wake-model", "Wake Model")
	second := testCatalogPayload(t, "wake-provider", "wake-model", "Wake Model Two")
	source := newWatchSource("watch-source")
	source.replies = []SourceRead{
		testSourceRead(t, "generation-1", first, published),
		testSourceRead(t, "generation-2", second, published.Add(time.Minute)),
	}

	timer := newStubScheduleTimer()

	// The require_source policy reads and publishes once before Open returns.
	// The timed wait then excludes the durable work of that first read.
	runtime := openTestRuntime(t,
		WithSource(source),
		WithSourceStartupPolicy(string(StartupRequireSource)),
		WithSourcePollInterval(time.Hour),
		WithStartupSpread(0),
		withScheduleTimer(timer.after),
	)
	if got := source.readCount(); got != 1 {
		t.Fatalf("source reads after Open = %d, want 1", got)
	}
	if got := runtime.Status().GenerationID; got == "" {
		t.Fatal("the runtime published no generation before the wake")
	}

	source.wake()
	waitForReads(t, source, 2)
}

// TestRuntimeHandsItsInstanceIdentityToTheSource proves that one replica
// spreads its runtime work and its source work on one identity. Open hands the
// derived identity over before the first read.
func TestRuntimeHandsItsInstanceIdentityToTheSource(t *testing.T) {
	t.Parallel()
	source := newWatchSource("adopting-source")
	runtime := openTestRuntime(t, WithSource(source))

	identity := runtime.Status().InstanceIdentity
	if identity == "" {
		t.Fatal("the runtime derived no instance identity")
	}
	adopted, adoptions := source.adoptedIdentity()
	if adopted != identity {
		t.Fatalf("adopted identity = %q, want the runtime identity %q", adopted, identity)
	}
	if adoptions != 1 {
		t.Fatalf("adoptions = %d, want exactly one", adoptions)
	}
}

// waitForReads blocks until the source answered the wanted number of reads.
func waitForReads(t *testing.T, source *watchSource, want int) {
	t.Helper()
	deadline := time.Now().Add(watchDeadline)
	for time.Now().Before(deadline) {
		if source.readCount() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("source reads = %d, want %d", source.readCount(), want)
}
