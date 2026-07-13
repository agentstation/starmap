package clients

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/providers/anthropic"
	"github.com/agentstation/starmap/internal/providers/cloudflare"
	"github.com/agentstation/starmap/internal/providers/cohere"
	"github.com/agentstation/starmap/internal/providers/databricks"
	"github.com/agentstation/starmap/internal/providers/google"
	"github.com/agentstation/starmap/internal/providers/huggingface"
	"github.com/agentstation/starmap/internal/providers/nvidia"
	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/internal/providers/sambanova"
	"github.com/agentstation/starmap/internal/providers/snowflake"
	"github.com/agentstation/starmap/internal/providers/together"
	"github.com/agentstation/starmap/internal/providers/watsonx"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestNewProviderRoutesByEndpointType(t *testing.T) {
	tests := []struct {
		name         string
		endpointType catalogs.EndpointType
		assertClient func(t *testing.T, client ProviderClient)
	}{
		{
			name:         "openai compatible",
			endpointType: catalogs.EndpointTypeOpenAI,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*openai.Client); !ok {
					t.Fatalf("client type = %T, want *openai.Client", client)
				}
			},
		},
		{
			name:         "anthropic",
			endpointType: catalogs.EndpointTypeAnthropic,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*anthropic.Client); !ok {
					t.Fatalf("client type = %T, want *anthropic.Client", client)
				}
			},
		},
		{
			name:         "cohere",
			endpointType: catalogs.EndpointTypeCohere,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*cohere.Client); !ok {
					t.Fatalf("client type = %T, want *cohere.Client", client)
				}
			},
		},
		{
			name:         "cloudflare",
			endpointType: catalogs.EndpointTypeCloudflare,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*cloudflare.Client); !ok {
					t.Fatalf("client type = %T, want *cloudflare.Client", client)
				}
			},
		},
		{
			name:         "sambanova",
			endpointType: catalogs.EndpointTypeSambaNova,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*sambanova.Client); !ok {
					t.Fatalf("client type = %T, want *sambanova.Client", client)
				}
			},
		},
		{
			name:         "together",
			endpointType: catalogs.EndpointTypeTogether,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*together.Client); !ok {
					t.Fatalf("client type = %T, want *together.Client", client)
				}
			},
		},
		{
			name:         "huggingface",
			endpointType: catalogs.EndpointTypeHuggingFace,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*huggingface.Client); !ok {
					t.Fatalf("client type = %T, want *huggingface.Client", client)
				}
			},
		},
		{
			name:         "nvidia",
			endpointType: catalogs.EndpointTypeNVIDIA,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*nvidia.Client); !ok {
					t.Fatalf("client type = %T, want *nvidia.Client", client)
				}
			},
		},
		{
			name:         "databricks",
			endpointType: catalogs.EndpointTypeDatabricks,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*databricks.Client); !ok {
					t.Fatalf("client type = %T, want *databricks.Client", client)
				}
			},
		},
		{
			name:         "snowflake",
			endpointType: catalogs.EndpointTypeSnowflake,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*snowflake.Client); !ok {
					t.Fatalf("client type = %T, want *snowflake.Client", client)
				}
			},
		},
		{
			name:         "watsonx",
			endpointType: catalogs.EndpointTypeWatsonx,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*watsonx.Client); !ok {
					t.Fatalf("client type = %T, want *watsonx.Client", client)
				}
			},
		},
		{
			name:         "google ai studio",
			endpointType: catalogs.EndpointTypeGoogle,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*google.Client); !ok {
					t.Fatalf("client type = %T, want *google.Client", client)
				}
			},
		},
		{
			name:         "google cloud",
			endpointType: catalogs.EndpointTypeGoogleCloud,
			assertClient: func(t *testing.T, client ProviderClient) {
				t.Helper()
				if _, ok := client.(*google.Client); !ok {
					t.Fatalf("client type = %T, want *google.Client", client)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewProvider(testProvider(tt.endpointType))
			if err != nil {
				t.Fatalf("NewProvider returned error: %v", err)
			}
			tt.assertClient(t, client)
		})
	}
}

func TestNewProviderRejectsUnsupportedEndpointType(t *testing.T) {
	client, err := NewProvider(testProvider(catalogs.EndpointType("unsupported")))
	if err == nil {
		t.Fatal("NewProvider returned nil error")
	}
	if client != nil {
		t.Fatalf("client = %#v, want nil", client)
	}

	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want ValidationError", err)
	}
	if validationErr.Field != "provider.catalog.endpoint.type" {
		t.Fatalf("validation field = %q, want provider.catalog.endpoint.type", validationErr.Field)
	}
}

func TestNewProviderRejectsInvalidOfferingDefaultsBeforeAdapterCreation(t *testing.T) {
	provider := testProvider(catalogs.EndpointTypeOpenAI)
	provider.Catalog.Offering = &catalogs.ProviderOfferingDefaults{
		Access: catalogs.OfferingAccess{
			Channel:     catalogs.OfferingAccessChannelServerToServer,
			Routability: catalogs.OfferingRoutabilityRoutable,
			APIs:        []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions},
		},
		Deployment: catalogs.ProviderDeployment{Type: "serverless"},
		// A routable offering deliberately omits its endpoint contract.
	}

	client, err := NewProvider(provider)
	if err == nil {
		t.Fatal("NewProvider accepted invalid offering defaults")
	}
	if client != nil {
		t.Fatalf("client = %#v, want nil after configuration validation failure", client)
	}
	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want wrapped ValidationError", err)
	}
	if validationErr.Field != "endpoint.type" {
		t.Fatalf("validation field = %q, want endpoint.type", validationErr.Field)
	}
}

