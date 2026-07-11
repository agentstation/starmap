package providers

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderSourceDelegatesFetchPolicyToPublicFetcher(t *testing.T) {
	typeOfSource := reflect.TypeOf(Source{})
	if _, found := typeOfSource.FieldByName("clientFactory"); found {
		t.Fatal("provider Source still owns a duplicate client factory/fetch policy")
	}
	field, found := typeOfSource.FieldByName("fetcher")
	if !found || field.Type != reflect.TypeOf((*sources.ProviderFetcher)(nil)) {
		t.Fatalf("provider Source fetcher field = %#v", field)
	}
}

func TestPublicInternalProviderFetchConformance(t *testing.T) {
	fetchErr := stderrors.New("upstream failed")
	tests := []struct {
		name     string
		provider catalogs.Provider
		factory  ClientFactory
		wantCode sources.ObservationIssueCode
	}{
		{
			name: "missing credentials",
			provider: func() catalogs.Provider {
				provider := providerForTest("missing")
				provider.Catalog.Endpoint.AuthRequired = true
				provider.APIKey = &catalogs.ProviderAPIKey{Name: "STARMAP_CONFORMANCE_MISSING_KEY"}
				return provider
			}(),
			factory: func(*catalogs.Provider) (sources.ProviderClient, error) {
				return fakeProviderClient{}, nil
			},
			wantCode: sources.ObservationIssueCodeMissingCredentials,
		},
		{
			name:     "configuration",
			provider: providerForTest("configuration"),
			factory: func(provider *catalogs.Provider) (sources.ProviderClient, error) {
				return nil, &pkgerrors.ConfigError{Component: string(provider.ID), Message: "invalid configuration"}
			},
			wantCode: sources.ObservationIssueCodeConfiguration,
		},
		{
			name:     "fetch failure",
			provider: providerForTest("fetch"),
			factory: func(*catalogs.Provider) (sources.ProviderClient, error) {
				return fakeProviderClient{err: fetchErr}, nil
			},
			wantCode: sources.ObservationIssueCodeFetchFailed,
		},
		{
			name:     "schema drift",
			provider: providerForTest("schema-drift"),
			factory: func(*catalogs.Provider) (sources.ProviderClient, error) {
				return fakeProviderClient{err: pkgerrors.NewParseError("json", "models response", "models changed from array to object", nil)}, nil
			},
			wantCode: sources.ObservationIssueCodeSchemaDrift,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			providerSet := newProviderSet(test.provider)
			publicFetcher := sources.NewProviderFetcher(providerSet, sources.WithProviderClientFactory(test.factory))
			providerCopy := test.provider
			if _, err := publicFetcher.FetchModels(context.Background(), &providerCopy); err == nil {
				t.Fatal("public fetch unexpectedly succeeded")
			}

			observation, err := New(providerSet, WithClientFactory(test.factory)).Observe(context.Background())
			if err != nil {
				t.Fatalf("internal observation returned source error: %v", err)
			}
			if len(observation.Issues) != 1 || observation.Issues[0].Code != test.wantCode {
				t.Fatalf("internal issues = %#v, want code %q", observation.Issues, test.wantCode)
			}
		})
	}
}

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

func TestSourceObserveAddsFetchedModels(t *testing.T) {
	providerSet := newProviderSet(providerForTest("provider-a"))
	src := New(providerSet, WithClientFactory(func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		return fakeProviderClient{
			models: []catalogs.Model{{ID: "model-a", Name: "Model A"}},
		}, nil
	}))

	observation, err := src.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("Validate observation: %v", err)
	}

	catalog := observation.Catalog
	if _, ok := any(catalog).(*catalogs.Builder); ok {
		t.Fatal("Source published a mutable builder instead of a snapshot")
	}
	provider, err := catalog.Provider("provider-a")
	if err != nil {
		t.Fatalf("Expected provider in fetched catalog: %v", err)
	}
	if _, ok := provider.Models["model-a"]; !ok {
		t.Fatalf("Expected fetched model to be associated with provider, got %#v", provider.Models)
	}
}

