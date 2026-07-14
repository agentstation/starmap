package sources

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type providerFetcherTestClient struct {
	models []catalogs.Model
	err    error
}

type retryingProviderFetcherTestClient struct {
	attempts int
	status   int
}

func (c *retryingProviderFetcherTestClient) ListModels(context.Context) ([]catalogs.Model, error) {
	c.attempts++
	if c.attempts < 3 || c.status != 0 {
		status := c.status
		if status == 0 {
			status = http.StatusTooManyRequests
		}
		return nil, &pkgerrors.APIError{Provider: "mistral", StatusCode: status, Message: "fixture"}
	}
	return []catalogs.Model{{ID: "mistral-small-latest", Name: "Mistral Small"}}, nil
}

func (c providerFetcherTestClient) ListModels(context.Context) ([]catalogs.Model, error) {
	return c.models, c.err
}

func TestProviderFetcherHasClientUsesInjectedFactory(t *testing.T) {
	fetcher := NewProviderFetcher(newFetcherProviderSet(providerForFetcherTest("supported")),
		WithProviderClientFactory(func(source acquisition.Source) (ProviderClient, error) {
			provider := source.Provider()
			if provider.ID == "supported" {
				return providerFetcherTestClient{}, nil
			}
			return nil, &pkgerrors.ConfigError{Component: string(provider.ID), Message: "unsupported"}
		}),
	)

	if !fetcher.HasClient("supported") {
		t.Fatal("Expected injected factory to report supported provider")
	}
	if fetcher.HasClient("missing") {
		t.Fatal("Expected missing provider to be unsupported")
	}
}

func TestProviderFetcherUsesDefaultProviderHooks(t *testing.T) {
	provider := providerForFetcherTest("supported")
	fetcher := NewProviderFetcher(newFetcherProviderSet(provider))

	if fetcher.options.clientFactory == nil {
		t.Fatal("Expected default provider client factory")
	}
	if fetcher.options.rawFetcher == nil {
		t.Fatal("Expected default provider raw fetcher")
	}
	if !fetcher.HasClient("supported") {
		t.Fatal("Expected default provider factory to support OpenAI-compatible provider")
	}
}

func TestProviderFetcherFetchModelsUsesInjectedFactory(t *testing.T) {
	provider := providerForFetcherTest("provider-a")
	fetcher := NewProviderFetcher(newFetcherProviderSet(provider),
		WithProviderClientFactory(func(source acquisition.Source) (ProviderClient, error) {
			return providerFetcherTestClient{
				models: []catalogs.Model{{ID: "model-a", Name: "Model A"}},
			}, nil
		}),
	)

	models, err := fetcher.FetchModels(context.Background(), &provider)
	if err != nil {
		t.Fatalf("FetchModels failed: %v", err)
	}
	if len(models) != 1 || models[0].ID != "model-a" {
		t.Fatalf("Expected fetched model, got %#v", models)
	}
}

func TestProviderFetcherRetainsLogicalSourceIdentityAndScope(t *testing.T) {
	provider := providerForFetcherTest("source-scope")
	provider.Credentials = map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{
		"api_key": {Env: catalogs.ProviderEnvironmentNames{"STARMAP_SOURCE_SCOPE_KEY"}},
	}
	provider.Catalog.Sources = append(provider.Catalog.Sources, catalogs.ProviderSource{
		ID: "private-models",
		ObservationScope: catalogs.ProviderObservationPolicy{
			Anonymous: catalogs.ProviderObservationScopeGlobalPublic, Authenticated: catalogs.ProviderObservationScopeCredentialScoped,
		},
		Auth:     catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeOptional},
		Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/private-models"},
	})
	t.Setenv("STARMAP_SOURCE_SCOPE_KEY", "secret")
	calls := map[string]int{}
	fetcher := NewProviderFetcher(newFetcherProviderSet(provider), WithProviderClientFactory(func(source acquisition.Source) (ProviderClient, error) {
		calls[source.SourceID()]++
		return providerFetcherTestClient{models: []catalogs.Model{{ID: source.SourceID()}}}, nil
	}))

	results, err := fetcher.FetchModelSources(context.Background(), &provider)
	if err != nil {
		t.Fatalf("FetchModelSources: %v", err)
	}
	if len(results) != 2 || calls["models"] != 1 || calls["private-models"] != 1 {
		t.Fatalf("results/calls = %#v %#v", results, calls)
	}
	if results[0].SourceID != "models" || results[0].Scope != catalogs.ProviderObservationScopeGlobalPublic || results[0].Authenticated {
		t.Fatalf("public source = %#v", results[0])
	}
	if results[1].SourceID != "private-models" || results[1].Scope != catalogs.ProviderObservationScopeCredentialScoped || !results[1].Authenticated {
		t.Fatalf("private source = %#v", results[1])
	}
}

