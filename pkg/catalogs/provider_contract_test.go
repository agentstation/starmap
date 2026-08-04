package catalogs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestCatalogAcquisitionAuthContract(t *testing.T) {
	t.Parallel()

	provider := Provider{
		ID:   ProviderIDGoogleVertex,
		Name: "Google Vertex AI",
		Catalog: &ProviderCatalog{
			Auth: ProviderCatalogAuth{
				Method:   ProviderCatalogAuthGoogleDefault,
				Required: true,
				Scopes:   []string{"https://www.googleapis.com/auth/cloud-platform"},
			},
			Endpoint: ProviderEndpoint{
				Type: EndpointTypeGoogleCloud,
				URL:  "https://example.test/models",
			},
		},
	}

	if got := provider.Catalog.Auth.Method; got != ProviderCatalogAuthGoogleDefault {
		t.Fatalf("catalog auth method = %q, want %q", got, ProviderCatalogAuthGoogleDefault)
	}
	if provider.IsAPIKeyRequired() {
		t.Fatal("Google default credentials were classified as an API key")
	}
	if !provider.IsCatalogAuthRequired() {
		t.Fatal("required catalog authentication was classified as optional")
	}

	provider.Catalog.Endpoint.Type = EndpointTypeOpenAI
	if got := provider.Catalog.Auth.Method; got != ProviderCatalogAuthGoogleDefault {
		t.Fatalf("endpoint type changed catalog auth method to %q", got)
	}
}

func TestAcquisitionCredentialsNeverSerialize(t *testing.T) {
	provider := Provider{
		ID:   "secret-test",
		Name: "Secret Test",
		APIKey: &ProviderAPIKey{
			Name:   "STARMAP_SECRET_TEST_KEY",
			Header: "Authorization",
			Scheme: ProviderAPIKeySchemeBearer,
		},
		EnvVars: []ProviderEnvVar{{Name: "STARMAP_SECRET_TEST_TOKEN"}},
		Catalog: &ProviderCatalog{Auth: ProviderCatalogAuth{
			Method:   ProviderCatalogAuthAPIKey,
			Required: true,
		}},
	}
	t.Setenv("STARMAP_SECRET_TEST_KEY", "catalog-api-key-secret")
	t.Setenv("STARMAP_SECRET_TEST_TOKEN", "workload-token-secret")
	provider.LoadAPIKey()
	provider.LoadEnvVars()

	for name, marshal := range map[string]func(any) ([]byte, error){
		"json": json.Marshal,
		"yaml": yaml.Marshal,
	} {
		t.Run(name, func(t *testing.T) {
			encoded, err := marshal(provider)
			if err != nil {
				t.Fatalf("marshal provider: %v", err)
			}
			for _, secret := range []string{"catalog-api-key-secret", "workload-token-secret"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("serialized provider contains acquisition credential %q", secret)
				}
			}
		})
	}
}

func TestProviderInferenceContract(t *testing.T) {
	t.Parallel()

	provider := Provider{
		ID:   ProviderIDMistralAI,
		Name: "Mistral AI",
		Inference: &ProviderInference{
			BaseURL: "https://api.mistral.ai",
			Endpoints: []ProviderInferenceEndpoint{
				{Operation: ProviderOperationChatCompletions, Type: EndpointTypeOpenAI, Path: "/v1/chat/completions"},
				{Operation: ProviderOperationEmbeddings, Type: EndpointTypeOpenAI, Path: "/v1/embeddings"},
			},
		},
	}

	chat, found := provider.Inference.Endpoint(ProviderOperationChatCompletions)
	if !found {
		t.Fatal("chat completions endpoint not found")
	}
	if got := provider.Inference.EndpointURL(chat, ""); got != "https://api.mistral.ai/v1/chat/completions" {
		t.Fatalf("chat completions URL = %q", got)
	}
	embeddings, found := provider.Inference.Endpoint(ProviderOperationEmbeddings)
	if !found || embeddings.Type != EndpointTypeOpenAI {
		t.Fatalf("embeddings endpoint = %#v, found = %t", embeddings, found)
	}
}

