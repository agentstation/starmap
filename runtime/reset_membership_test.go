package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestResetRemovesAcquiredOnlyOfferingFromProjection(t *testing.T) {
	connected, options, at := providerResetRuntime(t, storage.NewMemory())
	original := manualProviderObservation(t, 200, at)
	builder, err := catalogs.NewBuilderFrom(original.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Provider("provider")
	model := catalogs.DeepCopyModel(*provider.Models["model"])
	model.ID = "acquired-only"
	provider.Models[model.ID] = &model
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	observed, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	original = providerResetObservation(t, sources.ProvidersID, observed, at, nil)
	if _, err := connected.PublishObservations(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	acquired, _ := connected.State().Catalog.Provider("provider")
	if acquired.Models[model.ID] == nil {
		t.Fatal("fixture offering did not enter the acquired catalog")
	}
	local := providerResetObservation(t, sources.LocalCatalogID, connected.State().Catalog, at.Add(time.Minute), nil)
	replacement := manualProviderObservation(t, 0, at.Add(2*time.Minute))
	state, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
		return []sources.Observation{local, replacement}, nil
	}, ObservationReset{ProviderID: "provider"})
	if err != nil {
		t.Fatal(err)
	}
	provider, _ = state.Catalog.Provider("provider")
	if provider.Models[model.ID] != nil {
		t.Fatal("unchanged projection restored a reset offering")
	}
	if provider.Models["model"] == nil {
		t.Fatal("reset removed baseline offering")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestRuntime(t, options...)
	provider, _ = restarted.State().Catalog.Provider("provider")
	if provider.Models[model.ID] != nil || restarted.State().GenerationID != state.GenerationID {
		t.Fatal("restart restored reset offering or changed generation")
	}
}

func TestResetCannotIntroduceProviderOutsideSelectedBaseline(t *testing.T) {
	connected, options, at := providerResetRuntime(t, storage.NewMemory())
	builder, err := catalogs.NewBuilderFrom(manualProviderObservation(t, 200, at).Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Provider("provider")
	provider.ID = "acquired-provider"
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	observed, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	original := providerResetObservation(t, sources.ModelsDevHTTPID, observed, at, nil)
	if _, err := connected.PublishObservations(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	if _, err := connected.State().Catalog.Provider(provider.ID); err == nil {
		t.Fatal("acquisition added a provider outside the selected baseline")
	}
	local := providerResetObservation(t, sources.LocalCatalogID, connected.State().Catalog, at.Add(time.Minute), nil)
	empty, err := catalogs.NewObservationCatalog(catalogs.NewEmpty())
	if err != nil {
		t.Fatal(err)
	}
	replacement := providerResetObservation(t, sources.ModelsDevHTTPID, empty, at.Add(2*time.Minute), nil)
	state, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
		return []sources.Observation{local, replacement}, nil
	}, ObservationReset{SourceID: sources.ModelsDevHTTPID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.Catalog.Provider(provider.ID); err == nil {
		t.Fatal("unchanged projection restored a reset provider")
	}
	if _, err := state.Catalog.Provider("provider"); err != nil {
		t.Fatal("reset removed baseline provider")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestRuntime(t, options...)
	if _, err := restarted.State().Catalog.Provider(provider.ID); err == nil || restarted.State().GenerationID != state.GenerationID {
		t.Fatal("restart restored reset provider or changed generation")
	}
}
