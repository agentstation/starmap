package auth

import (
	"context"
	stderrors "errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

type testEnvironment struct {
	values  map[string]string
	lookups []string
}

func (environment *testEnvironment) LookupEnv(name string) (string, bool) {
	environment.lookups = append(environment.lookups, name)
	value, found := environment.values[name]
	return value, found
}

type testCloudSession struct{ provider catalogs.ProviderID }

func (session testCloudSession) ProviderID() catalogs.ProviderID { return session.provider }

type testCloudAdapter struct {
	provider catalogs.ProviderID
	err      error
	calls    int
}

func (adapter *testCloudAdapter) Resolve(context.Context) (CloudChainSession, error) {
	adapter.calls++
	if adapter.err != nil {
		return nil, adapter.err
	}
	return testCloudSession{provider: adapter.provider}, nil
}

func TestResolverNoneProhibitsCredentialLookup(t *testing.T) {
	environment := &testEnvironment{values: map[string]string{"EXAMPLE_API_KEY": "secret"}}
	cloud := &testCloudAdapter{provider: "example"}
	registry, err := NewCloudChainRegistry(CloudChainRegistration{Provider: "example", Adapter: cloud})
	if err != nil {
		t.Fatalf("cloud registry: %v", err)
	}
	resolver := NewResolver(WithEnvironment(environment), WithCloudChainRegistry(registry))
	provider, source := testResolverProvider(catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone})

	resolved, err := resolver.Resolve(context.Background(), &provider, source)
	if err != nil {
		t.Fatalf("resolve none: %v", err)
	}
	if !resolved.Anonymous() || len(environment.lookups) != 0 || cloud.calls != 0 {
		t.Fatalf("none resolved=%s lookups=%v cloud_calls=%d", resolved, environment.lookups, cloud.calls)
	}
}

func TestCheckerReportsEachLogicalSourceWithoutSecrets(t *testing.T) {
	environment := &testEnvironment{values: map[string]string{"EXAMPLE_API_KEY": "secret-value"}}
	provider, source := testResolverProvider(catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone})
	source.ObservationScope = catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic}
	source.Endpoint = catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/models"}
	provider.Catalog = &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{source}}
	provider.Catalog.Sources = append(provider.Catalog.Sources, catalogs.ProviderSource{
		ID: "authenticated", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
		Auth:     catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}},
		Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/private"},
	})
	checker := NewChecker(WithCheckerResolver(NewResolver(WithEnvironment(environment))))
	statuses := checker.CheckSources(context.Background(), &provider)
	if len(statuses) != 2 || statuses[0].State != StateUnauthenticated || statuses[1].State != StateReady {
		t.Fatalf("statuses = %#v", statuses)
	}
	if statuses[1].SourceID != "authenticated" || len(statuses[1].Environment) != 1 || statuses[1].Environment[0] != "EXAMPLE_API_KEY" {
		t.Fatalf("authenticated status = %#v", statuses[1])
	}
	for _, status := range statuses {
		if strings.Contains(status.Summary, "secret-value") {
			t.Fatalf("status exposed secret: %#v", status)
		}
	}
}

func TestCheckerReportsOnlyConfiguredOptionalMethodsAndEnvironmentNames(t *testing.T) {
	environment := &testEnvironment{values: map[string]string{}}
	cloud := &testCloudAdapter{provider: "example", err: errors.ErrNotFound}
	registry, err := NewCloudChainRegistry(CloudChainRegistration{Provider: "example", Adapter: cloud})
	if err != nil {
		t.Fatalf("cloud registry: %v", err)
	}
	provider, source := testResolverProvider(catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeOptional})
	provider.Credentials["api_key"] = catalogs.ProviderCredential{Env: catalogs.ProviderEnvironmentNames{"PRIMARY_KEY", "ALIAS_KEY"}}
	source.ObservationScope = catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic}
	source.Endpoint = catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: "https://example.test/models"}
	provider.Catalog = &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{source}}
	checker := NewChecker(WithCheckerResolver(NewResolver(WithEnvironment(environment), WithCloudChainRegistry(registry))))
	statuses := checker.CheckSources(context.Background(), &provider)
	if len(statuses) != 1 || statuses[0].State != StateUnauthenticated ||
		!slices.Equal(statuses[0].AcceptedMethods, []catalogs.ProviderCredentialID{"api_key", "cloud_chain"}) ||
		!slices.Equal(statuses[0].Environment, []string{"PRIMARY_KEY", "ALIAS_KEY"}) {
		t.Fatalf("optional source status = %#v", statuses)
	}

	provider.ID = "key-only"
	statuses = NewChecker(WithCheckerResolver(NewResolver(WithEnvironment(environment)))).CheckSources(context.Background(), &provider)
	if len(statuses) != 1 || !slices.Equal(statuses[0].AcceptedMethods, []catalogs.ProviderCredentialID{"api_key"}) {
		t.Fatalf("key-only optional methods = %#v", statuses)
	}
}

