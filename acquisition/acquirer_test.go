package acquisition_test

import (
	"context"
	stderrors "errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// stubObserver observes providers under test control. It reaches no provider,
// so every acquisition test stays hermetic.
type stubObserver struct {
	mu sync.Mutex

	// answers holds the scripted reply of one provider.
	answers map[catalogs.ProviderID]acquisition.ProviderObservation

	// failures holds the scripted error of one provider.
	failures map[catalogs.ProviderID]error

	// block holds one provider until the test releases it.
	block   map[catalogs.ProviderID]chan struct{}
	entered map[catalogs.ProviderID]chan struct{}

	// contexts records the context error each blocked provider observed.
	contexts map[catalogs.ProviderID]error

	// stopped closes when a blocked provider leaves its observation.
	stopped map[catalogs.ProviderID]chan struct{}
}

func newStubObserver() *stubObserver {
	return &stubObserver{
		answers:  make(map[catalogs.ProviderID]acquisition.ProviderObservation),
		failures: make(map[catalogs.ProviderID]error),
		block:    make(map[catalogs.ProviderID]chan struct{}),
		entered:  make(map[catalogs.ProviderID]chan struct{}),
		contexts: make(map[catalogs.ProviderID]error),
		stopped:  make(map[catalogs.ProviderID]chan struct{}),
	}
}

// ObserveProvider returns the scripted reply of one provider.
func (s *stubObserver) ObserveProvider(
	ctx context.Context,
	_ *catalogs.Catalog,
	id catalogs.ProviderID,
) (acquisition.ProviderObservation, error) {
	s.mu.Lock()
	block := s.block[id]
	entered := s.entered[id]
	stopped := s.stopped[id]
	answer, hasAnswer := s.answers[id]
	failure := s.failures[id]
	s.mu.Unlock()

	if entered != nil {
		close(entered)
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			s.mu.Lock()
			s.contexts[id] = ctx.Err()
			s.mu.Unlock()
			if stopped != nil {
				close(stopped)
			}
			return acquisition.ProviderObservation{}, ctx.Err()
		}
	}
	if failure != nil {
		return acquisition.ProviderObservation{}, failure
	}
	if !hasAnswer {
		return acquisition.ProviderObservation{}, stderrors.New("no scripted reply")
	}
	return answer, nil
}

// observed returns the context error one blocked provider recorded.
func (s *stubObserver) observed(id catalogs.ProviderID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contexts[id]
}

// setAnswer scripts one successful provider observation.
func (s *stubObserver) setAnswer(t testing.TB, id catalogs.ProviderID, modelID, name string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.answers[id] = acquisition.ProviderObservation{
		Layer: starmap.ProviderLayer{
			ProviderID: id,
			Payload:    providerPayload(t, id, modelID, name),
			ObservedAt: time.Now().UTC(),
		},
		Attempt: sources.ProviderAttempt{
			ProviderID: id,
			Outcome:    sources.ProviderOutcomeSucceeded,
			Requested:  true,
		},
	}
	delete(s.failures, id)
}

// setFailure scripts one failed provider attempt.
func (s *stubObserver) setFailure(id catalogs.ProviderID, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[id] = err
	delete(s.answers, id)
}

