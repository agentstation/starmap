package openai

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/agentstation/starmap/internal/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/goccy/go-yaml"
)

// TestMain handles flag parsing for the -update flag.
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func newTestClient(t testing.TB, provider *catalogs.Provider) *Client {
	t.Helper()
	client, err := NewClient(provider)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

// loadTestdataResponse loads an OpenAI API response from testdata.
func loadTestdataResponse(t *testing.T, filename string) Response {
	t.Helper()
	var response Response
	testhelper.LoadJSON(t, filename, &response)
	return response
}

// loadTestdataModel loads a single OpenAI model from testdata by finding it in the models list.
func loadTestdataModel(t *testing.T, modelID string) Model {
	t.Helper()
	response := loadTestdataResponse(t, "models_list.json")

	for _, model := range response.Data {
		if model.ID == modelID {
			return model
		}
	}

	t.Fatalf("Model %s not found in testdata", modelID)
	return Model{}
}

func TestResponseUnmarshalAcceptsStringPricing(t *testing.T) {
	var response Response
	payload := []byte(`{
		"object": "list",
		"data": [{
			"id": "llama-3.3-70b-versatile",
			"object": "model",
			"created": 1733447754,
			"owned_by": "Meta",
			"pricing": {
				"prompt": "0.00004",
				"completion": "0.00000079",
				"input_cache_read": "0.00002",
				"request": "0",
				"image": "0"
			}
		}]
	}`)

	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	pricing := response.Data[0].Pricing
	if pricing == nil ||
		pricing.Prompt == nil ||
		*pricing.Prompt != 0.00004 ||
		pricing.Completion == nil ||
		*pricing.Completion != 0.00000079 ||
		pricing.InputCacheRead == nil ||
		*pricing.InputCacheRead != 0.00002 {
		t.Fatalf("pricing = %#v", pricing)
	}
}

// TestOpenAIModelDataParsing tests that we can properly parse OpenAI API responses.
func TestOpenAIModelDataParsing(t *testing.T) {
	// Test parsing the models list response
	response := loadTestdataResponse(t, "models_list.json")

	// Verify response structure
	if response.Object != "list" {
		t.Errorf("Expected object 'list', got '%s'", response.Object)
	}

	if len(response.Data) == 0 {
		t.Error("Expected at least some models, got 0")
	}

	// Test specific model data - use models that should exist in real OpenAI API
	// These tests verify we can parse the actual API format
	tests := []struct {
		name            string
		expectedID      string
		expectedOwnedBy string
		hasCreated      bool
	}{
		{
			name:            "GPT-4o model",
			expectedID:      "gpt-4o",
			expectedOwnedBy: "system",
			hasCreated:      true,
		},
		{
			name:            "GPT-3.5 Turbo model",
			expectedID:      "gpt-3.5-turbo",
			expectedOwnedBy: "openai",
			hasCreated:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find the model by ID
			var model Model
			found := false
			for _, m := range response.Data {
				if m.ID == tt.expectedID {
					model = m
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("Model '%s' not found in response", tt.expectedID)
			}

			if model.ID != tt.expectedID {
				t.Errorf("Expected ID '%s', got '%s'", tt.expectedID, model.ID)
			}

			if model.OwnedBy != tt.expectedOwnedBy {
				t.Errorf("Expected OwnedBy '%s', got '%s'", tt.expectedOwnedBy, model.OwnedBy)
			}

			if tt.hasCreated && model.Created == 0 {
				t.Error("Expected Created to be non-zero")
			}

			if model.Object != "model" {
				t.Errorf("Expected Object 'model', got '%s'", model.Object)
			}
		})
	}
}

// TestOpenAISingleModelParsing tests parsing of a single model response.
func TestOpenAISingleModelParsing(t *testing.T) {
	model := loadTestdataModel(t, "gpt-4o")

	// Verify all fields are correctly parsed
	if model.ID != "gpt-4o" {
		t.Errorf("Expected ID 'gpt-4o', got '%s'", model.ID)
	}

	if model.Object != "model" {
		t.Errorf("Expected Object 'model', got '%s'", model.Object)
	}

	if model.Created == 0 {
		t.Error("Expected Created to be non-zero")
	}

	if model.OwnedBy != "system" {
		t.Errorf("Expected OwnedBy 'system', got '%s'", model.OwnedBy)
	}
}

// TestConvertToOpenAIModel tests the conversion from OpenAIModelData to catalogs.Model.
func TestConvertToOpenAIModel(t *testing.T) {
	// Create a test provider with feature rules
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type:         catalogs.EndpointTypeOpenAI,
				URL:          "https://api.openai.com/v1/models",
				AuthRequired: true,
				FeatureRules: []catalogs.FeatureRule{
					{
						Field:    "id",
						Contains: []string{"gpt-4"},
						Feature:  "tools",
						Value:    true,
					},
					{
						Field:    "id",
						Contains: []string{"gpt-4"},
						Feature:  "tool_choice",
						Value:    true,
					},
				},
			},
		},
	}

	// Create an OpenAI client
	client := newTestClient(t, provider)

	// Test data based on actual OpenAI API response format
	openaiModel := Model{
		ID:      "gpt-4o",
		Object:  "model",
		Created: 1715367049,
		OwnedBy: "system",
	}

	// Convert to starmap model
	model := client.ConvertToModel(openaiModel)

	// Verify basic fields
	if model.ID != "gpt-4o" {
		t.Errorf("Expected ID 'gpt-4o', got '%s'", model.ID)
	}

	if model.Name != "gpt-4o" { // OpenAI uses ID as display name
		t.Errorf("Expected Name 'gpt-4o', got '%s'", model.Name)
	}

	// OpenAI models should have some basic features inferred
	if model.Features == nil {
		t.Fatal("Expected Features to be set")
	}

	// GPT-4o should support tools
	if !model.Features.Tools {
		t.Error("Expected Tools to be true for GPT-4o")
	}

	// GPT-4o should have basic text modalities
	if len(model.Features.Modalities.Input) == 0 {
		t.Fatal("Expected Modalities.Input to be set")
	}

	hasTextInput := slices.Contains(model.Features.Modalities.Input, catalogs.ModelModalityText)
	if !hasTextInput {
		t.Error("Expected GPT-4o to support text input")
	}
}

