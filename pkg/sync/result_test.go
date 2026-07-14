package sync

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestChangesetToResultHandlesNilChangeset(t *testing.T) {
	result := ChangesetToResult(nil, true, "/tmp/catalog", nil, nil)

	if result == nil {
		t.Fatal("Expected result")
	}
	if result.HasChanges() {
		t.Fatal("Expected nil changeset to produce a no-change result")
	}
	if !result.DryRun {
		t.Fatal("Expected dry-run flag to be preserved")
	}
	if result.OutputDir != "/tmp/catalog" {
		t.Fatalf("Expected output dir to be preserved, got %q", result.OutputDir)
	}
}

func TestChangesetToResultCountsModelsDevEnrichmentOnlyWhenSourceActive(t *testing.T) {
	changeset := &differ.Changeset{
		Models: &differ.ModelChangeset{
			Updated: []differ.ModelUpdate{{
				ID:  "model-a",
				New: catalogs.Model{ID: "model-a"},
				Changes: []differ.FieldChange{{
					Path: "pricing.tokens.input",
					Type: differ.ChangeTypeUpdate,
				}},
			}},
		},
		Summary: differ.ChangesetSummary{
			ModelsUpdated: 1,
			TotalChanges:  1,
		},
	}
	providerMap := map[string]catalogs.ProviderID{"model-a": "provider-a"}

	result := ChangesetToResult(changeset, false, "", nil, providerMap)
	if result.ProviderResults["provider-a"].EnhancedCount != 0 {
		t.Fatalf("EnhancedCount without models.dev source = %d, want 0", result.ProviderResults["provider-a"].EnhancedCount)
	}

	result = ChangesetToResult(changeset, false, "", nil, providerMap, sources.ProvidersID, sources.ModelsDevHTTPID)
	if result.ProviderResults["provider-a"].EnhancedCount != 0 {
		t.Fatalf("EnhancedCount without field source = %d, want 0", result.ProviderResults["provider-a"].EnhancedCount)
	}

	changeset.Models.Updated[0].Changes[0].Source = sources.ModelsDevHTTPID
	result = ChangesetToResult(changeset, false, "", nil, providerMap, sources.ProvidersID, sources.ModelsDevHTTPID)
	if result.ProviderResults["provider-a"].EnhancedCount != 1 {
		t.Fatalf("EnhancedCount with models.dev field source = %d, want 1", result.ProviderResults["provider-a"].EnhancedCount)
	}
}

func TestChangesetToResultCountsModelsDevEnrichmentFromReconcilerProvenance(t *testing.T) {
	ctx := context.Background()
	baseline := catalogWithProvider(t, "provider-a", catalogs.Model{
		ID:   "gpt-4.1",
		Name: "Model A",
	})
	providerSource := catalogWithProvider(t, "provider-a", catalogs.Model{
		ID:   "gpt-4.1",
		Name: "Model A",
	})
	modelsDevSource := catalogWithProvider(t, "provider-a", catalogs.Model{
		ID:   "gpt-4.1",
		Name: "Model A",
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
			Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{Per1M: 1.25},
			},
		},
	})

	baselineSnapshot, err := baseline.Build()
	if err != nil {
		t.Fatalf("Failed to snapshot baseline: %v", err)
	}
	reconcile, err := reconciler.New(reconciler.WithBaseline(baselineSnapshot))
	if err != nil {
		t.Fatalf("Failed to create reconciler: %v", err)
	}
	reconcileResult, err := reconcile.Sources(ctx, sources.ProvidersID, reconciler.ConvertCatalogsMapToSources(map[sources.ID]*catalogs.Builder{
		sources.ProvidersID:     providerSource,
		sources.ModelsDevHTTPID: modelsDevSource,
	}))
	if err != nil {
		t.Fatalf("Failed to reconcile sources: %v", err)
	}
	if len(reconcileResult.Provenance) == 0 {
		t.Fatal("Expected reconciler provenance")
	}

	result := ChangesetToResultWithProvenance(
		reconcileResult.Changeset,
		false,
		"",
		reconcileResult.ProviderAPICounts,
		reconcileResult.ModelProviderMap,
		reconcileResult.Provenance,
		sources.ProvidersID,
		sources.ModelsDevHTTPID,
	)
	if result.ProviderResults["provider-a"].EnhancedCount != 1 {
		t.Fatalf("EnhancedCount from reconciler provenance = %d, want 1", result.ProviderResults["provider-a"].EnhancedCount)
	}
}

