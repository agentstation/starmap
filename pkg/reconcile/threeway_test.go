package reconcile_test

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/reconcile"
	"github.com/agentstation/utc"
)

func TestThreeWayMergeNoConflicts(t *testing.T) {
	// Create three-way merger
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	// Base version
	base := catalogs.Model{
		ID:          "test-model",
		Name:        "Original Name",
		Description: "Original Description",
		Limits: &catalogs.ModelLimits{
			ContextWindow: 1000,
		},
	}

	// Our version - changed name only
	ours := base
	ours.Name = "Our Name"

	// Their version - changed description only
	theirs := base
	theirs.Description = "Their Description"

	// Merge - should have no conflicts
	merged, conflicts, err := merger.MergeModels(base, ours, theirs)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts, got %d", len(conflicts))
	}

	// Both changes should be applied
	if merged.Name != "Our Name" {
		t.Errorf("Expected our name change to be applied")
	}

	if merged.Description != "Their Description" {
		t.Errorf("Expected their description change to be applied")
	}
}

func TestThreeWayMergeWithConflicts(t *testing.T) {
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	// Base version
	base := catalogs.Model{
		ID:          "test-model",
		Name:        "Original Name",
		Description: "Original Description",
	}

	// Our version - changed both fields
	ours := base
	ours.Name = "Our Name"
	ours.Description = "Our Description"

	// Their version - also changed both fields differently
	theirs := base
	theirs.Name = "Their Name"
	theirs.Description = "Their Description"

	// Merge - should have conflicts
	merged, conflicts, err := merger.MergeModels(base, ours, theirs)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	if len(conflicts) != 2 {
		t.Errorf("Expected 2 conflicts, got %d", len(conflicts))
	}

	// Check conflict details
	for _, conflict := range conflicts {
		if conflict.Type != reconcile.ConflictTypeModified {
			t.Errorf("Expected modified conflict type, got %s", conflict.Type)
		}

		switch conflict.Path {
		case "name":
			if conflict.Base != "Original Name" {
				t.Errorf("Expected base name to be 'Original Name'")
			}
			if conflict.Ours != "Our Name" {
				t.Errorf("Expected our name to be 'Our Name'")
			}
			if conflict.Theirs != "Their Name" {
				t.Errorf("Expected their name to be 'Their Name'")
			}

		case "description":
			if conflict.Base != "Original Description" {
				t.Errorf("Expected base description to be 'Original Description'")
			}
		}
	}

	// By default, ours wins in conflicts
	if merged.Name != "Our Name" {
		t.Errorf("Expected our changes to win by default, got %q", merged.Name)
	}
}

func TestThreeWayMergePricing(t *testing.T) {
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	// Base version with pricing
	base := catalogs.Model{
		ID: "test-model",
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
	}

	// Our version - changed input pricing
	ours := base
	ours.Pricing = &catalogs.ModelPricing{
		Tokens: &catalogs.TokenPricing{
			Input: &catalogs.TokenCost{
				Per1M: 15.0,
			},
			Output: &catalogs.TokenCost{
				Per1M: 20.0,
			},
		},
	}

	// Their version - changed output pricing
	theirs := base
	theirs.Pricing = &catalogs.ModelPricing{
		Tokens: &catalogs.TokenPricing{
			Input: &catalogs.TokenCost{
				Per1M: 10.0,
			},
			Output: &catalogs.TokenCost{
				Per1M: 30.0,
			},
		},
	}

	// Merge - should have no conflicts (different fields changed)
	merged, conflicts, err := merger.MergeModels(base, ours, theirs)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts, got %d", len(conflicts))
	}

	// Both changes should be applied
	if merged.Pricing.Tokens.Input.Per1M != 15.0 {
		t.Errorf("Expected our input pricing change")
	}

	if merged.Pricing.Tokens.Output.Per1M != 30.0 {
		t.Errorf("Expected their output pricing change")
	}
}

