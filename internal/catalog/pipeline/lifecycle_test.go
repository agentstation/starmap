package pipeline

import (
	"context"
	stderrors "errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type lifecycleTestSource struct {
	id         sources.ID
	catalog    *catalogs.Catalog
	deps       []sources.Dependency
	optional   bool
	fetchErr   error
	cleanupErr error
	issues     []sources.ObservationIssue

	mu              sync.Mutex
	fetchCalls      int
	fetchOptionSize int
	cleanupCalls    int
}

func (s *lifecycleTestSource) ID() sources.ID {
	return s.id
}

func (s *lifecycleTestSource) Name() string {
	return string(s.id)
}

func (s *lifecycleTestSource) Observe(_ context.Context, opts ...sources.Option) (sources.Observation, error) {
	s.mu.Lock()
	s.fetchCalls++
	s.fetchOptionSize = len(opts)
	s.mu.Unlock()
	if s.catalog == nil {
		return sources.Observation{}, s.fetchErr
	}
	completeness := sources.ObservationCompletenessComplete
	status := sources.ObservationStatusSucceeded
	issues := append([]sources.ObservationIssue(nil), s.issues...)
	if len(issues) > 0 {
		status = sources.ObservationStatusDegraded
		for _, issue := range issues {
			if issue.Scope != sources.ObservationIssueScopeStaleFallback {
				completeness = sources.ObservationCompletenessPartial
				break
			}
		}
	}
	if s.fetchErr != nil {
		completeness = sources.ObservationCompletenessPartial
		status = sources.ObservationStatusDegraded
		issues = []sources.ObservationIssue{{
			Scope: sources.ObservationIssueScopeSource, Code: sources.ObservationIssueCodeFetchFailed, Message: s.fetchErr.Error(),
		}}
	}
	observation, err := sources.NewObservation(s.id, s.catalog, sources.ObservationMetadata{
		ObservedAt:   time.Date(2026, time.July, 9, 12, 0, 0, 0, time.UTC),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: completeness,
		Status:       status,
		Issues:       issues,
	})
	if err != nil {
		return sources.Observation{}, err
	}
	return observation, s.fetchErr
}

func TestObserveRetainsClassifiedDegradationWithoutSourceFailure(t *testing.T) {
	src := &lifecycleTestSource{
		id:      "degraded",
		catalog: asSnapshot(catalogs.NewEmpty()),
		issues: []sources.ObservationIssue{{
			Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback, Message: "stale last-known-good cache",
		}},
	}
	observations, err := observe(context.Background(), []sources.Source{src}, nil)
	if err != nil {
		t.Fatalf("classified degradation returned source failure: %v", err)
	}
	if len(observations) != 1 || observations[0].Status != sources.ObservationStatusDegraded {
		t.Fatalf("observations = %#v", observations)
	}
}

func asSnapshot(reader catalogs.Reader) *catalogs.Catalog {
	snapshot, err := catalogs.NewCatalog(reader)
	if err != nil {
		panic(err)
	}
	return snapshot
}

func (s *lifecycleTestSource) Cleanup() error {
	s.mu.Lock()
	s.cleanupCalls++
	s.mu.Unlock()
	return s.cleanupErr
}

func (s *lifecycleTestSource) Dependencies() []sources.Dependency {
	return s.deps
}

func (s *lifecycleTestSource) IsOptional() bool {
	return s.optional
}

func (s *lifecycleTestSource) counts() (fetchCalls int, fetchOptionSize int, cleanupCalls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fetchCalls, s.fetchOptionSize, s.cleanupCalls
}

type boundedObservationSource struct {
	id       sources.ID
	started  chan<- struct{}
	release  <-chan struct{}
	inFlight *atomic.Int32
	maximum  *atomic.Int32
}

