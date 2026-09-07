package reconciler

import (
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPrimarySelectionPreservesUnselectedProviderEvidence(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	selected := scopedReconciliationObservation(t, "selected", "selected", 1000, at)
	baseline, err := catalogs.NewBuilderFrom(scopedReconciliationBaseline(t, selected))
	if err != nil {
		t.Fatal(err)
	}
	if err := baseline.SetProvider(catalogs.Provider{ID: "unselected", Name: "Unselected"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := baseline.Build()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(WithBaseline(snapshot), WithChangeTime(at))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Sources(t.Context(), sources.ProvidersID, []sources.Observation{selected, {SourceID: sources.EmbeddedCatalogID, Catalog: snapshot}})
	if err != nil {
		t.Fatal(err)
	}
	for key := range result.Catalog.Provenance().Map() {
		if strings.HasPrefix(key, "provider:unselected:") {
			t.Fatal("primary filtering synthesized evidence for an unselected provider")
		}
	}
	provider, err := result.Catalog.Provider("unselected")
	if err != nil || provider.Name != "Unselected" {
		t.Fatal("primary filtering removed an unselected baseline provider")
	}
}
