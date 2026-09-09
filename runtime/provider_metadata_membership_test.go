package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestMetadataOnlyLinkedMembershipSurvivesRuntimeLifecycle(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		name := "missing"
		if unknown {
			name = "unknown"
		}
		t.Run(name, func(t *testing.T) {
			at := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
			seed := manualProviderObservation(t, 0, at).Catalog
			build := func(name string) *catalogs.Catalog {
				builder, err := catalogs.NewBuilderFrom(seed)
				if err != nil {
					t.Fatal(err)
				}
				provider, err := builder.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				model := provider.Models["model"]
				model.Name = name
				for _, record := range []catalogs.ModelRecord{catalogs.ModelRecordPricing, catalogs.ModelRecordLimits} {
					model.UnsetRecord(record)
					if unknown {
						model.SetRecordUnknown(record)
					}
				}
				if err := builder.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				catalog, err := catalogs.NewObservationCatalog(builder)
				if err != nil {
					t.Fatal(err)
				}
				return catalog
			}
			baseline := build("Baseline metadata")
			payload, err := catalogs.EncodeCatalogPayload(baseline)
			if err != nil {
				t.Fatal(err)
			}
			source := newStubSource("metadata-membership")
			source.replies = []SourceRead{testSourceRead(t, "metadata-baseline", payload, at), testSourceRead(t, "metadata-refresh", payload, at.Add(time.Hour))}
			store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
			connected := openTestRuntime(t, options...)
			if _, err := connected.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			assertState := func(active *Runtime, wantName string) {
				t.Helper()
				catalog := active.State().Catalog
				provider, err := catalog.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				model := provider.Models["model"]
				if model == nil || model.ModelRef == "" {
					t.Fatal("metadata-only model lost its explicit link")
				}
				if model.Name != wantName || model.Pricing != nil || model.Limits != nil {
					t.Errorf("metadata-only model=%+v", model)
				}
				want := catalogs.ValueMissing
				if unknown {
					want = catalogs.ValueUnknown
				}
				for _, record := range []catalogs.ModelRecord{catalogs.ModelRecordPricing, catalogs.ModelRecordLimits} {
					if model.RecordPresence(record) != want {
						t.Errorf("record %s state=%v, want %v", record, model.RecordPresence(record), want)
					}
				}
				offering, err := catalog.Offering("provider", "model")
				if err != nil {
					t.Fatal(err)
				}
				if offering.DefinitionID != catalogs.ModelDefinitionID(model.ModelRef) || offering.Pricing != nil || offering.Limits != nil {
					t.Errorf("metadata-only offering=%+v", offering)
				}
				if _, err := catalog.Definition(offering.DefinitionID); err != nil {
					t.Fatal(err)
				}
				if _, err := catalog.FindModel("model"); err != nil {
					t.Fatal(err)
				}
				offerings, err := catalog.DefinitionOfferings(offering.DefinitionID)
				if err != nil || len(offerings) != 1 {
					t.Errorf("linked offerings=%+v, error=%v", offerings, err)
				}
			}
			assertState(connected, "Baseline metadata")
			observation, err := sources.NewObservation(sources.ProvidersID, build("Provider metadata"), sources.ObservationMetadata{ObservedAt: at.Add(time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
				t.Fatal(err)
			}
			assertState(connected, "Provider metadata")
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := openTestRuntime(t, options...)
			assertState(reopened, "Provider metadata")
			if _, err := reopened.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			assertState(reopened, "Provider metadata")
		})
	}
}
