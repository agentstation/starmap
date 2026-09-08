package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func providerResetObservation(t *testing.T, source sources.ID, catalog *catalogs.Catalog, at time.Time, binding *sources.ProviderAcquisitionBinding) sources.Observation {
	t.Helper()
	observation, err := sources.NewObservation(source, catalog, sources.ObservationMetadata{ObservedAt: at, ProviderBinding: binding, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Status: sources.ObservationStatusSucceeded, Completeness: sources.ObservationCompletenessComplete})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func TestProviderResetPreservesPeerBinding(t *testing.T) {
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	one := *scopedProviderLayer(t, "one", "1", at).Receipt.ProviderBinding
	two := *scopedProviderLayer(t, "two", "1", at).Receipt.ProviderBinding
	connected, options, _ := providerResetRuntime(t, storage.NewMemory(), WithProviderBindings(one, two))
	original := providerResetObservation(t, sources.ProvidersID, manualProviderObservation(t, 200, at).Catalog, at, &one)
	builder, err := catalogs.NewBuilderFrom(manualProviderObservation(t, 0, at).Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Provider("provider")
	provider.Models["model"].Limits = &catalogs.ModelLimits{OutputTokens: 300}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	peerCatalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	peer := providerResetObservation(t, sources.ProvidersID, peerCatalog, at.Add(time.Minute), &two)
	if _, err := connected.PublishObservations(t.Context(), original, peer); err != nil {
		t.Fatal(err)
	}
	replacement := providerResetObservation(t, sources.ProvidersID, manualProviderObservation(t, 0, at).Catalog, at.Add(2*time.Minute), &one)
	after := publishProviderReset(t, connected, replacement, ProviderObservationReset{ProviderID: "provider", BindingID: one.ID, BindingRevision: one.Revision})
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != after.GenerationID {
		t.Fatal("restart changed scoped reset")
	}
	providerPtr, _ := reopened.State().Catalog.Providers().Get("provider")
	limits := providerPtr.Models["model"].Limits
	if limits.ContextWindow != 100 || limits.OutputTokens != 300 {
		t.Fatalf("reset lost peer scope or kept cleared scope: %+v", limits)
	}
	for _, link := range reopened.layers.buildEvidence.SourceObservations {
		if link.ObservationID == original.ID {
			t.Fatal("reset scope remains in active source evidence")
		}
	}
}

func TestProviderResetProjectedFactsAndOperatorEdits(t *testing.T) {
	for _, edited := range []bool{false, true} {
		t.Run(fmt.Sprint(edited), func(t *testing.T) {
			connected, options, at := providerResetRuntime(t, storage.NewMemory())
			original := manualProviderObservation(t, 200, at)
			if _, err := connected.PublishObservations(t.Context(), original); err != nil {
				t.Fatal(err)
			}
			projected := connected.State().Catalog
			want := int64(100)
			if edited {
				builder, err := catalogs.NewBuilderFrom(projected)
				if err != nil {
					t.Fatal(err)
				}
				provider, _ := builder.Provider("provider")
				provider.Models["model"].Limits.ContextWindow = 300
				if err := builder.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				projected, err = builder.Build()
				if err != nil {
					t.Fatal(err)
				}
				want = 300
			}
			local := providerResetObservation(t, sources.LocalCatalogID, projected, at.Add(time.Minute), nil)
			replacement := manualProviderObservation(t, 0, at.Add(2*time.Minute))
			resets := []ProviderObservationReset{{ProviderID: "provider"}}
			state, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
				resets[0].ProviderID = "caller-change"
				return []sources.Observation{local, replacement}, nil
			}, resets...)
			if err != nil {
				t.Fatal(err)
			}
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := openTestRuntime(t, options...)
			if reopened.State().GenerationID != state.GenerationID {
				t.Fatal("restart changed reset with a local projection")
			}
			provider, _ := reopened.State().Catalog.Providers().Get("provider")
			if provider.Models["model"].Limits.ContextWindow != want {
				t.Fatalf("wrong projected field after reset: %+v", provider.Models["model"].Limits)
			}
		})
	}
}

