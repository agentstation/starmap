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
)

type providerFetcherTestClient struct {
	models []catalogs.Model
	err    error
}

func (c providerFetcherTestClient) ListModels(context.Context) ([]catalogs.Model, error) {
	return c.models, c.err
}

func (c providerFetcherTestClient) IsAPIKeyRequired() bool {
	return false
}

func (c providerFetcherTestClient) HasAPIKey() bool {
	return true
}

func TestProviderFetcherHasClientUsesInjectedFactory(t *testing.T) {
	fetcher := NewProviderFetcher(newFetcherProviderSet(providerForFetcherTest("supported")),
		WithProviderClientFactory(func(provider *catalogs.Provider) (ProviderClient, error) {
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
		WithProviderClientFactory(func(provider *catalogs.Provider) (ProviderClient, error) {
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

func TestProviderFetcherFetchRawResponseUsesInjectedFetcher(t *testing.T) {
	provider := providerForFetcherTest("provider-a")
	fetcher := NewProviderFetcher(newFetcherProviderSet(provider),
		WithProviderRawFetcher(func(_ context.Context, _ *catalogs.Provider, endpoint string) (*RawFetchResult, error) {
			return &RawFetchResult{
				Data:       []byte(`{"ok":true}`),
				Response:   &http.Response{StatusCode: http.StatusAccepted, Header: http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}},
				Latency:    12 * time.Millisecond,
				RequestURL: endpoint,
			}, nil
		}),
	)

	data, stats, err := fetcher.FetchRawResponse(context.Background(), &provider, "https://example.test/raw")
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
	if stats.URL != "https://example.test/raw" {
		t.Fatalf("Expected request URL in stats, got %q", stats.URL)
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
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type:         catalogs.EndpointTypeOpenAI,
				URL:          "https://example.test/models",
				AuthRequired: false,
			},
		},
	}
}