func TestResolverOptionalSelectionOrder(t *testing.T) {
	for name, test := range map[string]struct {
		environment map[string]string
		cloudErr    error
		wantMethod  catalogs.ProviderCredentialID
		wantCloud   int
	}{
		"key first":      {environment: map[string]string{"EXAMPLE_API_KEY": "secret"}, wantMethod: "api_key"},
		"chain second":   {environment: map[string]string{}, wantMethod: "cloud_chain", wantCloud: 1},
		"anonymous last": {environment: map[string]string{}, cloudErr: errors.ErrNotFound, wantCloud: 1},
	} {
		t.Run(name, func(t *testing.T) {
			environment := &testEnvironment{values: test.environment}
			cloud := &testCloudAdapter{provider: "example", err: test.cloudErr}
			registry, err := NewCloudChainRegistry(CloudChainRegistration{Provider: "example", Adapter: cloud})
			if err != nil {
				t.Fatalf("cloud registry: %v", err)
			}
			provider, source := testResolverProvider(catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeOptional})
			resolved, err := NewResolver(WithEnvironment(environment), WithCloudChainRegistry(registry)).Resolve(context.Background(), &provider, source)
			if err != nil {
				t.Fatalf("resolve optional: %v", err)
			}
			if resolved.Method() != test.wantMethod || cloud.calls != test.wantCloud {
				t.Fatalf("method=%q cloud_calls=%d, want %q/%d", resolved.Method(), cloud.calls, test.wantMethod, test.wantCloud)
			}
		})
	}
}

func TestResolverOrderedAliasesAndConflicts(t *testing.T) {
	provider, source := testResolverProvider(catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}})
	provider.Credentials["api_key"] = catalogs.ProviderCredential{
		Env: catalogs.ProviderEnvironmentNames{"PRIMARY_KEY", "ALIAS_KEY"},
	}
	for name, values := range map[string]map[string]string{
		"primary": {"PRIMARY_KEY": "secret"},
		"alias":   {"ALIAS_KEY": "secret"},
		"equal":   {"PRIMARY_KEY": "secret", "ALIAS_KEY": "secret"},
	} {
		t.Run(name, func(t *testing.T) {
			resolved, err := NewResolver(WithEnvironment(&testEnvironment{values: values})).Resolve(context.Background(), &provider, source)
			if err != nil || resolved.Method() != "api_key" {
				t.Fatalf("resolve aliases: method=%q err=%v", resolved.Method(), err)
			}
		})
	}
	_, err := NewResolver(WithEnvironment(&testEnvironment{values: map[string]string{
		"PRIMARY_KEY": "SECRET_ALPHA_123", "ALIAS_KEY": "SECRET_BETA_456",
	}})).Resolve(context.Background(), &provider, source)
	if err == nil || strings.Contains(err.Error(), "SECRET_ALPHA_123") || strings.Contains(err.Error(), "SECRET_BETA_456") {
		t.Fatalf("conflicting aliases error = %v", err)
	}
}

func TestResolverInvalidPresentStopsFallback(t *testing.T) {
	cloud := &testCloudAdapter{provider: "example"}
	registry, err := NewCloudChainRegistry(CloudChainRegistration{Provider: "example", Adapter: cloud})
	if err != nil {
		t.Fatalf("cloud registry: %v", err)
	}
	provider, source := testResolverProvider(catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeOptional})
	_, err = NewResolver(
		WithEnvironment(&testEnvironment{values: map[string]string{"EXAMPLE_API_KEY": "   "}}),
		WithCloudChainRegistry(registry),
	).Resolve(context.Background(), &provider, source)
	if err == nil || cloud.calls != 0 {
		t.Fatalf("invalid key err=%v cloud_calls=%d", err, cloud.calls)
	}
}

