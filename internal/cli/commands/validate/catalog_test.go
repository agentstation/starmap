package validate

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/testcatalog"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestValidateCatalogFormattedOutputReturnsErrorOnFailures(t *testing.T) {
	cat := testCatalogWithModelAuthor(t, catalogs.AuthorID("missing-author"))
	app := &testApplication{
		CatalogFunc: func() (*catalogs.Catalog, error) {
			return cat.Build()
		},
		OutputFormatFunc: func() string {
			return "json"
		},
	}

	err := runCatalog(&cobra.Command{}, nil, app)
	if err == nil {
		t.Fatal("runCatalog returned nil error for invalid catalog")
	}
	if !strings.Contains(err.Error(), "catalog validation failed") {
		t.Fatalf("error = %q, want catalog validation failed", err)
	}
}

func TestValidateCatalogResolvesAuthorAliases(t *testing.T) {
	cat := testCatalogWithModelAuthor(t, catalogs.AuthorID("system"))
	author := testAuthor()
	author.ID = catalogs.AuthorIDOpenAI
	author.Name = "OpenAI"
	author.Aliases = []catalogs.AuthorID{"system"}
	if err := cat.SetAuthor(*author); err != nil {
		t.Fatalf("SetAuthor returned error: %v", err)
	}

	app := &testApplication{
		CatalogFunc: func() (*catalogs.Catalog, error) {
			return cat.Build()
		},
	}

	if err := validateModelConsistency(app, false); err != nil {
		t.Fatalf("validateModelConsistency returned error: %v", err)
	}
	if err := validateCrossReferences(app, false); err != nil {
		t.Fatalf("validateCrossReferences returned error: %v", err)
	}
}

func testCatalogWithModelAuthor(t *testing.T, authorID catalogs.AuthorID) *catalogs.Builder {
	t.Helper()

	cat := catalogs.NewEmpty()

	author := testAuthor()
	if err := cat.SetAuthor(*author); err != nil {
		t.Fatalf("SetAuthor returned error: %v", err)
	}

	provider := testProvider()
	provider.Catalog.Authors = []catalogs.AuthorID{authorID}
	model := testModel()
	model.ModelRef = catalogs.AuthoredModelID(author.ID, model.ID)
	model.Authors = []catalogs.Author{{ID: authorID, Name: authorID.String()}}
	provider.Models = map[string]*catalogs.Model{model.ID: model}
	if err := cat.SetAuthorModel(author.ID, catalogs.Model{
		ID:      model.ID,
		Name:    model.Name,
		Authors: []catalogs.Author{*author},
	}); err != nil {
		t.Fatalf("SetAuthorModel returned error: %v", err)
	}

	if err := cat.SetProvider(*provider); err != nil {
		t.Fatalf("SetProvider returned error: %v", err)
	}

	return cat
}

func testAuthor() *catalogs.Author {
	return &catalogs.Author{ID: "test-author", Name: "Test Author"}
}

func testProvider() *catalogs.Provider {
	return &catalogs.Provider{
		ID:   "test-provider",
		Name: "Test Provider",
		Credentials: testcatalog.APIKeyCredentials(
			"TEST_PROVIDER_API_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer,
		),
		Catalog: &catalogs.ProviderCatalog{
			Authors: []catalogs.AuthorID{"test-author"},
			Endpoint: catalogs.ProviderEndpoint{
				Type:            catalogs.EndpointTypeOpenAI,
				URL:             "https://provider.example/models",
				ProtocolOptions: testcatalog.OpenAIProtocolOptions(),
			},
		},
	}
}

func testModel() *catalogs.Model {
	return &catalogs.Model{
		ID:          "test-model",
		Name:        "Test Model",
		Description: "A test model for unit tests",
	}
}