func TestNewProviderMappingValidationReturnsTypedFailureBeforeAdapterCreation(t *testing.T) {
	provider := testProvider(catalogs.EndpointTypeOpenAI)
	provider.Catalog.Endpoint.FieldMappings = []catalogs.FieldMapping{{
		From: "upstream_field_that_does_not_exist",
		To:   "limits.context_window",
	}}

	client, err := NewProvider(provider)
	if err == nil {
		t.Fatal("NewProvider accepted an invalid configured field mapping")
	}
	if client != nil {
		t.Fatalf("client = %#v, want nil after validation failure", client)
	}
	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want ValidationError", err)
	}
	if validationErr.Field != "field_mappings.from" {
		t.Fatalf("validation field = %q, want field_mappings.from", validationErr.Field)
	}
}

func TestNewProviderUsesProviderConfigurationWithoutNamedAdapter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"zai-org/GLM-5.2","owned_by":"zai-org"}]}`))
	}))
	defer server.Close()

	provider := testProvider(catalogs.EndpointTypeOpenAI)
	provider.ID = catalogs.ProviderIDHyperbolic
	provider.Catalog.Endpoint.URL = server.URL
	client, err := NewProvider(provider)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0].ID != "zai-org/GLM-5.2" {
		t.Fatalf("generic provider client result: %#v", models)
	}
}

func TestOpenAIProviderOptionsRegistersEveryProviderOwnedAdapter(t *testing.T) {
	for _, providerID := range []catalogs.ProviderID{
		catalogs.ProviderIDMistralAI,
		catalogs.ProviderIDNovita,
		catalogs.ProviderIDXAI,
	} {
		t.Run(providerID.String(), func(t *testing.T) {
			if options := openAIProviderOptions(providerID); len(options) == 0 {
				t.Fatalf("provider %q has no registered adapter options", providerID)
			}
		})
	}
	if options := openAIProviderOptions("generic-openai-compatible"); options != nil {
		t.Fatalf("generic provider options = %#v, want nil", options)
	}
	for _, providerID := range []catalogs.ProviderID{
		catalogs.ProviderIDBaseten,
		catalogs.ProviderIDHyperbolic,
		catalogs.ProviderIDScaleway,
	} {
		if options := openAIProviderOptions(providerID); options != nil {
			t.Fatalf("configuration-only provider %q options = %#v, want nil", providerID, options)
		}
	}
}

func TestFetchRawUsesTransportAuthenticationAndReturnsResponseMetadata(t *testing.T) {
	t.Setenv("STARMAP_TEST_PROVIDER_API_KEY", "secret")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Test-Key"); got != "secret" {
			t.Fatalf("X-Test-Key = %q, want secret", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	endpoint := server.URL + "/models"
	result, err := FetchRaw(context.Background(), testAuthenticatedProvider(endpoint), endpoint)
	if err != nil {
		t.Fatalf("FetchRaw returned error: %v", err)
	}
	if string(result.Data) != `{"ok":true}` {
		t.Fatalf("data = %q, want raw JSON response", string(result.Data))
	}
	if result.Response == nil || result.Response.StatusCode != http.StatusOK {
		t.Fatalf("response status = %#v, want 200", result.Response)
	}
	if result.RequestURL != endpoint {
		t.Fatalf("request URL = %q, want %q", result.RequestURL, endpoint)
	}
}

func TestFetchRawWrapsTransportFailuresAsAPIErrors(t *testing.T) {
	_, err := FetchRaw(context.Background(), testAuthenticatedProvider("http://127.0.0.1"), "http://127.0.0.1")
	if err == nil {
		t.Fatal("FetchRaw returned nil error")
	}

	var apiErr *pkgerrors.APIError
	if !stderrors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want APIError", err)
	}
	if apiErr.Provider != "test-provider" {
		t.Fatalf("api error provider = %q, want test-provider", apiErr.Provider)
	}
}

func TestFetchRawRejectsOversizedResponse(t *testing.T) {
	t.Setenv("STARMAP_TEST_PROVIDER_API_KEY", "secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", constants.MaxSourcePayloadBytes+1)))
	}))
	defer server.Close()

	_, err := FetchRaw(context.Background(), testAuthenticatedProvider(server.URL), server.URL)
	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error = %T %v, want *errors.ValidationError", err, err)
	}
	if validationErr.Field != "response.body" {
		t.Fatalf("field = %q, want response.body", validationErr.Field)
	}
}

func testProvider(endpointType catalogs.EndpointType) *catalogs.Provider {
	return &catalogs.Provider{
		ID:   "test-provider",
		Name: "Test Provider",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: endpointType,
				URL:  "https://example.test/models",
			},
		},
	}
}

func testAuthenticatedProvider(endpoint string) *catalogs.Provider {
	provider := testProvider(catalogs.EndpointTypeOpenAI)
	provider.APIKey = &catalogs.ProviderAPIKey{
		Name:   "STARMAP_TEST_PROVIDER_API_KEY",
		Header: "X-Test-Key",
		Scheme: catalogs.ProviderAPIKeySchemeDirect,
	}
	provider.Catalog.Endpoint.URL = endpoint
	provider.Catalog.Endpoint.AuthRequired = true
	return provider
}
