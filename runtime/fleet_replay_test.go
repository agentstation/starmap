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
	local := layerSet{publisherID: "deployment", embeddedManifest: &p.Generation.Manifest, embedded: starmap.CatalogState{GenerationID: p.Generation.Manifest.GenerationID,
		PayloadChecksum: p.Generation.Manifest.Payload.Checksum, Catalog: catalog, GeneratedAt: p.Generation.Manifest.GeneratedAt}}
	p.Recovery.Data, err = encodeFleetRecoveryWithPin(t.Context(), local, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.Recovery.Checksum = fleetRecoveryChecksum(p.Recovery.Data)
	return FleetSnapshot{Head: p.nextHead(), Publication: p}, local
}

func TestFleetReplayRequiresEquivalentPolicy(t *testing.T) {
	snapshot, local := fleetReplayFixture(t)
	if _, _, err := recoverFleetState(t.Context(), snapshot, local); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name   string
		change func(*layerSet)
	}{
		{"authority", func(l *layerSet) { l.requireAuthority = true }},
		{"explicit-no-providers", func(l *layerSet) {
			l.providerBindings = &providerBindingPolicy{bindings: map[string]sources.ProviderAcquisitionBinding{}}
		}},
		{"explicit-no-sources", func(l *layerSet) { l.acquisitionSources = &acquisitionSourcePolicy{ids: []sources.ID{}} }},
		{"source-capability", func(l *layerSet) {
			l.sourceConfiguration = []sources.SourceActivity{{Source: sources.ProvidersID, Supported: true}}
		}},
		{"publisher-aliases", func(l *layerSet) { l.publisherAliases = []string{"different"} }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			incompatible := local
			scenario.change(&incompatible)
			if _, _, err := recoverFleetState(t.Context(), snapshot, incompatible); err == nil {
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
	data, err := encodeFleetRecoveryWithPin(t.Context(), changed, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Publication.Recovery.Data = data
	snapshot.Publication.Recovery.Checksum = fleetRecoveryChecksum(data)
	snapshot.Head = snapshot.Publication.nextHead()
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := recoverFleetState(t.Context(), snapshot, local); err == nil {
		t.Fatal("recovery accepted valid inputs for a different effective catalog")
	}
	if local.manual != nil {
		t.Fatal("failed recovery changed local inputs")
	}
}

func TestFleetReplayRetainsBaselineAcrossBinaryUpgrade(t *testing.T) {
	snapshot, original := fleetReplayFixture(t)
	for _, name := range []string{"embedded-only", "configured-upstream"} {
		t.Run(name, func(t *testing.T) {
			snapshot, original := snapshot, original
			if name == "configured-upstream" {
				g := snapshot.Publication.Generation
				original.source = &sourceLayer{Identity: "upstream", GenerationID: g.Manifest.GenerationID, Checksum: g.Manifest.Payload.Checksum, Payload: g.Payload, Manifest: &g.Manifest, PublishedAt: g.Manifest.GeneratedAt}
				raw, err := encodeFleetRecoveryWithPin(t.Context(), original, nil)
				if err != nil {
					t.Fatal(err)
				}
				snapshot.Publication.Recovery.Data = raw
				snapshot.Publication.Recovery.Checksum = fleetRecoveryChecksum(raw)
				snapshot.Head = snapshot.Publication.nextHead()
			}
			upgraded := original
			next := aliasGeneration(t, "new-binary")
			decoded, err := catalogs.DecodeCatalogGeneration(next)
			if err != nil {
				t.Fatal(err)
			}
			upgraded.embedded = starmap.CatalogState{Catalog: decoded, GenerationID: next.Manifest.GenerationID, PayloadChecksum: next.Manifest.Payload.Checksum, GeneratedAt: next.Manifest.GeneratedAt}
			upgraded.embeddedManifest = &next.Manifest
			restored, _, err := recoverFleetState(t.Context(), snapshot, upgraded)
			if err != nil {
				t.Fatalf("binary upgrade cannot replay retained baseline: %v", err)
			}
			if restored.embedded.GenerationID != original.embedded.GenerationID || restored.embedded.PayloadChecksum != original.embedded.PayloadChecksum {
				t.Fatal("binary replaced the fleet baseline")
			}
			if restored.embeddedManifest == nil || restored.embeddedManifest.GenerationID != original.embedded.GenerationID {
				t.Fatal("recovery lost baseline manifest")
			}
		})
	}
}
