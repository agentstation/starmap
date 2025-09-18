package reconciler_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
)

// Helper function to add models to a catalog through a provider.
func addModelsToProvider(cat catalogs.Catalog, providerID string, models []*catalogs.Model) error {
	provider, err := cat.Provider(catalogs.ProviderID(providerID))
	if err != nil {
		// Create provider if it doesn't exist
		provider = catalogs.Provider{
			ID:     catalogs.ProviderID(providerID),
			Name:   providerID,
			Models: make(map[string]*catalogs.Model),
		}
	}
	if provider.Models == nil {
		provider.Models = make(map[string]*catalogs.Model)
	}
	for _, model := range models {
		provider.Models[model.ID] = model
	}
	return cat.SetProvider(provider)
}

// TestIntegrationFullReconciliationFlow tests the complete reconciliation workflow.
func TestIntegrationFullReconciliationFlow(t *testing.T) {
	ctx := context.Background()

	// Create three source catalogs with overlapping data
	embeddedCat, err := catalogs.New()
	if err != nil {
		t.Fatalf("Failed to create embedded catalog: %v", err)
	}

	// Add models to embedded catalog
	embeddedModels := []*catalogs.Model{
		{
			ID:     "gpt-4",
			Name:   "GPT-4 (Embedded)",
			Limits: &catalogs.ModelLimits{ContextWindow: 8192},
			Pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.ModelTokenPricing{
					Input:  &catalogs.ModelTokenCost{Per1M: 30.0},
					Output: &catalogs.ModelTokenCost{Per1M: 60.0},
				},
			},
			Metadata:  &catalogs.ModelMetadata{ReleaseDate: utc.Now()},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:        "claude-3",
			Name:      "Claude 3 (Embedded)",
			Limits:    &catalogs.ModelLimits{ContextWindow: 100000},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
	}
	if err := addModelsToProvider(embeddedCat, "test-provider", embeddedModels); err != nil {
		t.Fatalf("Failed to add models to embedded catalog: %v", err)
	}

	// Create API catalog with updated data
	apiCat, err := catalogs.New()
	if err != nil {
		t.Fatalf("Failed to create API catalog: %v", err)
	}

	apiModels := []*catalogs.Model{
		{
			ID:   "gpt-4",
			Name: "GPT-4 Turbo",
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 128000, // Updated context window
			},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:   "gpt-4o",
			Name: "GPT-4 Omni",
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage, catalogs.ModelModalityAudio},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityAudio},
				},
			},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
	}
	if err := addModelsToProvider(apiCat, "test-provider", apiModels); err != nil {
		t.Fatalf("Failed to add models to API catalog: %v", err)
	}

	// Create models.dev catalog with pricing data
	modelsDevCat, err := catalogs.New()
	if err != nil {
		t.Fatalf("Failed to create models.dev catalog: %v", err)
	}

	modelsDevModels := []*catalogs.Model{
		{
			ID:   "gpt-4",
			Name: "GPT-4",
			Pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.ModelTokenPricing{
					Input: &catalogs.ModelTokenCost{
						Per1M: 10.0, // More accurate pricing
					},
					Output: &catalogs.ModelTokenCost{
						Per1M: 30.0,
					},
				},
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 128000,
				OutputTokens:  4096,
			},
			Metadata: &catalogs.ModelMetadata{
				ReleaseDate: utc.New(time.Date(2023, 3, 14, 0, 0, 0, 0, time.UTC)),
				KnowledgeCutoff: func() *utc.Time {
					t := utc.New(time.Date(2023, 4, 1, 0, 0, 0, 0, time.UTC))
					return &t
				}(),
			},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:   "gpt-4o",
			Name: "GPT-4o",
			Pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.ModelTokenPricing{
					Input: &catalogs.ModelTokenCost{
						Per1M: 5.0,
					},
					Output: &catalogs.ModelTokenCost{
						Per1M: 15.0,
					},
				},
			},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:   "claude-3",
			Name: "Claude 3",
			Pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.ModelTokenPricing{
					Input: &catalogs.ModelTokenCost{
						Per1M: 15.0,
					},
					Output: &catalogs.ModelTokenCost{
						Per1M: 75.0,
					},
				},
			},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
	}
	if err := addModelsToProvider(modelsDevCat, "test-provider", modelsDevModels); err != nil {
		t.Fatalf("Failed to add models to models.dev catalog: %v", err)
	}

	// Setup reconciler with authority-based strategy and provenance
	authorities := authority.New()
	strategy := reconciler.NewAuthorityStrategy(authorities)

	reconcile, err := reconciler.New(
		reconciler.WithStrategy(strategy),
		reconciler.WithProvenance(true),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	// Reconcile catalogs from multiple sources
	srcMap := map[sources.ID]catalogs.Catalog{
		sources.LocalCatalogID:  embeddedCat,
		sources.ProvidersID:     apiCat,
		sources.ModelsDevHTTPID: modelsDevCat,
	}
	srcs := reconciler.ConvertCatalogsMapToSources(srcMap)

	result, err := reconcile.Sources(ctx, sources.ProvidersID, srcs)
	if err != nil {
		t.Fatalf("ReconcileCatalogs failed: %v", err)
	}

	// Validate the result
	if result == nil || result.Catalog == nil {
		t.Fatal("Expected non-nil result with catalog")
	}

	// Check merged models
	models := result.Catalog.Models().List()
	if len(models) != 3 {
		t.Errorf("Expected 3 models after reconciliation, got %d", len(models))
	}

	// Verify GPT-4 was properly merged
	gpt4, err := result.Catalog.FindModel("gpt-4")
	if err != nil {
		t.Fatal("Expected gpt-4 to exist after reconciliation")
	}

	// Check authority-based merging worked correctly
	// API should win for features
	if gpt4.Features == nil || len(gpt4.Features.Modalities.Input) == 0 {
		t.Error("Expected gpt-4 to have features from API")
	}

	// models.dev should win for pricing
	if gpt4.Pricing == nil || gpt4.Pricing.Tokens == nil ||
		gpt4.Pricing.Tokens.Input == nil || gpt4.Pricing.Tokens.Input.Per1M != 10.0 {
		t.Errorf("Expected gpt-4 pricing from models.dev, got %v", gpt4.Pricing)
	}

	// models.dev should win for metadata
	if gpt4.Metadata == nil || gpt4.Metadata.ReleaseDate.IsZero() {
		t.Error("Expected gpt-4 metadata from models.dev")
	}

	// Check provenance was tracked
	if len(result.Provenance) == 0 {
		t.Error("Expected provenance to be tracked")
	}

	// Verify changeset
	if result.Changeset == nil {
		t.Error("Expected changeset to be included in result")
	}

	// Check metadata
	if result.Metadata.StartTime.IsZero() {
		t.Error("Expected start time to be set")
	}

	if len(result.Metadata.Sources) != 3 {
		t.Errorf("Expected 3 sources in metadata, got %d", len(result.Metadata.Sources))
	}
}