func TestProviderOfferingSelectsAuthorProtocol(t *testing.T) {
	t.Parallel()

	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "anthropic", Name: "Anthropic"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel("anthropic", Model{
		ID: "opaque", Name: "Opaque", Authors: []Author{{ID: "anthropic", Name: "Anthropic"}},
		Features: &ModelFeatures{Modalities: ModelModalities{
			Input: []ModelModality{ModelModalityText}, Output: []ModelModality{ModelModalityText},
		}},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProvider(Provider{
		ID: "cloud", Name: "Cloud",
		Inference: &ProviderInference{
			BaseURL: "https://cloud.example",
			Endpoints: []ProviderInferenceEndpoint{{
				Operation:         ProviderOperationChatCompletions,
				Type:              EndpointTypeGoogleCloud,
				Path:              "/publishers/{publisher}/models/{provider_model_id}:invoke",
				StreamPath:        "/publishers/{publisher}/models/{provider_model_id}:streamInvoke",
				ProtocolsByAuthor: map[AuthorID]EndpointType{"anthropic": EndpointTypeAnthropic},
				PathsByAuthor: map[AuthorID]string{
					"anthropic": "/publishers/{publisher}/models/{provider_model_id}:rawPredict",
				},
				StreamPathsByAuthor: map[AuthorID]string{
					"anthropic": "/publishers/{publisher}/models/{provider_model_id}:streamRawPredict",
				},
			}},
		},
		Models: map[string]*Model{"opaque@001": {
			ID: "opaque@001", ModelRef: "anthropic/opaque", Name: "Opaque deployment",
			Features: &ModelFeatures{Modalities: ModelModalities{
				Input: []ModelModality{ModelModalityText}, Output: []ModelModality{ModelModalityText},
			}},
		}},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering("cloud", "opaque@001")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	endpoint, found := offering.Endpoint(ProviderOperationChatCompletions)
	if !found {
		t.Fatal("chat endpoint not found")
	}
	if endpoint.Type != EndpointTypeAnthropic {
		t.Fatalf("endpoint type = %q, want %q", endpoint.Type, EndpointTypeAnthropic)
	}
	if endpoint.URL != "https://cloud.example/publishers/anthropic/models/opaque@001:rawPredict" {
		t.Fatalf("endpoint URL = %q", endpoint.URL)
	}
	if endpoint.StreamURL != "https://cloud.example/publishers/anthropic/models/opaque@001:streamRawPredict" {
		t.Fatalf("stream endpoint URL = %q", endpoint.StreamURL)
	}
}

func TestBindOfferingEndpoint(t *testing.T) {
	t.Parallel()

	inference := &ProviderInference{BaseURL: "https://{location}.cloud.example"}
	endpoint := ProviderOfferingEndpoint{
		Operation: ProviderOperationChatCompletions,
		Type:      EndpointTypeGoogleCloud,
		URL:       "https://{location}.cloud.example/projects/{project}/models/opaque:invoke",
		StreamURL: "https://{location}.cloud.example/projects/{project}/models/opaque:streamInvoke",
	}
	bound, err := inference.BindOfferingEndpoint(endpoint, "https://private.example", map[string]string{
		"location": "us-test1",
		"project":  "tenant-project",
	})
	if err != nil {
		t.Fatalf("BindOfferingEndpoint: %v", err)
	}
	if bound.URL != "https://private.example/projects/tenant-project/models/opaque:invoke" {
		t.Fatalf("endpoint URL = %q", bound.URL)
	}
	if bound.StreamURL != "https://private.example/projects/tenant-project/models/opaque:streamInvoke" {
		t.Fatalf("stream endpoint URL = %q", bound.StreamURL)
	}

	_, err = inference.BindOfferingEndpoint(endpoint, "", map[string]string{"location": "us-test1"})
	if err == nil || !strings.Contains(err.Error(), "{project}") {
		t.Fatalf("missing project binding error = %v", err)
	}
}

func TestProviderOfferingServiceCapabilities(t *testing.T) {
	t.Parallel()

	cacheSupported := true
	offering := ProviderOffering{
		ProviderID:      "provider",
		ProviderModelID: "opaque/model@001",
		DefinitionID:    "author/model",
		Availability:    OfferingAvailabilityAvailable,
		Lifecycle:       OfferingLifecycleActive,
		Service: ProviderOfferingServiceCapabilities{
			Operations:  []ProviderOperation{ProviderOperationChatCompletions},
			PromptCache: &cacheSupported,
		},
	}
	if err := offering.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !offering.Supports(ProviderOperationChatCompletions) {
		t.Fatal("chat completions capability not found")
	}
	if offering.Supports(ProviderOperationEmbeddings) {
		t.Fatal("embedding capability was invented")
	}
	if offering.Service.PromptCache == nil || !*offering.Service.PromptCache {
		t.Fatal("prompt-cache capability not preserved")
	}

	copy := copyProviderOffering(offering)
	*copy.Service.PromptCache = false
	copy.Service.Operations[0] = ProviderOperationEmbeddings
	if !*offering.Service.PromptCache || offering.Service.Operations[0] != ProviderOperationChatCompletions {
		t.Fatal("offering capability copy shares mutable state")
	}
}
