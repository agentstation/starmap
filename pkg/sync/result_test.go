package sync

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
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
