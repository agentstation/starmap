package sources

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

type providerFetcherTestClient struct {
	models []catalogs.Model
	err    error
}

func (c providerFetcherTestClient) ListModels(
	context.Context,
	ProviderCredentialMaterial,
) ([]catalogs.Model, error) {
	return c.models, c.err
}

func TestProviderFetcherHasClientUsesInjectedFactory(t *testing.T) {
	fetcher := NewProviderFetcher(
		newFetcherProviderSet(providerForFetcherTest("supported")),
		WithProviderClientFactory(func(provider *catalogs.Provider) (ProviderClient, error) {
			if provider.ID == "supported" {
				return providerFetcherTestClient{}, nil
			}
			return nil, &pkgerrors.ConfigError{Component: string(provider.ID), Message: "unsupported"}
		}),
	)

	if !fetcher.HasClient("supported") {
		t.Fatal("injected factory did not report a supported provider")
	}
	if fetcher.HasClient("missing") {
		t.Fatal("missing provider was reported as supported")
	}
}

func TestProviderFetcherHasNoImplicitProviderHooks(t *testing.T) {
	fetcher := NewProviderFetcher(newFetcherProviderSet(providerForFetcherTest("supported")))
	if fetcher.options.clientFactory != nil || fetcher.options.rawFetcher != nil ||
		fetcher.options.credentialResolver != nil {
		t.Fatal("provider fetcher installed an implicit acquisition hook")
	}
	if fetcher.HasClient("supported") {
		t.Fatal("provider support did not require explicit composition")
	}
}

func TestProviderFetcherFetchModelsUsesInjectedRoles(t *testing.T) {
	provider := providerForFetcherTest("provider-a")
	fetcher := NewProviderFetcher(
		newFetcherProviderSet(provider),
		WithProviderCredentialResolver(staticFetcherResolver(noAuthFetcherMaterial(), nil)),
		WithProviderClientFactory(func(*catalogs.Provider) (ProviderClient, error) {
			return providerFetcherTestClient{
				models: []catalogs.Model{{ID: "model-a", Name: "Model A"}},
			}, nil
		}),
	)

	models, err := fetcher.FetchModels(context.Background(), &provider)
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 1 || models[0].ID != "model-a" {
		t.Fatalf("models = %#v", models)
	}
}

func TestProviderFetcherPreservesPartialModelsWithQuarantineError(t *testing.T) {
	provider := providerForFetcherTest("provider-a")
	quarantine := &sourcepayload.QuarantineError{
		Collection: "models",
		Report: sourcepayload.RecordReport{
			Accepted: 1, Rejected: 1,
			Issues: []sourcepayload.RecordIssue{{
				Subject: "data[1]",
				Err:     pkgerrors.NewParseError("json", "data[1]", "schema drift", nil),
			}},
		},
	}
	fetcher := NewProviderFetcher(
		newFetcherProviderSet(provider),
		WithProviderCredentialResolver(staticFetcherResolver(noAuthFetcherMaterial(), nil)),
		WithProviderClientFactory(func(*catalogs.Provider) (ProviderClient, error) {
			return providerFetcherTestClient{
				models: []catalogs.Model{{ID: "valid", Name: "Valid"}}, err: quarantine,
			}, nil
		}),
	)

	models, err := fetcher.FetchModels(context.Background(), &provider)
	var gotQuarantine *sourcepayload.QuarantineError
	if !stderrors.As(err, &gotQuarantine) {
		t.Fatalf("error = %T: %v, want quarantine error", err, err)
	}
	if len(models) != 1 || models[0].ID != "valid" {
		t.Fatalf("models = %#v", models)
	}
}

func TestProviderFetcherFetchModelsRequiresFactory(t *testing.T) {
	provider := providerForFetcherTest("provider-a")
	fetcher := NewProviderFetcher(
		newFetcherProviderSet(provider),
		WithProviderCredentialResolver(staticFetcherResolver(noAuthFetcherMaterial(), nil)),
	)

	_, err := fetcher.FetchModels(context.Background(), &provider)
	if err == nil || !strings.Contains(err.Error(), "provider client factory is not configured") {
		t.Fatalf("error = %T: %v", err, err)
	}
}

func TestProviderFetcherCredentialPolicyConformsAcrossModelAndRawFetch(t *testing.T) {
	provider := providerForFetcherTest("credential-policy")
	credentialErr := &pkgerrors.AuthenticationError{
		Provider: string(provider.ID), Method: "api-key", Message: "not configured",
	}
	clientCalls := 0
	rawCalls := 0
	fetcher := NewProviderFetcher(
		newFetcherProviderSet(provider),
		WithProviderCredentialResolver(staticFetcherResolver(ProviderCredentialMaterial{}, credentialErr)),
		WithProviderClientFactory(func(*catalogs.Provider) (ProviderClient, error) {
			clientCalls++
			return providerFetcherTestClient{}, nil
		}),
		WithProviderRawFetcher(func(
			context.Context,
			*catalogs.Provider,
			ProviderCredentialMaterial,
			string,
		) (*RawFetchResult, error) {
			rawCalls++
			return nil, nil
		}),
	)

	_, modelsErr := fetcher.FetchModels(context.Background(), &provider)
	_, _, rawErr := fetcher.FetchRawResponse(
		context.Background(),
		&provider,
		"https://example.test/raw",
	)
	for name, err := range map[string]error{"models": modelsErr, "raw": rawErr} {
		var authenticationErr *pkgerrors.AuthenticationError
		if !stderrors.As(err, &authenticationErr) {
			t.Fatalf("%s error = %T, want AuthenticationError: %v", name, err, err)
		}
	}
	if clientCalls != 1 || rawCalls != 0 {
		t.Fatalf("credential failure calls: client validation=%d raw transport=%d", clientCalls, rawCalls)
	}
}

