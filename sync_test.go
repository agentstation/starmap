package starmap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/save"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestSyncDryRunDoesNotPublishFetchedCatalog(t *testing.T) {
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"object": "list",
			"data": [
				{"id": "dry-run-model", "object": "model", "owned_by": "test-author", "created": 1700000000}
			]
		}`))
	}))
	defer modelServer.Close()

	outputPath := t.TempDir()
	localCatalog := catalogs.NewEmpty()
	provider := catalogs.Provider{
		ID:   "dry-run-provider",
		Name: "Dry Run Provider",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type:         catalogs.EndpointTypeOpenAI,
				URL:          modelServer.URL,
				AuthRequired: false,
			},
		},
	}
	if err := localCatalog.SetProvider(provider); err != nil {
		t.Fatalf("Failed to seed local provider: %v", err)
	}
	if err := localCatalog.Save(save.WithPath(outputPath)); err != nil {
		t.Fatalf("Failed to save local catalog: %v", err)
	}

	oldCatalog := catalogs.NewEmpty()
	if err := oldCatalog.SetProvider(catalogs.Provider{ID: "old-provider", Name: "Old Provider"}); err != nil {
		t.Fatalf("Failed to seed old catalog: %v", err)
	}

	c := &Client{
		catalog: mustTestCatalog(t, oldCatalog),
		hooks:   newHooks(),
	}

	result, err := c.Sync(
		context.Background(),
		pkgsync.WithDryRun(true),
		pkgsync.WithOutputPath(outputPath),
		pkgsync.WithSources(sources.LocalCatalogID, sources.ProvidersID),
	)
	if err != nil {
		t.Fatalf("Dry-run sync failed: %v", err)
	}
	if !result.DryRun {
		t.Fatal("Expected dry-run result to record DryRun=true")
	}
	if !result.HasChanges() {
		t.Fatal("Expected dry-run sync to detect fetched catalog changes")
	}

	current := c.Catalog()
	if _, err := current.Provider("old-provider"); err != nil {
		t.Fatalf("Expected dry-run to keep old catalog published: %v", err)
	}
	if _, err := current.Provider("dry-run-provider"); err == nil {
		t.Fatal("Dry-run sync published fetched provider into client catalog")
	}
	if _, err := current.FindModel("dry-run-model"); err == nil {
		t.Fatal("Dry-run sync published fetched model into client catalog")
	}
}
