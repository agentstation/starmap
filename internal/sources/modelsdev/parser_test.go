package modelsdev

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/sourcepayload"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/goccy/go-yaml"
)

func TestSchemaDriftMutationMatrix(t *testing.T) {
	tests := []struct {
		name         string
		payload      string
		wantParseErr bool
		wantIssues   int
		wantModels   int
		wantUnknown  bool
	}{
		{name: "valid", payload: `{"provider":{"id":"provider","name":"Provider","models":{"model-a":{"id":"model-a","name":"Model A","description":"valid"}}}}`, wantModels: 1},
		{name: "missing", payload: `{"provider":{"id":"provider","name":"Provider"}}`, wantIssues: 1},
		{name: "renamed", payload: `{"provider":{"id":"provider","name":"Provider","items":[]}}`, wantIssues: 1},
		{name: "null", payload: `{"provider":{"id":"provider","name":"Provider","models":null}}`, wantIssues: 1},
		{name: "wrong type", payload: `{"provider":{"id":"provider","name":"Provider","models":[]}}`, wantParseErr: true},
		{name: "unknown additive", payload: `{"provider":{"id":"provider","name":"Provider","models":{"model-a":{"id":"model-a","name":"Model A","description":"valid","new_capability":true}},"new_page":1}}`, wantModels: 1, wantUnknown: true},
		{name: "oversized", payload: strings.Repeat(" ", constants.MaxSourcePayloadBytes+1), wantParseErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api, err := parseAPIData([]byte(test.payload))
			if test.wantParseErr {
				if err == nil {
					t.Fatal("parseAPIData returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAPIData: %v", err)
			}
			builder := catalogs.NewEmpty()
			added, _, issues, err := processFetch(builder, api)
			if err != nil {
				t.Fatalf("processFetch: %v", err)
			}
			if added != test.wantModels || len(issues) != test.wantIssues {
				t.Fatalf("added/issues = %d/%d, want %d/%d: %#v", added, len(issues), test.wantModels, test.wantIssues, issues)
			}
			if test.wantUnknown {
				provider, providerErr := builder.Provider("provider")
				if providerErr != nil {
					t.Fatalf("Provider: %v", providerErr)
				}
				if len(provider.Extensions[modelsDevExtensionSource].Fields["unknown_fields"].([]any)) != 1 {
					t.Fatalf("provider unknown evidence = %#v", provider.Extensions)
				}
				if len(provider.Models["model-a"].Extensions[modelsDevExtensionSource].Fields["unknown_fields"].([]any)) != 1 {
					t.Fatalf("model unknown evidence = %#v", provider.Models["model-a"].Extensions)
				}
			}
		})
	}
}

func TestPayloadLimitModelsDevModelCount(t *testing.T) {
	models := make(map[string]Model, constants.MaxCatalogModels+1)
	for index := 0; index <= constants.MaxCatalogModels; index++ {
		id := fmt.Sprintf("model-%05d", index)
		models[id] = Model{ID: id, Name: "Model", Description: "catalog data"}
	}
	api := API{"provider": {ID: "provider", Name: "Provider", Models: models}}
	added, rejected, issues, err := processFetch(catalogs.NewEmpty(), &api)
	if err != nil {
		t.Fatalf("processFetch: %v", err)
	}
	if added != constants.MaxCatalogModels {
		t.Fatalf("added = %d, want %d", added, constants.MaxCatalogModels)
	}
	if rejected != 1 {
		t.Fatalf("rejected = %d, want 1", rejected)
	}
	if len(issues) != 1 || issues[0].Code != sources.ObservationIssueCodePayloadLimit {
		t.Fatalf("issues = %#v, want payload limit", issues)
	}
}

func TestUnknownSourceEnumProducesFingerprintEvidence(t *testing.T) {
	model, err := (&Model{ID: "model", Name: "Model", Status: "new-lifecycle"}).ToStarmapModel()
	if err != nil {
		t.Fatalf("ToStarmapModel: %v", err)
	}
	if model.Status != catalogs.ModelStatusUnknown {
		t.Fatalf("status = %q, want unknown", model.Status)
	}
	items := model.Extensions[modelsDevExtensionSource].Fields["unknown_fields"].([]any)
	if len(items) != 1 {
		t.Fatalf("unknown enum evidence = %#v", items)
	}
	evidence, ok := items[0].(sourcepayload.UnknownJSONField)
	if !ok || evidence.Path != "status" || !strings.HasPrefix(evidence.Checksum, "sha256:") {
		t.Fatalf("unknown enum evidence = %#v", items[0])
	}
}