func TestProviderResetHistoryVersionAndReplacementValidation(t *testing.T) {
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name    string
		version int
		resets  []ProviderObservationReset
		valid   bool
	}{
		{name: "legacy", version: 1, valid: true},
		{name: "current", version: 2, resets: []ProviderObservationReset{{ProviderID: "provider"}}, valid: true},
		{name: "legacy-reset", version: 1, resets: []ProviderObservationReset{{ProviderID: "provider"}}},
		{name: "missing-replacement", version: 2, resets: []ProviderObservationReset{{ProviderID: "other"}}},
		{name: "future", version: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, err := newLayerStore(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			observations, err := prepareManualObservations(t.Context(), []sources.Observation{manualProviderObservation(t, 200, at)})
			if err != nil {
				t.Fatal(err)
			}
			input, err := store.stageInput(t.Context(), observations[0])
			if err != nil {
				t.Fatal(err)
			}
			reference, err := store.stageInput(t.Context(), manualBatchRecord{Version: test.version, Observations: []string{input}, Resets: test.resets})
			if err != nil {
				t.Fatal(err)
			}
			_, err = store.readManualHistory(t.Context(), reference)
			if (err == nil) != test.valid {
				t.Fatalf("history validation = %v, want valid %v", err, test.valid)
			}
			if err := store.writeContext(t.Context(), store.directory, manualHistoryName, manualHistoryHead{Version: test.version, Batch: reference}); err != nil {
				t.Fatal(err)
			}
			_, err = store.loadManualHistory(t.Context())
			if (err == nil) != test.valid {
				t.Fatalf("head and history validation = %v, want valid %v", err, test.valid)
			}
		})
	}
}

func TestProviderResetProjectedLimitPresence(t *testing.T) {
	for _, test := range []struct {
		name     string
		original int64
		edit     func(*catalogs.Model)
		want     int64
	}{
		{name: "unchanged-explicit-zero", original: 0, want: 100},
		{name: "operator-explicit-zero", original: 200, edit: func(model *catalogs.Model) {
			model.Limits.Set(catalogs.ModelLimitContextWindow, 0)
		}, want: 0},
		{name: "operator-unknown", original: 200, edit: func(model *catalogs.Model) {
			model.Limits.SetUnknown(catalogs.ModelLimitContextWindow)
		}, want: 100},
		{name: "operator-missing", original: 200, edit: func(model *catalogs.Model) {
			model.Limits = nil
		}, want: 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			connected, options, at := providerResetRuntime(t, storage.NewMemory())
			builder, err := catalogs.NewBuilderFrom(manualProviderObservation(t, 200, at).Catalog)
			if err != nil {
				t.Fatal(err)
			}
			provider, _ := builder.Provider("provider")
			provider.Models["model"].Limits.Set(catalogs.ModelLimitContextWindow, test.original)
			if err := builder.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			observed, err := catalogs.NewObservationCatalog(builder)
			if err != nil {
				t.Fatal(err)
			}
			original := providerResetObservation(t, sources.ProvidersID, observed, at, nil)
			if _, err := connected.PublishObservations(t.Context(), original); err != nil {
				t.Fatal(err)
			}
			provider, _ = connected.State().Catalog.Provider("provider")
			if got, presence := provider.Models["model"].Limits.Value(catalogs.ModelLimitContextWindow); got != test.original || presence != catalogs.ValueKnown {
				t.Fatalf("original limit = %d, %v; want known %d", got, presence, test.original)
			}
			builder, err = catalogs.NewBuilderFrom(connected.State().Catalog)
			if err != nil {
				t.Fatal(err)
			}
			provider, _ = builder.Provider("provider")
			if test.edit != nil {
				test.edit(provider.Models["model"])
			}
			if err := builder.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			projected, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			local := providerResetObservation(t, sources.LocalCatalogID, projected, at.Add(time.Minute), nil)
			replacement := manualProviderObservation(t, 0, at.Add(2*time.Minute))
			state, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
				return []sources.Observation{local, replacement}, nil
			}, ObservationReset{ProviderID: "provider"})
			if err != nil {
				t.Fatal(err)
			}
			provider, _ = state.Catalog.Provider("provider")
			if got, presence := provider.Models["model"].Limits.Value(catalogs.ModelLimitContextWindow); got != test.want || presence != catalogs.ValueKnown {
				t.Fatalf("reset limit = %d, %v; want known %d", got, presence, test.want)
			}
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			restarted := openTestRuntime(t, options...)
			provider, _ = restarted.State().Catalog.Provider("provider")
			if got, presence := provider.Models["model"].Limits.Value(catalogs.ModelLimitContextWindow); got != test.want || presence != catalogs.ValueKnown {
				t.Fatalf("restarted limit = %d, %v; want known %d", got, presence, test.want)
			}
			if restarted.State().GenerationID != state.GenerationID {
				t.Fatal("restart changed the reset generation")
			}
		})
	}
}