func TestThreeWayMergeLimitsAutoMerge(t *testing.T) {
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	// Base version
	base := catalogs.Model{
		ID: "test-model",
		Limits: &catalogs.ModelLimits{
			ContextWindow: 1000,
			OutputTokens:  100,
		},
	}

	// Our version - increased context window
	ours := base
	ours.Limits = &catalogs.ModelLimits{
		ContextWindow: 2000,
		OutputTokens:  100,
	}

	// Their version - increased context window differently
	theirs := base
	theirs.Limits = &catalogs.ModelLimits{
		ContextWindow: 3000,
		OutputTokens:  100,
	}

	// Merge - should detect conflict but auto-merge to maximum
	merged, conflicts, err := merger.MergeModels(base, ours, theirs)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	// Should have a conflict
	if len(conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(conflicts))
	}

	// Check the conflict can be auto-merged
	if len(conflicts) > 0 {
		conflict := conflicts[0]
		if !conflict.CanMerge {
			t.Error("Expected conflict to be auto-mergeable")
		}
		if conflict.Suggested != int64(3000) {
			t.Errorf("Expected suggested value to be maximum (3000)")
		}
	}

	// Maximum value should win
	if merged.Limits.ContextWindow != 3000 {
		t.Errorf("Expected maximum context window (3000), got %d", merged.Limits.ContextWindow)
	}
}

func TestThreeWayMergeFeatures(t *testing.T) {
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	// Base version
	base := catalogs.Model{
		ID: "test-model",
		Features: &catalogs.ModelFeatures{
			ToolCalls: false,
			Tools:     false,
			Reasoning: false,
		},
	}

	// Our version - enabled tool calls
	ours := base
	ours.Features = &catalogs.ModelFeatures{
		ToolCalls: true,
		Tools:     false,
		Reasoning: false,
	}

	// Their version - enabled reasoning
	theirs := base
	theirs.Features = &catalogs.ModelFeatures{
		ToolCalls: false,
		Tools:     false,
		Reasoning: true,
	}

	// Merge - should OR the boolean features
	merged, conflicts, err := merger.MergeModels(base, ours, theirs)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	// No conflicts for different features
	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts, got %d", len(conflicts))
	}

	// Both features should be enabled
	if !merged.Features.ToolCalls {
		t.Error("Expected ToolCalls to be enabled")
	}

	if !merged.Features.Reasoning {
		t.Error("Expected Reasoning to be enabled")
	}
}

func TestThreeWayMergeModalities(t *testing.T) {
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	// Base version
	base := catalogs.Model{
		ID: "test-model",
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
		},
	}

	// Our version - added image input
	ours := base
	ours.Features = &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
	}

	// Their version - added audio input
	theirs := base
	theirs.Features = &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityAudio},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
	}

	// Merge - should union the modalities
	merged, conflicts, err := merger.MergeModels(base, ours, theirs)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	// Should have a conflict but auto-mergeable
	if len(conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(conflicts))
	}

	if len(conflicts) > 0 && !conflicts[0].CanMerge {
		t.Error("Expected modality conflict to be auto-mergeable")
	}

	// Should have all three modalities
	hasText := false
	hasImage := false
	hasAudio := false
	
	for _, m := range merged.Features.Modalities.Input {
		switch m {
		case catalogs.ModelModalityText:
			hasText = true
		case catalogs.ModelModalityImage:
			hasImage = true
		case catalogs.ModelModalityAudio:
			hasAudio = true
		}
	}

	if !hasText || !hasImage || !hasAudio {
		t.Errorf("Expected union of all modalities, got %v", merged.Features.Modalities.Input)
	}
}

func TestThreeWayMergeMetadata(t *testing.T) {
	now := utc.Now()
	later := now.Add(24 * time.Hour) // utc.Time.Add returns utc.Time

	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	// Base version
	base := catalogs.Model{
		ID: "test-model",
		Metadata: &catalogs.ModelMetadata{
			ReleaseDate: now,
			OpenWeights: false,
		},
	}

	// Our version - changed release date
	ours := base
	ours.Metadata = &catalogs.ModelMetadata{
		ReleaseDate: later,
		OpenWeights: false,
	}

	// Their version - changed open weights
	theirs := base
	theirs.Metadata = &catalogs.ModelMetadata{
		ReleaseDate: now,
		OpenWeights: true,
	}

	// Merge
	merged, conflicts, err := merger.MergeModels(base, ours, theirs)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	// Should have no conflicts (different fields)
	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts, got %d", len(conflicts))
	}

	// Both changes should apply
	if merged.Metadata.ReleaseDate != later {
		t.Error("Expected our release date change")
	}

	if !merged.Metadata.OpenWeights {
		t.Error("Expected their open weights change")
	}
}

