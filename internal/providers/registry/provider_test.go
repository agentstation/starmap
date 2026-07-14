package registry

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/internal/connectors/anthropic"
	"github.com/agentstation/starmap/internal/connectors/google"
	"github.com/agentstation/starmap/internal/connectors/openai"
	"github.com/agentstation/starmap/internal/providers/cloudflare"
	"github.com/agentstation/starmap/internal/providers/cohere"
	"github.com/agentstation/starmap/internal/providers/databricks"
	"github.com/agentstation/starmap/internal/providers/huggingface"
	"github.com/agentstation/starmap/internal/providers/nvidia"
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
		assertClient func(t *testing.T, client Connector)
	}{
		{
			name:         "openai compatible",
			endpointType: catalogs.EndpointTypeOpenAI,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*openai.Client); !ok {
					t.Fatalf("client type = %T, want *openai.Client", client)
				}
			},
		},
		{
			name:         "anthropic",
			endpointType: catalogs.EndpointTypeAnthropic,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*anthropic.Client); !ok {
					t.Fatalf("client type = %T, want *anthropic.Client", client)
				}
			},
		},
		{
			name:         "cohere",
			endpointType: catalogs.EndpointTypeCohere,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*cohere.Client); !ok {
					t.Fatalf("client type = %T, want *cohere.Client", client)
				}
			},
		},
		{
			name:         "cloudflare",
			endpointType: catalogs.EndpointTypeCloudflare,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*cloudflare.Client); !ok {
					t.Fatalf("client type = %T, want *cloudflare.Client", client)
				}
			},
		},
		{
			name:         "together",
			endpointType: catalogs.EndpointTypeTogether,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*together.Client); !ok {
					t.Fatalf("client type = %T, want *together.Client", client)
				}
			},
		},
		{
			name:         "huggingface",
			endpointType: catalogs.EndpointTypeHuggingFace,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*huggingface.Client); !ok {
					t.Fatalf("client type = %T, want *huggingface.Client", client)
				}
			},
		},
		{
			name:         "nvidia",
			endpointType: catalogs.EndpointTypeNVIDIA,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*nvidia.Client); !ok {
					t.Fatalf("client type = %T, want *nvidia.Client", client)
				}
			},
		},
		{
			name:         "databricks",
			endpointType: catalogs.EndpointTypeDatabricks,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*databricks.Client); !ok {
					t.Fatalf("client type = %T, want *databricks.Client", client)
				}
			},
		},
		{
			name:         "snowflake",
			endpointType: catalogs.EndpointTypeSnowflake,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*snowflake.Client); !ok {
					t.Fatalf("client type = %T, want *snowflake.Client", client)
				}
			},
		},
		{
			name:         "watsonx",
			endpointType: catalogs.EndpointTypeWatsonx,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*watsonx.Client); !ok {
					t.Fatalf("client type = %T, want *watsonx.Client", client)
				}
			},
		},
		{
			name:         "google ai studio",
			endpointType: catalogs.EndpointTypeGoogle,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*google.Client); !ok {
					t.Fatalf("client type = %T, want *google.Client", client)
				}
			},
		},
		{
			name:         "google cloud",
			endpointType: catalogs.EndpointTypeGoogleCloud,
			assertClient: func(t *testing.T, client Connector) {
				t.Helper()
				if _, ok := client.(*google.Client); !ok {
					t.Fatalf("client type = %T, want *google.Client", client)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(testsource.Unauthenticated(t, testProvider(tt.endpointType)))
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}
			tt.assertClient(t, client)
		})
	}
}

func TestNewProviderRejectsUnsupportedEndpointType(t *testing.T) {
	provider := testProvider(catalogs.EndpointType("unsupported"))
	resolved, err := acquisition.NewResolver().Resolve(context.Background(), provider, "models")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	client, err := New(resolved)
	if err == nil {
		t.Fatal("New returned nil error")
	}
	if client != nil {
		t.Fatalf("client = %#v, want nil", client)
	}

	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want ValidationError", err)
	}
	if validationErr.Field != "provider.catalog.source.endpoint.type" {
		t.Fatalf("validation field = %q, want provider.catalog.source.endpoint.type", validationErr.Field)
	}
}

func TestSupportsMatchesExecutableConnectorSet(t *testing.T) {
	supported := []catalogs.EndpointType{
		catalogs.EndpointTypeOpenAI,
		catalogs.EndpointTypeAnthropic,
		catalogs.EndpointTypeCohere,
		catalogs.EndpointTypeCloudflare,
		catalogs.EndpointTypeTogether,
		catalogs.EndpointTypeHuggingFace,
		catalogs.EndpointTypeNVIDIA,
		catalogs.EndpointTypeDatabricks,
		catalogs.EndpointTypeSnowflake,
		catalogs.EndpointTypeWatsonx,
		catalogs.EndpointTypeGoogle,
		catalogs.EndpointTypeGoogleCloud,
	}
	for _, endpointType := range supported {
		if !Supports(endpointType) {
			t.Errorf("Supports(%q) = false, want true", endpointType)
		}
	}
	for _, endpointType := range []catalogs.EndpointType{catalogs.EndpointTypeApplication, "unsupported"} {
		if Supports(endpointType) {
			t.Errorf("Supports(%q) = true, want false", endpointType)
		}
	}
}