func TestInvalidIdentityQuarantineMalformedProviderRecordsWithCounts(t *testing.T) {
	providerSet := newProviderSet(providerForTest("provider-a"))
	src := New(providerSet, WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
		return fakeProviderClient{models: []catalogs.Model{
			{ID: "valid-a", Name: "Valid A"},
			{ID: "", Name: "Missing ID"},
			{ID: "valid-a", Name: "Duplicate"},
			{ID: " whitespace-id ", Name: "Whitespace ID"},
			{ID: "control\n-id", Name: "Control ID"},
			{ID: "missing-name", Name: ""},
			{ID: "control-name", Name: "Control\nName"},
			{ID: "valid-b", Name: "Valid B"},
		}}, nil
	}))

	observation, err := src.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("Validate observation: %v", err)
	}
	if observation.Status != sources.ObservationStatusDegraded || observation.Completeness != sources.ObservationCompletenessPartial {
		t.Fatalf("observation state = (%q, %q), want degraded partial", observation.Status, observation.Completeness)
	}
	if len(observation.Issues) != 6 {
		t.Fatalf("issues = %#v, want six quarantined records", observation.Issues)
	}
	for _, issue := range observation.Issues {
		if issue.Scope != sources.ObservationIssueScopeRecord || issue.Code != sources.ObservationIssueCodeInvalidRecord || issue.Subject == "" {
			t.Fatalf("unclassified quarantine issue: %#v", issue)
		}
	}
	provider, err := observation.Catalog.Provider("provider-a")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if len(provider.Models) != 2 || provider.Models["valid-a"] == nil || provider.Models["valid-b"] == nil {
		t.Fatalf("accepted models = %#v, want valid-a and valid-b", provider.Models)
	}
	if observation.Records.Accepted != 2 || observation.Records.Rejected != 6 {
		t.Fatalf("record counts = %#v, want accepted=2 rejected=6", observation.Records)
	}
}

func TestSchemaDriftProviderFailurePreservesValidProvider(t *testing.T) {
	providerSet := newProviderSet(providerForTest("drifted"), providerForTest("valid"))
	src := New(providerSet, WithClientFactory(func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		if provider.ID == "drifted" {
			return fakeProviderClient{err: pkgerrors.NewParseError("json", "models response", "models changed from array to object", nil)}, nil
		}
		return fakeProviderClient{models: []catalogs.Model{{ID: "valid-model", Name: "Valid Model"}}}, nil
	}))

	observation, err := src.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("Validate observation: %v", err)
	}
	if observation.Status != sources.ObservationStatusDegraded || observation.Completeness != sources.ObservationCompletenessPartial {
		t.Fatalf("observation state = (%q, %q), want degraded partial", observation.Status, observation.Completeness)
	}
	if len(observation.Issues) != 1 || observation.Issues[0].Code != sources.ObservationIssueCodeSchemaDrift || observation.Issues[0].Subject != "drifted" {
		t.Fatalf("schema drift issues = %#v", observation.Issues)
	}
	provider, err := observation.Catalog.Provider("valid")
	if err != nil {
		t.Fatalf("valid Provider: %v", err)
	}
	if provider.Models["valid-model"] == nil {
		t.Fatalf("valid provider model was discarded: %#v", provider.Models)
	}
}

func TestPayloadLimitProviderModelCount(t *testing.T) {
	models := make([]catalogs.Model, constants.MaxCatalogModels+1)
	for index := range models {
		models[index] = catalogs.Model{ID: fmt.Sprintf("model-%05d", index), Name: "Model"}
	}
	accepted, rejected, issues := quarantineProviderModels("provider", models)
	if len(accepted) != constants.MaxCatalogModels {
		t.Fatalf("accepted = %d, want %d", len(accepted), constants.MaxCatalogModels)
	}
	if rejected != 1 {
		t.Fatalf("rejected = %d, want 1", rejected)
	}
	if len(issues) != 1 || issues[0].Code != sources.ObservationIssueCodePayloadLimit {
		t.Fatalf("issues = %#v, want payload limit", issues)
	}
}

func TestSourceObserveEmitsStructuredSourceProviderAndRunFields(t *testing.T) {
	providerSet := newProviderSet(providerForTest("provider-a"))
	src := New(providerSet, WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
		return fakeProviderClient{models: []catalogs.Model{{ID: "model-a"}}}, nil
	}))
	testLogger := logging.NewTestLogger(t)
	ctx := logging.WithLogger(context.Background(), testLogger.Logger)
	ctx = logging.WithRunID(ctx, "provider-run-123")
	if _, err := src.Observe(ctx); err != nil {
		t.Fatalf("Observe: %v", err)
	}

	foundProviderEvent := false
	for _, line := range testLogger.Lines() {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("structured log is not JSON: %v: %s", err, line)
		}
		if event["provider_id"] != "provider-a" {
			continue
		}
		foundProviderEvent = true
		if event["source"] != "providers" || event["run_id"] != "provider-run-123" {
			t.Fatalf("provider event fields = %#v", event)
		}
	}
	if !foundProviderEvent {
		t.Fatalf("no provider-scoped event in %s", testLogger.Output())
	}
}