// TestIntegrationWithDifferentStrategies tests reconciliation with various strategies.
func TestIntegrationWithDifferentStrategies(t *testing.T) {
	ctx := context.Background()

	// Create test catalogs
	catalog1, _ := catalogs.New()
	// Add the provider to catalog1 first (so it exists in primary source)
	provider1 := catalogs.Provider{
		ID:   "test-provider",
		Name: "Test Provider from Catalog 1",
	}
	if err := catalog1.SetProvider(provider1); err != nil {
		t.Fatalf("Failed to add provider to catalog1: %v", err)
	}
	// Now add models to the provider
	if err := addModelsToProvider(catalog1, "test-provider", []*catalogs.Model{{
		ID:   "model-1",
		Name: "Model 1 from Catalog 1",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{
					Per1M: 10.0,
				},
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}}); err != nil {
		t.Fatalf("Failed to add models to catalog1: %v", err)
	}

	catalog2, _ := catalogs.New()
	if err := addModelsToProvider(catalog2, "test-provider", []*catalogs.Model{{
		ID:   "model-1",
		Name: "Model 1 from Catalog 2",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{
					Per1M: 20.0,
				},
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}}); err != nil {
		t.Fatalf("Failed to add models to catalog2: %v", err)
	}

	srcMap := map[sources.ID]catalogs.Catalog{
		sources.ProvidersID:    catalog1,
		sources.ModelsDevGitID: catalog2,
	}
	srcs := reconciler.ConvertCatalogsMapToSources(srcMap)

	// Test with different strategies
	strategies := []struct {
		name      string
		strategy  reconciler.Strategy
		wantName  string
		wantPrice float64
	}{
		{
			name:      "Authority-based",
			strategy:  reconciler.NewAuthorityStrategy(authority.New()),
			wantName:  "Model 1 from Catalog 1", // ProviderAPI (priority 90) wins over ModelsDevGit (priority 80) for name
			wantPrice: 20.0,                     // ModelsDevGit has higher priority for pricing
		},
		{
			name: "Source Priority Order",
			strategy: reconciler.NewSourceOrderStrategy([]sources.ID{
				sources.ModelsDevGitID,
				sources.ProvidersID,
			}),
			wantName:  "Model 1 from Catalog 2", // ModelsDevGit has higher priority in the strategy
			wantPrice: 20.0,                     // Price from ModelsDevGit which has higher priority
		},
	}

	for _, tt := range strategies {
		t.Run(tt.name, func(t *testing.T) {
			reconcile, err := reconciler.New(reconciler.WithStrategy(tt.strategy))
			if err != nil {
				t.Fatalf("Failed to create reconciler: %v", err)
			}

			result, err := reconcile.Sources(ctx, sources.ProvidersID, srcs)
			if err != nil {
				t.Fatalf("ReconcileCatalogs failed: %v", err)
			}

			// Debug: Check what's in the result catalog
			allModels := result.Catalog.Models().List()
			t.Logf("Total models in result catalog: %d", len(allModels))
			for _, m := range allModels {
				t.Logf("  Model: ID=%s, Name=%s", m.ID, m.Name)
			}

			// Debug: Check providers
			providers := result.Catalog.Providers().List()
			t.Logf("Total providers in result catalog: %d", len(providers))
			for _, p := range providers {
				modelCount := 0
				if p.Models != nil {
					modelCount = len(p.Models)
				}
				t.Logf("  Provider: ID=%s, Models=%d", p.ID, modelCount)
			}

			model, err := result.Catalog.FindModel("model-1")
			if err != nil {
				t.Fatalf("Expected model-1 to exist, got error: %v", err)
			}

			if model.Name != tt.wantName {
				t.Errorf("Expected name %q, got %q", tt.wantName, model.Name)
			}

			if model.Pricing != nil && model.Pricing.Tokens != nil &&
				model.Pricing.Tokens.Input != nil && model.Pricing.Tokens.Input.Per1M != tt.wantPrice {
				t.Errorf("Expected price %f, got %f", tt.wantPrice, model.Pricing.Tokens.Input.Per1M)
			}
		})
	}
}

