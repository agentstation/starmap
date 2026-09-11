package runtime

import (
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestAuthoritySnapshotPreservesSelectedSourceHead(t *testing.T) {
	source, _ := authorityRuntimeFixture(t)
	generation := source.replies[0].Generation.Copy()
	layers := layerSet{requireAuthority: true, source: &sourceLayer{
		Manifest: &generation.Manifest, Payload: generation.Payload,
		GenerationID: generation.Manifest.GenerationID, Checksum: generation.Manifest.Payload.Checksum,
		PublishedAt: generation.Manifest.GeneratedAt,
	}}
	state, err := layers.build(t.Context(), starmap.CatalogState{})
	if err != nil {
		t.Fatal(err)
	}
	if state.AuthorityHead != generation.Manifest.AuthorityHead {
		t.Fatal("selected authority source lost its snapshot head")
	}
	layers.requireAuthority = false
	ordinary, err := layers.build(t.Context(), starmap.CatalogState{})
	if err != nil {
		t.Fatal(err)
	}
	if ordinary.AuthorityHead != (catalogs.CatalogAuthorityHead{}) {
		t.Fatal("ordinary reconciliation claimed upstream authority")
	}
}