func TestSourceObserveSeparatesBootstrapModelsWhenCredentialsAreMissing(t *testing.T) {
	provider := providerForTest("missing-key")
	provider.Catalog.Endpoint.AuthRequired = true
	provider.APIKey = &catalogs.ProviderAPIKey{Name: "STARMAP_PROVIDER_TEST_MISSING_KEY"}
	provider.Models = map[string]*catalogs.Model{
		"bootstrap-model": {ID: "bootstrap-model", Name: "Embedded Bootstrap Model"},
	}

	var factoryCalls int
	src := New(newProviderSet(provider), WithClientFactory(func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		factoryCalls++
		return fakeProviderClient{}, nil
	}))

	observation, err := src.Observe(context.Background())
	if err != nil {
		t.Fatalf("Fetch should skip missing credentials without failing: %v", err)
	}
	if factoryCalls != 0 {
		t.Fatalf("Expected missing credentials to skip client creation, got %d factory calls", factoryCalls)
	}

	fetchedProvider, err := observation.Catalog.Provider("missing-key")
	if err != nil {
		t.Fatalf("Expected skipped provider config to remain in catalog: %v", err)
	}
	if len(fetchedProvider.Models) != 0 {
		t.Fatalf("bootstrap models masqueraded as a live provider observation: %#v", fetchedProvider.Models)
	}
	if _, found := provider.Models["bootstrap-model"]; !found {
		t.Fatal("provider source mutated its bootstrap input while separating live evidence")
	}
	if observation.Completeness != sources.ObservationCompletenessPartial || observation.Status != sources.ObservationStatusDegraded ||
		len(observation.Issues) != 1 || observation.Issues[0].Code != sources.ObservationIssueCodeMissingCredentials {
		t.Fatalf("Missing-credential observation = %#v", observation)
	}
}

func TestSourceObserveSuccessfulFetchReplacesBootstrapModelsWithLiveModels(t *testing.T) {
	provider := providerForTest("provider-a")
	provider.Models = map[string]*catalogs.Model{
		"bootstrap-model": {ID: "bootstrap-model", Name: "Embedded Bootstrap Model"},
	}
	src := New(newProviderSet(provider), WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
		return fakeProviderClient{models: []catalogs.Model{{ID: "live-model", Name: "Live Model"}}}, nil
	}))

	observation, err := src.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	fetchedProvider, err := observation.Catalog.Provider("provider-a")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if _, found := fetchedProvider.Models["bootstrap-model"]; found {
		t.Fatalf("bootstrap model remained in successful live observation: %#v", fetchedProvider.Models)
	}
	if _, found := fetchedProvider.Models["live-model"]; !found {
		t.Fatalf("live model missing from successful observation: %#v", fetchedProvider.Models)
	}
}

func TestSourceObserveDoesNotSkipProviderWithMissingOptionalEnvVars(t *testing.T) {
	provider := providerForTest("optional-env")
	provider.EnvVars = []catalogs.ProviderEnvVar{{
		Name:     "STARMAP_PROVIDER_TEST_OPTIONAL_ENV",
		Required: false,
	}}

	var factoryCalls int
	src := New(newProviderSet(provider), WithClientFactory(func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		factoryCalls++
		return fakeProviderClient{
			models: []catalogs.Model{{ID: "model-a", Name: "Model A"}},
		}, nil
	}))

	observation, err := src.Observe(context.Background())
	if err != nil {
		t.Fatalf("Fetch should not skip missing optional env vars: %v", err)
	}
	if factoryCalls != 1 {
		t.Fatalf("Expected optional env provider to be fetched once, got %d factory calls", factoryCalls)
	}

	fetchedProvider, err := observation.Catalog.Provider("optional-env")
	if err != nil {
		t.Fatalf("Expected fetched provider config to remain in catalog: %v", err)
	}
	if _, ok := fetchedProvider.Models["model-a"]; !ok {
		t.Fatalf("Expected optional-env provider model to be fetched, got %#v", fetchedProvider.Models)
	}
}

