package starmap

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// stubSource is a deployment-owned source under test control. It records every
// read, so a test can prove that a read reached the source or did not.
type stubSource struct {
	identity string

	mu      sync.Mutex
	reads   int
	replies []SourceRead
	errs    []error

	// started signals the first read. entered closes once.
	started sync.Once
	entered chan struct{}

	// release blocks every read until a test closes it.
	release chan struct{}

	// observe runs inside the read, so a test can inspect the run context.
	observe func(context.Context)
}

func newStubSource(identity string) *stubSource {
	return &stubSource{identity: identity, entered: make(chan struct{})}
}

// Identity returns the safe identity of the stub.
func (s *stubSource) Identity() string { return s.identity }

// Read returns the next scripted reply.
func (s *stubSource) Read(ctx context.Context) (SourceRead, error) {
	s.started.Do(func() { close(s.entered) })
	if s.observe != nil {
		s.observe(ctx)
	}
	if s.release != nil {
		select {
		case <-s.release:
		case <-ctx.Done():
			return SourceRead{}, ctx.Err()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	var reply SourceRead
	var err error
	if len(s.replies) > 0 {
		reply = s.replies[0]
		if len(s.replies) > 1 {
			s.replies = s.replies[1:]
		}
	}
	if len(s.errs) > 0 {
		err = s.errs[0]
		if len(s.errs) > 1 {
			s.errs = s.errs[1:]
		}
	}
	return reply, err
}

// readCount returns how many reads the source answered.
func (s *stubSource) readCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reads
}

// stubAcquirer is a provider acquisition under test control.
type stubAcquirer struct {
	mu       sync.Mutex
	calls    int
	requests []AcquisitionRequest
	result   AcquisitionResult
	err      error
	observe  func(context.Context)

	// started signals the first run. entered closes once.
	started sync.Once
	entered chan struct{}
}

// AcquireProviders returns the scripted acquisition result.
func (a *stubAcquirer) AcquireProviders(ctx context.Context, request AcquisitionRequest) (AcquisitionResult, error) {
	if a.entered != nil {
		a.started.Do(func() { close(a.entered) })
	}
	if a.observe != nil {
		a.observe(ctx)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.requests = append(a.requests, request)
	return a.result, a.err
}

// callCount returns how many acquisition runs the stub answered.
func (a *stubAcquirer) callCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

// stubLeaseStore is a shared-storage lease under test control.
type stubLeaseStore struct {
	mu       sync.Mutex
	epoch    uint64
	holder   string
	renewErr error
	expiry   time.Time

	// takes counts every acquisition the store answered.
	takes int

	// refusals is how many acquisitions refuse before one succeeds. refuseAll
	// refuses every acquisition, so the instance never owns the lease.
	refusals  int
	refuseAll bool
}

// AcquireLease takes the lease and increases the epoch. A scripted refusal
// answers with the conflict that another holder produces.
func (s *stubLeaseStore) AcquireLease(_ context.Context, holder string, ttl time.Duration) (Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.takes++
	if s.refuseAll || s.refusals > 0 {
		if s.refusals > 0 {
			s.refusals--
		}
		return Lease{}, &errors.ConflictError{
			Resource: "runtime lease",
			Expected: string(leaseHeld),
			Actual:   string(leaseLost),
			Message:  "another instance owns the runtime lease",
		}
	}
	s.epoch++
	s.holder = holder
	expires := s.expiry
	if expires.IsZero() {
		expires = time.Now().Add(ttl)
	}
	return Lease{Holder: holder, Epoch: s.epoch, ExpiresAt: expires}, nil
}

// Renew extends the lease, or fails when a test scripted a loss.
func (s *stubLeaseStore) Renew(_ context.Context, lease Lease, ttl time.Duration) (Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.renewErr != nil {
		return Lease{}, s.renewErr
	}
	lease.ExpiresAt = time.Now().Add(ttl)
	lease.Epoch = s.epoch
	return lease, nil
}

// Release returns the lease.
func (s *stubLeaseStore) Release(context.Context, Lease) error { return nil }

// failLater scripts the next renewal to fail.
func (s *stubLeaseStore) failLater(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renewErr = err
}

// refuse scripts how many acquisitions another holder wins.
func (s *stubLeaseStore) refuse(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refusals = count
}

// refuseEvery scripts a store that another instance always owns.
func (s *stubLeaseStore) refuseEvery() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refuseAll = true
}

// acquireCount returns how many acquisitions the store answered.
func (s *stubLeaseStore) acquireCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.takes
}