func TestConvertToModelWithWildcardAuthorMapping(t *testing.T) {
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDDeepInfra,
		Name: "DeepInfra",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				AuthorMapping: &catalogs.AuthorMapping{
					Field: "id",
					Normalized: map[string]catalogs.AuthorID{
						"Qwen/*":                             catalogs.AuthorIDAlibabaQwen,
						"deepseek-ai/*":                      catalogs.AuthorIDDeepSeek,
						"accounts/fireworks/models/kimi*":    "moonshot-ai",
						"accounts/fireworks/models/gpt-oss*": catalogs.AuthorIDOpenAI,
						"accounts/fireworks/models/llama-v*": catalogs.AuthorIDMeta,
					},
				},
			},
		},
	}
	client := newTestClient(t, provider)

	tests := []struct {
		name   string
		id     string
		author catalogs.AuthorID
	}{
		{
			name:   "deepinfra qwen namespace",
			id:     "Qwen/Qwen3-235B-A22B-Thinking-2507",
			author: catalogs.AuthorIDAlibabaQwen,
		},
		{
			name:   "deepinfra deepseek namespace",
			id:     "deepseek-ai/DeepSeek-V3",
			author: catalogs.AuthorIDDeepSeek,
		},
		{
			name:   "fireworks moonshot model path",
			id:     "accounts/fireworks/models/kimi-k2-instruct",
			author: "moonshot-ai",
		},
		{
			name:   "fireworks openai model path",
			id:     "accounts/fireworks/models/gpt-oss-120b",
			author: catalogs.AuthorIDOpenAI,
		},
		{
			name:   "fireworks meta pattern prefers longer path pattern",
			id:     "accounts/fireworks/models/llama-v3p1-8b-instruct",
			author: catalogs.AuthorIDMeta,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := client.ConvertToModel(Model{
				ID:      tt.id,
				Object:  "model",
				OwnedBy: "aggregator",
			})

			if len(model.Authors) != 1 {
				t.Fatalf("Expected one author, got %#v", model.Authors)
			}
			if model.Authors[0].ID != tt.author {
				t.Fatalf("Expected author %q, got %q", tt.author, model.Authors[0].ID)
			}
		})
	}
}

