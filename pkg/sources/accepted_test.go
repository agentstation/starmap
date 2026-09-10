package sources

import "testing"

func TestAcceptedSourceStateValidationAndOwnership(t *testing.T) {
	original := AcceptedSourceState{GenerationID: "active", Sources: []ID{LocalCatalogID, ProvidersID}}
	if !original.Valid() {
		t.Fatal("valid accepted snapshot rejected")
	}
	cloned := original.Clone()
	cloned.Sources[0] = ModelsDevHTTPID
	if original.Sources[0] != LocalCatalogID {
		t.Fatal("clone aliases accepted source list")
	}
	for _, snapshot := range []AcceptedSourceState{
		{Sources: []ID{LocalCatalogID}},
		{GenerationID: "active", Sources: []ID{LocalCatalogID, LocalCatalogID}},
		{GenerationID: "active", Sources: []ID{EmbeddedCatalogID}},
		{GenerationID: "active", Sources: []ID{"private-source"}},
	} {
		if snapshot.Valid() {
			t.Fatalf("invalid snapshot accepted: %+v", snapshot)
		}
	}
	empty := AcceptedSourceState{GenerationID: "active"}.Clone()
	if !empty.Valid() || empty.Sources == nil {
		t.Fatal("known empty snapshot lost its explicit source list")
	}
}