func TestDecodeFixtureUsesSelectedConnectorSchema(t *testing.T) {
	source := testsource.Unauthenticated(t, testProvider(catalogs.EndpointTypeOpenAI))
	models, err := DecodeFixture(source, []byte(`{"object":"list","data":[{"id":"model-a","owned_by":"openai"}]}`))
	if err != nil {
		t.Fatalf("DecodeFixture: %v", err)
	}
	if len(models) != 1 || models[0].ID != "model-a" {
		t.Fatalf("models = %#v, want model-a", models)
	}
}

func TestDecodeFixtureRejectsConnectorWithoutReplaySchema(t *testing.T) {
	source := testsource.Unauthenticated(t, testProvider(catalogs.EndpointTypeDatabricks))
	models, err := DecodeFixture(source, []byte(`{}`))
	if err == nil {
		t.Fatal("DecodeFixture returned nil error")
	}
	if models != nil {
		t.Fatalf("models = %#v, want nil", models)
	}
	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want ValidationError", err)
	}
	if validationErr.Field != "provider.fixture.decoder" {
		t.Fatalf("validation field = %q, want provider.fixture.decoder", validationErr.Field)
	}
}

func TestNewProviderRejectsInvalidOfferingDefaultsBeforeAdapterCreation(t *testing.T) {
	provider := testProvider(catalogs.EndpointTypeOpenAI)
	provider.Catalog.Sources[0].Offering = &catalogs.ProviderOfferingDefaults{
		Access: catalogs.OfferingAccess{
			Channel:     catalogs.OfferingAccessChannelServerToServer,
			Routability: catalogs.OfferingRoutabilityRoutable,
			APIs:        []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions},
		},
		Deployment: catalogs.ProviderDeployment{Type: "serverless"},
		// A routable offering deliberately omits its endpoint contract.
	}

	_, err := acquisition.NewResolver().Resolve(context.Background(), provider, "models")
	var client Connector
	if err == nil {
		t.Fatal("New accepted invalid offering defaults")
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
	provider.Catalog.Sources[0].Endpoint.FieldMappings = []catalogs.FieldMapping{{
		From: "upstream[0]",
		To:   "limits.context_window",
	}}

	_, err := acquisition.NewResolver().Resolve(context.Background(), provider, "models")
	var client Connector
	if err == nil {
		t.Fatal("New accepted an invalid configured field mapping")
	}
	if client != nil {
		t.Fatalf("client = %#v, want nil after validation failure", client)
	}
	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want ValidationError", err)
	}
	if validationErr.Field != "provider.catalog.endpoint.field_mappings[0].from" {
		t.Fatalf("validation field = %q, want provider.catalog.endpoint.field_mappings[0].from", validationErr.Field)
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
	provider.Catalog.Sources[0].Endpoint.URL = server.URL
	client, err := New(testsource.Unauthenticated(t, provider))
	if err != nil {
		t.Fatalf("New: %v", err)
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
		catalogs.ProviderIDMistralAI,
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
	result, err := FetchRaw(context.Background(), testsource.Authenticated(t, testAuthenticatedProvider(endpoint)))
	if err != nil {
		t.Fatalf("FetchRaw returned error: %v", err)
	}
	if string(result.Data) != `{"ok":true}` {
		t.Fatalf("data = %q, want raw JSON response", string(result.Data))
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("response status = %d, want 200", result.StatusCode)
	}
}

func TestFetchRawUsesConnectorOwnedLogicalSourceURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("dedicated"); got != "true" {
			t.Fatalf("dedicated query = %q, want true", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDTogetherAI, Name: "Together AI",
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "dedicated-models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
			Auth:     catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone},
			Options:  map[string]catalogs.ProviderOptionBinding{"inventory": {Source: catalogs.ProviderBindingSourceStatic, Value: "dedicated"}},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeTogether, URL: server.URL},
		}}},
	}
	source, err := acquisition.NewResolver().Resolve(context.Background(), provider, "dedicated-models")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	_, err = FetchRaw(context.Background(), source)
	if err != nil {
		t.Fatalf("FetchRaw: %v", err)
	}
}

func TestFetchRawWrapsTransportFailuresAsAPIErrors(t *testing.T) {
	t.Setenv("STARMAP_TEST_PROVIDER_API_KEY", "secret")
	_, err := FetchRaw(context.Background(), testsource.Authenticated(t, testAuthenticatedProvider("http://127.0.0.1")))
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

	_, err := FetchRaw(context.Background(), testsource.Authenticated(t, testAuthenticatedProvider(server.URL)))
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
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
			Auth: catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}, Endpoint: catalogs.ProviderSourceEndpoint{Type: endpointType, URL: "https://example.test/models"},
		}}},
	}
}

func testAuthenticatedProvider(endpoint string) *catalogs.Provider {
	provider := testProvider(catalogs.EndpointTypeOpenAI)
	provider.Credentials = map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{
		"api_key": {Env: catalogs.ProviderEnvironmentNames{"STARMAP_TEST_PROVIDER_API_KEY"}, Transport: catalogs.ProviderCredentialTransport{Header: "X-Test-Key", Scheme: catalogs.ProviderCredentialSchemeDirect}},
	}
	provider.Catalog.Sources[0].Endpoint.URL = endpoint
	provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}}
	return provider
}