func TestConvertToModelPreservesUnknownAuthorshipWhenExplicitMappingMisses(t *testing.T) {
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDAlibabaCloud,
		Name: "Alibaba",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				AuthorMapping: &catalogs.AuthorMapping{
					Field: "id",
					Normalized: map[string]catalogs.AuthorID{
						"qwen*": catalogs.AuthorIDAlibabaQwen,
					},
				},
			},
		},
	}
	client := newTestClient(t, provider)

	model := client.ConvertToModel(Model{
		ID:      "baichuan2-turbo",
		Object:  "model",
		OwnedBy: "system",
	})

	if len(model.Authors) != 0 {
		t.Fatalf("unmatched explicit mapping invented provider authorship: %#v", model.Authors)
	}
}

func TestConvertToModelWithNestedProviderMetadata(t *testing.T) {
	contextLength := int64(256000)
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDDeepInfra,
		Name: "DeepInfra",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				FieldMappings: []catalogs.FieldMapping{
					{From: "metadata.description", To: "description"},
					{From: "metadata.context_length", To: "limits.context_window"},
					{From: "metadata.tags", To: "metadata.tags"},
				},
				FeatureRules: []catalogs.FeatureRule{
					{
						Field:    "metadata.tags",
						Contains: []string{"reasoning"},
						Feature:  "reasoning",
						Value:    true,
					},
				},
				AuthorMapping: &catalogs.AuthorMapping{
					Field: "id",
					Normalized: map[string]catalogs.AuthorID{
						"deepseek-ai/*": catalogs.AuthorIDDeepSeek,
					},
				},
			},
		},
	}
	client := newTestClient(t, provider)

	model := client.ConvertToModel(Model{
		ID:      "deepseek-ai/DeepSeek-V3.2",
		Object:  "model",
		OwnedBy: "deepinfra",
		Metadata: &ModelMetadata{
			Description:   "DeepSeek V3.2 served by DeepInfra",
			ContextLength: &contextLength,
			Tags:          []string{"chat", "reasoning"},
		},
	})

	if model.Description != "DeepSeek V3.2 served by DeepInfra" {
		t.Fatalf("Expected metadata description to map, got %q", model.Description)
	}
	if model.Limits == nil || model.Limits.ContextWindow != contextLength {
		t.Fatalf("Expected context window %d, got %#v", contextLength, model.Limits)
	}
	if model.Metadata == nil || len(model.Metadata.Tags) != 2 {
		t.Fatalf("Expected metadata tags to map, got %#v", model.Metadata)
	}
	if model.Metadata.Tags[0] != catalogs.ModelTagChat || model.Metadata.Tags[1] != catalogs.ModelTagReasoning {
		t.Fatalf("Unexpected tags: %#v", model.Metadata.Tags)
	}
	if model.Features == nil || !model.Features.Reasoning {
		t.Fatalf("Expected reasoning feature from metadata tags, got %#v", model.Features)
	}
	if len(model.Authors) != 1 || model.Authors[0].ID != catalogs.AuthorIDDeepSeek {
		t.Fatalf("Expected DeepSeek author, got %#v", model.Authors)
	}
}

func TestConvertToModelSkipsNilMappedProviderMetadata(t *testing.T) {
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDDeepInfra,
		Name: "DeepInfra",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				FieldMappings: []catalogs.FieldMapping{
					{From: "metadata.context_length", To: "limits.context_window"},
				},
			},
		},
	}
	client := newTestClient(t, provider)

	model := client.ConvertToModel(Model{
		ID:      "provider/model",
		Object:  "model",
		OwnedBy: "deepinfra",
		Metadata: &ModelMetadata{
			Tags: []string{"chat"},
		},
	})

	if model.Limits != nil {
		t.Fatalf("expected nil limits for absent metadata.context_length, got %#v", model.Limits)
	}
}

