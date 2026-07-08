package pipeline

import (
	"context"
	stderrors "errors"
	"strings"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type lifecycleTestSource struct {
	id         sources.ID
	catalog    catalogs.Catalog
	deps       []sources.Dependency
	optional   bool
	fetchErr   error
	cleanupErr error

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

func (s *lifecycleTestSource) Fetch(_ context.Context, opts ...sources.Option) error {
	s.mu.Lock()
	s.fetchCalls++
	s.fetchOptionSize = len(opts)
	s.mu.Unlock()
	return s.fetchErr
}

func (s *lifecycleTestSource) Catalog() catalogs.Catalog {
	return s.catalog
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

func TestFetchCollectsSourceErrorsAndKeepsPartialCatalogs(t *testing.T) {
	partialErr := stderrors.New("partial fetch failed")
	missingCatalogErr := stderrors.New("missing catalog")

	success := &lifecycleTestSource{
		id:      "success",
		catalog: catalogs.NewEmpty(),
	}
	partial := &lifecycleTestSource{
		id:       "partial",
		catalog:  catalogs.NewEmpty(),
		fetchErr: partialErr,
	}
	missing := &lifecycleTestSource{
		id:       "missing",
		fetchErr: missingCatalogErr,
	}

	err := fetch(context.Background(), []sources.Source{success, partial, missing}, []sources.Option{
		sources.WithFresh(true),
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

func TestFetchSkipsSourcesWhenContextAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	src := &lifecycleTestSource{
		id:      "cancelled",
		catalog: catalogs.NewEmpty(),
	}

	if err := fetch(ctx, []sources.Source{src}, nil); err != nil {
		t.Fatalf("Expected canceled fetch pre-check to return nil, got %v", err)
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
	if !strings.Contains(err.Error(), "required source required-missing has missing dependencies") {
		t.Fatalf("Expected required-source error, got %v", err)
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
