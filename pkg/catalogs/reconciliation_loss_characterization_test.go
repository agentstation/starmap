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

// TestF008CharacterizationMergeModelsClearsFalseButKeepsOtherZeroValues pins
// the current presence asymmetry. P4.6 must replace this reflection policy
// with explicit presence so false, zero, empty, missing, and unknown remain
// distinguishable.
func TestF008CharacterizationMergeModelsClearsFalseButKeepsOtherZeroValues(t *testing.T) {
	existing := Model{
		ID:          "presence",
		Name:        "Presence",
		Description: "non-empty",
		Features: &ModelFeatures{
			ToolCalls: true,
		},
		Limits: &ModelLimits{
			ContextWindow: 128000,
		},
	}
	updated := Model{
		ID:          existing.ID,
		Name:        existing.Name,
		Description: "",
		Features: &ModelFeatures{
			ToolCalls: false,
		},
		Limits: &ModelLimits{
			ContextWindow: 0,
		},
	}

	got := MergeModels(existing, updated)
	if got.Features == nil || got.Features.ToolCalls {
		t.Fatalf("F-008 characterization changed: false did not clear true: %#v", got.Features)
	}
	if got.Description != existing.Description {
		t.Fatalf("empty string cleared description: got %q, want %q", got.Description, existing.Description)
	}
	if got.Limits == nil || got.Limits.ContextWindow != existing.Limits.ContextWindow {
		t.Fatalf("zero cleared context window: got %#v, want %d", got.Limits, existing.Limits.ContextWindow)
	}
}
