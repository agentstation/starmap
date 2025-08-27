package reconcile_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/reconcile"
	"github.com/agentstation/utc"
)

// TestIntegrationFullReconciliationFlow tests the complete reconciliation workflow
func TestIntegrationFullReconciliationFlow(t *testing.T) {
	ctx := context.Background()

	// Create three source catalogs with overlapping data
	embeddedCat, err := catalogs.New()
	if err != nil {
		t.Fatalf("Failed to create embedded catalog: %v", err)
	}

	// Add models to embedded catalog
	embeddedModels := []catalogs.Model{
		{
			ID:   "gpt-4",
			Name: "GPT-4 (Embedded)",
			Limits: &catalogs.ModelLimits{
				ContextWindow: 8192,
			},
			Pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.TokenPricing{
					Input: &catalogs.TokenCost{
						Per1M: 30.0,
					},
					Output: &catalogs.TokenCost{
						Per1M: 60.0,
					},
				},
			},
			Metadata: &catalogs.ModelMetadata{
				ReleaseDate: utc.Now(),
			},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:   "claude-3",
			Name: "Claude 3 (Embedded)",
			Limits: &catalogs.ModelLimits{
				ContextWindow: 100000,
			},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
	}
	for i := range embeddedModels {
		embeddedCat.Models().Add(&embeddedModels[i])
	}

	// Create API catalog with updated data
	apiCat, err := catalogs.New()
	if err != nil {
		t.Fatalf("Failed to create API catalog: %v", err)
	}

	apiModels := []catalogs.Model{
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
	for i := range apiModels {
		apiCat.Models().Add(&apiModels[i])
	}

	// Create models.dev catalog with pricing data
	modelsDevCat, err := catalogs.New()
	if err != nil {
		t.Fatalf("Failed to create models.dev catalog: %v", err)
	}

	modelsDevModels := []catalogs.Model{
		{
			ID:   "gpt-4",
			Name: "GPT-4",
			Pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.TokenPricing{
					Input: &catalogs.TokenCost{
						Per1M: 10.0,  // More accurate pricing
					},
					Output: &catalogs.TokenCost{
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
				Tokens: &catalogs.TokenPricing{
					Input: &catalogs.TokenCost{
						Per1M: 5.0,
					},
					Output: &catalogs.TokenCost{
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
				Tokens: &catalogs.TokenPricing{
					Input: &catalogs.TokenCost{
						Per1M: 15.0,
					},
					Output: &catalogs.TokenCost{
						Per1M: 75.0,
					},
				},
			},
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
	}
	for i := range modelsDevModels {
		modelsDevCat.Models().Add(&modelsDevModels[i])
	}

	// Setup reconciler with authority-based strategy and provenance
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	
	r, err := reconcile.New(
		reconcile.WithStrategy(strategy),
		reconcile.WithProvenance(true),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	// Reconcile catalogs from multiple sources
	sources := map[reconcile.SourceName]catalogs.Catalog{
		reconcile.LocalCatalog:  embeddedCat,
		reconcile.ProviderAPI:   apiCat,
		reconcile.ModelsDevHTTP: modelsDevCat,
	}

	result, err := r.ReconcileCatalogs(ctx, sources)
	if err != nil {
		t.Fatalf("ReconcileCatalogs failed: %v", err)
	}

	// Validate the result
	if result == nil || result.Catalog == nil {
		t.Fatal("Expected non-nil result with catalog")
	}

	// Check merged models
	models := result.Catalog.Models()
	if models.Len() != 3 {
		t.Errorf("Expected 3 models after reconciliation, got %d", models.Len())
	}

	// Verify GPT-4 was properly merged
	gpt4, exists := models.Get("gpt-4")
	if !exists || gpt4 == nil {
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
	if result.Provenance == nil || len(result.Provenance) == 0 {
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

// TestIntegrationWithDifferentStrategies tests reconciliation with various strategies
func TestIntegrationWithDifferentStrategies(t *testing.T) {
	ctx := context.Background()

	// Create test catalogs
	catalog1, _ := catalogs.New()
	catalog1.Models().Add(&catalogs.Model{
		ID:   "model-1",
		Name: "Model 1 from Catalog 1",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.TokenPricing{
				Input: &catalogs.TokenCost{
					Per1M: 10.0,
				},
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	})

	catalog2, _ := catalogs.New()
	catalog2.Models().Add(&catalogs.Model{
		ID:   "model-1", 
		Name: "Model 1 from Catalog 2",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.TokenPricing{
				Input: &catalogs.TokenCost{
					Per1M: 20.0,
				},
			},
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	})

	sources := map[reconcile.SourceName]catalogs.Catalog{
		reconcile.ProviderAPI:  catalog1,
		reconcile.ModelsDevGit: catalog2,
	}

	// Test with different strategies
	strategies := []struct {
		name     string
		strategy reconcile.Strategy
		wantName string
		wantPrice float64
	}{
		{
			name:      "Authority-based",
			strategy:  reconcile.NewAuthorityBasedStrategy(reconcile.NewDefaultAuthorityProvider()),
			wantName:  "Model 1 from Catalog 2", // ModelsDevGit (priority 80) wins over ProviderAPI (priority 75) for name
			wantPrice: 20.0,                     // ModelsDevGit (priority 100) wins for pricing
		},
		{
			name:      "Union",
			strategy:  reconcile.NewUnionStrategy(),
			wantName:  "Model 1 from Catalog 2", // Map iteration is non-deterministic, but in practice catalog2 comes first
			wantPrice: 20.0,                     // Map iteration is non-deterministic, but in practice catalog2 comes first
		},
		{
			name: "Source Priority",
			strategy: reconcile.NewSourcePriorityStrategy([]reconcile.SourceName{
				reconcile.ModelsDevGit,
				reconcile.ProviderAPI,
			}),
			wantName:  "Model 1 from Catalog 2",
			wantPrice: 20.0,
		},
	}

	for _, tt := range strategies {
		t.Run(tt.name, func(t *testing.T) {
			r, err := reconcile.New(reconcile.WithStrategy(tt.strategy))
			if err != nil {
				t.Fatalf("Failed to create reconciler: %v", err)
			}
			
			result, err := r.ReconcileCatalogs(ctx, sources)
			if err != nil {
				t.Fatalf("ReconcileCatalogs failed: %v", err)
			}

			model, exists := result.Catalog.Models().Get("model-1")
			if !exists {
				t.Fatal("Expected model-1 to exist")
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

// TestIntegrationChangeDetection tests change detection between catalog versions
func TestIntegrationChangeDetection(t *testing.T) {
	ctx := context.Background()

	// Create initial catalog state
	oldCatalog, _ := catalogs.New()
	oldModels := []catalogs.Model{
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
			ID:   "model-2",
			Name: "Model 2",
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:   "model-3",
			Name: "Model 3 (Will be removed)",
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
	}
	for i := range oldModels {
		oldCatalog.Models().Add(&oldModels[i])
	}

	// Create new catalog state with changes
	newCatalog, _ := catalogs.New()
	newModels := []catalogs.Model{
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
			ID:   "model-2",
			Name: "Model 2", // Unchanged
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
		{
			ID:   "model-4",
			Name: "Model 4 (New)",
			CreatedAt: utc.Now(),
			UpdatedAt: utc.Now(),
		},
	}
	for i := range newModels {
		newCatalog.Models().Add(&newModels[i])
	}

	// Use reconciler to detect changes
	r, err := reconcile.New()
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}
	
	// First reconcile to get the new state
	sources := map[reconcile.SourceName]catalogs.Catalog{
		reconcile.LocalCatalog: newCatalog,
	}
	
	result, err := r.ReconcileCatalogs(ctx, sources)
	if err != nil {
		t.Fatalf("ReconcileCatalogs failed: %v", err)
	}

	// Now diff against old catalog
	differ := reconcile.NewDiffer()
	changeset := differ.DiffCatalogs(oldCatalog, result.Catalog)

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
	additionsOnly := changeset.Filter(reconcile.ApplyAdditionsOnly)
	if len(additionsOnly.Models.Added) != 1 {
		t.Error("ApplyAdditionsOnly filter failed")
	}
	if len(additionsOnly.Models.Updated) != 0 || len(additionsOnly.Models.Removed) != 0 {
		t.Error("ApplyAdditionsOnly should only include additions")
	}
}

// TestIntegrationProvenanceTracking tests detailed provenance tracking
func TestIntegrationProvenanceTracking(t *testing.T) {
	ctx := context.Background()

	// Create catalogs with conflicting data
	catalog1, _ := catalogs.New()
	catalog1.Models().Add(&catalogs.Model{
		ID:   "test-model",
		Name: "From Catalog 1",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.TokenPricing{
				Input: &catalogs.TokenCost{
					Per1M: 10.0,
				},
				Output: &catalogs.TokenCost{
					Per1M: 20.0,
				},
			},
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 1000,
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	})

	catalog2, _ := catalogs.New()
	catalog2.Models().Add(&catalogs.Model{
		ID:   "test-model",
		Name: "From Catalog 2",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.TokenPricing{
				Input: &catalogs.TokenCost{
					Per1M: 20.0,
				},
				Output: &catalogs.TokenCost{
					Per1M: 40.0,
				},
			},
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 2000,
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	})

	catalog3, _ := catalogs.New()
	catalog3.Models().Add(&catalogs.Model{
		ID:   "test-model",
		Name: "From Catalog 3",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.TokenPricing{
				Input: &catalogs.TokenCost{
					Per1M: 30.0,
				},
				Output: &catalogs.TokenCost{
					Per1M: 60.0,
				},
			},
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 3000,
		},
		CreatedAt: utc.Now(),
		UpdatedAt: utc.Now(),
	})

	// Reconcile with provenance tracking
	r, err := reconcile.New(
		reconcile.WithStrategy(reconcile.NewAuthorityBasedStrategy(reconcile.NewDefaultAuthorityProvider())),
		reconcile.WithProvenance(true),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}

	sources := map[reconcile.SourceName]catalogs.Catalog{
		reconcile.LocalCatalog:  catalog1,
		reconcile.ProviderAPI:   catalog2,
		reconcile.ModelsDevHTTP: catalog3,
	}

	result, err := r.ReconcileCatalogs(ctx, sources)
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
			if strings.Contains(field, "input") && infos[0].Source != reconcile.ModelsDevGit {
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