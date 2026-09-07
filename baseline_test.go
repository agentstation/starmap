package starmap_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type baselineCountingStore struct {
	storage.Store
	reads atomic.Int64
}

func (s *baselineCountingStore) Current(ctx context.Context) (catalogs.Generation, error) {
	s.reads.Add(1)
	return s.Store.Current(ctx)
}
func (s *baselineCountingStore) Get(ctx context.Context, id string) (catalogs.Generation, error) {
	s.reads.Add(1)
	return s.Store.Get(ctx, id)
}

func TestEmbeddedCatalogStateRemainsIndependentOfStoredCurrent(t *testing.T) {
	store := &baselineCountingStore{Store: storage.NewMemory()}
	client, err := starmap.NewContext(t.Context(), starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	baseline := client.EmbeddedCatalogState()
	if baseline.Catalog == nil || baseline.GenerationID == "" || baseline.PayloadChecksum == "" || baseline.GeneratedAt.IsZero() {
		t.Fatal("incomplete embedded baseline")
	}
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "stored-only-provider", Name: "Stored Provider"}); err != nil {
		t.Fatal(err)
	}
	custom, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*starmap.Candidate, error) {
		return starmap.NewCandidate(custom, starmap.CandidateEvidence{})
	}); err != nil {
		t.Fatal(err)
	}
	restarted, err := starmap.NewContext(t.Context(), starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, found := restarted.Catalog().Providers().Get("stored-only-provider"); !found {
		t.Fatal("explicit storage read did not preserve current")
	}
	reads := store.reads.Load()
	for _, instance := range []*starmap.Client{client, restarted} {
		got := instance.EmbeddedCatalogState()
		if got.GenerationID != baseline.GenerationID || got.PayloadChecksum != baseline.PayloadChecksum || !got.GeneratedAt.Equal(baseline.GeneratedAt) {
			t.Fatal("stored state changed embedded identity")
		}
		if _, found := got.Catalog.Providers().Get("stored-only-provider"); found {
			t.Fatal("stored records contaminated embedded baseline")
		}
		if got.Catalog != baseline.Catalog {
			t.Fatal("baseline getter duplicated the immutable catalog")
		}
	}
	var last starmap.CatalogState
	allocations := testing.AllocsPerRun(100, func() { last = restarted.EmbeddedCatalogState() })
	if allocations != 0 || last.Catalog == nil {
		t.Fatalf("baseline reads allocated %g times", allocations)
	}
	if store.reads.Load() != reads {
		t.Fatal("baseline getter read application storage")
	}
	var absent *starmap.Client
	if absent.EmbeddedCatalogState().Catalog != nil {
		t.Fatal("nil client returned a catalog")
	}
}