// providerPayload encodes one provider observation as a catalog payload.
func providerPayload(t testing.TB, id catalogs.ProviderID, modelID, name string) []byte {
	t.Helper()
	const authorID catalogs.AuthorID = "test-author"
	author := catalogs.Author{ID: authorID, Name: "Test Author"}
	slug := strings.ReplaceAll(string(id)+"--"+modelID, "/", "--")
	model := catalogs.Model{
		ID:       modelID,
		Name:     name,
		ModelRef: catalogs.AuthoredModelID(authorID, slug),
	}

	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID:     id,
		Name:   string(id),
		Models: map[string]*catalogs.Model{modelID: &model},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := builder.SetAuthorModel(authorID, catalogs.Model{
		ID:      slug,
		Name:    name,
		Authors: []catalogs.Author{author},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	return payload
}

// openRuntime opens a runtime that reaches no network and observes providers
// through the supplied acquirer.
func openRuntime(t *testing.T, acquirer starmap.Acquirer) *starmap.Runtime {
	t.Helper()
	runtime, err := starmap.Open(context.Background(),
		starmap.WithStateDirectory(t.TempDir()),
		starmap.WithCatalogSource("embedded"),
		starmap.WithSourcePollInterval(0),
		starmap.WithStartupSpread(0),
		starmap.WithAcquisitionEnabled(false),
		starmap.WithAcquirer(acquirer),
	)
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

// modelName returns the served name of one provider model.
func modelName(t testing.TB, catalog *catalogs.Catalog, id catalogs.ProviderID, modelID string) string {
	t.Helper()
	provider, found := catalog.Providers().Get(id)
	if !found {
		t.Fatalf("the catalog holds no provider %q", id)
	}
	model, found := provider.Models[modelID]
	if !found || model == nil {
		t.Fatalf("provider %q holds no model %q", id, modelID)
	}
	return model.Name
}

// TestSyncPartialFailurePublishesAndRetainsProviderLastKnownGood proves that a
// failed provider never removes records. The providers that answered publish,
// and the failed provider keeps its own last-known-good layer.
func TestSyncPartialFailurePublishesAndRetainsProviderLastKnownGood(t *testing.T) {
	t.Parallel()

	observer := newStubObserver()
	observer.setAnswer(t, "alpha", "alpha-model", "Alpha One")
	observer.setAnswer(t, "beta", "beta-model", "Beta One")

	acquirer, err := acquisition.NewAcquirer(acquisition.WithProviderObserver(observer))
	if err != nil {
		t.Fatalf("NewAcquirer: %v", err)
	}
	runtime := openRuntime(t, acquirer)
	ctx := context.Background()

	first, err := runtime.Sync(ctx, "alpha", "beta")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if first.Succeeded != 2 || !first.Published {
		t.Fatalf("first report = %+v, want two successes and a publication", first)
	}
	if got := modelName(t, runtime.Catalog(), "beta", "beta-model"); got != "Beta One" {
		t.Fatalf("beta model name = %q, want %q", got, "Beta One")
	}

	// The second run moves alpha forward while beta fails.
	observer.setAnswer(t, "alpha", "alpha-model", "Alpha Two")
	observer.setFailure("beta", stderrors.New("the provider closed the connection"))

	second, err := runtime.Sync(ctx, "alpha", "beta")
	if err != nil {
		t.Fatalf("Sync after a partial failure: %v", err)
	}
	if !second.Published {
		t.Error("a partial failure must still publish the providers that answered")
	}
	if second.Succeeded != 1 || second.Failed != 1 {
		t.Errorf("report = %+v, want one success and one failure", second)
	}
	if second.Health != starmap.HealthDegraded {
		t.Errorf("health = %q, want %q", second.Health, starmap.HealthDegraded)
	}
	if len(second.Retained) != 1 || second.Retained[0] != "beta" {
		t.Errorf("retained = %v, want [beta]", second.Retained)
	}

	catalog := runtime.Catalog()
	if got := modelName(t, catalog, "alpha", "alpha-model"); got != "Alpha Two" {
		t.Errorf("alpha model name = %q, want %q", got, "Alpha Two")
	}
	if got := modelName(t, catalog, "beta", "beta-model"); got != "Beta One" {
		t.Errorf("beta model name = %q, want the retained %q", got, "Beta One")
	}

	// The failed attempt carries a safe reason code and no provider text.
	for _, attempt := range second.Attempts {
		if attempt.ProviderID != "beta" {
			continue
		}
		if attempt.Outcome != sources.ProviderOutcomeFailed {
			t.Errorf("beta outcome = %q, want %q", attempt.Outcome, sources.ProviderOutcomeFailed)
		}
		if !attempt.Reason.Valid() {
			t.Errorf("beta reason = %q, want a defined safe reason code", attempt.Reason)
		}
	}
}

// TestSyncPublishesCompletedProvidersWhileAnotherBlocked proves the bounded
// coalescing window. A provider that answers publishes inside one window even
// while another provider still works, and the blocked provider stops.
func TestSyncPublishesCompletedProvidersWhileAnotherBlocked(t *testing.T) {
	t.Parallel()

	observer := newStubObserver()
	observer.setAnswer(t, "fast", "fast-model", "Fast One")
	observer.block["slow"] = make(chan struct{})
	observer.entered["slow"] = make(chan struct{})
	observer.stopped["slow"] = make(chan struct{})
	observer.setAnswer(t, "slow", "slow-model", "Slow One")

	// The injected timer closes the window as soon as the first answer opens
	// it, so the test needs no real delay.
	windows := make(chan time.Duration, 4)
	after := func(window time.Duration) <-chan time.Time {
		windows <- window
		closed := make(chan time.Time)
		close(closed)
		return closed
	}

	acquirer, err := acquisition.NewAcquirer(
		acquisition.WithProviderObserver(observer),
		acquisition.WithAcquirerCoalesceTimer(after),
	)
	if err != nil {
		t.Fatalf("NewAcquirer: %v", err)
	}
	runtime := openRuntime(t, acquirer)

	report, err := runtime.Sync(context.Background(), "fast", "slow")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !report.Published {
		t.Fatal("the completed provider must publish while another still works")
	}
	if report.Succeeded != 1 || report.Failed != 1 {
		t.Errorf("report = %+v, want one success and one unfinished provider", report)
	}
	if got := modelName(t, runtime.Catalog(), "fast", "fast-model"); got != "Fast One" {
		t.Errorf("fast model name = %q, want %q", got, "Fast One")
	}
	if _, found := runtime.Catalog().Providers().Get("slow"); found {
		t.Error("a provider that did not answer must publish no records")
	}

	// The window the runtime passed is the bounded thirty-second window.
	select {
	case window := <-windows:
		if window != 30*time.Second {
			t.Errorf("coalescing window = %s, want 30s", window)
		}
	default:
		t.Error("the first answer did not open a coalescing window")
	}

	// The unfinished provider reports a safe reason and stops with the run.
	for _, attempt := range report.Attempts {
		if attempt.ProviderID != "slow" {
			continue
		}
		if attempt.Reason != sources.ProviderReasonRequestTimeout {
			t.Errorf("slow reason = %q, want %q", attempt.Reason, sources.ProviderReasonRequestTimeout)
		}
	}
	<-observer.entered["slow"]
	select {
	case <-observer.stopped["slow"]:
	case <-time.After(5 * time.Second):
		t.Fatal("the blocked provider did not stop with the run")
	}
	if err := observer.observed("slow"); !stderrors.Is(err, context.Canceled) {
		t.Errorf("blocked provider context error = %v, want context.Canceled", err)
	}
}
