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

// TestF034PublicationCallbacksCompleteInGenerationOrder proves a later
// committed generation cannot overtake an earlier publication callback.
func TestF034PublicationCallbacksCompleteInGenerationOrder(t *testing.T) {
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
		t.Fatalf("sequence %d overtook blocked generation 1", sequence)
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseFirst)
	select {
	case sequence := <-completed:
		if sequence != 1 {
			t.Fatalf("first completed sequence = %d, want 1", sequence)
		}
	case <-time.After(time.Second):
		t.Fatal("first generation did not complete after release")
	}
	select {
	case sequence := <-completed:
		if sequence != 2 {
			t.Fatalf("second completed sequence = %d, want 2", sequence)
		}
	case <-time.After(time.Second):
		t.Fatal("second generation did not complete after generation 1")
	}
}

// TestF036HookOverloadCoalescesToNewestGeneration proves bounded asynchronous
// delivery retains the running generation plus the newest pending generation.
func TestF036HookOverloadCoalescesToNewestGeneration(t *testing.T) {
	hooks := newHooks()
	catalog := mustTestCatalog(t, catalogs.NewEmpty())
	release := make(chan struct{})
	started := make(chan uint64, 2)
	hooks.OnCatalogPublished(func(event CatalogPublishedEvent) error {
		started <- event.Sequence
		if event.Sequence == 1 {
			<-release
		}
		return nil
	})

	const newestSequence = uint64(17)
	for sequence := uint64(1); sequence <= newestSequence; sequence++ {
		hooks.dispatchUpdate(catalog, catalog, CatalogPublishedEvent{Sequence: sequence, Catalog: catalog})
	}
	if got := hooks.statsSnapshot().Coalesced; got != newestSequence-2 {
		t.Fatalf("coalesced generations = %d, want %d", got, newestSequence-2)
	}

	select {
	case sequence := <-started:
		if sequence != 1 {
			t.Fatalf("first started sequence = %d, want 1", sequence)
		}
	case <-time.After(time.Second):
		t.Fatal("generation 1 callback did not start")
	}
	select {
	case sequence := <-started:
		t.Fatalf("pending sequence %d started before generation 1 completed", sequence)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	select {
	case sequence := <-started:
		if sequence != newestSequence {
			t.Fatalf("post-overload sequence = %d, want newest %d", sequence, newestSequence)
		}
	case <-time.After(time.Second):
		t.Fatal("newest pending generation did not run")
	}
	deadline := time.Now().Add(time.Second)
	for hooks.statsSnapshot().Completed != 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := hooks.statsSnapshot().Completed; got != 2 {
		t.Fatalf("completed callbacks = %d, want 2", got)
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
