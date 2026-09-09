package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAllProviderLimitsRetainPresenceAcrossPublicationAndRestart(t *testing.T) {
	for _, mode := range []string{"positive", "zero", "unknown", "missing", "unknown-no-fallback", "missing-no-fallback"} {
		t.Run(mode, func(t *testing.T) {
			at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
			seed := manualProviderObservation(t, 0, at).Catalog
			build := func(value int64, observed bool) *catalogs.Catalog {
				builder, err := catalogs.NewBuilderFrom(seed)
				if err != nil {
					t.Fatal(err)
				}
				provider, err := builder.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				model := provider.Models["model"]
				model.Limits = &catalogs.ModelLimits{}
				for _, limit := range catalogs.PublishedModelLimits() {
					switch {
					case !observed && (mode == "unknown-no-fallback" || mode == "missing-no-fallback"):
						model.Limits.Unset(limit)
					case observed && (mode == "unknown" || mode == "unknown-no-fallback"):
						model.Limits.SetUnknown(limit)
					case observed && (mode == "missing" || mode == "missing-no-fallback"):
						model.Limits.Unset(limit)
					default:
						model.Limits.Set(limit, value)
					}
				}
				if err := builder.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				if observed {
					result, err := catalogs.NewObservationCatalog(builder)
					if err != nil {
						t.Fatal(err)
					}
					return result
				}
				result, err := builder.Build()
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			source := newStubSource("limit-presence")
			for index, value := range []int64{100, 300} {
				payload, err := catalogs.EncodeCatalogPayload(build(value, false))
				if err != nil {
					t.Fatal(err)
				}
				source.replies = append(source.replies, testSourceRead(t, mode+time.Duration(index).String(), payload, at.Add(time.Duration(index)*time.Hour)))
			}
			store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
			connected := openTestRuntime(t, options...)
			if _, err := connected.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			value := int64(200)
			if mode == "zero" {
				value = 0
			}
			observation, err := sources.NewObservation(sources.ProvidersID, build(value, true), sources.ObservationMetadata{
				ObservedAt: at.Add(time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
				Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertLimits := func(current *Runtime, fallback int64) {
				t.Helper()
				provider, err := current.State().Catalog.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				model := provider.Models["model"]
				if model == nil || model.ModelRef == "" {
					t.Fatal("publication lost linked offering membership or limits")
				}
				want := value
				wantPresence := catalogs.ValueKnown
				if mode == "unknown" || mode == "missing" {
					want = fallback
				}
				if mode == "unknown-no-fallback" {
					want, wantPresence = 0, catalogs.ValueUnknown
				} else if mode == "missing-no-fallback" {
					want, wantPresence = 0, catalogs.ValueMissing
				}
				for _, limit := range catalogs.PublishedModelLimits() {
					got, presence := model.Limits.Value(limit)
					if got != want || presence != wantPresence {
						t.Errorf("%s = %d/%v, want %d/%v", limit, got, presence, want, wantPresence)
					}
					if mode == "positive" || mode == "zero" || mode == "unknown-no-fallback" {
						entries := current.State().Catalog.Provenance().FindModelField("provider", "model", "limits."+string(limit))
						if len(entries) != 1 || entries[0].ObservationID != observation.ID {
							t.Errorf("%s lost the supplying provider receipt", limit)
						}
					}
				}
			}
			if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
				t.Fatal(err)
			}
			assertLimits(connected, 100)
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := openTestRuntime(t, options...)
			assertLimits(reopened, 100)
			if _, err := reopened.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			assertLimits(reopened, 300)
			batches := manualBatches(reopened.layers.manual)
			if len(batches) != 1 || len(batches[0].observations) != 1 {
				t.Fatal("restart lost original observation history")
			}
			restored, err := batches[0].observations[0].restore()
			if err != nil {
				t.Fatal(err)
			}
			provider, err := restored.Catalog.Provider("provider")
			if err != nil {
				t.Fatal(err)
			}
			wantPresence := catalogs.ValueKnown
			if mode == "unknown" || mode == "unknown-no-fallback" {
				wantPresence = catalogs.ValueUnknown
			} else if mode == "missing" || mode == "missing-no-fallback" {
				wantPresence = catalogs.ValueMissing
			}
			for _, limit := range catalogs.PublishedModelLimits() {
				_, presence := provider.Models["model"].Limits.Value(limit)
				if presence != wantPresence {
					t.Errorf("retained %s presence = %v, want %v", limit, presence, wantPresence)
				}
			}
		})
	}
}