func TestConflictResolution(t *testing.T) {
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	// Create some conflicts
	conflicts := []reconcile.Conflict{
		{
			Path:      "name",
			Base:      "Base",
			Ours:      "Ours",
			Theirs:    "Theirs",
			Type:      reconcile.ConflictTypeModified,
			CanMerge:  false,
			Suggested: nil,
		},
		{
			Path:      "description",
			Base:      "Base Desc",
			Ours:      "Our Desc",
			Theirs:    "Their Desc",
			Type:      reconcile.ConflictTypeModified,
			CanMerge:  true,
			Suggested: "Merged Desc",
		},
	}

	// Test different resolution strategies
	strategies := []struct {
		strategy reconcile.ConflictResolution
		wantName string
		wantDesc string
	}{
		{
			strategy: reconcile.ResolutionOurs,
			wantName: "Ours",
			wantDesc: "Our Desc",
		},
		{
			strategy: reconcile.ResolutionTheirs,
			wantName: "Theirs",
			wantDesc: "Their Desc",
		},
		{
			strategy: reconcile.ResolutionBase,
			wantName: "Base",
			wantDesc: "Base Desc",
		},
		{
			strategy: reconcile.ResolutionMerge,
			wantName: "Ours",      // Cannot auto-merge, falls back to ours
			wantDesc: "Merged Desc", // Can auto-merge
		},
	}

	for _, tt := range strategies {
		t.Run(string(tt.strategy), func(t *testing.T) {
			resolutions := merger.ResolveConflicts(conflicts, tt.strategy)

			if len(resolutions) != 2 {
				t.Fatalf("Expected 2 resolutions, got %d", len(resolutions))
			}

			// Check name resolution
			if resolutions[0].Value != tt.wantName {
				t.Errorf("Expected name resolution %q, got %q", tt.wantName, resolutions[0].Value)
			}

			// Check description resolution
			if resolutions[1].Value != tt.wantDesc {
				t.Errorf("Expected description resolution %q, got %q", tt.wantDesc, resolutions[1].Value)
			}
		})
	}
}

func TestThreeWayMergeProviders(t *testing.T) {
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	headquarters := "Base HQ"
	base := catalogs.Provider{
		ID:           "test-provider",
		Name:         "Base Name",
		Headquarters: &headquarters,
	}

	ours := base
	ours.Name = "Our Name"

	theirs := base
	theirsHQ := "Their HQ"
	theirs.Headquarters = &theirsHQ

	merged, conflicts, err := merger.MergeProviders(base, ours, theirs)
	if err != nil {
		t.Fatalf("MergeProviders failed: %v", err)
	}

	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts, got %d", len(conflicts))
	}

	if merged.Name != "Our Name" {
		t.Error("Expected our name change")
	}

	if merged.Headquarters == nil || *merged.Headquarters != "Their HQ" {
		t.Error("Expected their headquarters change")
	}
}

func TestThreeWayMergeAuthors(t *testing.T) {
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	website := "http://base.com"
	base := catalogs.Author{
		ID:      "test-author",
		Name:    "Base Name",
		Website: &website,
	}

	ours := base
	ours.Name = "Our Name"

	theirs := base
	theirsWebsite := "http://theirs.com"
	theirs.Website = &theirsWebsite

	merged, conflicts, err := merger.MergeAuthors(base, ours, theirs)
	if err != nil {
		t.Fatalf("MergeAuthors failed: %v", err)
	}

	if len(conflicts) != 0 {
		t.Errorf("Expected no conflicts, got %d", len(conflicts))
	}

	if merged.Name != "Our Name" {
		t.Error("Expected our name change")
	}

	if merged.Website == nil || *merged.Website != "http://theirs.com" {
		t.Error("Expected their website change")
	}
}

func BenchmarkThreeWayMerge(b *testing.B) {
	authorities := reconcile.NewDefaultAuthorityProvider()
	strategy := reconcile.NewAuthorityBasedStrategy(authorities)
	merger := reconcile.NewThreeWayMerger(authorities, strategy)

	// Create test models with various fields
	base := catalogs.Model{
		ID:          "test-model",
		Name:        "Base Model",
		Description: "Base Description",
		Pricing: &catalogs.ModelPricing{
			Tokens: &catalogs.TokenPricing{
				Input:  &catalogs.TokenCost{Per1M: 10.0},
				Output: &catalogs.TokenCost{Per1M: 20.0},
			},
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 1000,
			OutputTokens:  100,
		},
		Features: &catalogs.ModelFeatures{
			ToolCalls: true,
			Reasoning: false,
		},
	}

	ours := base
	ours.Name = "Our Model"
	ours.Pricing.Tokens.Input.Per1M = 15.0
	ours.Features.Reasoning = true

	theirs := base
	theirs.Description = "Their Description"
	theirs.Limits.ContextWindow = 2000
	theirs.Features.ToolCalls = false

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := merger.MergeModels(base, ours, theirs)
		if err != nil {
			b.Fatalf("MergeModels failed: %v", err)
		}
	}
}