// TestIntegrationChangeDetection tests change detection between catalog versions.
func TestIntegrationChangeDetection(t *testing.T) {
	ctx := context.Background()

	// Create initial catalog state
	oldCatalog, _ := catalogs.New()
	oldModels := []*catalogs.Model{
		{
			ID:   "model-1",
			Name: "Model 1",
			Limits: &catalogs.ModelLimits{
				ContextWindow: 1000,
			},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:        "model-2",
			Name:      "Model 2",
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:        "model-3",
			Name:      "Model 3 (Will be removed)",
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
	}
	if err := addModelsToProvider(oldCatalog, "test-provider", oldModels); err != nil {
		t.Fatalf("Failed to add models to old catalog: %v", err)
	}

	// Create new catalog state with changes
	newCatalog, _ := catalogs.New()
	newModels := []*catalogs.Model{
		{
			ID:   "model-1",
			Name: "Model 1 Updated",
			Limits: &catalogs.ModelLimits{
				ContextWindow: 2000, // Changed
			},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:        "model-2",
			Name:      "Model 2", // Unchanged
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:        "model-4",
			Name:      "Model 4 (New)",
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
	}
	if err := addModelsToProvider(newCatalog, "test-provider", newModels); err != nil {
		t.Fatalf("Failed to add models to new catalog: %v", err)
	}

	// Use reconciler to detect changes
	reconcile, err := reconciler.New()
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	// First reconcile to get the new state
	srcMap := map[sources.ID]catalogs.Catalog{
		sources.LocalCatalogID: newCatalog,
	}
	srcs := reconciler.ConvertCatalogsMapToSources(srcMap)

	// Use LocalCatalog as primary since that's what we have
	result, err := reconcile.Sources(ctx, sources.LocalCatalogID, srcs)
	if err != nil {
		t.Fatalf("ReconcileCatalogs failed: %v", err)
	}

	// Now diff against old catalog
	diff := differ.New()
	changeset := diff.Catalogs(oldCatalog, result.Catalog)

	// Verify changes detected
	if !changeset.HasChanges() {
		t.Error("Expected changes to be detected")
	}

	// Check specific changes
	if len(changeset.Models.Added) != 1 {
		t.Errorf("Expected 1 added model, got %d", len(changeset.Models.Added))
	}
	if changeset.Models.Added[0].ID != "model-4" {
		t.Errorf("Expected model-4 to be added, got %s", changeset.Models.Added[0].ID)
	}

	if len(changeset.Models.Updated) != 1 {
		t.Errorf("Expected 1 updated model, got %d", len(changeset.Models.Updated))
	}
	if changeset.Models.Updated[0].ID != "model-1" {
		t.Errorf("Expected model-1 to be updated, got %s", changeset.Models.Updated[0].ID)
	}

	if len(changeset.Models.Removed) != 1 {
		t.Errorf("Expected 1 removed model, got %d", len(changeset.Models.Removed))
	}
	if changeset.Models.Removed[0].ID != "model-3" {
		t.Errorf("Expected model-3 to be removed, got %s", changeset.Models.Removed[0].ID)
	}

	// Test changeset filtering
	additionsOnly := changeset.Filter(differ.ApplyAdditionsOnly)
	if len(additionsOnly.Models.Added) != 1 {
		t.Error("ApplyAdditionsOnly filter failed")
	}
	if len(additionsOnly.Models.Updated) != 0 || len(additionsOnly.Models.Removed) != 0 {
		t.Error("ApplyAdditionsOnly should only include additions")
	}
}

// TestIntegrationProvenanceTracking tests detailed provenance tracking.
func TestIntegrationProvenanceTracking(t *testing.T) {
	ctx := context.Background()

	// Create catalogs with conflicting data
	catalog1, _ := catalogs.New()
	if err := addModelsToProvider(catalog1, "test-provider", []*catalogs.Model{{
		ID:   "test-model",
		Name: "From Catalog 1",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.ModelTokenPricing{
				Input:  &catalogs.ModelTokenCost{Per1M: 10.0},
				Output: &catalogs.ModelTokenCost{Per1M: 20.0},
			},
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 1000,
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}}); err != nil {
		t.Fatalf("Failed to add models to catalog1: %v", err)
	}

	catalog2, _ := catalogs.New()
	if err := addModelsToProvider(catalog2, "test-provider", []*catalogs.Model{{
		ID:   "test-model",
		Name: "From Catalog 2",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.ModelTokenPricing{
				Input:  &catalogs.ModelTokenCost{Per1M: 20.0},
				Output: &catalogs.ModelTokenCost{Per1M: 40.0},
			},
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 2000,
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}}); err != nil {
		t.Fatalf("Failed to add models to catalog2: %v", err)
	}

	catalog3, _ := catalogs.New()
	if err := addModelsToProvider(catalog3, "test-provider", []*catalogs.Model{{
		ID:   "test-model",
		Name: "From Catalog 3",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.ModelTokenPricing{
				Input:  &catalogs.ModelTokenCost{Per1M: 30.0},
				Output: &catalogs.ModelTokenCost{Per1M: 60.0},
			},
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 3000,
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	}}); err != nil {
		t.Fatalf("Failed to add models to catalog3: %v", err)
	}

	// Reconcile with provenance tracking
	reconcile, err := reconciler.New(
		reconciler.WithStrategy(reconciler.NewAuthorityStrategy(authority.New())),
		reconciler.WithProvenance(true),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	srcMap := map[sources.ID]catalogs.Catalog{
		sources.LocalCatalogID:  catalog1,
		sources.ProvidersID:     catalog2,
		sources.ModelsDevHTTPID: catalog3,
	}
	srcs := reconciler.ConvertCatalogsMapToSources(srcMap)

	result, err := reconcile.Sources(ctx, sources.ProvidersID, srcs)
	if err != nil {
		t.Fatalf("ReconcileCatalogs failed: %v", err)
	}

	// Check provenance was tracked
	if result.Provenance == nil {
		t.Fatal("Expected provenance to be tracked")
	}

	// Look for pricing provenance - keys are prefixed with "models.<id>.<field>"
	foundPricingProvenance := false
	for field, infos := range result.Provenance {
		// Check for pricing fields (could be models.model-1.pricing.tokens.input or similar)
		if strings.Contains(field, "pricing") {
			foundPricingProvenance = true
			// Should have provenance info
			if len(infos) == 0 {
				t.Errorf("Expected provenance info for %s", field)
			}
			// The winning source should be ModelsDevGit (since we don't have ModelsDevHTTP in test)
			// ModelsDevGit has priority 100 for pricing.input
			if strings.Contains(field, "input") && infos[0].Source != sources.ModelsDevGitID {
				t.Errorf("Expected pricing.input from ModelsDevGit, got %s for field %s", infos[0].Source, field)
			}
		}
	}

	if !foundPricingProvenance {
		t.Error("Expected provenance tracking for pricing fields")
	}

	// Check if provenance tracked conflicting values
	if len(result.Provenance) == 0 {
		t.Error("Expected provenance data showing conflicting values")
	}
}