func TestResolverRequiredAlternativesAndRequestApplication(t *testing.T) {
	provider, source := testResolverProvider(catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key", "cloud_chain"}})
	resolved, err := NewResolver(WithEnvironment(&testEnvironment{values: map[string]string{
		"EXAMPLE_API_KEY": "secret-value",
	}})).Resolve(context.Background(), &provider, source)
	if err != nil {
		t.Fatalf("resolve required alternatives: %v", err)
	}
	request, err := http.NewRequest(http.MethodGet, "https://api.example.test/v1/models", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err := resolved.Apply(request); err != nil {
		t.Fatalf("apply auth: %v", err)
	}
	if got := request.Header.Get("Authorization"); got != "Bearer secret-value" {
		t.Fatalf("Authorization = %q", got)
	}
	if strings.Contains(resolved.String(), "secret-value") {
		t.Fatal("resolved auth diagnostic exposed secret")
	}

	_, err = NewResolver(WithEnvironment(&testEnvironment{values: map[string]string{}})).Resolve(context.Background(), &provider, source)
	if !stderrors.Is(err, errors.ErrAPIKeyRequired) {
		t.Fatalf("missing required auth error = %v", err)
	}
}

func TestResolverCompoundMethodRequiresAllOwnedInputs(t *testing.T) {
	provider := catalogs.Provider{ID: "example", Name: "Example", Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{
		"oauth_client": {
			Kind: catalogs.ProviderCredentialKindCompound,
			Inputs: map[string]catalogs.ProviderCredentialInput{
				"client_id":     {Env: catalogs.ProviderEnvironmentNames{"OAUTH_CLIENT_ID"}},
				"client_secret": {Env: catalogs.ProviderEnvironmentNames{"OAUTH_CLIENT_SECRET"}},
			},
		},
	}}
	source := catalogs.ProviderSource{ID: "models", Auth: catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"oauth_client"}}}

	resolved, err := NewResolver(WithEnvironment(&testEnvironment{values: map[string]string{
		"OAUTH_CLIENT_ID": "client-value", "OAUTH_CLIENT_SECRET": "secret-value",
	}})).Resolve(context.Background(), &provider, source)
	if err != nil || resolved.Method() != "oauth_client" {
		t.Fatalf("resolve compound: method=%q err=%v", resolved.Method(), err)
	}
	if strings.Contains(resolved.String(), "client-value") || strings.Contains(resolved.String(), "secret-value") {
		t.Fatalf("compound diagnostic exposed input: %s", resolved)
	}

	_, err = NewResolver(WithEnvironment(&testEnvironment{values: map[string]string{
		"OAUTH_CLIENT_ID": "client-value",
	}})).Resolve(context.Background(), &provider, source)
	if !stderrors.Is(err, errors.ErrAPIKeyInvalid) || strings.Contains(err.Error(), "client-value") {
		t.Fatalf("partial compound error = %v", err)
	}
}

func TestCloudChainRegistryRejectsDuplicatesAndInvalidSessions(t *testing.T) {
	first := &testCloudAdapter{provider: "example"}
	second := &testCloudAdapter{provider: "example"}
	if _, err := NewCloudChainRegistry(
		CloudChainRegistration{Provider: "example", Adapter: first},
		CloudChainRegistration{Provider: "example", Adapter: second},
	); err == nil {
		t.Fatal("duplicate cloud adapter registered successfully")
	}

	wrong := &testCloudAdapter{provider: "different"}
	registry, err := NewCloudChainRegistry(CloudChainRegistration{Provider: "example", Adapter: wrong})
	if err != nil {
		t.Fatalf("cloud registry: %v", err)
	}
	provider, source := testResolverProvider(catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"cloud_chain"}})
	_, err = NewResolver(WithEnvironment(&testEnvironment{}), WithCloudChainRegistry(registry)).Resolve(context.Background(), &provider, source)
	if err == nil {
		t.Fatal("wrong-provider cloud session resolved successfully")
	}
}

func TestCloudChainRegistryValidatesProviderContract(t *testing.T) {
	adapter := &testCloudAdapter{provider: catalogs.ProviderIDGoogleVertex}
	registry, err := NewCloudChainRegistry(CloudChainRegistration{Provider: catalogs.ProviderIDGoogleVertex, Adapter: adapter})
	if err != nil {
		t.Fatalf("cloud registry: %v", err)
	}
	provider := catalogs.Provider{
		ID: catalogs.ProviderIDGoogleVertex,
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "regional-models", Auth: catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"cloud_chain"}},
		}}},
	}
	if err := registry.ValidateProvider(&provider); err != nil {
		t.Fatalf("validate registered cloud provider: %v", err)
	}

	provider.ID = "unsupported"
	if err := registry.ValidateProvider(&provider); err == nil {
		t.Fatal("unsupported cloud-chain provider validated successfully")
	}
	provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"google_adc"}}
	if err := registry.ValidateProvider(&provider); err == nil {
		t.Fatal("vendor-specific chain method validated successfully")
	}
}

func testResolverProvider(policy catalogs.ProviderAuthPolicy) (catalogs.Provider, catalogs.ProviderSource) {
	provider := catalogs.Provider{
		ID: "example", Name: "Example",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{
			"api_key": {Env: catalogs.ProviderEnvironmentNames{"EXAMPLE_API_KEY"}},
		},
	}
	return provider, catalogs.ProviderSource{ID: "models", Auth: policy}
}
