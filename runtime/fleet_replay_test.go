package runtime

import (
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func fleetReplayFixture(t *testing.T) (FleetSnapshot, layerSet) {
	t.Helper()
	p := fleetTestPublication(t)
	catalog, err := catalogs.DecodeCatalogGeneration(p.Generation)
	if err != nil {
		t.Fatal(err)
	}
	local := layerSet{publisherID: "deployment", embedded: starmap.CatalogState{GenerationID: p.Generation.Manifest.GenerationID,
		PayloadChecksum: p.Generation.Manifest.Payload.Checksum, Catalog: catalog, GeneratedAt: p.Generation.Manifest.GeneratedAt}}
	p.Recovery.Data, err = encodeFleetRecovery(t.Context(), local)
	if err != nil {
		t.Fatal(err)
	}
	p.Recovery.Checksum = fleetRecoveryChecksum(p.Recovery.Data)
	return FleetSnapshot{Head: p.nextHead(), Publication: p}, local
}

func TestFleetReplayRequiresEquivalentBaselineAndPolicy(t *testing.T) {
	snapshot, local := fleetReplayFixture(t)
	if _, err := recoverFleetLayers(t.Context(), snapshot, local); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name   string
		change func(*layerSet)
	}{
		{"baseline-id", func(l *layerSet) { l.embedded.GenerationID = "other" }},
		{"baseline-bytes", func(l *layerSet) { l.embedded.PayloadChecksum = "other" }},
		{"authority", func(l *layerSet) { l.requireAuthority = true }},
		{"explicit-no-providers", func(l *layerSet) {
			l.providerBindings = &providerBindingPolicy{bindings: map[string]sources.ProviderAcquisitionBinding{}}
		}},
		{"explicit-no-sources", func(l *layerSet) { l.acquisitionSources = &acquisitionSourcePolicy{ids: []sources.ID{}} }},
		{"publisher-aliases", func(l *layerSet) { l.publisherAliases = []string{"different"} }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			incompatible := local
			scenario.change(&incompatible)
			if _, err := recoverFleetLayers(t.Context(), snapshot, incompatible); err == nil {
				t.Fatal("recovery accepted different acquisition semantics")
			}
		})
	}
}

func TestFleetReplayRejectsInputsThatDoNotReproduceCatalog(t *testing.T) {
	snapshot, local := fleetReplayFixture(t)
	observation := manualProviderObservation(t, 300, local.embedded.GeneratedAt)
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{observation})
	if err != nil {
		t.Fatal(err)
	}
	changed := local
	changed.manual = &manualBatch{observations: prepared}
	data, err := encodeFleetRecovery(t.Context(), changed)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Publication.Recovery.Data = data
	snapshot.Publication.Recovery.Checksum = fleetRecoveryChecksum(data)
	snapshot.Head = snapshot.Publication.nextHead()
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := recoverFleetLayers(t.Context(), snapshot, local); err == nil {
		t.Fatal("recovery accepted valid inputs for a different effective catalog")
	}
	if local.manual != nil {
		t.Fatal("failed recovery changed local inputs")
	}
}
