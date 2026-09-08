package runtime

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestRuntimeBaselineExcludesPreviouslyPublishedProviderEvidence(t *testing.T) {
	stateDirectory := privateRuntimeDirectory(t)
	catalogDirectory := filepath.Join(t.TempDir(), "catalog")
	open := func() *Runtime {
		t.Helper()
		store, err := storage.NewFilesystem(catalogDirectory)
		if err != nil {
			t.Fatal(err)
		}
		connected, err := Open(t.Context(), WithStateDirectory(stateDirectory), WithCatalogSource("embedded"), WithSourcePollInterval(0), WithAcquisitionEnabled(false), WithStartupSpread(0), WithClientOptions(starmap.WithCatalogStore(store)))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := connected.Close(); err != nil {
				t.Error(err)
			}
		})
		return connected
	}
	first := open()
	layer := testProviderLayer(t, "revocation-provider", "revocation-model", "Revocation Model", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	if _, err := first.publishProviders(t.Context(), []ProviderLayer{layer}, 0); err != nil {
		t.Fatal(err)
	}
	if _, found := first.Catalog().Providers().Get(layer.ProviderID); !found {
		t.Fatal("fixture did not publish provider evidence")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second := open()
	if _, found := second.Catalog().Providers().Get(layer.ProviderID); !found {
		t.Fatal("restart lost retained evidence before exclusion")
	}
	if _, found := second.layers.embedded.Catalog.Providers().Get(layer.ProviderID); found {
		t.Error("stored effective catalog became the embedded baseline")
	}
	// Exclude the retained layer before rebuilding. Active policy selection is separate.
	second.mu.Lock()
	clear(second.layers.providers)
	second.mu.Unlock()
	if _, err := second.rebuild(t.Context(), 0); err != nil {
		t.Fatal(err)
	}
	if _, found := second.Catalog().Providers().Get(layer.ProviderID); found {
		t.Error("excluded provider evidence returned through the baseline")
	}
}

func TestEffectiveGenerationPreservesOpaqueBaselineIdentity(t *testing.T) {
	payload := testCatalogPayload(t, "baseline-provider", "baseline-model", "Baseline")
	catalog, err := catalogs.DecodeCatalogPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	baseline := starmap.CatalogState{Catalog: catalog, GenerationID: "approved.local.original", PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum}
	layers := layerSet{}
	layers.setProvider(testProviderLayer(t, "observed-provider", "observed-model", "Observed", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)))
	state, err := layers.build(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(state.GenerationID, baseline.GenerationID+effectiveGenerationLocalSuffix) {
		t.Fatalf("derived identity lost its opaque baseline: %s", state.GenerationID)
	}
	repeated, err := layers.build(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.GenerationID != state.GenerationID {
		t.Fatal("rebuild nested a derived identity")
	}
}
