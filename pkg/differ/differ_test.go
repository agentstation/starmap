package differ

import (
	"testing"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestModelsDetectFirstClassModelSubstructureChanges(t *testing.T) {
	low := catalogs.ModelControlLevelLow
	diff := New()

	changes := diff.Models(
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
		}},
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Reasoning: &catalogs.ModelControlLevels{
				Levels:  []catalogs.ModelControlLevel{catalogs.ModelControlLevelLow},
				Default: &low,
			},
			Tools: &catalogs.ModelTools{
				ToolChoices: []catalogs.ToolChoice{catalogs.ToolChoiceAuto},
			},
			Lineage: &catalogs.ModelLineage{
				Family: "model-family",
			},
			Delivery: &catalogs.ModelDelivery{
				Formats: []catalogs.ModelResponseFormat{catalogs.ModelResponseFormatJSONSchema},
			},
		}},
	)

	if len(changes.Updated) != 1 {
		t.Fatalf("updated models = %d, want 1", len(changes.Updated))
	}
	paths := make(map[string]bool)
	for _, change := range changes.Updated[0].Changes {
		paths[change.Path] = true
	}
	for _, want := range []string{"reasoning", "tools", "lineage", "response"} {
		if !paths[want] {
			t.Fatalf("missing change path %q in %#v", want, changes.Updated[0].Changes)
		}
	}
}

func TestCatalogsDiffModelsWithinProviderScope(t *testing.T) {
	existing := catalogs.NewEmpty()
	updated := catalogs.NewEmpty()

	setProviderForDiffTest(t, existing, "provider-a", &catalogs.Model{ID: "shared", Name: "A"})
	setProviderForDiffTest(t, existing, "provider-b", &catalogs.Model{ID: "shared", Name: "B"})
	setProviderForDiffTest(t, updated, "provider-a", &catalogs.Model{ID: "shared", Name: "A"})
	setProviderForDiffTest(t, updated, "provider-b", &catalogs.Model{ID: "shared", Name: "B updated"})

	changes := New().Catalogs(existing, updated)
	if len(changes.Models.Updated) != 1 {
		t.Fatalf("updated models = %d, want 1: %#v", len(changes.Models.Updated), changes.Models.Updated)
	}
	if changes.Models.Updated[0].Existing.Name != "B" || changes.Models.Updated[0].New.Name != "B updated" {
		t.Fatalf("provider-scoped update compared wrong models: %#v", changes.Models.Updated[0])
	}
	if changes.Models.Updated[0].ProviderID != "provider-b" {
		t.Fatalf("provider-scoped update ProviderID = %q, want provider-b", changes.Models.Updated[0].ProviderID)
	}
}

func TestCatalogsDiffScopesAddedAndRemovedModelsByProvider(t *testing.T) {
	existing := catalogs.NewEmpty()
	updated := catalogs.NewEmpty()

	setProviderForDiffTest(t, existing, "provider-a", &catalogs.Model{ID: "shared", Name: "A"})
	setProviderForDiffTest(t, existing, "provider-b", &catalogs.Model{ID: "shared", Name: "B"})
	setProviderForDiffTest(t, updated, "provider-a", &catalogs.Model{ID: "shared", Name: "A"})
	setProviderForDiffTest(t, updated, "provider-c", &catalogs.Model{ID: "shared", Name: "C"})

	changes := New().Catalogs(existing, updated)
	if len(changes.Models.AddedScoped) != 1 {
		t.Fatalf("scoped added models = %d, want 1: %#v", len(changes.Models.AddedScoped), changes.Models.AddedScoped)
	}
	if changes.Models.AddedScoped[0].ProviderID != "provider-c" ||
		changes.Models.AddedScoped[0].Model.ID != "shared" {
		t.Fatalf("scoped added model = %#v", changes.Models.AddedScoped[0])
	}
	if len(changes.Models.RemovedScoped) != 1 {
		t.Fatalf("scoped removed models = %d, want 1: %#v", len(changes.Models.RemovedScoped), changes.Models.RemovedScoped)
	}
	if changes.Models.RemovedScoped[0].ProviderID != "provider-b" ||
		changes.Models.RemovedScoped[0].Model.ID != "shared" {
		t.Fatalf("scoped removed model = %#v", changes.Models.RemovedScoped[0])
	}
}

