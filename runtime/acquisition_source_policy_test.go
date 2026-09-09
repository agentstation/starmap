package runtime

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAcquisitionSourceSelectionRebuildsRetainedEvidence(t *testing.T) {
	store := storage.NewMemory()
	base := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store))}
	selected := []sources.ID{sources.LocalCatalogID, sources.ProvidersID}
	open := func(ids []sources.ID) *Runtime {
		return openTestRuntime(t, append(slices.Clone(base), WithAcquisitionSources(ids...))...)
	}
	first := open(selected)
	observation := acquisitionSourceObservation(t, false)
	if _, err := first.PublishObservations(t.Context(), observation); err != nil {
		t.Fatal(err)
	}
	layer := testProviderLayer(t, "provider", "model", "Provider", time.Now().UTC())
	if _, err := first.publishProviders(t.Context(), []ProviderLayer{layer}, first.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	original := first.State()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	providerOnly := open([]sources.ID{sources.ProvidersID})
	if _, err := providerOnly.State().Catalog.Provider("metadata"); err == nil {
		t.Fatal("excluded local evidence survived restart")
	}
	if _, err := providerOnly.State().Catalog.Provider("provider"); err != nil {
		t.Fatal("enabled provider evidence disappeared", err)
	}
	if providerOnly.Client().CurrentGenerationID() != providerOnly.State().GenerationID {
		t.Fatal("client still serves the old selection")
	}
	if _, err := providerOnly.PublishObservations(t.Context(), observation); err == nil {
		t.Fatal("excluded source published new evidence")
	}
	if err := providerOnly.Close(); err != nil {
		t.Fatal(err)
	}
	empty := open([]sources.ID{})
	for _, id := range []string{"metadata", "provider"} {
		if _, err := empty.State().Catalog.Provider(catalogs.ProviderID(id)); err == nil {
			t.Fatal("empty selection retained acquired provider", id)
		}
	}
	if err := empty.Close(); err != nil {
		t.Fatal(err)
	}
	restored := open(selected)
	if restored.State().GenerationID != original.GenerationID || restored.State().PayloadChecksum != original.PayloadChecksum {
		t.Fatal("re-enabled selection did not restore the original generation")
	}
}

func TestAcquisitionSourceSelectionReachesCollectorAndOwnsIDs(t *testing.T) {
	ids := []sources.ID{sources.LocalCatalogID}
	option := WithAcquisitionSources(ids...)
	ids[0] = sources.ProvidersID
	calls := 0
	collector := sourceAcquirerFunc(func(_ context.Context, request SourceAcquisitionRequest) ([]sources.Observation, error) {
		calls++
		if !slices.Equal(request.Sources, []sources.ID{sources.LocalCatalogID}) {
			t.Errorf("source selection = %v", request.Sources)
		}
		request.Sources[0] = sources.ProvidersID
		return nil, nil
	})
	connected := openTestRuntime(t, WithCatalogSource("embedded"), option, WithSourceAcquirer(collector))
	for range 2 {
		if _, err := connected.Sync(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatal("collector did not run twice")
	}
	selected, present := connected.AcquisitionSources()
	if !present || !slices.Equal(selected, []sources.ID{sources.LocalCatalogID}) {
		t.Fatalf("active selection = %v, %t", selected, present)
	}
	selected[0] = sources.ProvidersID
	again, _ := connected.AcquisitionSources()
	if again[0] != sources.LocalCatalogID {
		t.Fatal("accessor exposed active policy storage")
	}
}

func TestAcquisitionSourceSelectionRefusesFailedStartupPublication(t *testing.T) {
	store := storage.NewMemory()
	directory := privateRuntimeDirectory(t)
	first := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)))
	if _, err := first.PublishObservations(t.Context(), acquisitionSourceObservation(t, false)); err != nil {
		t.Fatal(err)
	}
	before := first.State()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	failed, err := Open(t.Context(), WithStateDirectory(directory), WithCatalogSource("embedded"), WithAcquisitionEnabled(false), WithAcquisitionSources(), WithClientOptions(starmap.WithCatalogStore(&policyFailingStore{store})))
	if failed != nil {
		_ = failed.Close()
		t.Fatal("runtime opened without enforcing source selection")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("startup failure = %v, want the store permission error", err)
	}
	retained, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if retained.Manifest.GenerationID != before.GenerationID {
		t.Fatal("failed startup replaced the accepted generation")
	}
}

func TestAcquisitionSourceSelectionNormalizesEmptyLists(t *testing.T) {
	var identity string
	for _, empty := range [][]sources.ID{nil, {}} {
		connected := openTestRuntime(t, WithCatalogSource("embedded"), WithAcquisitionSources(empty...))
		if identity != "" && connected.State().GenerationID != identity {
			t.Fatal("equivalent empty lists produce different generation identities")
		}
		identity = connected.State().GenerationID
	}
}

func TestGitAcquisitionPinReachesCollectorWithoutSharedStorage(t *testing.T) {
	pin := strings.Repeat("a", 40)
	calls := 0
	collector := sourceAcquirerFunc(func(_ context.Context, request SourceAcquisitionRequest) ([]sources.Observation, error) {
		calls++
		if request.ModelsDevGitCommit == nil || *request.ModelsDevGitCommit != pin {
			t.Fatal("collector lost the configured pin")
		}
		*request.ModelsDevGitCommit = "changed"
		return nil, nil
	})
	connected := openTestRuntime(t, WithCatalogSource("embedded"), WithAcquisitionSources(sources.ModelsDevGitID), WithModelsDevGitCommit(pin), WithSourceAcquirer(collector))
	for range 2 {
		if _, err := connected.Sync(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if value, present := connected.ModelsDevGitCommit(); !present || value != pin || calls != 2 {
		t.Fatal("collector changed the configured pin")
	}
}
