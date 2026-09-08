package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestSourceResetRestoresBaselineAcrossRestart(t *testing.T) {
	connected, options, at := providerResetRuntime(t, storage.NewMemory())
	original := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
	if _, err := connected.PublishObservations(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	replacement := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 0, at).Catalog, at.Add(time.Minute), nil)
	state, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
		return []sources.Observation{replacement}, nil
	}, ObservationReset{SourceID: sources.ModelsDevHTTPID})
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := state.Catalog.Providers().Get("provider")
	if provider.Models["model"].Limits.ContextWindow != 100 {
		t.Fatal("source reset retained previous acquisition data")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != state.GenerationID {
		t.Fatal("restart changed source reset identity")
	}
	provider, _ = reopened.State().Catalog.Providers().Get("provider")
	if provider.Models["model"].Limits.ContextWindow != 100 {
		t.Fatal("restart restored discarded metadata")
	}
}

func TestSourceResetPreservesSelectedBaselineWithMatchingReceipt(t *testing.T) {
	connected, options, at := providerResetRuntime(t, storage.NewMemory())
	original := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
	if _, err := connected.PublishObservations(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	retained := *connected.layers.source
	payload, err := catalogs.EncodeCatalogPayload(connected.State().Catalog)
	if err != nil {
		t.Fatal(err)
	}
	retained.Payload = payload
	retained.Checksum = catalogs.DescribeCatalogPayload(payload).Checksum
	retained.GenerationID = "upstream-with-matching-receipt"
	if _, err := connected.publishInputChanges(t.Context(), &retained, nil, connected.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	replacement := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 0, at).Catalog, at.Add(time.Minute), nil)
	state := publishProviderReset(t, connected, replacement, ObservationReset{SourceID: sources.ModelsDevHTTPID})
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != state.GenerationID {
		t.Fatal("restart changed selected baseline")
	}
	provider, _ := reopened.State().Catalog.Providers().Get("provider")
	if provider.Models["model"].Limits.ContextWindow != 200 {
		t.Fatal("local reset cleared a selected upstream fact")
	}
}
