package providers

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type fakeProviderClient struct {
	models []catalogs.Model
	err    error
}

func (client fakeProviderClient) ListModels(context.Context) ([]catalogs.Model, error) {
	return client.models, client.err
}

type isolatedEnvironment map[string]string

func (environment isolatedEnvironment) LookupEnv(name string) (string, bool) {
	value, found := environment[name]
	return value, found
}

func TestConfiguredSourceClassifiesFailuresWithoutInventingInventory(t *testing.T) {
	for _, test := range []struct {
		name     string
		provider catalogs.Provider
		factory  ClientFactory
		wantCode sources.ObservationIssueCode
	}{
		{
			name: "missing credentials",
			provider: func() catalogs.Provider {
				provider := providerForTest("missing")
				provider.Credentials = map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{
					"api_key": {Env: catalogs.ProviderEnvironmentNames{"STARMAP_CONFORMANCE_MISSING_KEY"}},
				}
				provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}}
				return provider
			}(),
			factory:  func(acquisition.Source) (sources.ProviderClient, error) { return fakeProviderClient{}, nil },
			wantCode: sources.ObservationIssueCodeMissingCredentials,
		},
		{
			name:     "configuration",
			provider: providerForTest("configuration"),
			factory: func(source acquisition.Source) (sources.ProviderClient, error) {
				return nil, &pkgerrors.ConfigError{Component: string(source.ProviderID()), Message: "invalid configuration"}
			},
			wantCode: sources.ObservationIssueCodeConfiguration,
		},
		{
			name:     "fetch failure",
			provider: providerForTest("fetch"),
			factory: func(acquisition.Source) (sources.ProviderClient, error) {
				return fakeProviderClient{err: stderrors.New("upstream failed")}, nil
			},
			wantCode: sources.ObservationIssueCodeFetchFailed,
		},
		{
			name:     "schema drift",
			provider: providerForTest("schema-drift"),
			factory: func(acquisition.Source) (sources.ProviderClient, error) {
				return fakeProviderClient{err: pkgerrors.NewParseError("json", "models response", "models changed from array to object", nil)}, nil
			},
			wantCode: sources.ObservationIssueCodeSchemaDrift,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			configured, err := NewConfigured(newProviderSet(test.provider), WithClientFactory(test.factory))
			if err != nil || len(configured) != 1 {
				t.Fatalf("NewConfigured = %d/%v", len(configured), err)
			}
			observation, err := configured[0].Observe(context.Background())
			if err != nil {
				t.Fatalf("Observe: %v", err)
			}
			if observation.Status != sources.ObservationStatusDegraded || observation.Completeness != sources.ObservationCompletenessPartial ||
				len(observation.Issues) != 1 || observation.Issues[0].Code != test.wantCode {
				t.Fatalf("observation = %#v, want issue %q", observation, test.wantCode)
			}
			provider, err := observation.Catalog.Provider(test.provider.ID)
			if err != nil || len(provider.Models) != 0 {
				t.Fatalf("failed source invented inventory: provider=%#v err=%v", provider, err)
			}
		})
	}
}

func TestConfiguredLogicalSourcesPreserveConcurrentScopeAndResultIsolation(t *testing.T) {
	provider := providerForTest("provider-a")
	provider.Credentials = map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{
		"api_key": {Env: catalogs.ProviderEnvironmentNames{"STARMAP_CONTEXT_ISOLATION_KEY"}},
	}
	provider.Catalog.Sources = []catalogs.ProviderSource{
		{
			ID: "public", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
			Auth:     catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/public"},
		},
		{
			ID: "account", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeCredentialScoped},
			Auth:     catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/account"},
		},
	}
	t.Setenv("STARMAP_CONTEXT_ISOLATION_KEY", "account-secret")
	configured, err := NewConfigured(newProviderSet(provider), WithClientFactory(func(source acquisition.Source) (sources.ProviderClient, error) {
		return fakeProviderClient{models: []catalogs.Model{{
			ID: source.SourceID() + "-model", Name: source.SourceID() + " model",
			Metadata: &catalogs.ModelMetadata{Tags: []catalogs.ModelTag{catalogs.ModelTagChat}},
		}}}, nil
	}))
	if err != nil || len(configured) != 2 {
		t.Fatalf("NewConfigured = %d/%v", len(configured), err)
	}

	observations := make([]sources.Observation, len(configured))
	errorsBySource := make([]error, len(configured))
	var wait sync.WaitGroup
	for index := range configured {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			observations[index], errorsBySource[index] = configured[index].Observe(context.Background())
		}(index)
	}
	wait.Wait()

	bySource := make(map[string]sources.Observation, len(observations))
	for index, observation := range observations {
		if errorsBySource[index] != nil {
			t.Fatalf("Observe: %v", errorsBySource[index])
		}
		if err := observation.Validate(); err != nil || len(observation.Metrics.Acquisitions) != 1 {
			t.Fatalf("observation = %#v, err=%v", observation, err)
		}
		bySource[observation.Metrics.Acquisitions[0].SourceID] = observation
	}
	if bySource["public"].Metrics.Scope != catalogmeta.ObservationScopeGlobalPublic ||
		bySource["account"].Metrics.Scope != catalogmeta.ObservationScopeCredentialScoped {
		t.Fatalf("scopes = public:%q account:%q", bySource["public"].Metrics.Scope, bySource["account"].Metrics.Scope)
	}
	publicProvider, _ := bySource["public"].Catalog.Provider("provider-a")
	accountProvider, _ := bySource["account"].Catalog.Provider("provider-a")
	if publicProvider.Models["public-model"] == nil || publicProvider.Models["account-model"] != nil ||
		accountProvider.Models["account-model"] == nil || accountProvider.Models["public-model"] != nil {
		t.Fatalf("cross-source catalogs = public:%#v account:%#v", publicProvider.Models, accountProvider.Models)
	}
	publicProvider.Models["public-model"].Metadata.Tags[0] = catalogs.ModelTagVision
	accountAgain, _ := bySource["account"].Catalog.Provider("provider-a")
	if accountAgain.Models["account-model"].Metadata.Tags[0] != catalogs.ModelTagChat {
		t.Fatal("nested model metadata aliased across concurrent observations")
	}
}

