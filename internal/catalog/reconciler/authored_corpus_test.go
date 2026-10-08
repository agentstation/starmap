package reconciler

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/sources/local"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestLocalWorkspacePreservesAbsentAuthorMetadata(t *testing.T) {
	cases := []struct {
		name    string
		write   bool
		sidecar string
		partial bool
	}{
		{name: "complete/missing sidecar"},
		{name: "complete/empty sidecar", write: true},
		{name: "complete/replacement sidecar", write: true, sidecar: "<svg>replacement</svg>"},
		{name: "partial/missing sidecar", partial: true},
		{name: "partial/empty sidecar", write: true, partial: true},
		{name: "partial/replacement sidecar", write: true, sidecar: "<svg>replacement</svg>", partial: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			baseline := corpusCatalog(t, "existing", "Existing Model")
			builder, err := catalogs.NewBuilderFrom(baseline)
			if err != nil {
				t.Fatal(err)
			}
			author, err := builder.Author("author")
			if err != nil {
				t.Fatal(err)
			}
			description := "Retained description"
			website := "https://example.com"
			author.Description = &description
			author.Website = &website
			author.Logo = []byte("<svg>retained</svg>")
			author.Aliases = []catalogs.AuthorID{"retained"}
			if err := builder.SetAuthor(author); err != nil {
				t.Fatal(err)
			}
			baseline, err = builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			models := filepath.Join(root, "authors", "author", "models")
			if err := os.MkdirAll(models, constants.DirPermissions); err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"authors.yaml":                   "- id: author\n  name: Edited Author\n  aliases: [new-alias, retained]\n",
				"authors/author/models/new.yaml": "id: new\nname: New Model\nauthors:\n  - id: author\n    name: Edited Author\n",
			}
			if test.partial {
				files["authors/author/models/broken.yaml"] = "id: broken\nname: [unterminated\n"
			}
			if test.write {
				files["authors/author/logo.svg"] = test.sidecar
			}
			for name, contents := range files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(contents), constants.FilePermissions); err != nil {
					t.Fatal(err)
				}
			}
			observed, err := local.New(local.WithCatalogPath(root)).Observe(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			completeness := sources.ObservationCompletenessComplete
			status := sources.ObservationStatusSucceeded
			rejected := 0
			if test.partial {
				completeness = sources.ObservationCompletenessPartial
				status = sources.ObservationStatusDegraded
				rejected = 1
			}
			if observed.Completeness != completeness || observed.Status != status || observed.Records.Accepted != 1 || observed.Records.Rejected != rejected {
				t.Fatalf("observation = %#v, want completeness %q, status %q, and %d rejected records", observed, completeness, status, rejected)
			}
			result := reconcileCorpus(t, baseline, []sources.Observation{
				completeCorpusObservation(sources.EmbeddedCatalogID, baseline), observed,
			})
			actual, err := result.Author("author")
			if err != nil {
				t.Fatal(err)
			}
			expectedLogo := author.Logo
			if test.sidecar != "" {
				expectedLogo = []byte(files["authors/author/logo.svg"])
			}
			if !bytes.Equal(actual.Logo, expectedLogo) || !reflect.DeepEqual(actual.Description, author.Description) || !reflect.DeepEqual(actual.Website, author.Website) {
				t.Fatalf("author = %#v, want retained absent metadata and logo %q", actual, expectedLogo)
			}
			if actual.Name != "Edited Author" || !reflect.DeepEqual(actual.Aliases, []catalogs.AuthorID{"new-alias", "retained"}) {
				t.Fatalf("author = %#v, want explicit name update and alias union", actual)
			}
			assertCorpusDefinition(t, result, "author/new", "New Model")
			assertCorpusDefinition(t, result, "author/existing", "Existing Model")
		})
	}
}