func TestProvidersDetectCanonicalProviderFieldChanges(t *testing.T) {
	hq := "San Francisco, CA, USA"
	iconURL := "https://example.com/icon.svg"
	statusURL := "https://status.example.com"
	chatURL := "https://api.example.com/v1/chat/completions"
	privacyURL := "https://example.com/privacy"
	retainsData := false
	retentionDetails := "No retention"
	moderated := true

	changes := New().Providers(
		[]catalogs.Provider{{
			ID:   "provider-a",
			Name: "Provider A",
		}},
		[]catalogs.Provider{{
			ID:           "provider-a",
			Aliases:      []catalogs.ProviderID{"provider-alias"},
			Name:         "Provider A",
			Headquarters: &hq,
			IconURL:      &iconURL,
			EnvVars: []catalogs.ProviderEnvVar{{
				Name:     "PROVIDER_API_KEY",
				Required: true,
			}},
			StatusPageURL: &statusURL,
			ChatCompletions: &catalogs.ProviderChatCompletions{
				URL: &chatURL,
			},
			PrivacyPolicy: &catalogs.ProviderPrivacyPolicy{
				PrivacyPolicyURL: &privacyURL,
				RetainsData:      &retainsData,
			},
			RetentionPolicy: &catalogs.ProviderRetentionPolicy{
				Type:    catalogs.ProviderRetentionTypeNone,
				Details: &retentionDetails,
			},
			GovernancePolicy: &catalogs.ProviderGovernancePolicy{
				Moderated: &moderated,
			},
		}},
	)

	if len(changes.Updated) != 1 {
		t.Fatalf("updated providers = %d, want 1", len(changes.Updated))
	}
	paths := modelChangePaths(changes.Updated[0].Changes)
	for _, want := range []string{
		"aliases",
		"headquarters",
		"icon_url",
		"env_vars",
		"status_page_url",
		"chat_completions",
		"privacy_policy",
		"retention_policy",
		"governance_policy",
	} {
		if !paths[want] {
			t.Fatalf("missing change path %q in %#v", want, changes.Updated[0].Changes)
		}
	}
}

func TestModelsDetectInputTokenLimitChanges(t *testing.T) {
	diff := New()

	changes := diff.Models(
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Limits: &catalogs.ModelLimits{
				ContextWindow: 128000,
				InputTokens:   64000,
				OutputTokens:  4096,
			},
		}},
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Limits: &catalogs.ModelLimits{
				ContextWindow: 128000,
				InputTokens:   96000,
				OutputTokens:  4096,
			},
		}},
	)

	if len(changes.Updated) != 1 {
		t.Fatalf("updated models = %d, want 1", len(changes.Updated))
	}
	if len(changes.Updated[0].Changes) != 1 {
		t.Fatalf("changes = %#v, want one input token limit change", changes.Updated[0].Changes)
	}
	if changes.Updated[0].Changes[0].Path != "limits.input_tokens" {
		t.Fatalf("change path = %q, want limits.input_tokens", changes.Updated[0].Changes[0].Path)
	}
}

func TestModelsDetectStatusChanges(t *testing.T) {
	diff := New()

	changes := diff.Models(
		[]*catalogs.Model{{
			ID:     "model",
			Name:   "Model",
			Status: catalogs.ModelStatusBeta,
		}},
		[]*catalogs.Model{{
			ID:     "model",
			Name:   "Model",
			Status: catalogs.ModelStatusDeprecated,
		}},
	)

	if len(changes.Updated) != 1 {
		t.Fatalf("updated models = %d, want 1", len(changes.Updated))
	}
	if len(changes.Updated[0].Changes) != 1 {
		t.Fatalf("changes = %#v, want one status change", changes.Updated[0].Changes)
	}
	if changes.Updated[0].Changes[0].Path != "status" {
		t.Fatalf("change path = %q, want status", changes.Updated[0].Changes[0].Path)
	}
}

func TestModelsDetectPricingTierChanges(t *testing.T) {
	diff := New()

	changes := diff.Models(
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Pricing: &catalogs.ModelPricing{
				Tiers: []catalogs.ModelPricingTier{{
					Type: catalogs.ModelPricingTierTypeContext,
					Size: 200000,
				}},
			},
		}},
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Pricing: &catalogs.ModelPricing{
				Tiers: []catalogs.ModelPricingTier{{
					Type: catalogs.ModelPricingTierTypeContext,
					Size: 272000,
				}},
			},
		}},
	)

	if len(changes.Updated) != 1 {
		t.Fatalf("updated models = %d, want 1", len(changes.Updated))
	}
	if len(changes.Updated[0].Changes) != 1 {
		t.Fatalf("changes = %#v, want one pricing tiers change", changes.Updated[0].Changes)
	}
	if changes.Updated[0].Changes[0].Path != "pricing.tiers" {
		t.Fatalf("change path = %q, want pricing.tiers", changes.Updated[0].Changes[0].Path)
	}
}

func TestModelsDetectPricingTokenAndOperationChanges(t *testing.T) {
	diff := New()
	requestCost := 0.01

	changes := diff.Models(
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Pricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyUSD,
				Tokens: &catalogs.ModelTokenPricing{
					Input: &catalogs.ModelTokenCost{Per1M: 1},
				},
			},
		}},
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Pricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyUSD,
				Tokens: &catalogs.ModelTokenPricing{
					Input:     &catalogs.ModelTokenCost{Per1M: 1},
					CacheRead: &catalogs.ModelTokenCost{Per1M: 0.25},
				},
				Operations: &catalogs.ModelOperationPricing{
					Request: &requestCost,
				},
			},
		}},
	)

	if len(changes.Updated) != 1 {
		t.Fatalf("updated models = %d, want 1", len(changes.Updated))
	}
	paths := modelChangePaths(changes.Updated[0].Changes)
	for _, want := range []string{"pricing.tokens", "pricing.operations"} {
		if !paths[want] {
			t.Fatalf("missing change path %q in %#v", want, changes.Updated[0].Changes)
		}
	}
}