func TestChangesetToResultScopesEnhancedCountByProvider(t *testing.T) {
	changeset := &differ.Changeset{
		Models: &differ.ModelChangeset{
			Updated: []differ.ModelUpdate{
				{
					ID:         "shared",
					ProviderID: "provider-a",
					New:        catalogs.Model{ID: "shared"},
					Changes: []differ.FieldChange{{
						Path: "pricing.tokens",
						Type: differ.ChangeTypeUpdate,
					}},
				},
				{
					ID:         "shared",
					ProviderID: "provider-b",
					New:        catalogs.Model{ID: "shared"},
					Changes: []differ.FieldChange{{
						Path: "pricing.tokens",
						Type: differ.ChangeTypeUpdate,
					}},
				},
			},
		},
		Summary: differ.ChangesetSummary{
			ModelsUpdated: 2,
			TotalChanges:  2,
		},
	}

	result := ChangesetToResultWithProvenance(
		changeset,
		false,
		"",
		nil,
		nil,
		provenance.Map{
			"models.provider-a.shared.pricing": {
				{Source: sources.ModelsDevHTTPID},
			},
			"models.provider-b.shared.pricing": {
				{Source: sources.ProvidersID},
			},
		},
		sources.ProvidersID,
		sources.ModelsDevHTTPID,
	)

	if result.ProviderResults["provider-a"].EnhancedCount != 1 {
		t.Fatalf("provider-a EnhancedCount = %d, want 1", result.ProviderResults["provider-a"].EnhancedCount)
	}
	if result.ProviderResults["provider-b"].EnhancedCount != 0 {
		t.Fatalf("provider-b EnhancedCount = %d, want 0", result.ProviderResults["provider-b"].EnhancedCount)
	}
}

func TestChangesetToResultGroupsProviderScopedModelUpdates(t *testing.T) {
	changeset := &differ.Changeset{
		Models: &differ.ModelChangeset{
			Updated: []differ.ModelUpdate{
				{
					ID:         "shared",
					ProviderID: "provider-a",
					New:        catalogs.Model{ID: "shared", Name: "A"},
				},
				{
					ID:         "shared",
					ProviderID: "provider-b",
					New:        catalogs.Model{ID: "shared", Name: "B"},
				},
			},
		},
		Summary: differ.ChangesetSummary{
			ModelsUpdated: 2,
			TotalChanges:  2,
		},
	}

	result := ChangesetToResult(changeset, false, "", nil, map[string]catalogs.ProviderID{
		"shared": "provider-b",
	})
	if result.ProvidersChanged != 2 {
		t.Fatalf("ProvidersChanged = %d, want 2: %#v", result.ProvidersChanged, result.ProviderResults)
	}
	if result.ProviderResults["provider-a"].UpdatedCount != 1 {
		t.Fatalf("provider-a UpdatedCount = %d, want 1", result.ProviderResults["provider-a"].UpdatedCount)
	}
	if result.ProviderResults["provider-b"].UpdatedCount != 1 {
		t.Fatalf("provider-b UpdatedCount = %d, want 1", result.ProviderResults["provider-b"].UpdatedCount)
	}
}

