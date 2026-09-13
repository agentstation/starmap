package runtime

import (
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestRetentionProtectsRuntimeSnapshotAfterDirectClientActivation(t *testing.T) {
	store := storage.NewMemory()
	served := aliasGeneration(t, "runtime-served")
	if err := store.Commit(t.Context(), served, ""); err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)),
		WithRetentionMaxGenerations(1), WithRetentionMaxBytes(1))
	before := r.State()
	next := aliasGeneration(t, "client-active")
	if _, err := r.Client().Activate(t.Context(), next); err != nil {
		t.Fatal(err)
	}
	if r.State().GenerationID != served.Manifest.GenerationID || r.Client().CurrentCatalogState().GenerationID != next.Manifest.GenerationID {
		t.Fatal("fixture did not separate runtime and client snapshots")
	}
	if err := r.collectRetainedState(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(t.Context(), before.GenerationID); err != nil {
		t.Fatalf("collection removed the catalog still served by the runtime: %v", err)
	}
	if r.State() != before {
		t.Fatal("collection changed the runtime snapshot")
	}
	if report := r.RetentionSnapshot(); report.ProtectedGenerations != 2 || !report.OverLimit {
		t.Fatalf("runtime and client snapshots need separate protection: %+v", report)
	}
}
