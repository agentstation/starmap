package runtime

import (
	"bytes"
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestOriginGenerationPinProtectsItsOriginalSelectionFromCollection(t *testing.T) {
	store := storage.NewMemory()
	source := newStubSource("pin-collection")
	first := aliasGeneration(t, "pin-collection-first")
	source.replies = []SourceRead{aliasRead(first)}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"),
		WithSource(source), WithSourceRefreshMode("manual"), WithAuthorityOrigin(store, originTestConfig())}
	r := openTestRuntime(t, options...)
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	selected, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	next := first.Copy()
	next.Manifest.GenerationID = "pin-collection-second"
	next.Payload = bytes.ReplaceAll(next.Payload, []byte(`"Current"`), []byte(`"Changed"`))
	next.Manifest.Payload = catalogs.DescribeCatalogPayload(next.Payload)
	source.mu.Lock()
	source.replies = []SourceRead{aliasRead(next)}
	source.mu.Unlock()
	if _, err := r.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	pinned := openTestRuntime(t, append(options, WithGenerationPin(selected.Manifest.GenerationID))...)
	current := pinned.State()
	if current.GenerationID == selected.Manifest.GenerationID {
		t.Fatal("fixture did not reissue the pinned payload at a new authority generation")
	}
	report, err := store.Collect(t.Context(), storage.RetentionRequest{
		ExpectedGenerationID: current.GenerationID, MaxGenerations: 1, MaxBytes: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), selected.Manifest.GenerationID); err != nil {
		t.Fatalf("collection removed the running origin's original pin selection: %v; report = %+v", err, report)
	}
	if report.Protected.Generations != 2 || !report.OverLimit {
		t.Fatalf("current and original selection were not both protected: %+v", report)
	}
	// Simulate an operator retaining only the accepted artifact after shutdown.
	if err := pinned.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Collect(t.Context(), storage.RetentionRequest{
		ExpectedGenerationID: current.GenerationID, MaxGenerations: 1, MaxBytes: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), selected.Manifest.GenerationID); !errors.IsNotFound(err) {
		t.Fatalf("shutdown did not release the original selection: %v", err)
	}
}

type invalidPinnedLeaseStore struct {
	*storage.Memory
	invalidID string
}

func (s *invalidPinnedLeaseStore) AcquireGeneration(ctx context.Context, id string) (catalogs.Generation, func() error, error) {
	generation, release, err := s.Memory.AcquireGeneration(ctx, id)
	if err == nil && id == s.invalidID {
		generation.Manifest.GenerationID = "unexpected-generation"
	}
	return generation, release, err
}

func TestGenerationPinReleasesItsLeaseAfterFailedOpen(t *testing.T) {
	store := storage.NewMemory()
	selected := aliasGeneration(t, "failed-pin-selected")
	current := aliasGeneration(t, "failed-pin-current")
	if err := store.Commit(t.Context(), selected, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), current, selected.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	invalid := &invalidPinnedLeaseStore{Memory: store, invalidID: selected.Manifest.GenerationID}
	r, err := Open(t.Context(), WithCatalogSource("embedded"), WithGenerationPin(selected.Manifest.GenerationID), WithClientOptions(starmap.WithCatalogStore(invalid)))
	if err == nil || r != nil {
		if r != nil {
			_ = r.Close()
		}
		t.Fatal("invalid pin generation did not fail startup")
	}
	if _, err := store.Collect(t.Context(), storage.RetentionRequest{ExpectedGenerationID: current.Manifest.GenerationID, MaxGenerations: 1, MaxBytes: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), selected.Manifest.GenerationID); !errors.IsNotFound(err) {
		t.Fatalf("failed startup leaked its pin lease: %v", err)
	}
}

func TestGenerationPinKeepsItsLeaseUntilOwnedWorkStops(t *testing.T) {
	store := storage.NewMemory()
	selected := aliasGeneration(t, "closing-pin-selected")
	if err := store.Commit(t.Context(), selected, ""); err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithGenerationPin(selected.Manifest.GenerationID), WithClientOptions(starmap.WithCatalogStore(store)))
	current := aliasGeneration(t, "closing-pin-current")
	if err := store.Commit(t.Context(), current, selected.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	stopWork := make(chan struct{})
	r.work.Go(func() { <-stopWork })
	closed := make(chan error, 1)
	go func() { closed <- r.Close() }()
	<-r.ctx.Done()
	_, collectionErr := store.Collect(t.Context(), storage.RetentionRequest{ExpectedGenerationID: current.Manifest.GenerationID, MaxGenerations: 1, MaxBytes: 1})
	_, readErr := store.Get(t.Context(), selected.Manifest.GenerationID)
	close(stopWork)
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if collectionErr != nil || readErr != nil {
		t.Fatalf("lease ended before work stopped: collect=%v read=%v", collectionErr, readErr)
	}
	if _, err := store.Collect(t.Context(), storage.RetentionRequest{ExpectedGenerationID: current.Manifest.GenerationID, MaxGenerations: 1, MaxBytes: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), selected.Manifest.GenerationID); !errors.IsNotFound(err) {
		t.Fatalf("stopped runtime retained its lease: %v", err)
	}
}

type pinStartupFailureLeaseStore struct {
	stubLeaseStore
	fail func() error
}

func (s *pinStartupFailureLeaseStore) AcquireLease(context.Context, string, time.Duration) (Lease, error) {
	return Lease{}, s.fail()
}

func TestGenerationPinReleasesItsLeaseAfterLaterStartupFailure(t *testing.T) {
	store := storage.NewMemory()
	selected := aliasGeneration(t, "late-failure-selected")
	current := aliasGeneration(t, "late-failure-current")
	if err := store.Commit(t.Context(), selected, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), current, selected.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	request := storage.RetentionRequest{ExpectedGenerationID: current.Manifest.GenerationID, MaxGenerations: 1, MaxBytes: 1}
	failure := stderrors.New("writer lease service unavailable")
	leases := &pinStartupFailureLeaseStore{fail: func() error {
		report, err := store.Collect(t.Context(), request)
		if err != nil || report.Protected.Generations != 2 {
			t.Fatalf("pin was not protected before later startup failure: %+v, %v", report, err)
		}
		return failure
	}}
	r, err := Open(t.Context(), WithCatalogSource("embedded"), WithGenerationPin(selected.Manifest.GenerationID),
		WithClientOptions(starmap.WithCatalogStore(store)), WithLeaseStore(leases))
	if r != nil || !stderrors.Is(err, failure) {
		if r != nil {
			_ = r.Close()
		}
		t.Fatalf("later startup failure = %v", err)
	}
	if _, err := store.Collect(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), selected.Manifest.GenerationID); !errors.IsNotFound(err) {
		t.Fatalf("failed runtime retained its pin lease: %v", err)
	}
}