func TestProviderFetcherAppliesEachLogicalSourcesOfferingDefaultsBeforeAggregation(t *testing.T) {
	provider := providerForFetcherTest("source-defaults")
	provider.Catalog.Sources[0].Offering = &catalogs.ProviderOfferingDefaults{
		Access:     catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}},
		Endpoint:   catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeOpenAI},
		Deployment: catalogs.ProviderDeployment{Type: "serverless"},
	}
	provider.Catalog.Sources = append(provider.Catalog.Sources, catalogs.ProviderSource{
		ID: "dedicated", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
		Auth:     catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone},
		Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/dedicated"},
		Offering: &catalogs.ProviderOfferingDefaults{
			Access:     catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityRoutable, APIs: []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}},
			Endpoint:   catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeOpenAI},
			Deployment: catalogs.ProviderDeployment{Type: "dedicated"},
			Regions:    []catalogs.CloudRegion{{ID: "region-a", Residency: &catalogs.GeographicBoundary{ID: "geo", Kind: catalogs.GeographicBoundaryCountry, Countries: []string{"US"}}}},
		},
	})
	fetcher := NewProviderFetcher(newFetcherProviderSet(provider), WithProviderClientFactory(func(source acquisition.Source) (ProviderClient, error) {
		return providerFetcherTestClient{models: []catalogs.Model{{ID: source.SourceID(), Name: source.SourceID()}}}, nil
	}))
	results, err := fetcher.FetchModelSources(context.Background(), &provider)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Models[0].OfferingDeployment.Type != "serverless" || results[1].Models[0].OfferingDeployment.Type != "dedicated" || results[1].Models[0].OfferingRegions[0].ID != "region-a" {
		t.Fatalf("source-specific defaults = %#v", results)
	}
	results[1].Models[0].OfferingRegions[0].Residency.Countries[0] = "changed"
	if provider.Catalog.Sources[1].Offering.Regions[0].Residency.Countries[0] != "US" {
		t.Fatal("source default regions alias provider configuration")
	}
}

func TestProviderFetcherAppliesBoundedRetryPolicy(t *testing.T) {
	provider := providerForFetcherTest(catalogs.ProviderIDMistralAI)
	client := &retryingProviderFetcherTestClient{}
	policy := ProviderRetryPolicy{MaxAttempts: 3, BaseDelay: time.Nanosecond, MaxDelay: time.Nanosecond}
	fetcher := NewProviderFetcher(newFetcherProviderSet(provider),
		WithProviderClientFactory(func(acquisition.Source) (ProviderClient, error) { return client, nil }),
		WithProviderRetryPolicy(policy),
	)
	models, err := fetcher.FetchModels(context.Background(), &provider)
	if err != nil || client.attempts != 3 || len(models) != 1 {
		t.Fatalf("retry result = attempts %d models %#v err %v", client.attempts, models, err)
	}

	terminal := &retryingProviderFetcherTestClient{status: http.StatusUnauthorized}
	fetcher = NewProviderFetcher(newFetcherProviderSet(provider),
		WithProviderClientFactory(func(acquisition.Source) (ProviderClient, error) { return terminal, nil }),
		WithProviderRetryPolicy(policy),
	)
	if _, err := fetcher.FetchModels(context.Background(), &provider); err == nil || terminal.attempts != 1 {
		t.Fatalf("terminal retry = attempts %d err %v", terminal.attempts, err)
	}
}

