package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestBuildWithoutObservationsReusesImmutableBaseline(t *testing.T) {
	payload := testCatalogPayload(t, "provider", "model", "Baseline")
	catalog, err := catalogs.DecodeCatalogPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	baseline := starmap.CatalogState{
		Catalog: catalog, GenerationID: "approved.local.original", Sequence: 7,
		PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum,
		GeneratedAt:     time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC),
	}
	layers := layerSet{}
	state, err := layers.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	if state.GenerationID != baseline.GenerationID || state.PayloadChecksum != baseline.PayloadChecksum || !state.GeneratedAt.Equal(baseline.GeneratedAt) || state.Sequence != 8 {
		t.Fatalf("baseline identity changed: %+v", state)
	}
	if state.Catalog != catalog {
		t.Fatal("build copied the immutable baseline without any observations")
	}
	provider, err := state.Catalog.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	provider.Models["model"].Name = "caller mutation"
	unchanged, err := baseline.Catalog.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Models["model"].Name != "Baseline" {
		t.Fatal("caller mutation changed the shared immutable baseline")
	}
}
