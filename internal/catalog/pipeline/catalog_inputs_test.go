package pipeline

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/save"
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
	if err := human.Save(save.WithPath(path)); err != nil {
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
