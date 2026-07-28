package catalogs

import "testing"

// TestF004CharacterizationEnrichMergeDropsManualModelWithoutPricingOrLimits
// pins the current "substantial data" filter. P3/P4 must invert this
// expectation so a human model is not discarded merely because pricing and
// limits are absent.
func TestF004CharacterizationEnrichMergeDropsManualModelWithoutPricingOrLimits(t *testing.T) {
	destination := NewEmpty()
	if err := destination.SetProvider(Provider{ID: "manual", Name: "Manual"}); err != nil {
		t.Fatalf("SetProvider destination: %v", err)
	}

	manualModel := Model{
		ID:          "operator-model",
		Name:        "Operator Model",
		Description: "hand-authored fallback metadata",
	}
	source := NewEmpty()
	if err := source.SetProvider(Provider{
		ID:   "manual",
		Name: "Manual",
		Models: map[string]*Model{
			manualModel.ID: &manualModel,
		},
	}); err != nil {
		t.Fatalf("SetProvider source: %v", err)
	}
	sourceCatalog, err := source.Build()
	if err != nil {
		t.Fatalf("Build source: %v", err)
	}

	if err := destination.MergeWith(sourceCatalog, WithStrategy(MergeEnrichEmpty)); err != nil {
		t.Fatalf("MergeWith: %v", err)
	}
	provider, err := destination.Provider("manual")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if _, exists := provider.Models[manualModel.ID]; exists {
		t.Fatal("F-004 characterization changed: metadata-only manual model survived enrichment")
	}
}

// TestF008MergeModelsUsesExplicitPresence proves zero values clear only when a
// source explicitly supplied them; missing and unknown claims retain a known
// baseline.
func TestF008MergeModelsUsesExplicitPresence(t *testing.T) {
	existing := Model{
		ID:          "presence",
		Name:        "Presence",
		Description: "non-empty",
		Features: &ModelFeatures{
			ToolCalls: true,
		},
		Metadata: &ModelMetadata{OpenWeights: true},
		Limits: &ModelLimits{
			ContextWindow: 128000,
			InputTokens:   64000,
			OutputTokens:  8192,
		},
	}
	updated := Model{
		ID:   existing.ID,
		Name: existing.Name,
		Features: &ModelFeatures{
			Tools: false,
		},
		Metadata: &ModelMetadata{},
		Limits:   &ModelLimits{},
	}
	updated.SetDescription("")
	updated.Features.SetSupport(ModelFeatureToolCalls, false)
	updated.Features.SetSupportUnknown(ModelFeatureTools)
	updated.Metadata.SetOpenWeights(false)
	updated.Limits.Set(ModelLimitContextWindow, 0)
	updated.Limits.SetUnknown(ModelLimitInputTokens)

	got := MergeModels(existing, updated)
	if got.Features == nil || got.Features.ToolCalls {
		t.Fatalf("explicit false did not clear true: %#v", got.Features)
	}
	if got.Description != "" {
		t.Fatalf("explicit empty description = %q, want empty", got.Description)
	}
	if got.Limits == nil || got.Limits.ContextWindow != 0 {
		t.Fatalf("explicit zero did not clear context window: %#v", got.Limits)
	}
	if got.Limits.InputTokens != existing.Limits.InputTokens {
		t.Fatalf("unknown input limit cleared known baseline: got %d, want %d", got.Limits.InputTokens, existing.Limits.InputTokens)
	}
	if got.Limits.OutputTokens != existing.Limits.OutputTokens {
		t.Fatalf("missing output limit cleared known baseline: got %d, want %d", got.Limits.OutputTokens, existing.Limits.OutputTokens)
	}
	if got.Metadata == nil || got.Metadata.OpenWeights {
		t.Fatalf("explicit false did not clear open weights: %#v", got.Metadata)
	}
	if _, state := got.Features.Support(ModelFeatureToolCalls); state != ValueKnown {
		t.Fatalf("tool_calls presence = %v, want known", state)
	}
	if _, state := got.DescriptionValue(); state != ValueKnown {
		t.Fatalf("description presence = %v, want known", state)
	}
	if _, state := got.Limits.Value(ModelLimitContextWindow); state != ValueKnown {
		t.Fatalf("context window presence = %v, want known", state)
	}
}
