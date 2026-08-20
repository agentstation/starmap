package reconciler

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// TestMergeModelsWithProvenance tests provenance tracking.
func TestMergeModelsWithProvenance(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	tracker := provenance.NewTracker(true)

	merger := newMergerWithProvenance(authorities, strategy, tracker, nil)

	sources := map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			createTestModel("model-1", "API Model", 1000),
		},
		sources.ModelsDevHTTPID: {
			createTestModelWithPricing("model-1", "ModelsDev Model", 0.5, 1.0),
		},
	}

	merged, prov, err := merger.Models(sources)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	if len(merged) != 1 {
		t.Errorf("Expected 1 merged model, got %d", len(merged))
	}

	if len(prov) == 0 {
		t.Error("Expected provenance to be tracked")
	}

	// Check specific provenance entries
	if _, exists := prov["models.model-1.Name"]; !exists {
		t.Error("Expected provenance for model name")
	}

	if _, exists := prov["models.model-1.pricing"]; !exists {
		t.Error("Expected provenance for model pricing")
	}
}

func TestReconcilerScopesResultProvenanceByProvider(t *testing.T) {
	providerCatalog := catalogs.NewEmpty()
	mustSetProviderForReconcilerTest(t, providerCatalog, catalogs.Provider{
		ID:   "provider-a",
		Name: "Provider A",
		Models: map[string]*catalogs.Model{
			"shared": createTestModel("shared", "Shared A", 8192),
		},
	})
	mustSetProviderForReconcilerTest(t, providerCatalog, catalogs.Provider{
		ID:   "provider-b",
		Name: "Provider B",
		Models: map[string]*catalogs.Model{
			"shared": createTestModel("shared", "Shared B", 8192),
		},
	})

	modelsDevCatalog := catalogs.NewEmpty()
	mustSetProviderForReconcilerTest(t, modelsDevCatalog, catalogs.Provider{
		ID:   "provider-a",
		Name: "Provider A",
		Models: map[string]*catalogs.Model{
			"shared": createTestModelWithPricing("shared", "Shared A", 1, 2),
		},
	})
	mustSetProviderForReconcilerTest(t, modelsDevCatalog, catalogs.Provider{
		ID:   "provider-b",
		Name: "Provider B",
		Models: map[string]*catalogs.Model{
			"shared": createTestModelWithPricing("shared", "Shared B", 3, 4),
		},
	})

	reconcile, err := New()
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}
	result, err := reconcile.Sources(context.Background(), sources.ProvidersID, ConvertCatalogsMapToSources(map[sources.ID]*catalogs.Builder{
		sources.ProvidersID:     providerCatalog,
		sources.ModelsDevHTTPID: modelsDevCatalog,
	}))
	if err != nil {
		t.Fatalf("Failed to reconcile sources: %v", err)
	}

	if _, ok := result.Provenance["models.provider-a.shared.pricing"]; !ok {
		t.Fatalf("missing provider-a scoped pricing provenance: %#v", result.Provenance)
	}
	if _, ok := result.Provenance["models.provider-b.shared.pricing"]; !ok {
		t.Fatalf("missing provider-b scoped pricing provenance: %#v", result.Provenance)
	}
	if _, ok := result.Provenance["models.shared.pricing"]; ok {
		t.Fatalf("found unscoped shared pricing provenance: %#v", result.Provenance)
	}
}

// TestMergeProviders tests provider merging.