func TestExperimentalModeProviderBodyPreservesNestedJSON(t *testing.T) {
	const payload = `{
		"openai": {
			"id": "openai",
			"name": "OpenAI",
			"models": {
				"gpt-5.6": {
					"id": "gpt-5.6",
					"name": "GPT-5.6",
					"experimental": {
						"modes": {
							"pro": {
								"provider": {
									"body": {"reasoning": {"mode": "pro"}}
								}
							}
						}
					}
				}
			}
		}
	}`

	var api API
	if err := json.Unmarshal([]byte(payload), &api); err != nil {
		t.Fatalf("unmarshal current models.dev mode body: %v", err)
	}

	sourceModel := api["openai"].Models["gpt-5.6"]
	reasoning, ok := any(sourceModel.Experimental.Modes["pro"].Provider.Body["reasoning"]).(map[string]any)
	if !ok || reasoning["mode"] != "pro" {
		t.Fatalf("source nested body = %#v, want reasoning.mode=pro", sourceModel.Experimental.Modes["pro"].Provider.Body)
	}

	model, err := sourceModel.ToStarmapModel()
	if err != nil {
		t.Fatalf("convert current models.dev mode body: %v", err)
	}
	convertedReasoning, ok := model.Modes["pro"].Provider.Body["reasoning"].(map[string]any)
	if !ok || convertedReasoning["mode"] != "pro" {
		t.Fatalf("converted nested body = %#v, want reasoning.mode=pro", model.Modes["pro"].Provider.Body)
	}
}

