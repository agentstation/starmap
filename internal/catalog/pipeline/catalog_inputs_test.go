package pipeline

import (
	"context"
	"fmt"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestLoadHumanWorkspaceReadsOnlySelectedYAML(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	human := catalogs.NewEmpty()
	if err := human.SetProvider(catalogs.Provider{
		ID:   "human-only",
		Name: "Human Only",
		Models: map[string]*catalogs.Model{
			"manual": {ID: "manual", Name: "Manual"},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := human.SaveTo(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := loadHumanWorkspace(path)
	if err != nil {
		t.Fatalf("loadHumanWorkspace: %v", err)
	}
	catalog := buildCatalog(t, loaded)
	providers := catalog.Providers().List()
	if len(providers) != 1 || providers[0].ID != "human-only" {
		t.Fatalf("workspace providers = %#v, want only selected YAML", providers)
	}
}

func TestComposeProviderCatalogAddsEmbeddedProvidersAndPreservesHumanConfig(t *testing.T) {
	t.Parallel()

	embedded := providerConfigurationCatalog(
		t,
		"shared",
		"https://embedded.example/v2",
		"embedded-new",
		"https://embedded.example/new",
	)
	human := providerConfigurationCatalog(
		t,
		"shared",
		"https://human.example/config",
		"",
		"",
	)

	composed, err := composeProviderCatalog(embedded, human, true)
	if err != nil {
		t.Fatalf("composeProviderCatalog: %v", err)
	}
	shared, err := composed.Provider("shared")
	if err != nil {
		t.Fatalf("shared provider: %v", err)
	}
	if got := shared.Catalog.Endpoint.URL; got != "https://human.example/config" {
		t.Fatalf("shared endpoint = %q, want human configuration", got)
	}
	added, err := composed.Provider("embedded-new")
	if err != nil {
		t.Fatalf("new embedded provider: %v", err)
	}
	if got := added.Catalog.Endpoint.URL; got != "https://embedded.example/new" {
		t.Fatalf("new provider endpoint = %q", got)
	}
}

func providerConfigurationCatalog(
	t testing.TB,
	firstID catalogs.ProviderID,
	firstURL string,
	secondID catalogs.ProviderID,
	secondURL string,
) *catalogs.Catalog {
	t.Helper()

	builder := catalogs.NewEmpty()
	setProviderConfiguration(t, builder, firstID, firstURL)
	if secondID != "" {
		setProviderConfiguration(t, builder, secondID, secondURL)
	}
	return buildCatalog(t, builder)
}

func setProviderConfiguration(
	t testing.TB,
	builder *catalogs.Builder,
	id catalogs.ProviderID,
	url string,
) {
	t.Helper()

	if err := builder.SetProvider(catalogs.Provider{
		ID:   id,
		Name: id.String(),
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				URL:  url,
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider(%s): %v", id, err)
	}
}

func TestAcceptedProviderConfigurationKeepsLocalOverridesAndExcludesEmbeddedOnlyProviders(t *testing.T) {
	accepted := providerConfigurationCatalog(t, "shared", "https://accepted.example/models", "accepted-only", "https://accepted.example/new")
	embedded := providerConfigurationCatalog(t, "shared", "https://embedded.example/models", "embedded-only", "https://embedded.example/new")
	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprint(local), func(t *testing.T) {
			path := ""
			if local {
				path = t.TempDir()
				builder, err := catalogs.NewBuilderFrom(providerConfigurationCatalog(t, "shared", "https://operator.example/models", "", ""))
				if err != nil {
					t.Fatal(err)
				}
				if err := builder.SaveTo(path); err != nil {
					t.Fatal(err)
				}
			}
			p := New(nil)
			p.loadEmbedded = func() (*catalogs.Catalog, error) { return embedded, nil }
			inputs, err := p.loadCatalogInputs(t.Context(), path, accepted)
			if err != nil {
				t.Fatal(err)
			}
			if inputs.providerConfig.Providers().Len() != 2 {
				t.Fatal("selected registry expanded beyond accepted providers and explicit local configuration")
			}
			shared, err := inputs.providerConfig.Provider("shared")
			if err != nil {
				t.Fatal(err)
			}
			expected := "https://accepted.example/models"
			if local {
				expected = "https://operator.example/models"
			}
			if shared.Catalog.Endpoint.URL != expected {
				t.Fatal("provider endpoint precedence changed")
			}
			if _, err := inputs.providerConfig.Provider("accepted-only"); err != nil {
				t.Fatal(err)
			}
			if _, err := inputs.providerConfig.Provider("embedded-only"); err == nil {
				t.Fatal("embedded provider escaped accepted registry")
			}
			if !local && inputs.providerConfig != accepted {
				t.Fatal("unchanged immutable provider registry was rebuilt")
			}
		})
	}
}

func TestPrepareRejectsProviderOutsideAcceptedCatalogBeforeAcquisition(t *testing.T) {
	accepted := providerConfigurationCatalog(t, "accepted", "https://accepted.example/models", "", "")
	embedded := providerConfigurationCatalog(t, "excluded", "https://embedded.example/models", "", "")
	p := NewAcquisition(func(*catalogs.Provider) (sources.ProviderClient, error) {
		t.Fatal("provider outside the accepted registry created a client")
		return nil, nil
	}, sources.ProviderCredentialResolverFunc(func(context.Context, *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		t.Fatal("provider outside the accepted registry resolved credentials")
		return sources.ProviderCredentialMaterial{}, nil
	}))
	p.loadEmbedded = func() (*catalogs.Catalog, error) { return embedded, nil }
	if _, err := p.Prepare(t.Context(), accepted, pkgsync.WithDryRun(true), pkgsync.WithSources(sources.ProvidersID), pkgsync.WithProvider("excluded")); err == nil {
		t.Fatal("acquired an embedded provider outside the accepted registry")
	}
}
