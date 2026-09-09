package runtime

import (
	"slices"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAuthorshipRecoveryPreservesContributions(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "retained"
		if replace {
			name = "replaced"
		}
		t.Run(name, func(t *testing.T) {
			at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			seed := manualProviderObservation(t, 0, at).Catalog
			payload, err := catalogs.EncodeCatalogPayload(seed)
			if err != nil {
				t.Fatal(err)
			}
			baselineProvider, err := seed.Provider("provider")
			if err != nil {
				t.Fatal(err)
			}
			wantRef := baselineProvider.Models["model"].ModelRef
			source := newStubSource("authorship-recovery")
			source.replies = []SourceRead{testSourceRead(t, "authorship-baseline", payload, at), testSourceRead(t, "authorship-refresh", payload, at.Add(time.Hour))}
			store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
			connected := openTestRuntime(t, options...)
			if _, err := connected.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			observe := func(partial bool) sources.Observation {
				builder, err := catalogs.NewBuilderFrom(seed)
				if err != nil {
					t.Fatal(err)
				}
				first := catalogs.Author{ID: "first-author", Name: "First Author"}
				second := catalogs.Author{ID: "second-author", Name: "Second Author"}
				for _, author := range []catalogs.Author{first, second} {
					if err := builder.SetAuthor(author); err != nil {
						t.Fatal(err)
					}
				}
				provider, err := builder.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				provider.Models["model"].Authors = []catalogs.Author{second}
				if partial || replace {
					provider.Models["model"].Authors = []catalogs.Author{first}
				}
				if err := builder.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				catalog, err := catalogs.NewObservationCatalog(builder)
				if err != nil {
					t.Fatal(err)
				}
				metadata := sources.ObservationMetadata{ObservedAt: at.Add(2 * time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded}
				if partial {
					metadata.ObservedAt = at.Add(time.Minute)
					metadata.Completeness, metadata.Status = sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded
					metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/invalid", Message: "A source record is invalid."}}
				}
				observation, err := sources.NewObservation(sources.ProvidersID, catalog, metadata)
				if err != nil {
					t.Fatal(err)
				}
				return observation
			}
			prior, current := observe(true), observe(false)
			for _, observation := range []sources.Observation{prior, current} {
				if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
					t.Fatal(err)
				}
			}
			assertState := func(connected *Runtime) {
				t.Helper()
				catalog := connected.State().Catalog
				provider, err := catalog.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				model := provider.Models["model"]
				if model == nil || model.ModelRef != wantRef {
					t.Fatal("authorship changed the serving identity")
				}
				if !slices.ContainsFunc(model.Authors, func(author catalogs.Author) bool { return author.ID == "first-author" }) {
					t.Error("accepted authorship disappeared")
				}
				if !replace && !slices.ContainsFunc(model.Authors, func(author catalogs.Author) bool { return author.ID == "second-author" }) {
					t.Error("new authorship disappeared")
				}
				wantID := prior.ID
				if replace {
					wantID = current.ID
				}
				entries := catalog.Provenance().FindModelField("provider", "model", `Authors["first-author"].present`)
				if len(entries) != 1 || entries[0].ObservationID != wantID {
					t.Errorf("authorship receipt=%+v, want %s", entries, wantID)
				}
				generation, err := store.Current(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				oldFound, newFound := false, false
				for _, receipt := range generation.Manifest.SourceObservations {
					oldFound = oldFound || receipt.ObservationID == prior.ID
					newFound = newFound || receipt.ObservationID == current.ID
				}
				if oldFound == replace || !newFound || generation.Manifest.Degraded == replace {
					t.Errorf("receipts old=%t new=%t degraded=%t, replacement=%t", oldFound, newFound, generation.Manifest.Degraded, replace)
				}
				if len(manualBatches(connected.layers.manual)) != 2 {
					t.Error("recovery changed original observation history")
				}
			}
			assertState(connected)
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := openTestRuntime(t, options...)
			assertState(reopened)
			if _, err := reopened.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			assertState(reopened)
		})
	}
}
