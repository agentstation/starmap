package catalogs

import (
	"testing"
	"time"
)

func storedMembershipScope() ProviderMembershipScope {
	scope := testMembershipScope()
	scope.Inventory = &MembershipInventory{ObservationID: "complete", ObservedAt: time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC), ModelIDs: []string{"model"}}
	return scope
}

func TestCatalogMembershipScopeOwnsCopiesAndIndexesReads(t *testing.T) {
	builder := NewEmpty()
	scope := storedMembershipScope()
	if err := builder.SetMembershipScopes([]ProviderMembershipScope{scope}); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	key := membershipKey(scope)
	scope.Inventory.ModelIDs[0] = "caller"
	read := catalog.MembershipScopes()
	read[0].Inventory.ModelIDs[0] = "reader"
	if err := builder.SetMembershipScopes(nil); err != nil {
		t.Fatal(err)
	}
	if present, known := catalog.ScopeMembership(key, "provider", "model"); !present || !known {
		t.Fatal("external mutation changed immutable membership")
	}
	if present, known := catalog.ScopeMembership(key, "provider", "absent"); present || !known {
		t.Fatal("known absence changed")
	}
	if present, known := catalog.ScopeMembership(key, "other", "model"); present || known {
		t.Fatal("scope crossed provider boundary")
	}
	for _, missing := range []MembershipScopeKey{{"other", key.BindingID, key.BindingRevision}, {key.PublisherID, key.BindingID, "2"}} {
		if present, known := catalog.ScopeMembership(missing, "provider", "model"); present || known {
			t.Fatal("scope crossed publisher or revision boundary")
		}
	}
	if allocations := testing.AllocsPerRun(100, func() { catalog.ScopeMembership(key, "provider", "model") }); allocations != 0 {
		t.Fatalf("membership lookup allocated %v times", allocations)
	}
	copied, err := NewBuilderFrom(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied.MembershipScopes()) != 1 {
		t.Fatal("builder copy lost membership")
	}
}

func TestCatalogMembershipScopeRejectsConflictingGenericMerge(t *testing.T) {
	one, two := NewEmpty(), NewEmpty()
	scope := storedMembershipScope()
	if err := one.SetMembershipScopes([]ProviderMembershipScope{scope}); err != nil {
		t.Fatal(err)
	}
	scope.Inventory.ModelIDs = []string{}
	if err := two.SetMembershipScopes([]ProviderMembershipScope{scope}); err != nil {
		t.Fatal(err)
	}
	if err := one.MergeWith(two); err == nil {
		t.Fatal("generic merge selected conflicting authority")
	}
	if len(one.MembershipScopes()[0].Inventory.ModelIDs) != 1 {
		t.Fatal("rejected merge changed accepted membership")
	}
	if err := one.SetMembershipScopes([]ProviderMembershipScope{scope, scope}); err == nil {
		t.Fatal("duplicate scope accepted")
	}
	if len(one.MembershipScopes()[0].Inventory.ModelIDs) != 1 {
		t.Fatal("rejected replacement changed accepted membership")
	}
}

func TestCatalogMembershipScopeSurvivesPayloadRoundTrip(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetMembershipScopes([]ProviderMembershipScope{storedMembershipScope()}); err != nil {
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
	if len(decoded.MembershipScopes()) != 1 {
		t.Fatal("catalog transport discarded effective membership scope")
	}
}

func TestMembershipScopeChangesSemanticIdentityWithoutModelChanges(t *testing.T) {
	builder := NewEmpty()
	scope := storedMembershipScope()
	if err := builder.SetMembershipScopes([]ProviderMembershipScope{scope}); err != nil {
		t.Fatal(err)
	}
	initial, err := CatalogSemanticChecksum(builder)
	if err != nil {
		t.Fatal(err)
	}
	scope.Inventory.ModelIDs = []string{}
	if err := builder.SetMembershipScopes([]ProviderMembershipScope{scope}); err != nil {
		t.Fatal(err)
	}
	removed, err := CatalogSemanticChecksum(builder)
	if err != nil {
		t.Fatal(err)
	}
	if removed == initial {
		t.Fatal("scope removal reused the prior semantic identity")
	}
	scope.Inventory.ObservationID = "renewed"
	scope.Inventory.ObservedAt = scope.Inventory.ObservedAt.Add(time.Minute)
	if err := builder.SetMembershipScopes([]ProviderMembershipScope{scope}); err != nil {
		t.Fatal(err)
	}
	renewed, err := CatalogSemanticChecksum(builder)
	if err != nil {
		t.Fatal(err)
	}
	if renewed == removed {
		t.Fatal("scope evidence renewal reused the prior semantic identity")
	}
}
