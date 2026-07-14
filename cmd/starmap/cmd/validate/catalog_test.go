package validate

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/application"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestValidateCatalogFormattedOutputReturnsErrorOnFailures(t *testing.T) {
	cat := testCatalogWithModelAuthor(t, catalogs.AuthorID("missing-author"))
	app := &application.Mock{
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
	author := catalogs.TestAuthor(t)
	author.ID = catalogs.AuthorIDOpenAI
	author.Name = "OpenAI"
	author.Aliases = []catalogs.AuthorID{"system"}
	if err := cat.SetAuthor(*author); err != nil {
		t.Fatalf("SetAuthor returned error: %v", err)
	}

	app := &application.Mock{
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

	author := catalogs.TestAuthor(t)
	if err := cat.SetAuthor(*author); err != nil {
		t.Fatalf("SetAuthor returned error: %v", err)
	}

	provider := catalogs.TestProvider(t)
	provider.Catalog.Sources[0].Authors = []catalogs.AuthorID{authorID}
	model := catalogs.TestModel(t)
	model.Authors = []catalogs.Author{{ID: authorID, Name: authorID.String()}}
	provider.Models = map[string]*catalogs.Model{model.ID: model}

	if err := cat.SetProvider(*provider); err != nil {
		t.Fatalf("SetProvider returned error: %v", err)
	}

	return cat
}
