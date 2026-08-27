package reconciler

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/sources"
)

// TestMergeModelsDropsReasoningControlsWhenProviderDeniesReasoning proves the
// merger never emits a model that the catalog will refuse.
//
// The two policies disagree about an absent value. Features merges under
// EmptyAuthoritative, so a present provider capability record wins even where
// it denies a capability. Reasoning merges under EmptyAbsent, so models.dev
// fills the gap the provider left. Without a guard the merge produces effort
// levels on a model that declares no reasoning support, and
// catalogs.Validate rejects that pair. The whole catalog generation then
// aborts on one model.
func TestMergeModelsDropsReasoningControlsWhenProviderDeniesReasoning(t *testing.T) {
	authorities := authority.New()
	merger := newMerger(authorities, NewAuthorityStrategy(authorities), nil)

	// The provider record states that the model does not reason. SetSupport
	// records a known false, which is what separates this case from a field
	// the provider simply left unset.
	providerFeatures := &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
	}
	providerFeatures.SetSupport(catalogs.ModelFeatureReasoning, false)

	merged, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {{
			ID:       "model-1",
			Name:     "Provider Model",
			Features: providerFeatures,
		}},
		sources.ModelsDevHTTPID: {{
			ID: "model-1",
			Features: &catalogs.ModelFeatures{
				Reasoning:       true,
				ReasoningEffort: true,
			},
			Reasoning: &catalogs.ModelControlLevels{
				Levels: []catalogs.ModelControlLevel{
					catalogs.ModelControlLevelLow,
					catalogs.ModelControlLevelHigh,
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(merged) != 1 {
		t.Fatalf("merged %d models, want 1", len(merged))
	}
	model := merged[0]

	if model.Features == nil {
		t.Fatal("merged features are nil")
	}
	if model.Features.Reasoning {
		t.Fatalf("reasoning support = true, want the provider record's false; features = %#v", model.Features)
	}
	if model.Reasoning != nil {
		t.Errorf("reasoning levels = %#v, want nil beside a denied capability", model.Reasoning)
	}
	if model.ReasoningTokens != nil {
		t.Errorf("reasoning token range = %#v, want nil beside a denied capability", model.ReasoningTokens)
	}
	// These four assertions restate the rule that catalog indexing enforces in
	// validateReasoningFacts: a reasoning control or a subordinate reasoning
	// capability requires reasoning support. Indexing reaches that rule
	// through an unexported path, so the merged shape is asserted directly.
	if model.Features.ReasoningEffort || model.Features.ReasoningTokens || model.Features.IncludeReasoning {
		t.Errorf("subordinate reasoning capabilities survived: %#v", model.Features)
	}
}

// TestMergeModelsKeepsReasoningControlsWhenSupportSurvives holds the other
// edge. The guard must not strip controls from a model that does reason.
func TestMergeModelsKeepsReasoningControlsWhenSupportSurvives(t *testing.T) {
	authorities := authority.New()
	merger := newMerger(authorities, NewAuthorityStrategy(authorities), nil)

	merged, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {{
			ID:   "model-1",
			Name: "Provider Model",
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
				Reasoning: true,
			},
		}},
		sources.ModelsDevHTTPID: {{
			ID:        "model-1",
			Features:  &catalogs.ModelFeatures{Reasoning: true, ReasoningEffort: true},
			Reasoning: &catalogs.ModelControlLevels{Levels: []catalogs.ModelControlLevel{catalogs.ModelControlLevelHigh}},
		}},
	})
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	model := merged[0]
	if model.Reasoning == nil || len(model.Reasoning.Levels) != 1 {
		t.Fatalf("reasoning levels = %#v, want the one models.dev documents", model.Reasoning)
	}
	if !model.Features.ReasoningEffort {
		t.Errorf("reasoning effort = false, want the capability models.dev documents")
	}
}
