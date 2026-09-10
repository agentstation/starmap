package catalogs

import "testing"

func TestCanonicalRemovalFollowsRetainedRenameIdentity(t *testing.T) {
	for _, state := range []CanonicalAliasState{CanonicalAliasActive, CanonicalAliasRemoved} {
		t.Run(string(state), func(t *testing.T) {
			builder := NewEmpty()
			setTestReadViewDefinition(t, builder, "current", "Current Model")
			setTestReadViewDefinition(t, builder, "unrelated", "Unrelated Model")
			if err := builder.SetCanonicalAliasRecords([]CanonicalAlias{
				canonicalAlias("author/old", "author/middle", state),
				canonicalAlias("author/middle", "author/current", CanonicalAliasRemoved),
			}); err != nil {
				t.Fatal(err)
			}
			target, err := NewCanonicalRemovalTarget("author/old")
			if err != nil {
				t.Fatal(err)
			}
			if err := builder.SetRemovalPolicies([]CatalogRemovalPolicy{{PublisherID: "operator", Targets: []CatalogRemovalTarget{target}}}); err != nil {
				t.Fatal(err)
			}
			catalog, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			payload, err := EncodeCatalogPayload(catalog)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeCatalogPayload(payload)
			if err != nil {
				t.Fatal(err)
			}
			for _, snapshot := range []*Catalog{catalog, decoded} {
				if !snapshot.Removals().ContainsCanonical("author/current") {
					t.Fatal("canonical rename restored an explicitly removed model")
				}
				if snapshot.Removals().ContainsCanonical("author/unrelated") || snapshot.Removals().ContainsAlias("author/old") {
					t.Fatal("canonical removal widened its selector")
				}
				policies := snapshot.RemovalPolicies()
				if len(policies) != 1 || len(policies[0].Targets) != 1 || policies[0].Targets[0].DefinitionID != "author/old" {
					t.Fatal("rename changed the operator's original removal target")
				}
				if allocations := testing.AllocsPerRun(100, func() {
					if !snapshot.Removals().ContainsCanonical("author/current") {
						t.Fatal("removal lookup changed")
					}
				}); allocations != 0 {
					t.Fatalf("canonical removal lookup allocated %v times", allocations)
				}
			}
		})
	}
}
