package reconciler

import (
	"strconv"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProjectedOriginMustBelongToFieldSourceOrder(t *testing.T) {
	metadata := &catalogs.ModelMetadata{}
	metadata.SetOpenWeights(true)
	input := sourceIdentityCatalog(t, "https://provider.example/original", catalogs.Model{ID: "shared", Name: "Shared", Metadata: metadata})
	original := sourceIdentityObservation(t, sources.ProvidersID, input, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	generated := sourceIdentityReconcile(t, sources.ProvidersID, original)
	for _, included := range []bool{false, true} {
		for _, edited := range []bool{false, true} {
			t.Run("included="+strconv.FormatBool(included)+"/edited="+strconv.FormatBool(edited), func(t *testing.T) {
				localCatalog := snapshotForTest(t, generated.Catalog)
				local := sourceIdentityObservation(t, sources.LocalCatalogID, localCatalog, original.ObservedAt.Add(time.Minute))
				policies := authority.New()
				merger := newMerger(policies, NewAuthorityStrategy(policies), localCatalog)
				merger.setObservations([]sources.Observation{local})
				provider, err := localCatalog.Provider("provider-a")
				if err != nil {
					t.Fatal(err)
				}
				if edited {
					provider.Catalog.Endpoint.URL = "https://provider.example/edited"
					provider.Models["shared"].Metadata.SetOpenWeights(false)
				}
				order := []sources.ID{sources.LocalCatalogID}
				if included {
					order = append(order, sources.ProvidersID)
				}
				modelInputs := map[sources.ID]*catalogs.Model{sources.LocalCatalogID: provider.Models["shared"]}
				modelSources := merger.modelSourcesForValue("provider-a", "shared", authority.Policy{
					Path: "Metadata", EvidencePath: "metadata.open_weights", SourceOrder: order,
				}, modelInputs, func(model *catalogs.Model) any {
					value, _ := model.Metadata.OpenWeightsValue()
					return value
				})
				providerInputs := map[sources.ID]*catalogs.Provider{sources.LocalCatalogID: &provider}
				providerSources := merger.providerSourcesForPolicy("provider-a", authority.Policy{
					Path: "Catalog", SourceOrder: order,
				}, providerInputs)
				if (modelSources[sources.LocalCatalogID] != nil) != edited {
					t.Error("only a semantic model edit can retain local authority")
				}
				if (providerSources[sources.LocalCatalogID] != nil) != edited {
					t.Error("only a semantic provider edit can retain local authority")
				}
				if (modelSources[sources.ProvidersID] != nil) != (included && !edited) {
					t.Error("model origin eligibility differs from the field source order")
				}
				if (providerSources[sources.ProvidersID] != nil) != (included && !edited) {
					t.Error("provider origin eligibility differs from the field source order")
				}
				if len(modelInputs) != 1 || len(providerInputs) != 1 || modelInputs[sources.LocalCatalogID] == nil || providerInputs[sources.LocalCatalogID] == nil {
					t.Error("source selection changed caller-owned maps")
				}
			})
		}
	}
}