func TestChangesetToResultGroupsProviderScopedModelAddsAndRemoves(t *testing.T) {
	changeset := &differ.Changeset{
		Models: &differ.ModelChangeset{
			Added: []catalogs.Model{{ID: "shared", Name: "Added"}},
			AddedScoped: []differ.ModelChange{{
				ProviderID: "provider-a",
				Model:      catalogs.Model{ID: "shared", Name: "Added"},
			}},
			Removed: []catalogs.Model{{ID: "shared", Name: "Removed"}},
			RemovedScoped: []differ.ModelChange{{
				ProviderID: "provider-b",
				Model:      catalogs.Model{ID: "shared", Name: "Removed"},
			}},
		},
		Summary: differ.ChangesetSummary{
			ModelsAdded:   1,
			ModelsRemoved: 1,
			TotalChanges:  2,
		},
	}

	result := ChangesetToResult(changeset, false, "", nil, map[string]catalogs.ProviderID{
		"shared": "provider-c",
	})
	if result.ProvidersChanged != 2 {
		t.Fatalf("ProvidersChanged = %d, want 2: %#v", result.ProvidersChanged, result.ProviderResults)
	}
	if result.ProviderResults["provider-a"].AddedCount != 1 {
		t.Fatalf("provider-a AddedCount = %d, want 1", result.ProviderResults["provider-a"].AddedCount)
	}
	if result.ProviderResults["provider-b"].RemovedCount != 1 {
		t.Fatalf("provider-b RemovedCount = %d, want 1", result.ProviderResults["provider-b"].RemovedCount)
	}
	if _, ok := result.ProviderResults["provider-c"]; ok {
		t.Fatalf("ambiguous provider map was used despite scoped changes: %#v", result.ProviderResults["provider-c"])
	}
}

func TestChangesetToResultHandlesPartiallyInitializedChangeset(t *testing.T) {
	result := ChangesetToResult(&differ.Changeset{
		Summary: differ.ChangesetSummary{
			TotalChanges: 1,
		},
	}, false, "", nil, nil)

	if result == nil {
		t.Fatal("Expected result")
	}
	if !result.HasChanges() {
		t.Fatal("Expected summary total to be preserved")
	}
	if result.ProvidersChanged != 0 {
		t.Fatalf("Expected no provider-specific results without model changes, got %d", result.ProvidersChanged)
	}
}

func TestChangesetToResultGroupsAddedModelsByProvider(t *testing.T) {
	result := ChangesetToResult(&differ.Changeset{
		Models: &differ.ModelChangeset{
			Added: []catalogs.Model{{ID: "model-a"}},
		},
		Summary: differ.ChangesetSummary{
			ModelsAdded:  1,
			TotalChanges: 1,
		},
	}, false, "", map[catalogs.ProviderID]int{
		"provider-a": 7,
	}, map[string]catalogs.ProviderID{
		"model-a": "provider-a",
	})

	providerResult, ok := result.ProviderResults["provider-a"]
	if !ok {
		t.Fatalf("Expected provider result for provider-a, got %#v", result.ProviderResults)
	}
	if providerResult.AddedCount != 1 {
		t.Fatalf("Expected one added model, got %d", providerResult.AddedCount)
	}
	if providerResult.APIModelsCount != 7 {
		t.Fatalf("Expected provider API count to be preserved, got %d", providerResult.APIModelsCount)
	}
}

func TestChangesetToResultReportsCanonicalOfferingChangesByProvider(t *testing.T) {
	changes := &differ.Changeset{
		Offerings: &differ.ProviderOfferingChangeset{
			Updated: []differ.ProviderOfferingUpdate{{Key: catalogs.OfferingKey{ProviderID: "amazon-bedrock", ProviderModelID: "model"}}},
		},
		Summary: differ.ChangesetSummary{OfferingsUpdated: 1, TotalChanges: 1},
	}
	result := ChangesetToResult(changes, false, "", nil, nil, sources.AmazonBedrockID)
	if !result.HasChanges() || result.OfferingsUpdated != 1 || result.ProvidersChanged != 1 {
		t.Fatalf("canonical sync result = %#v", result)
	}
	provider := result.ProviderResults["amazon-bedrock"]
	if provider == nil || !provider.HasChanges() || provider.OfferingUpdatedCount != 1 {
		t.Fatalf("canonical provider result = %#v", provider)
	}
}

func catalogWithProvider(t *testing.T, providerID catalogs.ProviderID, model catalogs.Model) *catalogs.Builder {
	t.Helper()

	cat := catalogs.NewEmpty()
	modelCopy := catalogs.DeepCopyModel(model)
	if err := cat.SetProvider(catalogs.Provider{
		ID:   providerID,
		Name: string(providerID),
		Models: map[string]*catalogs.Model{
			modelCopy.ID: &modelCopy,
		},
	}); err != nil {
		t.Fatalf("Failed to seed provider %q: %v", providerID, err)
	}
	return cat
}