func TestProviderFetcherRejectsInvalidAdapterConfigurationBeforeCredentialResolution(t *testing.T) {
	provider := providerForFetcherTest("invalid-adapter-config")
	resolverCalls := 0
	fetcher := NewProviderFetcher(
		newFetcherProviderSet(provider),
		WithProviderCredentialResolver(ProviderCredentialResolverFunc(func(
			context.Context,
			*catalogs.Provider,
		) (ProviderCredentialMaterial, error) {
			resolverCalls++
			return noAuthFetcherMaterial(), nil
		})),
		WithProviderClientFactory(func(*catalogs.Provider) (ProviderClient, error) {
			return nil, &pkgerrors.ValidationError{
				Field: "field_mappings.from", Value: "unsupported", Message: "unsupported mapping",
			}
		}),
	)

	_, err := fetcher.FetchModels(context.Background(), &provider)
	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error = %T: %v, want ValidationError", err, err)
	}
	if resolverCalls != 0 {
		t.Fatalf("credential resolver calls = %d, want 0", resolverCalls)
	}
}

func TestProviderFetcherFetchRawResponseUsesInjectedRoles(t *testing.T) {
	provider := providerForFetcherTest("provider-a")
	fetcher := rawTestFetcher(provider, noAuthFetcherMaterial())

	data, stats, err := fetcher.FetchRawResponse(
		context.Background(),
		&provider,
		"https://example.test/raw",
	)
	if err != nil {
		t.Fatalf("FetchRawResponse: %v", err)
	}
	if string(data) != `{"ok":true}` || stats.StatusCode != http.StatusAccepted ||
		stats.ContentType != "application/json" || stats.URL != "https://example.test/raw" {
		t.Fatalf("data = %s, stats = %#v", data, stats)
	}
	if stats.AuthMethod != "None" {
		t.Fatalf("AuthMethod = %q, want None", stats.AuthMethod)
	}
}

func TestProviderFetcherFetchRawResponseReportsCatalogPlacement(t *testing.T) {
	provider := providerForFetcherTest("provider-a")
	fetcher := rawTestFetcher(provider, apiKeyFetcherMaterial("secret"))

	_, stats, err := fetcher.FetchRawResponse(
		context.Background(),
		&provider,
		"https://example.test/raw",
	)
	if err != nil {
		t.Fatalf("FetchRawResponse: %v", err)
	}
	if stats.AuthMethod != "Header" || stats.AuthLocation != "Authorization" ||
		stats.AuthScheme != "Bearer" {
		t.Fatalf("auth stats = %#v", stats)
	}
}

func rawTestFetcher(
	provider catalogs.Provider,
	material ProviderCredentialMaterial,
) *ProviderFetcher {
	return NewProviderFetcher(
		newFetcherProviderSet(provider),
		WithProviderCredentialResolver(staticFetcherResolver(material, nil)),
		WithProviderRawFetcher(func(
			_ context.Context,
			_ *catalogs.Provider,
			_ ProviderCredentialMaterial,
			endpoint string,
		) (*RawFetchResult, error) {
			return &RawFetchResult{
				Data: []byte(`{"ok":true}`),
				Response: &http.Response{
					StatusCode: http.StatusAccepted,
					Header: http.Header{
						"Content-Type": []string{"application/json; charset=utf-8"},
					},
				},
				Latency: 12 * time.Millisecond, RequestURL: endpoint,
			}, nil
		}),
	)
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
		ID: id, Name: string(id),
		Credentials: &catalogs.ProviderCredentials{
			Profiles: []catalogs.ProviderCredentialProfile{{
				ID: "unauthenticated", Primitive: catalogs.ProviderAuthenticationNone,
			}},
			CatalogAcquisition: catalogs.ProviderCredentialPlane{
				Alternatives: []catalogs.ProviderCredentialProfileID{"unauthenticated"},
			},
		},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type: catalogs.EndpointTypeOpenAI,
			URL:  "https://example.test/models",
			ProtocolOptions: catalogs.ProviderCatalogProtocolOptions{
				OpenAI: &catalogs.ProviderOpenAICatalogProtocolOptions{
					TokenPriceUnit: catalogs.ProviderTokenPriceUnitPerMillion,
				},
			},
		}},
	}
}

func staticFetcherResolver(
	material ProviderCredentialMaterial,
	err error,
) ProviderCredentialResolver {
	return ProviderCredentialResolverFunc(func(
		context.Context,
		*catalogs.Provider,
	) (ProviderCredentialMaterial, error) {
		return material, err
	})
}

func noAuthFetcherMaterial() ProviderCredentialMaterial {
	return NewProviderCredentialMaterial(catalogs.ProviderCredentialProfile{
		ID: "unauthenticated", Primitive: catalogs.ProviderAuthenticationNone,
	}, nil, ProviderCredentialMetadata{Version: "test"})
}

func apiKeyFetcherMaterial(value string) ProviderCredentialMaterial {
	profile := catalogs.ProviderCredentialProfile{
		ID: "api-key", Primitive: catalogs.ProviderAuthenticationAPIKey,
		Fields: []catalogs.ProviderCredentialFieldID{"api-key"},
		Placements: []catalogs.ProviderCredentialPlacement{{
			Field: "api-key", Kind: catalogs.ProviderCredentialPlacementHeader,
			Name: "Authorization", Scheme: catalogs.ProviderCredentialSchemeBearer,
		}},
	}
	return NewProviderCredentialMaterial(
		profile,
		map[catalogs.ProviderCredentialFieldID]string{"api-key": value},
		ProviderCredentialMetadata{Version: "test"},
	)
}