func TestModelToStarmapModelPreservesCurrentModelsDevFields(t *testing.T) {
	zeroCost := 0.0
	outputCost := 2.0
	reasoningCost := 3.0
	cacheReadCost := 0.1
	cacheWriteCost := 0.2
	inputAudioCost := 0.003
	outputAudioCost := 0.004
	tierInputCost := 4.0
	tierOutputCost := 12.0
	tierCacheReadCost := 0.4
	tierCacheWriteCost := 1.2
	tierInputAudioCost := 0.006
	contextInputCost := 5.0
	contextOutputCost := 15.0
	contextCacheReadCost := 0.5
	modeInputCost := 6.0
	modeOutputCost := 18.0
	modeCacheReadCost := 0.6
	modeCacheWriteCost := 1.8
	knowledge := "2025-01"
	reasoningBudgetMin := 0
	reasoningBudgetMax := 8192

	model, err := (&Model{
		ID:          "gpt-5",
		Name:        "GPT-5",
		Description: "Original GPT-5 workhorse",
		Family:      "gpt-5",
		Status:      "beta",
		Attachment:  true,
		Reasoning:   true,
		ReasoningOptions: []ReasoningOption{
			{Type: "effort", Values: []string{"minimal", "low", "medium", "high", "max", "xhigh"}},
			{Type: "budget_tokens", Min: &reasoningBudgetMin, Max: &reasoningBudgetMax},
			{Type: "toggle", Values: []string{"none", "default"}},
		},
		StructuredOutput: true,
		Temperature:      true,
		ToolCall:         true,
		Knowledge:        &knowledge,
		Provider: &ModelProvider{
			NPM:   "@ai-sdk/openai-compatible",
			API:   "https://example.test/v1",
			Shape: "responses",
		},
		Interleaved: &Interleaved{
			Enabled: true,
			Field:   "reasoning_content",
		},
		ReleaseDate: "2025-08-07",
		LastUpdated: "2025-08-08",
		Modalities: Modalities{
			Input:  []string{"text", "image", "pdf", "audio", "embedding"},
			Output: []string{"text", "audio", "video", "pdf"},
		},
		OpenWeights: boolPtr(true),
		Limit:       Limit{Context: 400000, Input: 272000, Output: 128000},
		Experimental: &Experimental{
			Modes: map[string]ExperimentalMode{
				"fast": {
					Cost: &TierPrices{
						Input:      &modeInputCost,
						Output:     &modeOutputCost,
						CacheRead:  &modeCacheReadCost,
						CacheWrite: &modeCacheWriteCost,
					},
					Provider: &ExperimentalModeProvider{
						Body: map[string]any{
							"service_tier": "priority",
						},
						Headers: map[string]string{
							"anthropic-beta": "fast-mode-2026-02-01",
						},
					},
				},
			},
		},
		Cost: &Cost{
			Input:       &zeroCost,
			Output:      &outputCost,
			Reasoning:   &reasoningCost,
			CacheRead:   &cacheReadCost,
			CacheWrite:  &cacheWriteCost,
			InputAudio:  &inputAudioCost,
			OutputAudio: &outputAudioCost,
			Tiers: []CostTier{{
				TierPrices: TierPrices{
					Input:      &tierInputCost,
					Output:     &tierOutputCost,
					CacheRead:  &tierCacheReadCost,
					CacheWrite: &tierCacheWriteCost,
					InputAudio: &tierInputAudioCost,
				},
				Tier: CostTierInfo{Type: "context", Size: 272000},
			}},
			ContextOver200K: &TierPrices{
				Input:     &contextInputCost,
				Output:    &contextOutputCost,
				CacheRead: &contextCacheReadCost,
			},
		},
	}).ToStarmapModel()
	if err != nil {
		t.Fatalf("ToStarmapModel returned error: %v", err)
	}

	if model.Description != "Original GPT-5 workhorse" {
		t.Fatalf("Description = %q, want models.dev description", model.Description)
	}
	if model.Status != catalogs.ModelStatusBeta {
		t.Fatalf("Status = %q, want %q", model.Status, catalogs.ModelStatusBeta)
	}
	if model.Lineage == nil || model.Lineage.Family != "gpt-5" {
		t.Fatalf("Lineage = %#v, want family gpt-5", model.Lineage)
	}
	if model.Metadata == nil || !model.Metadata.OpenWeights {
		t.Fatal("Metadata.OpenWeights was not preserved")
	}
	if model.Features == nil {
		t.Fatal("Features is nil")
	}
	for _, modality := range []catalogs.ModelModality{
		catalogs.ModelModalityText,
		catalogs.ModelModalityImage,
		catalogs.ModelModalityPDF,
		catalogs.ModelModalityAudio,
		catalogs.ModelModalityEmbedding,
	} {
		if !containsModality(model.Features.Modalities.Input, modality) {
			t.Fatalf("input modalities missing %q: %#v", modality, model.Features.Modalities.Input)
		}
	}
	if !model.Features.ToolCalls || !model.Features.Tools || !model.Features.ToolChoice {
		t.Fatal("tool-call feature flags were not preserved")
	}
	if !model.Features.Reasoning || !model.Features.ReasoningEffort {
		t.Fatal("reasoning feature flags were not preserved")
	}
	if !model.Features.ReasoningTokens {
		t.Fatal("reasoning token feature flag was not preserved")
	}
	if !model.Features.StructuredOutputs {
		t.Fatal("structured output feature flag was not preserved")
	}
	if model.Reasoning == nil {
		t.Fatal("Reasoning control levels is nil")
	}
	wantLevels := []catalogs.ModelControlLevel{
		catalogs.ModelControlLevelMinimum,
		catalogs.ModelControlLevelLow,
		catalogs.ModelControlLevelMedium,
		catalogs.ModelControlLevelHigh,
		catalogs.ModelControlLevelMaximum,
	}
	if len(model.Reasoning.Levels) != len(wantLevels) {
		t.Fatalf("reasoning levels = %#v, want %#v", model.Reasoning.Levels, wantLevels)
	}
	for i, want := range wantLevels {
		if model.Reasoning.Levels[i] != want {
			t.Fatalf("reasoning level %d = %q, want %q", i, model.Reasoning.Levels[i], want)
		}
	}
	if model.ReasoningTokens == nil ||
		model.ReasoningTokens.Min != reasoningBudgetMin ||
		model.ReasoningTokens.Max != reasoningBudgetMax {
		t.Fatalf("reasoning token range = %#v", model.ReasoningTokens)
	}
	if model.Pricing == nil || model.Pricing.Tokens == nil {
		t.Fatal("Pricing.Tokens is nil")
	}
	if model.Pricing.Tokens.Input == nil || model.Pricing.Tokens.Input.Per1M != 0 {
		t.Fatalf("input pricing = %#v, want explicit zero price", model.Pricing.Tokens.Input)
	}
	if model.Pricing.Tokens.Output == nil || model.Pricing.Tokens.Output.Per1M != outputCost {
		t.Fatalf("output pricing = %#v, want %v", model.Pricing.Tokens.Output, outputCost)
	}
	if model.Pricing.Tokens.Reasoning == nil || model.Pricing.Tokens.Reasoning.Per1M != reasoningCost {
		t.Fatalf("reasoning pricing = %#v, want %v", model.Pricing.Tokens.Reasoning, reasoningCost)
	}
	if model.Pricing.Tokens.Cache == nil ||
		model.Pricing.Tokens.Cache.Read == nil ||
		model.Pricing.Tokens.Cache.Read.Per1M != cacheReadCost ||
		model.Pricing.Tokens.Cache.Write == nil ||
		model.Pricing.Tokens.Cache.Write.Per1M != cacheWriteCost {
		t.Fatalf("cache pricing = %#v", model.Pricing.Tokens.Cache)
	}
	if model.Pricing.Operations == nil ||
		model.Pricing.Operations.AudioInput == nil ||
		*model.Pricing.Operations.AudioInput != inputAudioCost ||
		model.Pricing.Operations.AudioGen == nil ||
		*model.Pricing.Operations.AudioGen != outputAudioCost {
		t.Fatalf("audio operation pricing = %#v", model.Pricing.Operations)
	}
	if len(model.Pricing.Tiers) != 2 {
		t.Fatalf("pricing tiers = %#v, want 2 tiers", model.Pricing.Tiers)
	}
	if model.Pricing.Tiers[0].Type != catalogs.ModelPricingTierTypeContext ||
		model.Pricing.Tiers[0].Size != 272000 ||
		model.Pricing.Tiers[0].Tokens == nil ||
		model.Pricing.Tiers[0].Tokens.Input == nil ||
		model.Pricing.Tiers[0].Tokens.Input.Per1M != tierInputCost ||
		model.Pricing.Tiers[0].Operations == nil ||
		model.Pricing.Tiers[0].Operations.AudioInput == nil ||
		*model.Pricing.Tiers[0].Operations.AudioInput != tierInputAudioCost {
		t.Fatalf("first pricing tier = %#v", model.Pricing.Tiers[0])
	}
	if model.Pricing.Tiers[1].Name != "context_over_200k" ||
		model.Pricing.Tiers[1].Type != catalogs.ModelPricingTierTypeContext ||
		model.Pricing.Tiers[1].Size != 200000 ||
		model.Pricing.Tiers[1].Tokens == nil ||
		model.Pricing.Tiers[1].Tokens.Input == nil ||
		model.Pricing.Tiers[1].Tokens.Input.Per1M != contextInputCost {
		t.Fatalf("context_over_200k pricing tier = %#v", model.Pricing.Tiers[1])
	}
	fastMode, ok := model.Modes["fast"]
	if !ok {
		t.Fatalf("fast mode missing from %#v", model.Modes)
	}
	if fastMode.Pricing == nil ||
		fastMode.Pricing.Tokens == nil ||
		fastMode.Pricing.Tokens.Input == nil ||
		fastMode.Pricing.Tokens.Input.Per1M != modeInputCost ||
		fastMode.Pricing.Tokens.Cache == nil ||
		fastMode.Pricing.Tokens.Cache.Write == nil ||
		fastMode.Pricing.Tokens.Cache.Write.Per1M != modeCacheWriteCost {
		t.Fatalf("fast mode pricing = %#v", fastMode.Pricing)
	}
	if fastMode.Provider == nil ||
		fastMode.Provider.Body["service_tier"] != "priority" ||
		fastMode.Provider.Headers["anthropic-beta"] != "fast-mode-2026-02-01" {
		t.Fatalf("fast mode provider overrides = %#v", fastMode.Provider)
	}
	if model.Limits == nil ||
		model.Limits.ContextWindow != 400000 ||
		model.Limits.InputTokens != 272000 ||
		model.Limits.OutputTokens != 128000 {
		t.Fatalf("limits = %#v", model.Limits)
	}
	modelsDevExtension, ok := model.Extensions["models.dev"]
	if !ok {
		t.Fatalf("models.dev extension missing from %#v", model.Extensions)
	}
	providerExtension := modelsDevExtension.Fields["provider"].(map[string]any)
	if providerExtension["npm"] != "@ai-sdk/openai-compatible" ||
		providerExtension["api"] != "https://example.test/v1" ||
		providerExtension["shape"] != "responses" {
		t.Fatalf("provider extension = %#v", providerExtension)
	}
	interleavedExtension := modelsDevExtension.Fields["interleaved"].(map[string]any)
	if interleavedExtension["enabled"] != true || interleavedExtension["field"] != "reasoning_content" {
		t.Fatalf("interleaved extension = %#v", interleavedExtension)
	}
	reasoningExtensions := modelsDevExtension.Fields["reasoning_options"].([]any)
	if len(reasoningExtensions) != 2 {
		t.Fatalf("reasoning option extensions = %#v, want effort and toggle options", reasoningExtensions)
	}
	effortOption := reasoningExtensions[0].(map[string]any)
	if effortOption["type"] != "effort" {
		t.Fatalf("reasoning effort extension = %#v", effortOption)
	}
	effortValues := effortOption["values"].([]any)
	if len(effortValues) != 1 || effortValues[0] != "xhigh" {
		t.Fatalf("reasoning effort extension values = %#v", effortValues)
	}
	toggleOption := reasoningExtensions[1].(map[string]any)
	if toggleOption["type"] != "toggle" {
		t.Fatalf("reasoning option extension = %#v", toggleOption)
	}
}