func TestProviderFetcherFetchModelsRequiresFactory(t *testing.T) {
	provider := providerForFetcherTest("provider-a")
	fetcher := NewProviderFetcher(newFetcherProviderSet(provider), WithProviderClientFactory(nil))

	_, err := fetcher.FetchModels(context.Background(), &provider)
	if err == nil {
		t.Fatal("Expected missing factory to fail")
	}
	var configErr *pkgerrors.ConfigError
	if !stderrors.As(err, &configErr) && !strings.Contains(err.Error(), "provider client factory is not configured") {
		t.Fatalf("Expected config error for missing factory, got %T: %v", err, err)
	}
}

func TestProviderFetcherCredentialPolicyConformsAcrossModelAndRawFetch(t *testing.T) {
	provider := providerForFetcherTest("credential-policy")
	provider.Credentials = map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"STARMAP_FETCHER_CONFORMANCE_KEY"}}}
	provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}}
	t.Setenv("STARMAP_FETCHER_CONFORMANCE_KEY", "")
	clientCalls := 0
	rawCalls := 0
	fetcher := NewProviderFetcher(newFetcherProviderSet(provider),
		WithProviderClientFactory(func(acquisition.Source) (ProviderClient, error) {
			clientCalls++
			return providerFetcherTestClient{}, nil
		}),
		WithProviderRawFetcher(func(context.Context, acquisition.Source) (*RawFetchResult, error) {
			rawCalls++
			return nil, nil
		}),
	)

	providerForModels := provider
	_, modelsErr := fetcher.FetchModels(context.Background(), &providerForModels)
	providerForRaw := provider
	_, _, rawErr := fetcher.FetchRawResponse(context.Background(), &providerForRaw, "models")
	for name, err := range map[string]error{"models": modelsErr, "raw": rawErr} {
		var authenticationErr *pkgerrors.AuthenticationError
		if !stderrors.As(err, &authenticationErr) {
			t.Fatalf("%s error = %T, want *errors.AuthenticationError: %v", name, err, err)
		}
	}
	if clientCalls != 0 || rawCalls != 0 {
		t.Fatalf("credential preflight reached adapters: client=%d raw=%d", clientCalls, rawCalls)
	}
}

func TestProviderFetcherOptionalSourceSkipsOnlyAbsentCredentials(t *testing.T) {
	provider := providerForFetcherTest("optional-source")
	provider.Credentials = map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{
		"api_key": {Env: catalogs.ProviderEnvironmentNames{"STARMAP_OPTIONAL_SOURCE_ABSENT_KEY"}},
	}
	provider.Catalog.Sources[0].Optional = true
	provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}}
	clientCalls := 0
	fetcher := NewProviderFetcher(newFetcherProviderSet(provider), WithProviderClientFactory(func(acquisition.Source) (ProviderClient, error) {
		clientCalls++
		return providerFetcherTestClient{}, nil
	}))

	models, err := fetcher.FetchModels(context.Background(), &provider)
	if err != nil || len(models) != 0 || clientCalls != 0 {
		t.Fatalf("absent optional source = models %#v calls %d err %v", models, clientCalls, err)
	}

	t.Setenv("STARMAP_OPTIONAL_SOURCE_ABSENT_KEY", "")
	if _, err := fetcher.FetchModels(context.Background(), &provider); err == nil {
		t.Fatal("present-empty optional credential must fail closed")
	}
	if clientCalls != 0 {
		t.Fatalf("present-invalid credential reached connector: calls=%d", clientCalls)
	}
}