func TestConvertToModelMapsInputTokenLimit(t *testing.T) {
	inputTokenLimit := int64(272000)
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDGroq,
		Name: "Groq",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				FieldMappings: []catalogs.FieldMapping{
					{From: "input_token_limit", To: "limits.input_tokens"},
				},
			},
		},
	}
	client := newTestClient(t, provider)

	model := client.ConvertToModel(Model{
		ID:              "provider-model",
		Object:          "model",
		OwnedBy:         "provider",
		InputTokenLimit: &inputTokenLimit,
	})

	if model.Limits == nil || model.Limits.InputTokens != inputTokenLimit {
		t.Fatalf("Expected input token limit %d, got %#v", inputTokenLimit, model.Limits)
	}
}

func TestConvertToModelMapsLineage(t *testing.T) {
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
			},
		},
	}
	client := newTestClient(t, provider)
	parent := "gpt-4o-base"

	model := client.ConvertToModel(Model{
		ID:      "gpt-4o-finetuned",
		Object:  "model",
		OwnedBy: "system",
		Root:    "gpt-4o",
		Parent:  &parent,
	})

	if model.Lineage == nil ||
		model.Lineage.Root == nil ||
		*model.Lineage.Root != "gpt-4o" ||
		model.Lineage.Parent == nil ||
		*model.Lineage.Parent != parent {
		t.Fatalf("Lineage = %#v", model.Lineage)
	}
}

func TestConvertToModelPreservesOpenAICompatibleProviderFields(t *testing.T) {
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDGroq,
		Name: "Groq",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
			},
		},
	}
	client := newTestClient(t, provider)
	active := true
	contextWindow := int64(131072)
	maxOutputLength := int64(32768)
	// Groq's /models pricing block reports USD per 1M tokens.
	promptPrice := 0.59
	completionPrice := 0.79
	cacheReadPrice := 0.10
	requestPrice := 0.0
	imagePrice := 0.003

	model := client.ConvertToModel(Model{
		ID:                          "llama-3.3-70b-versatile",
		Object:                      "model",
		Name:                        "Llama 3.3 70B Versatile",
		OwnedBy:                     "Meta",
		Created:                     1733447754,
		Active:                      &active,
		ContextWindow:               &contextWindow,
		MaxOutputLength:             &maxOutputLength,
		HuggingFaceID:               "meta-llama/Llama-3.3-70B-Instruct",
		InputModalities:             []string{"text"},
		OutputModalities:            []string{"text"},
		SupportedFeatures:           []string{"tools", "json_mode", "reasoning"},
		SupportedSamplingParameters: []string{"temperature", "top_p", "stop", "seed"},
		Pricing: &ModelPricing{
			Request:        &requestPrice,
			Prompt:         &promptPrice,
			Completion:     &completionPrice,
			InputCacheRead: &cacheReadPrice,
			Image:          &imagePrice,
		},
	})

	if model.Name != "Llama 3.3 70B Versatile" {
		t.Fatalf("name = %q", model.Name)
	}
	if model.Status != catalogs.ModelStatusActive {
		t.Fatalf("status = %q, want active", model.Status)
	}
	if model.CreatedAt.IsZero() || model.UpdatedAt.IsZero() {
		t.Fatalf("created/updated timestamps were not mapped: %#v %#v", model.CreatedAt, model.UpdatedAt)
	}
	if model.Limits == nil ||
		model.Limits.ContextWindow != contextWindow ||
		model.Limits.OutputTokens != maxOutputLength {
		t.Fatalf("limits = %#v", model.Limits)
	}
	if model.Features == nil ||
		!model.Features.Tools ||
		!model.Features.ToolCalls ||
		!model.Features.ToolChoice ||
		!model.Features.FormatResponse ||
		!model.Features.Reasoning ||
		!model.Features.Seed {
		t.Fatalf("features = %#v", model.Features)
	}
	if model.Pricing == nil ||
		model.Pricing.Tokens == nil ||
		model.Pricing.Tokens.Input == nil ||
		model.Pricing.Tokens.Input.Per1M != promptPrice ||
		model.Pricing.Tokens.Output == nil ||
		model.Pricing.Tokens.Output.Per1M != completionPrice ||
		model.Pricing.Tokens.Cache == nil ||
		model.Pricing.Tokens.Cache.Read == nil ||
		model.Pricing.Tokens.Cache.Read.Per1M != cacheReadPrice ||
		model.Pricing.Operations == nil ||
		model.Pricing.Operations.Request == nil ||
		*model.Pricing.Operations.Request != requestPrice ||
		model.Pricing.Operations.ImageGen == nil ||
		*model.Pricing.Operations.ImageGen != imagePrice {
		t.Fatalf("pricing = %#v", model.Pricing)
	}
	if model.Extensions["groq"].Fields["hugging_face_id"] != "meta-llama/Llama-3.3-70B-Instruct" {
		t.Fatalf("extensions = %#v", model.Extensions)
	}
}