func TestConfiguredSourceSetsIsolateConcurrentCredentialContexts(t *testing.T) {
	provider := providerForTest("provider-a")
	provider.Credentials = map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{
		"api_key": {Env: catalogs.ProviderEnvironmentNames{"ACCOUNT_API_KEY"}},
	}
	provider.Catalog.Sources[0].ObservationScope = catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeCredentialScoped}
	provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}}

	configuredContext := func(secret, modelID string) sources.Source {
		environment := isolatedEnvironment{"ACCOUNT_API_KEY": secret}
		resolver := acquisition.NewResolver(
			acquisition.WithEnvironment(environment),
			acquisition.WithAuthResolver(auth.NewResolver(auth.WithEnvironment(environment))),
		)
		configured, err := NewConfigured(
			newProviderSet(provider), WithSourceResolver(resolver),
			WithClientFactory(func(source acquisition.Source) (sources.ProviderClient, error) {
				key, found := source.Auth().APIKey()
				if !found || key != secret {
					return nil, stderrors.New("wrong isolated credential context")
				}
				return fakeProviderClient{models: []catalogs.Model{{ID: modelID, Name: modelID}}}, nil
			}),
		)
		if err != nil || len(configured) != 1 {
			t.Fatalf("NewConfigured = %d/%v", len(configured), err)
		}
		return configured[0]
	}

	contexts := []sources.Source{
		configuredContext("account-a-secret", "account-a-model"),
		configuredContext("account-b-secret", "account-b-model"),
	}
	observations := make([]sources.Observation, len(contexts))
	errorsByContext := make([]error, len(contexts))
	var wait sync.WaitGroup
	for index := range contexts {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			observations[index], errorsByContext[index] = contexts[index].Observe(context.Background())
		}(index)
	}
	wait.Wait()
	for index, modelID := range []string{"account-a-model", "account-b-model"} {
		if errorsByContext[index] != nil {
			t.Fatalf("context %d Observe: %v", index, errorsByContext[index])
		}
		providerResult, err := observations[index].Catalog.Provider("provider-a")
		if err != nil || providerResult.Models[modelID] == nil || len(providerResult.Models) != 1 {
			t.Fatalf("context %d catalog = %#v err=%v", index, providerResult.Models, err)
		}
		encoded, err := json.Marshal(observations[index])
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "account-a-secret") || strings.Contains(string(encoded), "account-b-secret") {
			t.Fatalf("context %d serialized a credential", index)
		}
	}
}

func TestConfiguredSourcesExcludeApplicationOnlyProviders(t *testing.T) {
	provider := providerForTest("cursor")
	provider.Catalog.Sources[0].Endpoint.Type = catalogs.EndpointTypeApplication
	configured, err := NewConfigured(newProviderSet(provider), WithClientFactory(func(acquisition.Source) (sources.ProviderClient, error) {
		t.Fatal("application-only provider reached client factory")
		return nil, nil
	}))
	if err != nil || len(configured) != 0 {
		t.Fatalf("configured application sources = %d/%v", len(configured), err)
	}
}

func TestQuarantineProviderModelsRejectsMalformedRecordsAndBoundsCount(t *testing.T) {
	models := []catalogs.Model{
		{ID: "valid-a", Name: "Valid A"},
		{ID: "", Name: "Missing ID"},
		{ID: "valid-a", Name: "Duplicate"},
		{ID: " whitespace-id ", Name: "Whitespace ID"},
		{ID: "control\n-id", Name: "Control ID"},
		{ID: "missing-name", Name: ""},
		{ID: "control-name", Name: "Control\nName"},
		{ID: "valid-b", Name: "Valid B"},
	}
	accepted, rejected, issues := quarantineProviderModels("provider", models)
	if len(accepted) != 2 || rejected != 6 || len(issues) != 6 {
		t.Fatalf("accepted/rejected/issues = %d/%d/%#v", len(accepted), rejected, issues)
	}

	bounded := make([]catalogs.Model, constants.MaxCatalogModels+1)
	for index := range bounded {
		bounded[index] = catalogs.Model{ID: fmt.Sprintf("model-%05d", index), Name: "Model"}
	}
	accepted, rejected, issues = quarantineProviderModels("provider", bounded)
	if len(accepted) != constants.MaxCatalogModels || rejected != 1 || len(issues) != 1 || issues[0].Code != sources.ObservationIssueCodePayloadLimit {
		t.Fatalf("bounded accepted/rejected/issues = %d/%d/%#v", len(accepted), rejected, issues)
	}
}

func newProviderSet(providers ...catalogs.Provider) *catalogs.Providers {
	result := catalogs.NewProviders()
	for index := range providers {
		provider := providers[index]
		_ = result.Add(&provider)
	}
	return result
}

func providerForTest(id catalogs.ProviderID) catalogs.Provider {
	return catalogs.Provider{
		ID: id, Name: string(id),
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
			Auth:     catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/models"},
		}}},
	}
}