func (source *boundedObservationSource) ID() sources.ID { return source.id }
func (source *boundedObservationSource) Name() string   { return string(source.id) }
func (source *boundedObservationSource) Cleanup() error { return nil }
func (source *boundedObservationSource) Dependencies() []sources.Dependency {
	return nil
}
func (source *boundedObservationSource) IsOptional() bool { return false }

func (source *boundedObservationSource) Observe(context.Context, ...sources.Option) (sources.Observation, error) {
	current := source.inFlight.Add(1)
	for {
		maximum := source.maximum.Load()
		if current <= maximum || source.maximum.CompareAndSwap(maximum, current) {
			break
		}
	}
	source.started <- struct{}{}
	<-source.release
	source.inFlight.Add(-1)
	catalog := asSnapshot(catalogs.NewEmpty())
	return sources.NewObservation(source.id, catalog, sources.ObservationMetadata{
		ObservedAt:   time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete,
		Status:       sources.ObservationStatusSucceeded,
	})
}

func TestObserveBoundsConfiguredSourceConcurrency(t *testing.T) {
	count := constants.MaxConcurrentProviders + 2
	started := make(chan struct{}, count)
	release := make(chan struct{})
	var inFlight, maximum atomic.Int32
	configured := make([]sources.Source, count)
	for index := range configured {
		configured[index] = &boundedObservationSource{
			id: sources.ID("provider-source-" + string(rune('a'+index))), started: started, release: release,
			inFlight: &inFlight, maximum: &maximum,
		}
	}
	done := make(chan error, 1)
	go func() {
		_, err := observe(context.Background(), configured, nil)
		done <- err
	}()
	for range constants.MaxConcurrentProviders {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for bounded observations")
		}
	}
	select {
	case <-started:
		t.Fatal("more configured sources started than the concurrency bound")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("observe: %v", err)
	}
	if maximum.Load() > constants.MaxConcurrentProviders {
		t.Fatalf("maximum concurrency = %d, want <= %d", maximum.Load(), constants.MaxConcurrentProviders)
	}
}

func TestObserveCollectsSourceErrorsAndKeepsPartialCatalogs(t *testing.T) {
	partialErr := stderrors.New("partial fetch failed")
	missingCatalogErr := stderrors.New("missing catalog")

	success := &lifecycleTestSource{
		id:      "success",
		catalog: asSnapshot(catalogs.NewEmpty()),
	}
	partial := &lifecycleTestSource{
		id:       "partial",
		catalog:  asSnapshot(catalogs.NewEmpty()),
		fetchErr: partialErr,
	}
	missing := &lifecycleTestSource{
		id:       "missing",
		fetchErr: missingCatalogErr,
	}

	observations, err := observe(context.Background(), []sources.Source{success, partial, missing}, []sources.Option{
		sources.WithReformat(true),
	})
	if err == nil {
		t.Fatal("Expected fetch to return joined source errors")
	}
	if !stderrors.Is(err, partialErr) {
		t.Fatalf("Expected joined error to include partial source error, got %v", err)
	}
	if !stderrors.Is(err, missingCatalogErr) {
		t.Fatalf("Expected joined error to include missing-catalog source error, got %v", err)
	}
	if len(observations) != 2 {
		t.Fatalf("Expected complete and partial observations to be retained, got %d", len(observations))
	}

	for _, src := range []*lifecycleTestSource{success, partial, missing} {
		fetchCalls, optionSize, _ := src.counts()
		if fetchCalls != 1 {
			t.Fatalf("Expected source %s to be fetched once, got %d", src.ID(), fetchCalls)
		}
		if optionSize != 1 {
			t.Fatalf("Expected source %s to receive source options, got %d", src.ID(), optionSize)
		}
	}
}

func TestObserveReturnsContextErrorWhenAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	src := &lifecycleTestSource{
		id:      "cancelled",
		catalog: asSnapshot(catalogs.NewEmpty()),
	}

	observations, err := observe(ctx, []sources.Source{src}, nil)
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("Expected canceled fetch pre-check to return context cancellation, got %v", err)
	}
	if len(observations) != 0 {
		t.Fatalf("Expected canceled context to produce no observations, got %d", len(observations))
	}

	fetchCalls, _, _ := src.counts()
	if fetchCalls != 0 {
		t.Fatalf("Expected canceled context to skip source fetch, got %d calls", fetchCalls)
	}
}

func TestCleanupReturnsContextErrorWithoutCallingSources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	src := &lifecycleTestSource{id: "cancelled"}
	err := cleanup(ctx, []sources.Source{src})
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("Expected cleanup to return context cancellation, got %v", err)
	}

	_, _, cleanupCalls := src.counts()
	if cleanupCalls != 0 {
		t.Fatalf("Expected canceled cleanup to skip source cleanup, got %d calls", cleanupCalls)
	}
}

func TestCleanupCollectsSourceErrors(t *testing.T) {
	firstErr := stderrors.New("first cleanup failed")
	secondErr := stderrors.New("second cleanup failed")

	first := &lifecycleTestSource{id: "first", cleanupErr: firstErr}
	second := &lifecycleTestSource{id: "second", cleanupErr: secondErr}

	err := cleanup(context.Background(), []sources.Source{first, second})
	if err == nil {
		t.Fatal("Expected cleanup to return joined source errors")
	}
	if !stderrors.Is(err, firstErr) {
		t.Fatalf("Expected joined error to include first cleanup error, got %v", err)
	}
	if !stderrors.Is(err, secondErr) {
		t.Fatalf("Expected joined error to include second cleanup error, got %v", err)
	}

	for _, src := range []*lifecycleTestSource{first, second} {
		_, _, cleanupCalls := src.counts()
		if cleanupCalls != 1 {
			t.Fatalf("Expected source %s to be cleaned once, got %d", src.ID(), cleanupCalls)
		}
	}
}

func TestResolveDependenciesSkipsOptionalSourcesWhenPromptsAreDisabled(t *testing.T) {
	available := &lifecycleTestSource{id: "available"}
	optionalMissing := &lifecycleTestSource{
		id:       "optional-missing",
		optional: true,
		deps:     []sources.Dependency{missingDependencyForTest()},
	}

	resolved, err := resolveDependencies(context.Background(), []sources.Source{available, optionalMissing}, &pkgsync.Options{
		SkipDepPrompts: true,
	})
	if err != nil {
		t.Fatalf("Expected optional missing source to be skipped without error, got %v", err)
	}
	if len(resolved) != 1 || resolved[0].ID() != available.ID() {
		t.Fatalf("Expected only available source to remain, got %#v", sourceIDs(resolved))
	}
}

func TestResolveDependenciesDefaultsToNonInteractiveOptionalSkip(t *testing.T) {
	available := &lifecycleTestSource{id: "available"}
	optionalMissing := &lifecycleTestSource{
		id:       "optional-missing",
		optional: true,
		deps:     []sources.Dependency{missingDependencyForTest()},
	}

	resolved, err := resolveDependencies(
		context.Background(),
		[]sources.Source{available, optionalMissing},
		pkgsync.Defaults(),
	)
	if err != nil {
		t.Fatalf("Default noninteractive resolution returned error: %v", err)
	}
	if len(resolved) != 1 || resolved[0].ID() != available.ID() {
		t.Fatalf("Expected only available source to remain, got %#v", sourceIDs(resolved))
	}
}

func TestResolveDependenciesDoesNotReadStdin(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("Create stdin sentinel pipe: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = readEnd
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = readEnd.Close()
		_ = writeEnd.Close()
	})

	available := &lifecycleTestSource{id: "available"}
	optionalMissing := &lifecycleTestSource{
		id:       "optional-missing",
		optional: true,
		deps:     []sources.Dependency{missingDependencyForTest()},
	}
	done := make(chan error, 1)
	go func() {
		_, resolveErr := resolveDependencies(
			context.Background(),
			[]sources.Source{available, optionalMissing},
			pkgsync.Defaults(),
		)
		done <- resolveErr
	}()

	select {
	case resolveErr := <-done:
		if resolveErr != nil {
			t.Fatalf("Noninteractive resolution returned error: %v", resolveErr)
		}
	case <-time.After(250 * time.Millisecond):
		_ = writeEnd.Close()
		<-done
		t.Fatal("Core dependency resolution attempted to read stdin")
	}
}