func TestModelsDetectMetadataTagsArchitectureAndKnowledgeCutoffChanges(t *testing.T) {
	diff := New()
	knowledge := utc.Now()

	changes := diff.Models(
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Metadata: &catalogs.ModelMetadata{
				OpenWeights: false,
			},
		}},
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Metadata: &catalogs.ModelMetadata{
				OpenWeights:     false,
				KnowledgeCutoff: &knowledge,
				Tags:            []catalogs.ModelTag{catalogs.ModelTagCoding},
				Architecture: &catalogs.ModelArchitecture{
					Tokenizer: catalogs.TokenizerGPT,
				},
			},
		}},
	)

	if len(changes.Updated) != 1 {
		t.Fatalf("updated models = %d, want 1", len(changes.Updated))
	}
	paths := modelChangePaths(changes.Updated[0].Changes)
	for _, want := range []string{"metadata.knowledge_cutoff", "metadata.tags", "metadata.architecture"} {
		if !paths[want] {
			t.Fatalf("missing change path %q in %#v", want, changes.Updated[0].Changes)
		}
	}
}

func TestModelsDetectModeChanges(t *testing.T) {
	diff := New()

	changes := diff.Models(
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Modes: map[string]catalogs.ModelMode{
				"fast": {
					Provider: &catalogs.ModelProviderMode{
						Body: map[string]any{"service_tier": "standard"},
					},
				},
			},
		}},
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Modes: map[string]catalogs.ModelMode{
				"fast": {
					Provider: &catalogs.ModelProviderMode{
						Body: map[string]any{"service_tier": "priority"},
					},
				},
			},
		}},
	)

	if len(changes.Updated) != 1 {
		t.Fatalf("updated models = %d, want 1", len(changes.Updated))
	}
	if len(changes.Updated[0].Changes) != 1 {
		t.Fatalf("changes = %#v, want one mode change", changes.Updated[0].Changes)
	}
	if changes.Updated[0].Changes[0].Path != "modes" {
		t.Fatalf("change path = %q, want modes", changes.Updated[0].Changes[0].Path)
	}
}

func TestModelsDetectExtensionChanges(t *testing.T) {
	diff := New()

	changes := diff.Models(
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Extensions: catalogs.SourceExtensions{
				"provider": {Fields: map[string]any{"shape": "chat"}},
			},
		}},
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Extensions: catalogs.SourceExtensions{
				"provider": {Fields: map[string]any{"shape": "responses"}},
			},
		}},
	)

	if len(changes.Updated) != 1 {
		t.Fatalf("updated models = %d, want 1", len(changes.Updated))
	}
	if len(changes.Updated[0].Changes) != 1 {
		t.Fatalf("changes = %#v, want one extension change", changes.Updated[0].Changes)
	}
	if changes.Updated[0].Changes[0].Path != "extensions" {
		t.Fatalf("change path = %q, want extensions", changes.Updated[0].Changes[0].Path)
	}
}

func TestModelsIgnoreExtensionDynamicTypeOnlyChanges(t *testing.T) {
	diff := New()

	changes := diff.Models(
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Extensions: catalogs.SourceExtensions{
				"models.dev": {Fields: map[string]any{
					"values": []any{"xhigh"},
					"limit": map[string]any{
						"min": int64(1),
					},
				}},
			},
		}},
		[]*catalogs.Model{{
			ID:   "model",
			Name: "Model",
			Extensions: catalogs.SourceExtensions{
				"models.dev": {Fields: map[string]any{
					"values": []string{"xhigh"},
					"limit": map[string]int{
						"min": 1,
					},
				}},
			},
		}},
	)

	if len(changes.Updated) != 0 {
		t.Fatalf("updated models = %#v, want no dynamic-type-only extension changes", changes.Updated)
	}
}

func modelChangePaths(changes []FieldChange) map[string]bool {
	paths := make(map[string]bool, len(changes))
	for _, change := range changes {
		paths[change.Path] = true
	}
	return paths
}

func setProviderForDiffTest(t *testing.T, catalog *catalogs.Builder, providerID catalogs.ProviderID, model *catalogs.Model) {
	t.Helper()
	if err := catalog.SetProvider(catalogs.Provider{
		ID: providerID,
		Models: map[string]*catalogs.Model{
			model.ID: model,
		},
	}); err != nil {
		t.Fatalf("SetProvider(%s) failed: %v", providerID, err)
	}
}
