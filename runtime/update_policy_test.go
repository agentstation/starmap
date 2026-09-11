package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestOfflineVerifiedLocalImportSurvivesRestart(t *testing.T) {
	options := []Option{WithCatalogSource("embedded"), WithCatalogNetworkMode("offline"),
		WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory()))}
	connected := openTestRuntime(t, options...)
	observation := manualTestObservation(t, "offline-import", time.Date(2026, 9, 11, 20, 0, 0, 0, time.UTC), false)
	before := connected.State()
	prepare := func(context.Context, ObservationInputs) (ObservationUpdate, error) {
		return ObservationUpdate{Observations: []sources.Observation{observation}}, nil
	}
	preview, err := connected.PreviewAcquisition(t.Context(), prepare, sources.ReleaseArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	if preview.GenerationID == before.GenerationID || connected.State().GenerationID != before.GenerationID {
		t.Fatal("local preview changed accepted state or lost the proposed import")
	}
	if _, err := connected.UpdateAcquisition(t.Context(), prepare, sources.LocalCatalogID); err == nil {
		t.Fatal("local preparation accepted a result from an undeclared source")
	}
	if connected.State().GenerationID != before.GenerationID {
		t.Fatal("invalid local preparation changed the accepted generation")
	}
	imported, err := connected.PublishObservations(t.Context(), observation)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := imported.Catalog.Provider("manual-provider")
	if err != nil || provider.Models["offline-import"] == nil {
		t.Fatal("offline import lost its verified offering")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != imported.GenerationID || reopened.State().PayloadChecksum != imported.PayloadChecksum {
		t.Fatal("offline restart changed the imported generation")
	}
}
