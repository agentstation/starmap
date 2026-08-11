package google

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/genai"

	"github.com/agentstation/starmap/internal/sourcepayload"
	"github.com/agentstation/starmap/internal/testcatalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestGetOrCreateVertexClientUsesRequestMaterial(t *testing.T) {
	client := NewClient(&catalogs.Provider{
		ID:   catalogs.ProviderIDGoogleVertex,
		Name: "Google Vertex AI",
	})
	profile := catalogs.ProviderCredentialProfile{
		ID: "workload-identity", Primitive: catalogs.ProviderAuthenticationGoogleDefault,
		Fields: []catalogs.ProviderCredentialFieldID{"access-token", "project", "location"},
		Placements: []catalogs.ProviderCredentialPlacement{{
			Field: "access-token", Kind: catalogs.ProviderCredentialPlacementHeader,
			Name: "Authorization", Scheme: catalogs.ProviderCredentialSchemeBearer,
		}},
		EndpointBindings: []catalogs.ProviderCredentialEndpointBinding{
			{Field: "project", Variable: "project", Format: catalogs.ProviderCredentialEndpointBindingPathSegment},
			{Field: "location", Variable: "location", Format: catalogs.ProviderCredentialEndpointBindingPathSegment},
		},
	}
	material := sources.NewProviderCredentialMaterial(
		profile,
		map[catalogs.ProviderCredentialFieldID]string{
			"access-token": "test-token", "project": "test-project", "location": "us-central1",
		},
		sources.ProviderCredentialMetadata{
			Version: "test", ExpiresAt: time.Now().Add(time.Hour),
		},
	)
	if _, err := client.newGenAIClient(context.Background(), true, material); err != nil {
		t.Fatalf("newGenAIClient: %v", err)
	}
}

func TestConvertGenAIModelPreservesProviderFields(t *testing.T) {
	client := NewClient(&catalogs.Provider{
		ID:   catalogs.ProviderIDGoogleAIStudio,
		Name: "Google AI Studio",
	})

	model := client.convertGenAIModel(&genai.Model{
		Name:                "models/gemini-3-pro",
		DisplayName:         "Gemini 3 Pro",
		Description:         "Representative Google model.",
		Version:             "003",
		DefaultCheckpointID: "checkpoint-003",
		Labels: map[string]string{
			"tier": "preview",
		},
		InputTokenLimit:  1048576,
		OutputTokenLimit: 65536,
		SupportedActions: []string{
			"generateContent",
			"streamGenerateContent",
			"countTokens",
		},
	})

	if model.ID != "gemini-3-pro" || model.Name != "Gemini 3 Pro" {
		t.Fatalf("identity = %s/%s", model.ID, model.Name)
	}
	if model.Limits == nil ||
		model.Limits.ContextWindow != 1048576 ||
		model.Limits.InputTokens != 1048576 ||
		model.Limits.OutputTokens != 65536 {
		t.Fatalf("limits = %#v", model.Limits)
	}
	if model.Features == nil || !model.Features.Streaming {
		t.Fatalf("features = %#v", model.Features)
	}
	if model.Features.Temperature || model.Features.TopP || model.Features.MaxTokens {
		t.Fatalf("supported actions inferred parameter support: %#v", model.Features)
	}
	extension := model.Extensions["google-ai-studio"].Fields
	if extension["version"] != "003" ||
		extension["default_checkpoint_id"] != "checkpoint-003" {
		t.Fatalf("extension = %#v", extension)
	}
	labels := extension["labels"].(map[string]any)
	if labels["tier"] != "preview" {
		t.Fatalf("labels extension = %#v", labels)
	}
	actions := extension["supported_actions"].([]any)
	if len(actions) != 3 {
		t.Fatalf("supported actions extension = %#v", actions)
	}
}

func TestConvertGenAIModelDoesNotInferModelFacts(t *testing.T) {
	client := NewClient(&catalogs.Provider{
		ID:   catalogs.ProviderIDGoogleAIStudio,
		Name: "Google AI Studio",
	})

	model := client.convertGenAIModel(&genai.Model{Name: "models/gemini-unknown"})

	if model.ID != "gemini-unknown" || model.Name != "gemini-unknown" {
		t.Fatalf("identity = %q/%q", model.ID, model.Name)
	}
	if model.Description != "" || !model.CreatedAt.IsZero() || !model.UpdatedAt.IsZero() {
		t.Fatalf("invented metadata = description %q, created %v, updated %v", model.Description, model.CreatedAt, model.UpdatedAt)
	}
	if len(model.Authors) != 0 || model.Features != nil || model.Limits != nil {
		t.Fatalf("invented facts = authors %#v, features %#v, limits %#v", model.Authors, model.Features, model.Limits)
	}
}