func TestProviderToStarmapProviderPreservesModelsDevMetadata(t *testing.T) {
	apiURL := "https://example.test/v1"

	provider, err := (&Provider{
		ID:   "example",
		Name: "Example",
		Env:  []string{"EXAMPLE_API_KEY"},
		NPM:  "@ai-sdk/example",
		API:  &apiURL,
		Doc:  "https://example.test/docs",
	}).ToStarmapProvider()
	if err != nil {
		t.Fatalf("ToStarmapProvider returned error: %v", err)
	}

	if provider.Catalog == nil ||
		provider.Catalog.Docs == nil ||
		*provider.Catalog.Docs != "https://example.test/docs" ||
		provider.Catalog.Endpoint.URL != "" ||
		provider.Catalog.Endpoint.Type != "" {
		t.Fatalf("provider catalog = %#v", provider.Catalog)
	}
	if len(provider.EnvVars) != 1 ||
		provider.EnvVars[0].Name != "EXAMPLE_API_KEY" ||
		provider.EnvVars[0].Required {
		t.Fatalf("provider env vars = %#v", provider.EnvVars)
	}
	if provider.Extensions["models.dev"].Fields["npm"] != "@ai-sdk/example" {
		t.Fatalf("provider extensions = %#v", provider.Extensions)
	}
	if provider.Extensions["models.dev"].Fields["api"] != apiURL {
		t.Fatalf("provider api extension = %#v", provider.Extensions)
	}
}

