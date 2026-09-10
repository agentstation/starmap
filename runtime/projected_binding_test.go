package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProjectedProviderFactsRespectActiveBindingAndOperatorEdits(t *testing.T) {
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	binding := *scopedProviderLayer(t, "binding", "1", at).Receipt.ProviderBinding
	baseline := starmap.CatalogState{Catalog: manualProviderObservation(t, 100, at).Catalog, GenerationID: "baseline", GeneratedAt: at}
	observedBuilder, err := catalogs.NewBuilderFrom(manualProviderObservation(t, 200, at.Add(time.Minute)).Catalog)
	if err != nil {
		t.Fatal(err)
	}
	observedProvider, _ := observedBuilder.Provider("provider")
	observedProvider.Name = "Observed provider"
	if err := observedBuilder.SetProvider(observedProvider); err != nil {
		t.Fatal(err)
	}
	observedCatalog, err := catalogs.NewObservationCatalog(observedBuilder)
	if err != nil {
		t.Fatal(err)
	}
	baselineProvider, _ := baseline.Catalog.Provider("provider")
	observation, err := sources.NewObservation(sources.ProvidersID, observedCatalog, sources.ObservationMetadata{
		ObservedAt: at.Add(time.Minute), ProviderBinding: &binding, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	layer, err := NewProviderLayer("provider", observation)
	if err != nil {
		t.Fatal(err)
	}
	active := &providerBindingPolicy{bindings: map[string]sources.ProviderAcquisitionBinding{binding.ID: binding}}
	original := layerSet{publisherID: "test-runtime", providerBindings: active}
	original.setProvider(layer)
	accepted, err := original.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name           string
		active, edited bool
		want           int64
		wantName       string
	}{
		{name: "active", active: true, want: 200, wantName: "Observed provider"},
		{name: "retired", want: 100, wantName: baselineProvider.Name},
		{name: "new-revision", want: 100, wantName: baselineProvider.Name},
		{name: "different-provider", want: 100, wantName: baselineProvider.Name},
		{name: "undeclared-binding", want: 100, wantName: baselineProvider.Name},
		{name: "operator-edit", edited: true, want: 300, wantName: "Operator provider"},
	} {
		t.Run(test.name, func(t *testing.T) {
			projected := accepted.Catalog
			if test.edited {
				builder, err := catalogs.NewBuilderFrom(projected)
				if err != nil {
					t.Fatal(err)
				}
				provider, _ := builder.Provider("provider")
				provider.Name = "Operator provider"
				provider.Models["model"].Limits.ContextWindow = 300
				if err := builder.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				projected, err = builder.Build()
				if err != nil {
					t.Fatal(err)
				}
			}
			local, err := sources.NewObservation(sources.LocalCatalogID, projected, sources.ObservationMetadata{ObservedAt: at.Add(2 * time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded})
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := prepareManualObservations(t.Context(), []sources.Observation{local})
			if err != nil {
				t.Fatal(err)
			}
			policy := &providerBindingPolicy{bindings: map[string]sources.ProviderAcquisitionBinding{}}
			if test.active {
				policy = active
			}
			switch test.name {
			case "new-revision":
				changed := binding
				changed.Revision = "2"
				policy.bindings[changed.ID] = changed
			case "different-provider":
				changed := binding
				changed.ProviderID = "peer"
				policy.bindings[changed.ID] = changed
			case "undeclared-binding":
				policy = nil
			}
			replayed := layerSet{publisherID: "test-runtime", providerBindings: policy, manual: &manualBatch{observations: prepared}}
			state, err := replayed.build(t.Context(), baseline)
			if err != nil {
				t.Fatal(err)
			}
			provider, err := state.Catalog.Provider("provider")
			if err != nil {
				t.Fatal(err)
			}
			if provider.Name != test.wantName {
				t.Fatalf("projected provider name = %q, want %q", provider.Name, test.wantName)
			}
			if provider.Models["model"].Limits.ContextWindow != test.want {
				t.Fatalf("projected limit = %v, want %d, error %v", provider.Models["model"].Limits, test.want, err)
			}
		})
	}
}