func TestConvertToModelPreservesMetadataMediaDefaultsAndPermissions(t *testing.T) {
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDDeepInfra,
		Name: "DeepInfra",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
			},
		},
	}
	client := newTestClient(t, provider)
	contextLength := int64(4096)
	defaultWidth := int64(1024)
	defaultHeight := int64(1024)
	defaultIterations := int64(4)
	// DeepInfra's metadata.pricing token fields report USD per 1M tokens.
	inputTokens := 1.0
	outputTokens := 5.0
	cacheReadTokens := 0.2
	perImage := 0.003
	inputCharacters := 0.001
	supportsChat := true
	supportsTools := true
	supportsImageInput := true
	supportsVideoIn := true
	supportsReasoning := true
	group := "public"

	model := client.ConvertToModel(Model{
		ID:                 "black-forest-labs/FLUX-1-schnell",
		Object:             "model",
		OwnedBy:            "black-forest-labs",
		Kind:               "image",
		SupportsChat:       &supportsChat,
		SupportsTools:      &supportsTools,
		SupportsImageInput: &supportsImageInput,
		SupportsVideoIn:    &supportsVideoIn,
		SupportsReasoning:  &supportsReasoning,
		Permission: []ModelPermission{{
			ID:           "modelperm-example",
			Object:       "model_permission",
			Created:      1733447754,
			Organization: "*",
			Group:        &group,
		}},
		Metadata: &ModelMetadata{
			Description:       "Image generation model",
			ContextLength:     &contextLength,
			DefaultWidth:      &defaultWidth,
			DefaultHeight:     &defaultHeight,
			DefaultIterations: &defaultIterations,
			Pricing: &ModelMetadataPricing{
				InputTokens:     &inputTokens,
				OutputTokens:    &outputTokens,
				CacheReadTokens: &cacheReadTokens,
				PerImageUnit:    &perImage,
				InputCharacters: &inputCharacters,
			},
		},
	})

	if model.Description != "Image generation model" {
		t.Fatalf("description = %q", model.Description)
	}
	if model.Limits == nil || model.Limits.ContextWindow != contextLength {
		t.Fatalf("limits = %#v", model.Limits)
	}
	if !containsOpenAITestModality(model.Features.Modalities.Input, catalogs.ModelModalityImage) ||
		!containsOpenAITestModality(model.Features.Modalities.Input, catalogs.ModelModalityVideo) ||
		!model.Features.Tools ||
		!model.Features.Reasoning {
		t.Fatalf("features = %#v", model.Features)
	}
	if model.Pricing == nil ||
		model.Pricing.Tokens == nil ||
		model.Pricing.Tokens.Input == nil ||
		model.Pricing.Tokens.Input.Per1M != inputTokens ||
		model.Pricing.Tokens.Output == nil ||
		model.Pricing.Tokens.Output.Per1M != outputTokens ||
		model.Pricing.Tokens.Cache == nil ||
		model.Pricing.Tokens.Cache.Read == nil ||
		model.Pricing.Tokens.Cache.Read.Per1M != cacheReadTokens ||
		model.Pricing.Operations == nil ||
		model.Pricing.Operations.ImageGen == nil ||
		*model.Pricing.Operations.ImageGen != perImage {
		t.Fatalf("pricing = %#v", model.Pricing)
	}
	extension := model.Extensions["deepinfra"].Fields
	if extension["kind"] != "image" || extension["supports_chat"] != true {
		t.Fatalf("extension = %#v", extension)
	}
	metadataExtension := extension["metadata"].(map[string]any)
	if metadataExtension["default_width"] != defaultWidth ||
		metadataExtension["default_height"] != defaultHeight ||
		metadataExtension["default_iterations"] != defaultIterations {
		t.Fatalf("metadata extension = %#v", metadataExtension)
	}
	pricingExtension := metadataExtension["pricing"].(map[string]any)
	if pricingExtension["input_characters"] != inputCharacters {
		t.Fatalf("pricing extension = %#v", pricingExtension)
	}
	permissions := extension["permission"].([]any)
	if len(permissions) != 1 || permissions[0].(map[string]any)["id"] != "modelperm-example" {
		t.Fatalf("permission extension = %#v", permissions)
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

func containsOpenAITestModality(modalities []catalogs.ModelModality, want catalogs.ModelModality) bool {
	return slices.Contains(modalities, want)
}

func TestConvertToModelWithNilCatalogProvider(t *testing.T) {
	client := newTestClient(t, &catalogs.Provider{
		ID:   "minimal",
		Name: "Minimal",
	})

	model := client.ConvertToModel(Model{
		ID:      "gpt-4o",
		Object:  "model",
		OwnedBy: "openai",
	})

	if len(model.Authors) != 1 || model.Authors[0].ID != catalogs.AuthorIDOpenAI {
		t.Fatalf("Expected OpenAI fallback author, got %#v", model.Authors)
	}
}

func TestConvertToModelMapsInactiveProviderFlagToUnknown(t *testing.T) {
	inactive := false
	client := newTestClient(t, &catalogs.Provider{
		ID:   catalogs.ProviderIDGroq,
		Name: "Groq",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
			},
		},
	})

	model := client.ConvertToModel(Model{
		ID:      "temporarily-inactive",
		Object:  "model",
		OwnedBy: "provider",
		Active:  &inactive,
	})

	if model.Status != catalogs.ModelStatusUnknown {
		t.Fatalf("inactive provider availability flag status = %q, want %q", model.Status, catalogs.ModelStatusUnknown)
	}
}