func TestInterleavedUnmarshalAcceptsBooleanAndObject(t *testing.T) {
	var enabled Interleaved
	if err := enabled.UnmarshalJSON([]byte(`true`)); err != nil {
		t.Fatalf("boolean interleaved unmarshal failed: %v", err)
	}
	if !enabled.Enabled || enabled.Field != "" {
		t.Fatalf("boolean interleaved = %#v", enabled)
	}

	var withField Interleaved
	if err := withField.UnmarshalJSON([]byte(`{"field":"reasoning_content"}`)); err != nil {
		t.Fatalf("object interleaved unmarshal failed: %v", err)
	}
	if !withField.Enabled || withField.Field != "reasoning_content" {
		t.Fatalf("object interleaved = %#v", withField)
	}
}

func TestProcessFetchIncludesModelsWithNonCoreCostData(t *testing.T) {
	reasoningCost := 1.5
	apiURL := "https://example.test/v1"
	api := API{
		"provider": Provider{
			ID:   "provider",
			Name: "Provider",
			Env:  []string{"PROVIDER_API_KEY"},
			NPM:  "@ai-sdk/provider",
			API:  &apiURL,
			Doc:  "https://example.test/docs",
			Models: map[string]Model{
				"reasoning-only": {
					ID:   "reasoning-only",
					Name: "Reasoning Only",
					Cost: &Cost{Reasoning: &reasoningCost},
				},
				"description-only": {
					ID:          "description-only",
					Name:        "Description Only",
					Description: "Useful enrichment without pricing.",
				},
			},
		},
	}
	catalog := catalogs.NewEmpty()

	added, _, issues, err := processFetch(catalog, &api)
	if err != nil {
		t.Fatalf("processFetch returned error: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("processFetch issues: %#v", issues)
	}
	if added != 2 {
		t.Fatalf("added = %d, want 2", added)
	}

	provider, err := catalog.Provider("provider")
	if err != nil {
		t.Fatalf("provider not found: %v", err)
	}
	if provider.Name != "Provider" {
		t.Fatalf("provider name = %q, want models.dev provider name", provider.Name)
	}
	if provider.Catalog == nil ||
		provider.Catalog.Docs == nil ||
		*provider.Catalog.Docs != "https://example.test/docs" ||
		provider.Catalog.Endpoint.URL != "" ||
		provider.Catalog.Endpoint.Type != "" {
		t.Fatalf("provider catalog = %#v", provider.Catalog)
	}
	if len(provider.EnvVars) != 1 ||
		provider.EnvVars[0].Name != "PROVIDER_API_KEY" ||
		provider.EnvVars[0].Required {
		t.Fatalf("provider env vars = %#v", provider.EnvVars)
	}
	if provider.Extensions["models.dev"].Fields["npm"] != "@ai-sdk/provider" {
		t.Fatalf("provider extensions = %#v", provider.Extensions)
	}
	if provider.Extensions["models.dev"].Fields["api"] != apiURL {
		t.Fatalf("provider api extension = %#v", provider.Extensions)
	}
	model := provider.Models["reasoning-only"]
	if model == nil {
		t.Fatal("reasoning-only model was not added")
	}
	if model.Pricing == nil || model.Pricing.Tokens == nil ||
		model.Pricing.Tokens.Reasoning == nil ||
		model.Pricing.Tokens.Reasoning.Per1M != reasoningCost {
		t.Fatalf("reasoning pricing = %#v", model.Pricing)
	}
	model = provider.Models["description-only"]
	if model == nil {
		t.Fatal("description-only model was not added")
	}
	if model.Description != "Useful enrichment without pricing." {
		t.Fatalf("description = %q", model.Description)
	}
}

func TestProcessFetchHonorsProviderFilter(t *testing.T) {
	api := API{
		"selected": Provider{
			ID:   "selected",
			Name: "Selected",
			Models: map[string]Model{
				"selected-model": {
					ID:          "selected-model",
					Name:        "Selected Model",
					Description: "selected provider model",
				},
			},
		},
		"skipped": Provider{
			ID:   "skipped",
			Name: "Skipped",
			Models: map[string]Model{
				"skipped-model": {
					ID:          "skipped-model",
					Name:        "Skipped Model",
					Description: "skipped provider model",
				},
			},
		},
	}
	catalog := catalogs.NewEmpty()

	added, _, issues, err := processFetch(catalog, &api, sources.WithProviderFilter("selected"))
	if err != nil {
		t.Fatalf("processFetch returned error: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("processFetch issues: %#v", issues)
	}
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if _, err := catalog.Provider("selected"); err != nil {
		t.Fatalf("selected provider not found: %v", err)
	}
	if _, err := catalog.Provider("skipped"); err == nil {
		t.Fatal("skipped provider was included despite provider filter")
	}
}

func TestModelToStarmapModelDoesNotInventOpenWeightsWhenAbsent(t *testing.T) {
	model, err := (&Model{
		ID:   "metadata-free",
		Name: "Metadata Free",
	}).ToStarmapModel()
	if err != nil {
		t.Fatalf("ToStarmapModel returned error: %v", err)
	}
	if model.Metadata != nil {
		t.Fatalf("Metadata = %#v, want nil when models.dev omits metadata fields", model.Metadata)
	}
}

func TestModelToStarmapModelExtensionsRoundTripWithoutTypeChurn(t *testing.T) {
	minTokens := 1
	maxTokens := 3
	model, err := (&Model{
		ID:   "extension-model",
		Name: "Extension Model",
		ReasoningOptions: []ReasoningOption{
			{Type: "effort", Values: []string{"low", "xhigh"}},
			{Type: "custom_budget", Values: []string{"auto"}, Min: &minTokens, Max: &maxTokens},
		},
	}).ToStarmapModel()
	if err != nil {
		t.Fatalf("ToStarmapModel returned error: %v", err)
	}

	data, err := yaml.Marshal(model)
	if err != nil {
		t.Fatalf("marshal model: %v", err)
	}
	var roundTrip catalogs.Model
	if err := yaml.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal model: %v", err)
	}
	if !reflect.DeepEqual(model.Extensions, roundTrip.Extensions) {
		t.Fatalf("extensions changed after YAML round trip:\n got %#v\nwant %#v", roundTrip.Extensions, model.Extensions)
	}
}

func TestModelHasCatalogDataTreatsExplicitOpenWeightsFalseAsData(t *testing.T) {
	if !((Model{OpenWeights: boolPtr(false)}).hasCatalogData()) {
		t.Fatal("explicit open_weights=false should still count as source data")
	}
}

func containsModality(modalities []catalogs.ModelModality, want catalogs.ModelModality) bool {
	return slices.Contains(modalities, want)
}

func boolPtr(v bool) *bool {
	return &v
}