func TestLocalAuthorMetadataUsesConfiguredPolicy(t *testing.T) {
	for _, test := range []struct {
		name     string
		empty    authority.EmptyPolicy
		sources  []sources.ID
		incoming []byte
		want     string
	}{
		{name: "absent retains", empty: authority.EmptyAbsent, sources: []sources.ID{sources.LocalCatalogID}, want: "retained"},
		{name: "empty retains", empty: authority.EmptyAbsent, sources: []sources.ID{sources.LocalCatalogID}, incoming: []byte{}, want: "retained"},
		{name: "present replaces", empty: authority.EmptyAbsent, sources: []sources.ID{sources.LocalCatalogID}, incoming: []byte("edited"), want: "edited"},
		{name: "ineligible local retains", empty: authority.EmptyAbsent, sources: []sources.ID{sources.ModelsDevHTTPID}, incoming: []byte("edited"), want: "retained"},
		{name: "authoritative empty clears", empty: authority.EmptyAuthoritative, sources: []sources.ID{sources.LocalCatalogID}},
	} {
		t.Run(test.name, func(t *testing.T) {
			strategy := NewAuthorityStrategy(seamAuthority{policy: authority.Policy{
				Resource: evidence.ResourceTypeAuthor, Path: "Logo", SourceOrder: test.sources,
				Empty: test.empty, Merge: authority.MergeReplace,
			}})
			retained := catalogs.Author{ID: "author", Name: "Retained", Logo: []byte("retained")}
			incoming := catalogs.Author{ID: "author", Name: "Edited", Logo: test.incoming}
			incomingLogo := bytes.Clone(incoming.Logo)
			actual := mergeLocalAuthor(retained, incoming, strategy)
			if string(actual.Logo) != test.want || actual.Name != retained.Name {
				t.Fatalf("author = %#v, want logo %q and retained fields outside policy", actual, test.want)
			}
			if len(actual.Logo) != 0 {
				actual.Logo[0] = 'X'
			}
			if string(retained.Logo) != "retained" || !bytes.Equal(incoming.Logo, incomingLogo) {
				t.Fatal("result mutation changed an input author")
			}
		})
	}
}

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
		baseline = withCorpusAuthorName(t, baseline, "Existing Author")
		embedded := withCorpusAuthorName(
			t,
			corpusCatalog(t, "model", "New Embedded Model"),
			"Embedded Author",
		)
		result := reconcileCorpus(t, baseline, []sources.Observation{
			completeCorpusObservation(sources.EmbeddedCatalogID, embedded),
		})
		assertCorpusDefinition(t, result, "author/model", "Existing Model")
		author, err := result.Author("author")
		if err != nil {
			t.Fatalf("Author: %v", err)
		}
		if author.Name != "Existing Author" {
			t.Fatalf("author name = %q, want retained baseline metadata", author.Name)
		}
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
		human := withCorpusAuthorName(
			t,
			corpusCatalog(t, "model", "Human Model"),
			"Human Author",
		)
		embedded := corpusCatalog(t, "model", "Embedded Model")
		result := reconcileCorpus(t, baseline, []sources.Observation{
			completeCorpusObservation(sources.EmbeddedCatalogID, embedded),
			completeCorpusObservation(sources.LocalCatalogID, human),
		})
		assertCorpusDefinition(t, result, "author/model", "Human Model")
		author, err := result.Author("author")
		if err != nil {
			t.Fatalf("Author: %v", err)
		}
		if author.Name != "Human Author" {
			t.Fatalf("author name = %q, want human workspace metadata", author.Name)
		}
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

func withCorpusAuthorName(
	t testing.TB,
	source *catalogs.Catalog,
	name string,
) *catalogs.Catalog {
	t.Helper()
	builder, err := catalogs.NewBuilderFrom(source)
	if err != nil {
		t.Fatalf("NewBuilderFrom: %v", err)
	}
	author, err := builder.Author("author")
	if err != nil {
		t.Fatalf("Author: %v", err)
	}
	author.Name = name
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
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
