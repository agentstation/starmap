package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestCatalogCredentialEnvironmentPrecedence(t *testing.T) {
	provider := ambientCredentialProvider()

	t.Run("conventional precedes product alias", func(t *testing.T) {
		resolver := newResolver(mapEnvironment(map[string]string{
			"OPENAI_API_KEY":         "conventional",
			"STARMAP_OPENAI_API_KEY": "product-alias",
		}))
		material, err := resolver.ResolveCatalog(context.Background(), &provider)
		if err != nil {
			t.Fatalf("ResolveCatalog: %v", err)
		}
		if got, _ := material.Value("api-key"); got != "conventional" {
			t.Fatalf("api-key = %q, want conventional", got)
		}
	})

	t.Run("product alias is the final ambient candidate", func(t *testing.T) {
		resolver := newResolver(mapEnvironment(map[string]string{
			"STARMAP_OPENAI_API_KEY": "product-alias",
		}))
		material, err := resolver.ResolveCatalog(context.Background(), &provider)
		if err != nil {
			t.Fatalf("ResolveCatalog: %v", err)
		}
		if got, _ := material.Value("api-key"); got != "product-alias" {
			t.Fatalf("api-key = %q, want product-alias", got)
		}
	})

	t.Run("invalid selected value is terminal", func(t *testing.T) {
		lookups := make([]string, 0, 2)
		resolver := newResolver(func(name string) (string, bool) {
			lookups = append(lookups, name)
			values := map[string]string{
				"OPENAI_API_KEY":         "invalid",
				"STARMAP_OPENAI_API_KEY": "valid-product-alias",
			}
			value, exists := values[name]
			return value, exists
		})
		_, err := resolver.ResolveCatalog(context.Background(), &provider)
		if err == nil || !strings.Contains(err.Error(), "selected value") {
			t.Fatalf("ResolveCatalog error = %v", err)
		}
		if len(lookups) != 1 || lookups[0] != "OPENAI_API_KEY" {
			t.Fatalf("lookups = %#v, want terminal conventional selection", lookups)
		}
	})
}

func TestCloudCredentialChainSelection(t *testing.T) {
	provider := defaultChainCredentialProvider()
	material, err := newResolver(mapEnvironment(nil)).ResolveCatalog(
		context.Background(),
		&provider,
	)
	if err != nil {
		t.Fatalf("ResolveCatalog: %v", err)
	}
	profile := material.Profile()
	if profile.ID != "workload-identity" ||
		profile.Primitive != catalogs.ProviderAuthenticationGoogleDefault {
		t.Fatalf("profile = %#v", profile)
	}
	if got := material.EndpointBindings()["location"]; got != "us-central1" {
		t.Fatalf("location = %q, want default", got)
	}
}

func TestUnadmittedDefaultChainsFailClosed(t *testing.T) {
	provider := defaultChainCredentialProvider()
	provider.Credentials.Profiles[0].Primitive = catalogs.ProviderAuthenticationAzureDefault
	provider.Credentials.Profiles[0].ProtocolOptions = catalogs.ProviderAuthenticationProtocolOptions{}

	_, err := newResolver(mapEnvironment(nil)).ResolveCatalog(context.Background(), &provider)
	if err == nil || !strings.Contains(err.Error(), "no catalog-acquisition credential profile") {
		t.Fatalf("ResolveCatalog error = %v, want unavailable chain failure", err)
	}
}

func TestCheckerReportsSelectedCatalogPrimitive(t *testing.T) {
	provider := ambientCredentialProvider()
	checker := NewChecker(WithCredentialResolver(newResolver(mapEnvironment(map[string]string{
		"OPENAI_API_KEY": "valid-key",
	}))))
	status := checker.CheckProvider(&provider, map[string]bool{string(provider.ID): true})
	if status.State != StateConfigured || status.Profile == nil ||
		status.Profile.Primitive != catalogs.ProviderAuthenticationAPIKey {
		t.Fatalf("status = %#v", status)
	}
}

func ambientCredentialProvider() catalogs.Provider {
	return catalogs.Provider{
		ID: "openai", Name: "OpenAI",
		Credentials: &catalogs.ProviderCredentials{
			Fields: []catalogs.ProviderCredentialField{{
				ID: "api-key", Kind: catalogs.ProviderCredentialFieldSecret, Required: true,
				Environment: []string{"OPENAI_API_KEY"}, Pattern: `^valid|conventional|product-alias`,
			}},
			Profiles: []catalogs.ProviderCredentialProfile{{
				ID: "api-key", Primitive: catalogs.ProviderAuthenticationAPIKey,
				Fields: []catalogs.ProviderCredentialFieldID{"api-key"},
				Placements: []catalogs.ProviderCredentialPlacement{{
					Field: "api-key", Kind: catalogs.ProviderCredentialPlacementHeader,
					Name: "Authorization", Scheme: catalogs.ProviderCredentialSchemeBearer,
				}},
			}},
			CatalogAcquisition: catalogs.ProviderCredentialPlane{
				Required: true, Alternatives: []catalogs.ProviderCredentialProfileID{"api-key"},
			},
		},
	}
}

func defaultChainCredentialProvider() catalogs.Provider {
	return catalogs.Provider{
		ID: "cloud", Name: "Cloud",
		Credentials: &catalogs.ProviderCredentials{
			Fields: []catalogs.ProviderCredentialField{
				{ID: "access-token", Kind: catalogs.ProviderCredentialFieldSecret, Required: true},
				{ID: "project", Kind: catalogs.ProviderCredentialFieldParameter, Required: true},
				{ID: "location", Kind: catalogs.ProviderCredentialFieldParameter, Required: true, Default: "us-central1"},
			},
			Profiles: []catalogs.ProviderCredentialProfile{{
				ID: "workload-identity", Primitive: catalogs.ProviderAuthenticationGoogleDefault,
				Fields: []catalogs.ProviderCredentialFieldID{"access-token", "project", "location"},
				Placements: []catalogs.ProviderCredentialPlacement{{
					Field: "access-token", Kind: catalogs.ProviderCredentialPlacementHeader,
					Name: "Authorization", Scheme: catalogs.ProviderCredentialSchemeBearer,
				}},
				Scopes: []string{"https://example.test/scope"},
				ProtocolOptions: catalogs.ProviderAuthenticationProtocolOptions{
					GoogleDefault: &catalogs.ProviderGoogleDefaultProtocolOptions{
						ProjectField: "project",
					},
				},
				EndpointBindings: []catalogs.ProviderCredentialEndpointBinding{{
					Field: "location", Variable: "location",
					Format: catalogs.ProviderCredentialEndpointBindingPathSegment,
				}},
			}},
			CatalogAcquisition: catalogs.ProviderCredentialPlane{
				Required: true, Alternatives: []catalogs.ProviderCredentialProfileID{"workload-identity"},
			},
		},
	}
}

func mapEnvironment(values map[string]string) environmentLookup {
	return func(name string) (string, bool) {
		value, exists := values[name]
		return value, exists
	}
}
