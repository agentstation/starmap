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
func addModelsToProvider(cat *catalogs.Builder, providerID string, models []*catalogs.Model) error {
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
	embeddedCat := catalogs.NewEmpty()

	// Add models to embedded catalog
	embeddedModels := []*catalogs.Model{
		{
			ID:     "gpt-4",
			Name:   "GPT-4 (Embedded)",
			Limits: &catalogs.ModelLimits{ContextWindow: 8192},
			Pricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyUSD,
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
	apiCat := catalogs.NewEmpty()

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
	modelsDevCat := catalogs.NewEmpty()

	modelsDevModels := []*catalogs.Model{
		{
			ID:   "gpt-4",
			Name: "GPT-4",
			Pricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyUSD,
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
				Currency: catalogs.ModelPricingCurrencyUSD,
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
				Currency: catalogs.ModelPricingCurrencyUSD,
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
	srcMap := map[sources.ID]*catalogs.Builder{
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

	// Check merged models. The provider API is the primary source, so
	// models.dev/local data can enrich provider-returned models but cannot add
	// models that the provider API did not return.
	models := result.Catalog.Models().List()
	if len(models) != 2 {
		t.Errorf("Expected 2 primary models after reconciliation, got %d", len(models))
	}
	if _, err := result.Catalog.FindModel("claude-3"); err == nil {
		t.Error("Expected claude-3 to be excluded because it is not in the primary provider API source")
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

	// The reviewed local catalog is the last-known-good price ahead of
	// models.dev when the live provider inventory omits pricing.
	if gpt4.Pricing == nil || gpt4.Pricing.Tokens == nil ||
		gpt4.Pricing.Tokens.Input == nil || gpt4.Pricing.Tokens.Input.Per1M != 30.0 {
		t.Errorf("Expected gpt-4 last-known-good local pricing, got %v", gpt4.Pricing)
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

func TestPricingAuthorityProviderOfferingSelection(t *testing.T) {
	expired := utc.New(time.Now().Add(-time.Hour))
	tests := []struct {
		name             string
		providerPricing  *catalogs.ModelPricing
		wantCurrency     catalogs.ModelPricingCurrency
		wantInputPer1M   float64
		wantOutputAbsent bool
	}{
		{
			name: "valid provider price wins atomically",
			providerPricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyEUR,
				Tokens:   &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: 2}},
			},
			wantCurrency:     catalogs.ModelPricingCurrencyEUR,
			wantInputPer1M:   2,
			wantOutputAbsent: true,
		},
		{
			name: "invalid provider price falls back",
			providerPricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyUSD,
				Tokens:   &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: -1}},
			},
			wantCurrency:   catalogs.ModelPricingCurrencyUSD,
			wantInputPer1M: 0.5,
		},
		{
			name: "expired provider price falls back",
			providerPricing: &catalogs.ModelPricing{
				Currency:       catalogs.ModelPricingCurrencyEUR,
				EffectiveUntil: &expired,
				Tokens:         &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: 3}},
			},
			wantCurrency:   catalogs.ModelPricingCurrencyUSD,
			wantInputPer1M: 0.5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			providerCatalog := catalogs.NewEmpty()
			if err := addModelsToProvider(providerCatalog, "test-provider", []*catalogs.Model{{
				ID: "model-1", Name: "Provider Model", Pricing: test.providerPricing,
			}}); err != nil {
				t.Fatalf("add provider model: %v", err)
			}
			fallbackCatalog := catalogs.NewEmpty()
			if err := addModelsToProvider(fallbackCatalog, "test-provider", []*catalogs.Model{{
				ID:   "model-1",
				Name: "Fallback Model",
				Pricing: &catalogs.ModelPricing{
					Currency: catalogs.ModelPricingCurrencyUSD,
					Tokens: &catalogs.ModelTokenPricing{
						Input:  &catalogs.ModelTokenCost{Per1M: 0.5},
						Output: &catalogs.ModelTokenCost{Per1M: 1},
					},
				},
			}}); err != nil {
				t.Fatalf("add fallback model: %v", err)
			}

			reconcile, err := reconciler.New(
				reconciler.WithStrategy(reconciler.NewAuthorityStrategy(authority.New())),
				reconciler.WithProvenance(true),
			)
			if err != nil {
				t.Fatalf("new reconciler: %v", err)
			}
			observations := reconciler.ConvertCatalogsMapToSources(map[sources.ID]*catalogs.Builder{
				sources.ProvidersID:     providerCatalog,
				sources.ModelsDevHTTPID: fallbackCatalog,
			})
			result, err := reconcile.Sources(context.Background(), sources.ProvidersID, observations)
			if err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			published, err := result.Catalog.Build()
			if err != nil {
				t.Fatalf("publish catalog: %v", err)
			}
			offering, err := published.Offering("test-provider", "model-1")
			if err != nil {
				t.Fatalf("Offering: %v", err)
			}
			if offering.Pricing == nil || offering.Pricing.Currency != test.wantCurrency {
				t.Fatalf("offering pricing = %#v, want currency %q", offering.Pricing, test.wantCurrency)
			}
			if offering.Pricing.Tokens == nil || offering.Pricing.Tokens.Input == nil || offering.Pricing.Tokens.Input.Per1M != test.wantInputPer1M {
				t.Fatalf("offering input pricing = %#v, want %v", offering.Pricing.Tokens, test.wantInputPer1M)
			}
			if test.wantOutputAbsent && offering.Pricing.Tokens.Output != nil {
				t.Fatalf("fallback output price leaked into provider offering: %#v", offering.Pricing.Tokens.Output)
			}
		})
	}
}