// bumpEpoch simulates another instance taking the lease.
func (s *stubLeaseStore) bumpEpoch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epoch++
}

// testCatalogPayload returns a canonical payload that holds one provider and
// one model. Tests use it as an upstream generation and as a provider layer.
func testCatalogPayload(t testing.TB, providerID catalogs.ProviderID, modelID, name string) []byte {
	t.Helper()
	builder := catalogs.NewEmpty()
	model := catalogs.Model{ID: modelID, Name: name}
	if err := builder.SetProvider(catalogs.Provider{
		ID:     providerID,
		Name:   string(providerID),
		Models: map[string]*catalogs.Model{modelID: &model},
	}); err != nil {
		t.Fatalf("SetProvider(%s): %v", providerID, err)
	}
	payload, err := catalogs.EncodeCatalogPayload(mustTestCatalog(t, builder))
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	return payload
}

// testSourceRead returns one changed upstream read that carries the payload.
func testSourceRead(t testing.TB, generationID string, payload []byte, published time.Time) SourceRead {
	t.Helper()
	return SourceRead{
		Changed: true,
		Generation: catalogs.Generation{
			Manifest: catalogs.GenerationManifest{
				GenerationID: generationID,
				GeneratedAt:  published,
				Payload:      catalogs.DescribeCatalogPayload(payload),
			},
			Payload: payload,
		},
		PublishedAt:      published,
		ChannelUpdatedAt: published,
		Health:           HealthOK,
	}
}

// testProviderLayer returns one retained provider observation.
func testProviderLayer(t testing.TB, providerID catalogs.ProviderID, modelID, name string, observed time.Time) ProviderLayer {
	t.Helper()
	payload := testCatalogPayload(t, providerID, modelID, name)
	return ProviderLayer{
		ProviderID: providerID,
		Payload:    payload,
		Digest:     catalogs.DescribeCatalogPayload(payload).Checksum,
		ObservedAt: observed,
	}
}

// testAttempt returns one terminal provider attempt.
func testAttempt(providerID catalogs.ProviderID, outcome sources.ProviderOutcome, reason sources.ProviderReason) sources.ProviderAttempt {
	return sources.ProviderAttempt{
		ProviderID: providerID,
		Outcome:    outcome,
		Reason:     reason,
		Requested:  outcome != sources.ProviderOutcomeSkippedNotConfigured,
	}
}

// openTestRuntime opens a runtime that reaches no network. Every test that
// needs a runtime uses it, so no test depends on an external system.
func openTestRuntime(t *testing.T, opts ...Option) *Runtime {
	t.Helper()
	base := []Option{
		WithStateDirectory(t.TempDir()),
		WithStartupSpread(0),
		WithAcquisitionEnabled(false),
		WithSourcePollInterval(0),
	}
	runtime, err := Open(context.Background(), append(base, opts...)...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return runtime
}

// stubScheduleTimer records every schedule wait and never fires. A test then
// observes the pacing decision of one worker without a real delay.
type stubScheduleTimer struct {
	waits chan time.Duration
}

func newStubScheduleTimer() *stubScheduleTimer {
	return &stubScheduleTimer{waits: make(chan time.Duration, 8)}
}

// after records the wait and returns a channel that never fires.
func (s *stubScheduleTimer) after(wait time.Duration) <-chan time.Time {
	select {
	case s.waits <- wait:
	default:
	}
	return make(chan time.Time)
}

// waited returns the next recorded wait. It fails the test when no worker
// waited inside the bound.
func (s *stubScheduleTimer) waited(t *testing.T, within time.Duration) time.Duration {
	t.Helper()
	select {
	case wait := <-s.waits:
		return wait
	case <-time.After(within):
		t.Fatal("no runtime schedule waited for its next run")
		return 0
	}
}

// eventually waits until the condition holds. It fails the test otherwise.
func eventually(t *testing.T, within time.Duration, reason string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(reason)
}

// withScheduleTimer injects the timer that paces the periodic workers. It stays
// a test-only option, so the public surface keeps one clock and one random
// source. A test uses it to drive a schedule without a real delay.
func withScheduleTimer(after func(time.Duration) <-chan time.Time) Option {
	return runtimeOption("withScheduleTimer", func(r *runtimeOptions) error {
		if after == nil {
			return &errors.ValidationError{Field: "schedule_timer", Message: "is required"}
		}
		r.scheduleTimer = after
		return nil
	})
}
