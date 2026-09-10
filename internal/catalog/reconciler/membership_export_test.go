package reconciler

import (
	"reflect"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestMembershipExportRetainsIndependentReceiptsAndOwnsCopies(t *testing.T) {
	complete := membershipObservation(t, "account-a", sources.ProviderMembershipScope, false, false, true, false, 1)
	partial := membershipObservation(t, "account-a", sources.ProviderMembershipScope, false, true, false, false, 2)
	peer := membershipObservation(t, "account-b", sources.ProviderMembershipScope, false, true, true, false, 1)
	observations := []sources.Observation{partial, peer, complete}
	state, err := ResolveMembership(t.Context(), observations)
	if err != nil {
		t.Fatal(err)
	}
	scopes, err := state.ExportScopes("publisher")
	if err != nil {
		t.Fatal(err)
	}
	if len(scopes) != 2 {
		t.Fatalf("scope count = %d", len(scopes))
	}
	first := scopes[0]
	if first.AccountID != "account-a" || first.Inventory == nil || len(first.Inventory.ModelIDs) != 0 || first.Inventory.ObservationID != complete.ID {
		t.Fatal("complete empty account inventory changed")
	}
	if len(first.Additions) != 1 || first.Additions[0].ObservationID != partial.ID {
		t.Fatal("partial positive lost its original receipt")
	}
	if scopes[1].Inventory.ObservationID != peer.ID {
		t.Fatal("peer inventory lost its original receipt")
	}
	links := []catalogs.SourceObservationLink{complete.Link(), partial.Link(), peer.Link()}
	if err := catalogs.ValidateMembershipEvidence(scopes, links); err != nil {
		t.Fatal(err)
	}
	repeated, err := state.ExportScopes("publisher")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scopes, repeated) {
		t.Fatal("export is not deterministic")
	}
	scopes[0].Inventory.ObservationID = "changed"
	scopes[0].Additions[0].ModelID = "changed"
	again, err := state.ExportScopes("publisher")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, repeated) {
		t.Fatal("caller mutation reached resolved membership")
	}
	if _, err := state.ExportScopes(""); err == nil {
		t.Fatal("export accepted a missing publisher")
	}
}

func TestMembershipExportDoesNotInventCompleteAuthority(t *testing.T) {
	for _, authority := range []sources.ProviderMembershipAuthority{sources.ProviderMembershipEvidenceOnly, sources.ProviderMembershipScope, sources.ProviderMembershipProvider} {
		t.Run(string(authority), func(t *testing.T) {
			observation := membershipObservation(t, "public", authority, true, true, true, false, 1)
			state, err := ResolveMembership(t.Context(), []sources.Observation{observation})
			if err != nil {
				t.Fatal(err)
			}
			scopes, err := state.ExportScopes("publisher")
			if err != nil {
				t.Fatal(err)
			}
			if (scopes[0].Inventory != nil) != (authority != sources.ProviderMembershipEvidenceOnly) {
				t.Fatal("export changed inventory authority")
			}
			if err := catalogs.ValidateMembershipEvidence(scopes, []catalogs.SourceObservationLink{observation.Link()}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
