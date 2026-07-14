package google

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/auth"
	"google.golang.org/genai"

	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

type staticTokenProvider struct{}

func (staticTokenProvider) Token(context.Context) (*auth.Token, error) {
	return &auth.Token{Value: "test-token", Expiry: time.Now().Add(time.Hour)}, nil
}

func TestGetOrCreateVertexClientDoesNotDeadlockOnCachedCredentials(t *testing.T) {
	client := &Client{provider: &catalogs.Provider{
		ID:   catalogs.ProviderIDGoogleVertex,
		Name: "Google Vertex AI",
	}}
	client.projectID = "test-project"
	client.location = "us-central1"
	client.credentials = auth.NewCredentials(&auth.CredentialsOptions{
		TokenProvider: staticTokenProvider{},
	})

	result := make(chan error, 1)
	go func() {
		_, err := client.getOrCreateGenAIClient(context.Background(), true)
		result <- err
	}()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("getOrCreateGenAIClient: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("getOrCreateGenAIClient deadlocked while reusing cached credentials")
	}
}

func TestConvertGenAIModelPreservesProviderFields(t *testing.T) {
	client := &Client{provider: &catalogs.Provider{
		ID:   catalogs.ProviderIDGoogleAIStudio,
		Name: "Google AI Studio",
	}}

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
	if model.Features == nil ||
		!model.Features.Temperature ||
		!model.Features.TopP ||
		!model.Features.MaxTokens ||
		!model.Features.Streaming {
		t.Fatalf("features = %#v", model.Features)
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

func TestListModelsAIStudioFallsBackWhenRESTReturnsNoModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDGoogleAIStudio,
		Name: "Google AI Studio",
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
			Auth: catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}, Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeGoogle, URL: server.URL},
		}}},
	}
	client := NewClient(testsource.Unauthenticated(t, provider))

	models, err := client.listModelsAIStudio(context.Background())
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
			provider := &catalogs.Provider{
				ID: catalogs.ProviderIDGoogleAIStudio, Name: "Google AI Studio",
				Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
					ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
					Auth: catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}, Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeGoogle, URL: server.URL},
				}}},
			}
			client := NewClient(testsource.Unauthenticated(t, provider))
			models, err := client.listModelsAIStudioREST(context.Background())
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

func TestConvertAIStudioModelPreservesRESTOnlyFields(t *testing.T) {
	client := &Client{provider: &catalogs.Provider{
		ID:   catalogs.ProviderIDGoogleAIStudio,
		Name: "Google AI Studio",
	}}
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
		model.Generation.Temperature.Max != maxTemperature ||
		model.Generation.TopP == nil ||
		model.Generation.TopP.Default != topP ||
		model.Generation.TopK == nil ||
		model.Generation.TopK.Default != int(topK) {
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
