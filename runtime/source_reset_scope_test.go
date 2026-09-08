package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func sourceResetFixture(t *testing.T) (*Runtime, []Option, time.Time) {
	t.Helper()
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	builder, err := catalogs.NewBuilderFrom(manualProviderObservation(t, 100, at).Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Provider("provider")
	provider.Aliases = []catalogs.ProviderID{"provider-alias"}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	provider.ID, provider.Name, provider.Aliases = "other", "Other", nil
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	baseline, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(baseline)
	if err != nil {
		t.Fatal(err)
	}
	source := newStubSource("metadata-reset-baseline")
	source.replies = []SourceRead{testSourceRead(t, "metadata-reset-baseline", payload, at.Add(-time.Hour))}
	options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory()))}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	return connected, options, at
}

func TestSourceResetPreservesPeerRecordsAliasesAndOperatorEdits(t *testing.T) {
	for _, edited := range []bool{false, true} {
		t.Run(fmt.Sprint(edited), func(t *testing.T) {
			connected, options, at := sourceResetFixture(t)
			builder, err := catalogs.NewBuilderFrom(manualProviderObservation(t, 200, at).Catalog)
			if err != nil {
				t.Fatal(err)
			}
			provider, _ := builder.Provider("provider")
			if err := builder.DeleteProvider("provider"); err != nil {
				t.Fatal(err)
			}
			provider.ID = "provider-alias"
			if err := builder.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			provider.ID, provider.Name = "other", "Other"
			provider.Models["model"].Limits.ContextWindow = 300
			if err := builder.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			snapshot, err := catalogs.NewObservationCatalog(builder)
			if err != nil {
				t.Fatal(err)
			}
			original := providerResetObservation(t, sources.ModelsDevHTTPID, snapshot, at, nil)
			if _, err := connected.PublishObservations(t.Context(), original); err != nil {
				t.Fatal(err)
			}
			projected := connected.State().Catalog
			want := int64(100)
			if edited {
				local, err := catalogs.NewBuilderFrom(projected)
				if err != nil {
					t.Fatal(err)
				}
				provider, _ := local.Provider("provider")
				provider.Models["model"].Limits.ContextWindow = 400
				if err := local.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				projected, err = local.Build()
				if err != nil {
					t.Fatal(err)
				}
				want = 400
			}
			operator := providerResetObservation(t, sources.LocalCatalogID, projected, at.Add(time.Minute), nil)
			empty, err := catalogs.NewObservationCatalog(catalogs.NewEmpty())
			if err != nil {
				t.Fatal(err)
			}
			replacement := providerResetObservation(t, sources.ModelsDevHTTPID, empty, at.Add(2*time.Minute), nil)
			resets := []ObservationReset{{SourceID: sources.ModelsDevHTTPID, ProviderID: "provider-alias"}}
			after, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
				resets[0].SourceID = sources.ModelsDevGitID
				return []sources.Observation{operator, replacement}, nil
			}, resets...)
			if err != nil {
				t.Fatal(err)
			}
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := openTestRuntime(t, options...)
			if reopened.State().GenerationID != after.GenerationID {
				t.Fatal("restart changed partial metadata reset")
			}
			for id, limit := range map[catalogs.ProviderID]int64{"provider": want, "other": 300} {
				provider, _ := reopened.State().Catalog.Providers().Get(id)
				if provider.Models["model"].Limits.ContextWindow != limit {
					t.Fatalf("wrong %s value after reset: %+v", id, provider.Models["model"].Limits)
				}
			}
			peer := reopened.State().Catalog.Provenance().FindModelField("other", "model", "limits.context_window")
			if len(peer) != 1 || peer[0].ObservationID != original.ID || peer[0].EvidenceChecksum != original.EvidenceChecksum {
				t.Fatal("peer lost original metadata receipt")
			}
		})
	}
}

func TestSourceResetPreservesOtherMetadataSource(t *testing.T) {
	connected, options, at := providerResetRuntime(t, storage.NewMemory())
	original := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
	builder, err := catalogs.NewBuilderFrom(manualProviderObservation(t, 0, at).Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Provider("provider")
	provider.Models["model"].Limits = &catalogs.ModelLimits{OutputTokens: 300}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	snapshot, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	peer := providerResetObservation(t, sources.ModelsDevGitID, snapshot, at.Add(time.Minute), nil)
	if _, err := connected.PublishObservations(t.Context(), original, peer); err != nil {
		t.Fatal(err)
	}
	replacement := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 0, at).Catalog, at.Add(2*time.Minute), nil)
	state := publishProviderReset(t, connected, replacement, ObservationReset{SourceID: sources.ModelsDevHTTPID})
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != state.GenerationID {
		t.Fatal("restart changed source reset")
	}
	got, _ := reopened.State().Catalog.Providers().Get("provider")
	if got.Models["model"].Limits.ContextWindow != 100 || got.Models["model"].Limits.OutputTokens != 300 {
		t.Fatalf("reset lost a peer source: %+v", got.Models["model"].Limits)
	}
	for _, link := range reopened.layers.buildEvidence.SourceObservations {
		if link.ObservationID == original.ID {
			t.Fatal("whole-source reset retained its old active receipt")
		}
	}
}
