package types_test

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	legacytypes "github.com/agentstation/starmap/pkg/types"
)

func TestCompatibilityAliases(t *testing.T) {
	t.Parallel()

	var sourceID catalogmeta.SourceID = legacytypes.ProvidersID
	if sourceID != catalogmeta.ProvidersID {
		t.Fatalf("source alias = %q, want %q", sourceID, catalogmeta.ProvidersID)
	}

	var revision catalogmeta.ObservationRevision = legacytypes.ObservationRevision{
		Kind:  legacytypes.ObservationRevisionKindContentDigest,
		Value: "sha256:test",
	}
	if revision.Kind != catalogmeta.ObservationRevisionKindContentDigest {
		t.Fatalf("revision kind = %q", revision.Kind)
	}

	if got := legacytypes.SourceIDs(); len(got) != len(catalogmeta.SourceIDs()) {
		t.Fatalf("legacy source IDs = %d, canonical source IDs = %d", len(got), len(catalogmeta.SourceIDs()))
	}
}
