package catalogs

import "testing"

func TestCanonicalAliasAuthoritySuccessorScopesHistory(t *testing.T) {
	previous, err := NewCanonicalAliasIndex([]ModelDefinitionID{"author/current"}, canonicalAlias("author/old", "author/current", CanonicalAliasActive))
	if err != nil {
		t.Fatal(err)
	}
	base := permissionEnvelopeFixture().Head
	other := base
	other.AuthorityID = "another-enterprise"
	policy := base
	policy.PolicyID = "another-policy"
	invalid := other
	invalid.PayloadChecksum = "invalid"
	for _, scenario := range []struct {
		name           string
		previous, next CatalogAuthorityHead
		wantErr        bool
	}{
		{"ordinary history", CatalogAuthorityHead{}, CatalogAuthorityHead{}, true},
		{"same authority", base, base, true},
		{"new authority", base, other, false},
		{"new policy", base, policy, false},
		{"initial authority", CatalogAuthorityHead{}, base, false},
		{"authority removal", base, CatalogAuthorityHead{}, true},
		{"invalid replacement", base, invalid, true},
		{"invalid predecessor", invalid, base, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			err := previous.ValidateAuthoritySuccessor(nil, scenario.previous, scenario.next)
			if (err != nil) != scenario.wantErr {
				t.Fatalf("alias history transition: %v, want error %t", err, scenario.wantErr)
			}
		})
	}
	if err := previous.ValidateAuthoritySuccessor(previous, base, base); err != nil {
		t.Fatalf("unchanged authority rejected retained history: %v", err)
	}
}