func TestSourceObserveSkipsConfigurationErrors(t *testing.T) {
	src := New(newProviderSet(providerForTest("bad-config")), WithClientFactory(func(provider *catalogs.Provider) (sources.ProviderClient, error) {
		return nil, &pkgerrors.ConfigError{
			Component: string(provider.ID),
			Message:   "misconfigured test provider",
		}
	}))

	observation, err := src.Observe(context.Background())
	if err != nil {
		t.Fatalf("Expected configuration errors to be skipped, got %v", err)
	}

	fetchedProvider, err := observation.Catalog.Provider("bad-config")
	if err != nil {
		t.Fatalf("Expected skipped provider config to remain in catalog: %v", err)
	}
	if len(fetchedProvider.Models) != 0 {
		t.Fatalf("Expected config-error provider to have no models, got %#v", fetchedProvider.Models)
	}
	if observation.Completeness != sources.ObservationCompletenessPartial || len(observation.Issues) != 1 ||
		observation.Issues[0].Code != sources.ObservationIssueCodeConfiguration {
		t.Fatalf("Configuration observation = %#v", observation)
	}
}

func TestSourceObserveReturnsPartialFailuresAndKeepsSuccessfulModels(t *testing.T) {
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

	observation, err := src.Observe(context.Background())
	if err != nil {
		t.Fatalf("Partial provider observation returned a source-level error: %v", err)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("Validate partial observation: %v", err)
	}
	if observation.Completeness != sources.ObservationCompletenessPartial || observation.Status != sources.ObservationStatusDegraded {
		t.Fatalf("Partial state = (%q, %q)", observation.Completeness, observation.Status)
	}
	if len(observation.Issues) != 1 || observation.Issues[0].Scope != sources.ObservationIssueScopeProvider ||
		observation.Issues[0].Subject != "failure" || observation.Issues[0].Code != sources.ObservationIssueCodeFetchFailed {
		t.Fatalf("Partial issues = %#v", observation.Issues)
	}

	successProvider, providerErr := observation.Catalog.Provider("success")
	if providerErr != nil {
		t.Fatalf("Expected successful provider in catalog: %v", providerErr)
	}
	if _, ok := successProvider.Models["successful-model"]; !ok {
		t.Fatalf("Expected successful model to remain after partial failure, got %#v", successProvider.Models)
	}
}

func TestSourceObserveBoundsProviderConcurrency(t *testing.T) {
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
		_, err := src.Observe(context.Background())
		done <- err
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

func TestSourceObserveIsConcurrentAndReturnsIndependentCatalogs(t *testing.T) {
	src := New(newProviderSet(providerForTest("provider-a")), WithClientFactory(func(*catalogs.Provider) (sources.ProviderClient, error) {
		return fakeProviderClient{
			models: []catalogs.Model{{ID: "model-a", Name: "Model A"}},
		}, nil
	}))

	const calls = 16
	observations := make([]sources.Observation, calls)
	errs := make([]error, calls)
	var wg sync.WaitGroup
	for i := range calls {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			observations[index], errs[index] = src.Observe(context.Background())
		}(i)
	}
	wg.Wait()

	seenCatalogs := make(map[*catalogs.Catalog]struct{}, calls)
	for i := range calls {
		if errs[i] != nil {
			t.Fatalf("Observe %d: %v", i, errs[i])
		}
		observation := observations[i]
		if observation.SourceID != sources.ProvidersID || observation.Catalog == nil {
			t.Fatalf("Observe %d = %#v", i, observation)
		}
		if err := observation.Validate(); err != nil {
			t.Fatalf("Observe %d validation: %v", i, err)
		}
		if _, duplicate := seenCatalogs[observation.Catalog]; duplicate {
			t.Fatalf("Observe %d reused a result catalog from another call", i)
		}
		seenCatalogs[observation.Catalog] = struct{}{}
		provider, err := observation.Catalog.Provider("provider-a")
		if err != nil {
			t.Fatalf("Observe %d provider: %v", i, err)
		}
		if _, ok := provider.Models["model-a"]; !ok {
			t.Fatalf("Observe %d missing model-a", i)
		}
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