func TestResolveDependenciesDefaultsToTypedRequiredError(t *testing.T) {
	requiredMissing := &lifecycleTestSource{
		id:   "required-missing",
		deps: []sources.Dependency{missingDependencyForTest()},
	}

	_, err := resolveDependencies(
		context.Background(),
		[]sources.Source{requiredMissing},
		pkgsync.Defaults(),
	)
	if err == nil {
		t.Fatal("Expected required missing source to fail")
	}
	var dependencyErr *pkgerrors.DependencyError
	if !stderrors.As(err, &dependencyErr) {
		t.Fatalf("Error = %T, want *errors.DependencyError: %v", err, err)
	}
}

func TestResolveDependenciesUsesConfiguredDecisionHandler(t *testing.T) {
	available := &lifecycleTestSource{id: "available"}
	optionalMissing := &lifecycleTestSource{
		id:       "optional-missing",
		optional: true,
		deps:     []sources.Dependency{missingDependencyForTest()},
	}
	decisionCalls := 0
	opts := pkgsync.Defaults().Apply(pkgsync.WithDependencyDecisionHandler(
		func(_ context.Context, sourceID sources.ID, dep sources.Dependency, optional bool) (pkgsync.DependencyDecision, error) {
			decisionCalls++
			if sourceID != optionalMissing.ID() || dep.Name != missingDependencyForTest().Name || !optional {
				t.Fatalf("Unexpected dependency decision input: source=%s dependency=%s optional=%t", sourceID, dep.Name, optional)
			}
			return pkgsync.DependencyDecisionSkip, nil
		},
	))

	resolved, err := resolveDependencies(
		context.Background(),
		[]sources.Source{available, optionalMissing},
		opts,
	)
	if err != nil {
		t.Fatalf("Interactive decision resolution returned error: %v", err)
	}
	if decisionCalls != 1 {
		t.Fatalf("Dependency decision calls = %d, want 1", decisionCalls)
	}
	if len(resolved) != 1 || resolved[0].ID() != available.ID() {
		t.Fatalf("Expected only available source to remain, got %#v", sourceIDs(resolved))
	}
}

func TestResolveDependenciesFailsRequiredSourceWhenPromptsAreDisabled(t *testing.T) {
	requiredMissing := &lifecycleTestSource{
		id:   "required-missing",
		deps: []sources.Dependency{missingDependencyForTest()},
	}

	_, err := resolveDependencies(context.Background(), []sources.Source{requiredMissing}, &pkgsync.Options{
		SkipDepPrompts: true,
	})
	if err == nil {
		t.Fatal("Expected required missing source to fail")
	}
	var dependencyErr *pkgerrors.DependencyError
	if !stderrors.As(err, &dependencyErr) {
		t.Fatalf("Error = %T, want *errors.DependencyError: %v", err, err)
	}
	if !strings.Contains(dependencyErr.Message, "required-missing") {
		t.Fatalf("Dependency error does not identify source: %v", err)
	}
}

func missingDependencyForTest() sources.Dependency {
	return sources.Dependency{
		Name:          "starmap-missing-test-tool",
		DisplayName:   "Starmap Missing Test Tool",
		CheckCommands: []string{"starmap-definitely-missing-tool-for-tests"},
	}
}

func sourceIDs(srcs []sources.Source) []sources.ID {
	ids := make([]sources.ID, len(srcs))
	for i, src := range srcs {
		ids[i] = src.ID()
	}
	return ids
}
