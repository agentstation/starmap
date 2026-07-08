package providers

import (
	"context"
	stderrors "errors"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type fakeProviderClient struct {
	models []catalogs.Model
	err    error

	onList func()
}

func (c fakeProviderClient) ListModels(context.Context) ([]catalogs.Model, error) {
	if c.onList != nil {
		c.onList()
	}
	return c.models, c.err
}

func (c fakeProviderClient) IsAPIKeyRequired() bool {
	return false
}

func (c fakeProviderClient) HasAPIKey() bool {
	return true
}

func TestSourceFetchAddsFetchedModels(t *testing.T) {
	providerSet := newProviderSet(providerForTest("provider-a"))
	src := New(providerSet, WithClientFactory(func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		return fakeProviderClient{
			models: []catalogs.Model{{ID: "model-a", Name: "Model A"}},
		}, nil
	}))

	if err := src.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	catalog := src.Catalog()
	provider, err := catalog.Provider("provider-a")
	if err != nil {
		t.Fatalf("Expected provider in fetched catalog: %v", err)
	}
	if _, ok := provider.Models["model-a"]; !ok {
		t.Fatalf("Expected fetched model to be associated with provider, got %#v", provider.Models)
	}
}

func TestSourceFetchSkipsProviderWhenCredentialsAreMissing(t *testing.T) {
	provider := providerForTest("missing-key")
	provider.Catalog.Endpoint.AuthRequired = true
	provider.APIKey = &catalogs.ProviderAPIKey{Name: "STARMAP_PROVIDER_TEST_MISSING_KEY"}

	var factoryCalls int
	src := New(newProviderSet(provider), WithClientFactory(func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		factoryCalls++
		return fakeProviderClient{}, nil
	}))

	if err := src.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch should skip missing credentials without failing: %v", err)
	}
	if factoryCalls != 0 {
		t.Fatalf("Expected missing credentials to skip client creation, got %d factory calls", factoryCalls)
	}

	fetchedProvider, err := src.Catalog().Provider("missing-key")
	if err != nil {
		t.Fatalf("Expected skipped provider config to remain in catalog: %v", err)
	}
	if len(fetchedProvider.Models) != 0 {
		t.Fatalf("Expected skipped provider to have no models, got %#v", fetchedProvider.Models)
	}
}

func TestSourceFetchSkipsConfigurationErrors(t *testing.T) {
	src := New(newProviderSet(providerForTest("bad-config")), WithClientFactory(func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		return nil, &pkgerrors.ConfigError{
			Component: string(provider.ID),
			Message:   "misconfigured test provider",
		}
	}))

	if err := src.Fetch(context.Background()); err != nil {
		t.Fatalf("Expected configuration errors to be skipped, got %v", err)
	}

	fetchedProvider, err := src.Catalog().Provider("bad-config")
	if err != nil {
		t.Fatalf("Expected skipped provider config to remain in catalog: %v", err)
	}
	if len(fetchedProvider.Models) != 0 {
		t.Fatalf("Expected config-error provider to have no models, got %#v", fetchedProvider.Models)
	}
}

func TestSourceFetchReturnsPartialFailuresAndKeepsSuccessfulModels(t *testing.T) {
	fetchErr := stderrors.New("provider api failed")
	src := New(newProviderSet(
		providerForTest("success"),
		providerForTest("failure"),
	), WithClientFactory(func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		if provider.ID == "failure" {
			return fakeProviderClient{err: fetchErr}, nil
		}
		return fakeProviderClient{
			models: []catalogs.Model{{ID: "successful-model", Name: "Successful Model"}},
		}, nil
	}))

	err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("Expected partial provider failure to return an error")
	}
	if !stderrors.Is(err, fetchErr) {
		t.Fatalf("Expected joined error to include provider error, got %v", err)
	}

	successProvider, providerErr := src.Catalog().Provider("success")
	if providerErr != nil {
		t.Fatalf("Expected successful provider in catalog: %v", providerErr)
	}
	if _, ok := successProvider.Models["successful-model"]; !ok {
		t.Fatalf("Expected successful model to remain after partial failure, got %#v", successProvider.Models)
	}
}

func TestSourceFetchBoundsProviderConcurrency(t *testing.T) {
	const maxConcurrency = 2

	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var mu sync.Mutex
	var inFlight int
	var observedMax int

	src := New(newProviderSet(
		providerForTest("provider-a"),
		providerForTest("provider-b"),
		providerForTest("provider-c"),
		providerForTest("provider-d"),
	), WithMaxConcurrency(maxConcurrency), WithClientFactory(func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		return fakeProviderClient{
			models: []catalogs.Model{{ID: string(provider.ID) + "-model"}},
			onList: func() {
				mu.Lock()
				inFlight++
				if inFlight > observedMax {
					observedMax = inFlight
				}
				mu.Unlock()

				started <- struct{}{}
				<-release

				mu.Lock()
				inFlight--
				mu.Unlock()
			},
		}, nil
	}))

	done := make(chan error, 1)
	go func() {
		done <- src.Fetch(context.Background())
	}()

	for i := 0; i < maxConcurrency; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("Timed out waiting for provider %d to start", i+1)
		}
	}

	select {
	case <-started:
		t.Fatal("Observed more provider fetches than the configured concurrency limit")
	case <-time.After(25 * time.Millisecond):
	}

	close(release)

	if err := <-done; err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if observedMax > maxConcurrency {
		t.Fatalf("Expected max concurrency <= %d, got %d", maxConcurrency, observedMax)
	}
}

func newProviderSet(providers ...catalogs.Provider) *catalogs.Providers {
	result := catalogs.NewProviders()
	for i := range providers {
		provider := providers[i]
		_ = result.Add(&provider)
	}
	return result
}

func providerForTest(id catalogs.ProviderID) catalogs.Provider {
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
