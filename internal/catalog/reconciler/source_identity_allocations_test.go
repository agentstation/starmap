package reconciler

import (
	"strconv"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAbsentProjectedEvidenceDoesNotCopyUnrelatedModels(t *testing.T) {
	measure := func(peers int) float64 {
		seed := sourceIdentityCatalog(t, "", catalogs.Model{ID: "model", Name: "Model"})
		builder, err := catalogs.NewBuilderFrom(seed)
		if err != nil {
			t.Fatal(err)
		}
		provider, err := builder.Provider("provider-a")
		if err != nil {
			t.Fatal(err)
		}
		for index := range peers {
			model := catalogs.DeepCopyModel(*provider.Models["model"])
			model.ID = "peer-" + strconv.Itoa(index)
			provider.Models[model.ID] = &model
		}
		if err := builder.SetProvider(provider); err != nil {
			t.Fatal(err)
		}
		catalog, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		merger := &merger{sourceCatalogs: map[sources.ID]*catalogs.Catalog{sources.LocalCatalogID: catalog}}
		return testing.AllocsPerRun(3, func() {
			if _, found := merger.projectedModelEvidence("provider-a", "model", "Features.tools", true); found {
				t.Error("absent evidence became a source claim")
			}
		})
	}
	small, large := measure(0), measure(100)
	t.Logf("absent evidence lookup allocations: one model=%g, 101 models=%g", small, large)
	if large > small+4 {
		t.Fatalf("absent evidence lookup copied unrelated models: %g -> %g allocations", small, large)
	}
}
