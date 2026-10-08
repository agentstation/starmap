package runtime

import (
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestOrdinaryNoopRetainsCommittedEffectiveState(t *testing.T) {
	upstream := aliasGeneration(t, "ordinary-noop-upstream")
	source := newStubSource("ordinary-noop-source")
	source.replies = []SourceRead{aliasRead(upstream)}
	store := storage.NewMemory()
	directory := privateRuntimeDirectory(t)
	options := []Option{WithSource(source), WithSourceStartupPolicy("require_source"), WithAcquisitionSources(), WithStateDirectory(directory), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	original, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if original.Manifest.GenerationID == upstream.Manifest.GenerationID || !original.Manifest.GeneratedAt.After(upstream.Manifest.GeneratedAt) {
		t.Fatal("the explicit local policy must create a separately dated derived generation")
	}
	assertState := func(phase string, runtime *Runtime) {
		t.Helper()
		current, err := store.Current(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if current.Manifest.GenerationID != original.Manifest.GenerationID || !current.Manifest.GeneratedAt.Equal(original.Manifest.GeneratedAt) {
			t.Fatalf("%s rewrote the immutable effective generation", phase)
		}
		state, client := runtime.State(), runtime.Client().CurrentCatalogState()
		if state.GenerationID != current.Manifest.GenerationID || state.PayloadChecksum != current.Manifest.Payload.Checksum || !state.GeneratedAt.Equal(current.Manifest.GeneratedAt) || state.Sequence != client.Sequence {
			t.Errorf("%s reports state %s/%s/%s/%d instead of committed %s/%s/%s/%d", phase, state.GenerationID, state.PayloadChecksum, state.GeneratedAt, state.Sequence, current.Manifest.GenerationID, current.Manifest.Payload.Checksum, current.Manifest.GeneratedAt, client.Sequence)
		}
		runtime.mu.RLock()
		retained := runtime.layers.source
		runtime.mu.RUnlock()
		if retained == nil || retained.Manifest == nil || retained.GenerationID != upstream.Manifest.GenerationID || !retained.Manifest.GeneratedAt.Equal(upstream.Manifest.GeneratedAt) || !retained.PublishedAt.Equal(upstream.Manifest.GeneratedAt) {
			t.Errorf("%s changed original upstream publication evidence", phase)
		}
	}
	assertState("startup", connected)
	for range 2 {
		if _, err := connected.RefreshSource(t.Context()); err != nil {
			t.Fatal(err)
		}
		assertState("source replay", connected)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	assertState("restart", reopened)
	current, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalogs.DecodeCatalogGeneration(current); err != nil {
		t.Fatal(err)
	}
}