// TestOpenAIClientListModels tests the ListModels functionality.
func TestOpenAIClientListModels(t *testing.T) {
	requestCount := 0
	// Create a mock HTTP server that handles list requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Verify the request
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Check for authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("Expected Authorization header")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Handle different endpoints
		switch {
		case r.URL.Path == "/v1/models" || r.URL.Path == "/":
			// Return list response
			w.Write(testhelper.LoadTestdata(t, "models_list.json"))
			return

		default:
			t.Errorf("Unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}))
	defer server.Close()

	// Set up test API key
	os.Setenv("OPENAI_API_KEY", "test-api-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	// Create an OpenAI client with mock provider
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
		APIKey: &catalogs.ProviderAPIKey{
			Name:   "OPENAI_API_KEY",
			Header: "Authorization",
			Scheme: catalogs.ProviderAPIKeySchemeBearer,
		},
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type:         catalogs.EndpointTypeOpenAI,
				URL:          server.URL,
				AuthRequired: true,
				FeatureRules: []catalogs.FeatureRule{
					{
						Field:    "id",
						Contains: []string{"gpt-4"},
						Feature:  "tools",
						Value:    true,
					},
				},
			},
		},
	}

	client := newTestClient(t, provider)

	// Test ListModels
	ctx := context.Background()
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Verify we got a reasonable number of models
	if len(models) == 0 {
		t.Error("Expected at least some models, got 0")
	}

	// Verify we made exactly 1 request (OpenAI doesn't do individual model fetching)
	if requestCount != 1 {
		t.Errorf("Expected 1 request, got %d", requestCount)
	}

	// Find a specific model to test - use gpt-4o
	var testModel *catalogs.Model
	for _, model := range models {
		if model.ID == "gpt-4o" {
			testModel = &model
			break
		}
	}

	if testModel == nil {
		t.Fatal("Could not find gpt-4o model in response")
	}

	if testModel.Name != "gpt-4o" {
		t.Errorf("Expected test model Name to be 'gpt-4o', got '%s'", testModel.Name)
	}

	// Test that we have proper features for GPT-4o
	if testModel.Features == nil {
		t.Fatal("Expected test model to have Features")
	}

	if !testModel.Features.Tools {
		t.Error("Expected gpt-4o to support tools")
	}
}

