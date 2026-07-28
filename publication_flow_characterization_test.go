package starmap

import (
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestF019CharacterizationPublicationCallbacksPrecedeModelDiffCallbacks pins
// current within-generation callback ordering. P7 must retain a documented
// post-commit order while replacing this unjoined asynchronous delivery path.
func TestF019CharacterizationPublicationCallbacksPrecedeModelDiffCallbacks(t *testing.T) {
	hooks := newHooks()
	oldCatalog := mustTestCatalog(t, catalogs.NewEmpty())
	updatedBuilder := catalogs.NewEmpty()
	model := catalogs.Model{ID: "new-model", Name: "New Model"}
	if err := updatedBuilder.SetProvider(catalogs.Provider{
		ID:   "provider",
		Name: "Provider",
		Models: map[string]*catalogs.Model{
			model.ID: &model,
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	updatedCatalog := mustTestCatalog(t, updatedBuilder)

	publicationStarted := make(chan struct{})
	releasePublication := make(chan struct{})
	modelAdded := make(chan string, 1)
	var startedOnce sync.Once
	hooks.OnCatalogPublished(func(CatalogPublishedEvent) error {
		startedOnce.Do(func() { close(publicationStarted) })
		<-releasePublication
		return nil
	})
	hooks.OnModelAdded(func(model catalogs.Model) {
		modelAdded <- model.ID
	})

	hooks.dispatchUpdate(oldCatalog, updatedCatalog, CatalogPublishedEvent{Sequence: 1, Catalog: updatedCatalog})
	waitCharacterizationSignal(t, publicationStarted, "publication callback")
	select {
	case id := <-modelAdded:
		t.Fatalf("F-019 characterization changed: model callback %q ran before publication callbacks completed", id)
	case <-time.After(25 * time.Millisecond):
	}
	close(releasePublication)
	select {
	case id := <-modelAdded:
		if id != model.ID {
			t.Fatalf("model callback ID = %q, want %q", id, model.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("model callback did not run after publication callback")
	}
}

// TestF034CharacterizationPublicationCallbacksCanCompleteOutOfGenerationOrder
// pins independent per-generation goroutines. P7.2 must serialize committed
// publication visibility and notification so later generations cannot overtake
// earlier ones.
func TestF034CharacterizationPublicationCallbacksCanCompleteOutOfGenerationOrder(t *testing.T) {
	hooks := newHooks()
	catalog := mustTestCatalog(t, catalogs.NewEmpty())
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	completed := make(chan uint64, 2)
	var firstOnce sync.Once
	hooks.OnCatalogPublished(func(event CatalogPublishedEvent) error {
		if event.Sequence == 1 {
			firstOnce.Do(func() { close(firstStarted) })
			<-releaseFirst
		}
		completed <- event.Sequence
		return nil
	})

	hooks.dispatchUpdate(catalog, catalog, CatalogPublishedEvent{Sequence: 1, Catalog: catalog})
	waitCharacterizationSignal(t, firstStarted, "first publication callback")
	hooks.dispatchUpdate(catalog, catalog, CatalogPublishedEvent{Sequence: 2, Catalog: catalog})

	select {
	case sequence := <-completed:
		if sequence != 2 {
			t.Fatalf("first completed sequence = %d, want overtaking sequence 2", sequence)
		}
	case <-time.After(time.Second):
		t.Fatal("second generation did not overtake blocked first generation")
	}
	close(releaseFirst)
	select {
	case sequence := <-completed:
		if sequence != 1 {
			t.Fatalf("second completed sequence = %d, want 1", sequence)
		}
	case <-time.After(time.Second):
		t.Fatal("first generation did not complete after release")
	}
}

// TestF036CharacterizationHookOverloadDropsWholeGeneration pins the fixed
// delivery-slot overload behavior. P7 must coalesce toward the newest
// generation or terminate the affected stream; it cannot silently discard the
// publication while reporting healthy delivery.
func TestF036CharacterizationHookOverloadDropsWholeGeneration(t *testing.T) {
	hooks := newHooks()
	catalog := mustTestCatalog(t, catalogs.NewEmpty())
	release := make(chan struct{})
	started := make(chan uint64, defaultHookDeliveryConcurrency)
	hooks.OnCatalogPublished(func(event CatalogPublishedEvent) error {
		started <- event.Sequence
		<-release
		return nil
	})

	for sequence := uint64(1); sequence <= defaultHookDeliveryConcurrency+1; sequence++ {
		hooks.dispatchUpdate(catalog, catalog, CatalogPublishedEvent{Sequence: sequence, Catalog: catalog})
	}
	if got := hooks.statsSnapshot().Dropped; got != 1 {
		t.Fatalf("F-036 characterization changed: dropped generations = %d, want 1", got)
	}

	seen := make(map[uint64]struct{}, defaultHookDeliveryConcurrency)
	for range defaultHookDeliveryConcurrency {
		select {
		case sequence := <-started:
			seen[sequence] = struct{}{}
		case <-time.After(time.Second):
			t.Fatalf("started callbacks = %d, want %d", len(seen), defaultHookDeliveryConcurrency)
		}
	}
	if _, exists := seen[defaultHookDeliveryConcurrency+1]; exists {
		t.Fatalf("overload generation %d unexpectedly delivered", defaultHookDeliveryConcurrency+1)
	}
	close(release)

	deadline := time.Now().Add(time.Second)
	for hooks.statsSnapshot().Completed != defaultHookDeliveryConcurrency && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := hooks.statsSnapshot().Completed; got != defaultHookDeliveryConcurrency {
		t.Fatalf("completed callbacks = %d, want %d", got, defaultHookDeliveryConcurrency)
	}
}

func waitCharacterizationSignal(t testing.TB, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}