func TestFieldEvidencePricingFallbackQueryable(t *testing.T) {
	providerCatalog := catalogs.NewEmpty()
	if err := addModelsToProvider(providerCatalog, "test-provider", []*catalogs.Model{{
		ID: "model-1", Name: "Provider Model", Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
			Tokens:   &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: -1}},
		},
	}}); err != nil {
		t.Fatalf("add provider model: %v", err)
	}
	fallbackCatalog := catalogs.NewEmpty()
	if err := addModelsToProvider(fallbackCatalog, "test-provider", []*catalogs.Model{{
		ID: "model-1", Name: "Fallback Model", Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
			Tokens:   &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: 0.5}},
		},
	}}); err != nil {
		t.Fatalf("add fallback model: %v", err)
	}
	providerSnapshot, err := providerCatalog.Build()
	if err != nil {
		t.Fatalf("build provider catalog: %v", err)
	}
	fallbackSnapshot, err := fallbackCatalog.Build()
	if err != nil {
		t.Fatalf("build fallback catalog: %v", err)
	}
	providerObservedAt := time.Date(2026, time.July, 10, 10, 0, 0, 0, time.UTC)
	fallbackObservedAt := providerObservedAt.Add(time.Minute)
	observations := []sources.Observation{
		{
			ID:               "provider-observation",
			SourceID:         sources.ProvidersID,
			ObservedAt:       providerObservedAt,
			Revision:         sources.Revision{Kind: sources.RevisionKindETag, Value: "provider-v1"},
			EvidenceChecksum: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Catalog:          providerSnapshot,
		},
		{
			ID:               "modelsdev-observation",
			SourceID:         sources.ModelsDevHTTPID,
			ObservedAt:       fallbackObservedAt,
			Revision:         sources.Revision{Kind: sources.RevisionKindETag, Value: "modelsdev-v2"},
			EvidenceChecksum: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Catalog:          fallbackSnapshot,
		},
	}

	reconcile, err := reconciler.New(
		reconciler.WithStrategy(reconciler.NewAuthorityStrategy(authority.New())),
		reconciler.WithProvenance(true),
	)
	if err != nil {
		t.Fatalf("new reconciler: %v", err)
	}
	result, err := reconcile.Sources(context.Background(), sources.ProvidersID, observations)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	entries := result.Provenance["models.test-provider.model-1.pricing"]
	if len(entries) != 1 {
		t.Fatalf("pricing evidence entries = %d, want 1: %#v", len(entries), entries)
	}
	evidence := entries[0]
	if evidence.Source != sources.ModelsDevHTTPID || evidence.ObservationID != "modelsdev-observation" {
		t.Fatalf("winning evidence = %#v, want models.dev observation", evidence)
	}
	if !evidence.ObservedAt.Equal(fallbackObservedAt) || evidence.Revision.Value != "modelsdev-v2" {
		t.Fatalf("winning observation metadata = %#v", evidence)
	}
	if len(evidence.Rejections) != 1 || evidence.Rejections[0].Source != sources.ProvidersID {
		t.Fatalf("rejections = %#v, want rejected provider price", evidence.Rejections)
	}
	if !strings.Contains(evidence.Rejections[0].Reason, "must not be negative") {
		t.Fatalf("provider rejection reason = %q", evidence.Rejections[0].Reason)
	}
}

// TestIntegrationWithDifferentStrategies tests reconciliation with various strategies.
func TestIntegrationWithDifferentStrategies(t *testing.T) {
	ctx := context.Background()

	// Create test catalogs
	catalog1 := catalogs.NewEmpty()
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
			Currency: catalogs.ModelPricingCurrencyUSD,
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

	catalog2 := catalogs.NewEmpty()
	if err := addModelsToProvider(catalog2, "test-provider", []*catalogs.Model{{
		ID:   "model-1",
		Name: "Model 1 from Catalog 2",
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
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

	srcMap := map[sources.ID]*catalogs.Builder{
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
			wantPrice: 10.0,                     // Provider-offering pricing is canonical regardless of general merge strategy
		},
		{
			name: "Source Priority Order",
			strategy: reconciler.NewSourceOrderStrategy([]sources.ID{
				sources.ModelsDevGitID,
				sources.ProvidersID,
			}),
			wantName:  "Model 1 from Catalog 2", // ModelsDevGit has higher priority in the strategy
			wantPrice: 10.0,                     // Provider-offering pricing retains canonical provider authority
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
	oldCatalog := catalogs.NewEmpty()
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
	newCatalog := catalogs.NewEmpty()
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
	srcMap := map[sources.ID]*catalogs.Builder{
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
	catalog1 := catalogs.NewEmpty()
	if err := addModelsToProvider(catalog1, "test-provider", []*catalogs.Model{{
		ID:   "test-model",
		Name: "From Catalog 1",
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
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

	catalog2 := catalogs.NewEmpty()
	if err := addModelsToProvider(catalog2, "test-provider", []*catalogs.Model{{
		ID:   "test-model",
		Name: "From Catalog 2",
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
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

	catalog3 := catalogs.NewEmpty()
	if err := addModelsToProvider(catalog3, "test-provider", []*catalogs.Model{{
		ID:   "test-model",
		Name: "From Catalog 3",
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
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

	srcMap := map[sources.ID]*catalogs.Builder{
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
		// Pricing is selected atomically, so it has one field-level provenance record.
		if strings.Contains(field, "pricing") {
			foundPricingProvenance = true
			// Should have provenance info
			if len(infos) == 0 {
				t.Errorf("Expected provenance info for %s", field)
			}
			if infos[0].Source != sources.ProvidersID {
				t.Errorf("Expected pricing from Providers, got %s for field %s", infos[0].Source, field)
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
