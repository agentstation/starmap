package acquisition

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/catalogs"
)

type testEnvironment map[string]string

func (environment testEnvironment) LookupEnv(name string) (string, bool) {
	value, found := environment[name]
	return value, found
}

func TestResolverResolvesOneExactSourceWithoutMutatingConfiguration(t *testing.T) {
	t.Parallel()
	environment := testEnvironment{
		"EXAMPLE_API_KEY":  "top-secret-key",
		"EXAMPLE_ACCOUNT":  "account-123",
		"EXAMPLE_BASE_URL": "https://tenant.example.test",
		"EXAMPLE_MODE":     "enabled",
	}
	provider := acquisitionTestProvider()
	before, err := json.Marshal(provider)
	if err != nil {
		t.Fatalf("Marshal before: %v", err)
	}
	resolver := NewResolver(
		WithEnvironment(environment),
		WithAuthResolver(auth.NewResolver(auth.WithEnvironment(environment))),
	)

	resolved, err := resolver.Resolve(context.Background(), &provider, "private-models")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got, want := resolved.EndpointURL(), "https://tenant.example.test/v1/models"; got != want {
		t.Fatalf("EndpointURL = %q, want %q", got, want)
	}
	if got, found := resolved.Binding("account"); !found || got != "account-123" {
		t.Fatalf("Binding(account) = (%q, %t)", got, found)
	}
	if got, found := resolved.Option("mode"); !found || got != "enabled" {
		t.Fatalf("Option(mode) = (%q, %t)", got, found)
	}
	if got := resolved.Auth().Method(); got != "api_key" {
		t.Fatalf("Auth.Method = %q", got)
	}
	for _, text := range []string{resolved.String(), resolved.Auth().String()} {
		if strings.Contains(text, "top-secret-key") || strings.Contains(text, "account-123") {
			t.Fatalf("safe diagnostic leaked runtime value: %q", text)
		}
	}
	after, err := json.Marshal(provider)
	if err != nil {
		t.Fatalf("Marshal after: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("resolver mutated provider configuration\nbefore: %s\nafter:  %s", before, after)
	}
	encoded, err := json.Marshal(resolved)
	if err != nil {
		t.Fatalf("Marshal resolved source: %v", err)
	}
	if string(encoded) != "{}" {
		t.Fatalf("resolved runtime must have no serializable fields: %s", encoded)
	}
}

func TestResolverRejectsUnknownSourceAndConflictingAliasesBeforeTransport(t *testing.T) {
	t.Parallel()
	provider := acquisitionTestProvider()
	resolver := NewResolver(
		WithEnvironment(testEnvironment{"EXAMPLE_ACCOUNT": "first", "EXAMPLE_ACCOUNT_ALIAS": "second"}),
		WithAuthResolver(auth.NewResolver(auth.WithEnvironment(testEnvironment{"EXAMPLE_API_KEY": "key"}))),
	)
	if _, err := resolver.Resolve(context.Background(), &provider, "missing"); err == nil || !strings.Contains(err.Error(), "provider source") {
		t.Fatalf("unknown source error = %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), &provider, "private-models"); err == nil || !strings.Contains(err.Error(), "source input account is unavailable") {
		t.Fatalf("conflicting alias error = %v", err)
	}
}

func TestResolverNonePerformsNoCredentialLookup(t *testing.T) {
	t.Parallel()
	provider := acquisitionTestProvider()
	provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}
	environment := &countingEnvironment{values: testEnvironment{"EXAMPLE_ACCOUNT": "account"}}
	resolver := NewResolver(
		WithEnvironment(environment),
		WithAuthResolver(auth.NewResolver(auth.WithEnvironment(environment))),
	)
	resolved, err := resolver.Resolve(context.Background(), &provider, "private-models")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !resolved.Auth().Anonymous() {
		t.Fatal("none auth resolved as authenticated")
	}
	if environment.lookups["EXAMPLE_API_KEY"] != 0 {
		t.Fatalf("none auth looked up credential %d times", environment.lookups["EXAMPLE_API_KEY"])
	}
}

type countingEnvironment struct {
	values  testEnvironment
	lookups map[string]int
}

func (environment *countingEnvironment) LookupEnv(name string) (string, bool) {
	if environment.lookups == nil {
		environment.lookups = make(map[string]int)
	}
	environment.lookups[name]++
	return environment.values.LookupEnv(name)
}

func acquisitionTestProvider() catalogs.Provider {
	return catalogs.Provider{
		ID: "example",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{
			"api_key": {Kind: catalogs.ProviderCredentialKindAPIKey, Env: catalogs.ProviderEnvironmentNames{"EXAMPLE_API_KEY"}},
		},
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID:               "private-models",
			ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeCredentialScoped},
			Auth:             catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}},
			Scopes: map[string]catalogs.ProviderScopeBinding{
				"account": {
					Source: catalogs.ProviderBindingSourceEnv,
					Name:   catalogs.ProviderEnvironmentNames{"EXAMPLE_ACCOUNT", "EXAMPLE_ACCOUNT_ALIAS"},
					Role:   catalogs.ProviderBindingRoleRequiredInput,
				},
			},
			Options: map[string]catalogs.ProviderOptionBinding{
				"mode": {Source: catalogs.ProviderBindingSourceEnv, Name: catalogs.ProviderEnvironmentNames{"EXAMPLE_MODE"}},
			},
			Endpoint: catalogs.ProviderSourceEndpoint{
				Type: catalogs.EndpointTypeOpenAI, URL: "https://public.example.test/v1/models",
				BaseURLEnv: "EXAMPLE_BASE_URL", Path: "/v1/models",
			},
		}}},
	}
}