func TestConvertGenAIModelUsesCatalogPublisherMapping(t *testing.T) {
	client := NewClient(&catalogs.Provider{
		ID:   catalogs.ProviderIDGoogleVertex,
		Name: "Google Vertex AI",
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			AuthorMapping: &catalogs.AuthorMapping{
				Field: "publisher",
				Normalized: map[string]catalogs.AuthorID{
					"anthropic": catalogs.AuthorIDAnthropic,
				},
			},
		}},
	})

	for _, name := range []string{
		"publishers/anthropic/models/claude-opus",
		"projects/project/locations/us-central1/publishers/anthropic/models/claude-opus",
	} {
		model := client.convertGenAIModel(&genai.Model{Name: name})
		if model.ID != "claude-opus" {
			t.Fatalf("model ID for %q = %q", name, model.ID)
		}
		if len(model.Authors) != 1 || model.Authors[0].ID != catalogs.AuthorIDAnthropic {
			t.Fatalf("authors for %q = %#v", name, model.Authors)
		}
	}
}

func TestListModelsAIStudioFallsBackWhenRESTReturnsNoModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := testAIStudioProvider(server.URL)
	client := NewClient(provider)

	models, err := client.listModelsAIStudio(
		context.Background(), testAIStudioMaterial(provider),
	)
	if err == nil {
		t.Fatalf("expected SDK fallback error after empty REST response, got nil with %d models", len(models))
	}
	if len(models) != 0 {
		t.Fatalf("models = %d, want 0 on fallback error", len(models))
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
		{name: "valid", payload: `{"models":[{"name":"models/model-a","displayName":"Model A"}]}`, wantModels: 1},
		{name: "missing", payload: `{}`, wantErr: true},
		{name: "renamed", payload: `{"data":[]}`, wantErr: true},
		{name: "null", payload: `{"models":null}`, wantErr: true},
		{name: "wrong type", payload: `{"models":{}}`, wantErr: true},
		{name: "unknown additive", payload: `{"models":[{"name":"models/model-a","displayName":"Model A","newCapability":true}],"newPage":1}`, wantModels: 1, wantUnknown: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(test.payload))
			}))
			defer server.Close()
			provider := testAIStudioProvider(server.URL)
			client := NewClient(provider)
			models, err := client.listModelsAIStudioREST(
				context.Background(), testAIStudioMaterial(provider),
			)
			if test.wantErr && err == nil {
				t.Fatal("listModelsAIStudioREST returned nil error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("listModelsAIStudioREST: %v", err)
			}
			if len(models) != test.wantModels {
				t.Fatalf("models = %d, want %d", len(models), test.wantModels)
			}
			if test.wantUnknown > 0 {
				items := models[0].Extensions["google-ai-studio"].Fields["unknown_fields"].([]sourcepayload.UnknownJSONField)
				if len(items) != test.wantUnknown {
					t.Fatalf("unknown evidence = %#v", items)
				}
			}
		})
	}
}

func testAIStudioProvider(endpoint string) *catalogs.Provider {
	return &catalogs.Provider{
		ID: catalogs.ProviderIDGoogleAIStudio, Name: "Google AI Studio",
		Credentials: testcatalog.QueryAPIKeyCredentials("GOOGLE_API_KEY", "key"),
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type: catalogs.EndpointTypeGoogle, URL: endpoint,
		}},
	}
}

func testAIStudioMaterial(provider *catalogs.Provider) sources.ProviderCredentialMaterial {
	return testcatalog.APIKeyMaterial(provider.Credentials, "test-api-key")
}

func TestConvertAIStudioModelPreservesRESTOnlyFields(t *testing.T) {
	client := NewClient(&catalogs.Provider{
		ID:   catalogs.ProviderIDGoogleAIStudio,
		Name: "Google AI Studio",
	})
	temperature := 1.0
	maxTemperature := 2.0
	topP := 0.95
	topK := int32(40)
	thinking := true

	model := client.convertAIStudioModel(aiStudioModel{
		Name:                       "models/gemini-3-pro",
		DisplayName:                "Gemini 3 Pro",
		Description:                "Representative Google model.",
		Version:                    "003",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65536,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent", "countTokens"},
		Temperature:                &temperature,
		MaxTemperature:             &maxTemperature,
		TopP:                       &topP,
		TopK:                       &topK,
		Thinking:                   &thinking,
	})

	if model.Generation == nil ||
		model.Generation.Temperature == nil ||
		model.Generation.Temperature.Default != temperature ||
		model.Generation.Temperature.Min != 0 ||
		model.Generation.Temperature.Max != maxTemperature ||
		model.Generation.TopP == nil ||
		model.Generation.TopP.Min != 0 ||
		model.Generation.TopP.Max != 1 ||
		model.Generation.TopP.Default != topP ||
		model.Generation.TopK != nil {
		t.Fatalf("generation = %#v", model.Generation)
	}
	if model.Features == nil ||
		!model.Features.Temperature ||
		!model.Features.TopP ||
		!model.Features.TopK ||
		!model.Features.Streaming ||
		!model.Features.Reasoning {
		t.Fatalf("features = %#v", model.Features)
	}
	extension := model.Extensions["google-ai-studio"].Fields
	if extension["thinking"] != true {
		t.Fatalf("extension = %#v", extension)
	}
	methods := extension["supported_generation_methods"].([]any)
	if len(methods) != 3 {
		t.Fatalf("supported generation methods = %#v", methods)
	}
}
