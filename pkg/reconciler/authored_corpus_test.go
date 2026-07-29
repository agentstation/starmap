package reconciler

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestReconciliationRetainsCanonicalAuthoredCorpus(t *testing.T) {
	t.Run("embedded seeds an empty baseline", func(t *testing.T) {
		embedded := corpusCatalog(t, "model", "Embedded Model")
		result := reconcileCorpus(t, emptyCorpusBaseline(t), []sources.Observation{
			completeCorpusObservation(sources.EmbeddedCatalogID, embedded),
		})
		assertCorpusDefinition(t, result, "author/model", "Embedded Model")
	})

	t.Run("existing baseline remains authoritative", func(t *testing.T) {
		baseline := corpusCatalog(t, "model", "Existing Model")
		embedded := corpusCatalog(t, "model", "New Embedded Model")
		result := reconcileCorpus(t, baseline, []sources.Observation{
			completeCorpusObservation(sources.EmbeddedCatalogID, embedded),
		})
		assertCorpusDefinition(t, result, "author/model", "Existing Model")
	})

	t.Run("embedded adds a definition missing from the baseline", func(t *testing.T) {
		baseline := corpusCatalog(t, "existing", "Existing Model")
		embedded := corpusCatalog(t, "new", "New Embedded Model")
		result := reconcileCorpus(t, baseline, []sources.Observation{
			completeCorpusObservation(sources.EmbeddedCatalogID, embedded),
		})
		assertCorpusDefinition(t, result, "author/existing", "Existing Model")
		assertCorpusDefinition(t, result, "author/new", "New Embedded Model")
	})

	t.Run("human workspace overrides an existing definition", func(t *testing.T) {
		baseline := corpusCatalog(t, "model", "Existing Model")
		human := corpusCatalog(t, "model", "Human Model")
		embedded := corpusCatalog(t, "model", "Embedded Model")
		result := reconcileCorpus(t, baseline, []sources.Observation{
			completeCorpusObservation(sources.EmbeddedCatalogID, embedded),
			completeCorpusObservation(sources.LocalCatalogID, human),
		})
		assertCorpusDefinition(t, result, "author/model", "Human Model")
	})

	t.Run("complete human workspace deletes a local-only definition", func(t *testing.T) {
		baseline := authoredOnlyCatalog(t, "local-only", "Local Model")
		human := emptyAuthoredCorpus(t)
		embedded := emptyAuthoredCorpus(t)
		result := reconcileCorpus(t, baseline, []sources.Observation{
			completeCorpusObservation(sources.EmbeddedCatalogID, embedded),
			completeCorpusObservation(sources.LocalCatalogID, human),
		})
		if _, err := result.Definition("author/local-only"); err == nil {
			t.Fatal("local-only definition survived explicit complete-workspace deletion")
		}
	})
}

func corpusCatalog(t testing.TB, slug, name string) *catalogs.Catalog {
	t.Helper()
	authored := authoredOnlyCatalog(t, slug, name)
	builder, err := catalogs.NewBuilderFrom(authored)
	if err != nil {
		t.Fatalf("NewBuilderFrom: %v", err)
	}
	author, err := builder.Author("author")
	if err != nil {
		t.Fatalf("Author: %v", err)
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID:   "provider",
		Name: "Provider",
		Models: map[string]*catalogs.Model{
			"deployment": {
				ID:       "deployment",
				ModelRef: catalogs.AuthoredModelID(author.ID, slug),
				Name:     "Provider Model",
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}

func authoredOnlyCatalog(t testing.TB, slug, name string) *catalogs.Catalog {
	t.Helper()
	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "author", Name: "Author"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel(author.ID, catalogs.Model{
		ID:      slug,
		Name:    name,
		Authors: []catalogs.Author{author},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return catalog
}

func emptyAuthoredCorpus(t testing.TB) *catalogs.Catalog {
	t.Helper()
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build empty authored corpus: %v", err)
	}
	return catalog
}

func completeCorpusObservation(sourceID sources.ID, catalog *catalogs.Catalog) sources.Observation {
	return sources.Observation{
		SourceID:     sourceID,
		Catalog:      catalog,
		Status:       sources.ObservationStatusSucceeded,
		Completeness: sources.ObservationCompletenessComplete,
	}
}

func emptyCorpusBaseline(t testing.TB) *catalogs.Catalog {
	t.Helper()
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build empty baseline: %v", err)
	}
	return catalog
}

func reconcileCorpus(
	t testing.TB,
	baseline *catalogs.Catalog,
	observations []sources.Observation,
) *catalogs.Catalog {
	t.Helper()
	reconcile, err := New(WithBaseline(baseline))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := reconcile.Sources(context.Background(), sources.EmbeddedCatalogID, observations)
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	catalog, err := result.Catalog.Build()
	if err != nil {
		t.Fatalf("Build result: %v", err)
	}
	return catalog
}

func assertCorpusDefinition(
	t testing.TB,
	catalog *catalogs.Catalog,
	id catalogs.ModelDefinitionID,
	wantName string,
) {
	t.Helper()
	definition, err := catalog.Definition(id)
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if definition.Name != wantName {
		t.Fatalf("definition name = %q, want %q", definition.Name, wantName)
	}
}