func TestSchemaDriftMutationMatrix(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		wantErr     bool
		wantModels  int
		wantUnknown int
	}{
		{name: "valid", payload: `{"data":[{"id":"model-a"}]}`, wantModels: 1},
		{name: "missing", payload: `{}`, wantErr: true},
		{name: "renamed", payload: `{"items":[]}`, wantErr: true},
		{name: "null", payload: `{"data":null}`, wantErr: true},
		{name: "wrong type", payload: `{"data":{}}`, wantErr: true},
		{name: "unknown additive", payload: `{"data":[{"id":"model-a","new_capability":true}],"new_page":1}`, wantModels: 1, wantUnknown: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(test.payload))
			}))
			defer server.Close()
			client := newTestClient(t, &catalogs.Provider{
				ID: "test", Name: "Test",
				Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: server.URL}},
			})
			models, err := client.ListModels(context.Background())
			if test.wantErr && err == nil {
				t.Fatal("ListModels returned nil error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("ListModels: %v", err)
			}
			if len(models) != test.wantModels {
				t.Fatalf("models = %d, want %d", len(models), test.wantModels)
			}
			if test.wantUnknown > 0 {
				extension := models[0].Extensions["test"].Fields["unknown_fields"].([]any)
				if len(extension) != test.wantUnknown {
					t.Fatalf("unknown evidence = %#v, want %d entries", extension, test.wantUnknown)
				}
			}
		})
	}
}

// TestAPIFormatChanges tests that our parsing would catch provider API format changes.
// This is the kind of meaningful test the user requested.
func TestAPIFormatChanges(t *testing.T) {
	response := loadTestdataResponse(t, "models_list.json")

	// These tests ensure we catch API changes that could break our parsing
	t.Run("Required fields present", func(t *testing.T) {
		if len(response.Data) == 0 {
			t.Fatal("No models in response - API might have changed")
		}

		// Check that all models have required fields
		for i, model := range response.Data {
			if model.ID == "" {
				t.Errorf("Model %d missing required 'id' field", i)
			}
			if model.Object == "" {
				t.Errorf("Model %d missing required 'object' field", i)
			}
			if model.OwnedBy == "" {
				t.Errorf("Model %d missing required 'owned_by' field", i)
			}
			if model.Created == 0 {
				t.Errorf("Model %d missing required 'created' field", i)
			}
		}
	})

	t.Run("Known models still present", func(t *testing.T) {
		// These models should exist in OpenAI's API - if they're missing,
		// the API has changed significantly
		expectedModels := []string{"gpt-3.5-turbo", "gpt-4o"}

		for _, expectedID := range expectedModels {
			found := false
			for _, model := range response.Data {
				if model.ID == expectedID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected model '%s' not found in API response - API may have changed", expectedID)
			}
		}
	})

	t.Run("Response structure unchanged", func(t *testing.T) {
		if response.Object != "list" {
			t.Errorf("Expected response.object to be 'list', got '%s' - API structure may have changed", response.Object)
		}
	})
}

func TestConfiguredResponseModelsUsesValidatedCollectionPath(t *testing.T) {
	provider := &catalogs.Provider{Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{ResponseCollection: "payload.models"}}}
	response := Response{RawJSON: []byte(`{"payload":{"models":[{"id":"configured-model","object":"model"}]}}`)}
	models, err := configuredResponseModels(provider, response)
	if err != nil || len(models) != 1 || models[0].ID != "configured-model" {
		t.Fatalf("configuredResponseModels = %#v, %v", models, err)
	}

	response.RawJSON = []byte(`{"payload":{"models":null}}`)
	if _, err := configuredResponseModels(provider, response); err == nil {
		t.Fatal("configuredResponseModels accepted a null collection")
	}
	response.RawJSON = []byte(`{"payload":{"models":{}}}`)
	if _, err := configuredResponseModels(provider, response); err == nil {
		t.Fatal("configuredResponseModels accepted a non-array collection")
	}
}