func TestProviderFetcherFetchRawResponseUsesInjectedFetcher(t *testing.T) {
	provider := providerForFetcherTest("provider-a")
	fetcher := NewProviderFetcher(newFetcherProviderSet(provider),
		WithProviderRawFetcher(func(_ context.Context, source acquisition.Source) (*RawFetchResult, error) {
			return &RawFetchResult{
				Data:       []byte(`{"ok":true}`),
				StatusCode: http.StatusAccepted,
				Header:     http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
				Latency:    12 * time.Millisecond,
			}, nil
		}),
	)

	data, stats, err := fetcher.FetchRawResponse(context.Background(), &provider, "models")
	if err != nil {
		t.Fatalf("FetchRawResponse failed: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("Unexpected raw data: %s", data)
	}
	if stats.StatusCode != http.StatusAccepted {
		t.Fatalf("Expected status %d, got %d", http.StatusAccepted, stats.StatusCode)
	}
	if stats.ContentType != "application/json" {
		t.Fatalf("Expected cleaned content type, got %q", stats.ContentType)
	}
}

func TestProviderFetcherFetchRawResponseReportsNoAuthWhenOptionalKeyMissing(t *testing.T) {
	provider := providerForFetcherTest("optional-auth")
	provider.Credentials = map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"STARMAP_TEST_OPTIONAL_AUTH_KEY_MUST_BE_UNSET"}}}
	provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeOptional}

	fetcher := NewProviderFetcher(newFetcherProviderSet(provider),
		WithProviderRawFetcher(func(_ context.Context, source acquisition.Source) (*RawFetchResult, error) {
			return &RawFetchResult{
				Data:       []byte(`{"ok":true}`),
				StatusCode: http.StatusOK,
				Header:     http.Header{},
			}, nil
		}),
	)

	_, stats, err := fetcher.FetchRawResponse(context.Background(), &provider, "models")
	if err != nil {
		t.Fatalf("FetchRawResponse failed: %v", err)
	}
	if stats.AuthMethod != "None" {
		t.Fatalf("AuthMethod = %q, want None", stats.AuthMethod)
	}
}

func TestProviderFetcherFetchRawResponseReportsAuthWhenOptionalKeyPresent(t *testing.T) {
	provider := providerForFetcherTest("optional-auth")
	provider.Credentials = map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"OPTIONAL_AUTH_API_KEY"}}}
	provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeOptional}
	t.Setenv("OPTIONAL_AUTH_API_KEY", "secret")

	fetcher := NewProviderFetcher(newFetcherProviderSet(provider),
		WithProviderRawFetcher(func(_ context.Context, source acquisition.Source) (*RawFetchResult, error) {
			return &RawFetchResult{
				Data:       []byte(`{"ok":true}`),
				StatusCode: http.StatusOK,
				Header:     http.Header{},
			}, nil
		}),
	)

	_, stats, err := fetcher.FetchRawResponse(context.Background(), &provider, "models")
	if err != nil {
		t.Fatalf("FetchRawResponse failed: %v", err)
	}
	if stats.AuthMethod != "Header" {
		t.Fatalf("AuthMethod = %q, want Header", stats.AuthMethod)
	}
	if stats.AuthLocation != "Authorization" {
		t.Fatalf("AuthLocation = %q, want Authorization", stats.AuthLocation)
	}
	if stats.AuthScheme != "Bearer" {
		t.Fatalf("AuthScheme = %q, want Bearer", stats.AuthScheme)
	}
}

func newFetcherProviderSet(providers ...catalogs.Provider) *catalogs.Providers {
	result := catalogs.NewProviders()
	for i := range providers {
		provider := providers[i]
		_ = result.Add(&provider)
	}
	return result
}

func providerForFetcherTest(id catalogs.ProviderID) catalogs.Provider {
	return catalogs.Provider{
		ID:   id,
		Name: string(id),
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
			Auth: catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}, Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/models"},
		}}},
	}
}
