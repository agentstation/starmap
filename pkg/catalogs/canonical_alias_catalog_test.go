package catalogs

import (
	"encoding/json"
	"testing"
)

func TestCanonicalAliasCatalogRetainsRenameThroughPayloadAndCopy(t *testing.T) {
	b := NewEmpty()
	setTestReadViewDefinition(t, b, "current", "Current Model")
	input := []CanonicalAlias{canonicalAlias("author/old", "author/current", CanonicalAliasActive)}
	if err := b.SetCanonicalAliasRecords(input); err != nil {
		t.Fatal(err)
	}
	input[0].TargetID = "author/other"
	catalog, err := b.Build()
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
	copyBuilder, err := NewBuilderFrom(decoded)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := copyBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range []*Catalog{catalog, decoded, copy} {
		definition, err := snapshot.FindModel("author/old")
		if err != nil || definition.ID != "author/current" || len(snapshot.Definitions()) != 1 {
			t.Fatalf("renamed catalog = %s/%v", definition.ID, err)
		}
		exported := snapshot.CanonicalAliasRecords()
		exported[0].State = CanonicalAliasRemoved
		if _, state, found := snapshot.CanonicalAliases().Lookup("author/old"); !found || state != CanonicalAliasActive {
			t.Fatal("caller changed the immutable alias index")
		}
	}
	if _, err := NewObservationCatalog(catalog); err == nil {
		t.Fatal("provider observation accepted canonical rename authority")
	}
	var envelope map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope["schema_version"] = 8
	legacy, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCatalogPayload(legacy); err == nil {
		t.Fatal("legacy format accepted canonical rename records")
	}
}

func TestRemovedCanonicalAliasBlocksConvenienceLookup(t *testing.T) {
	b := NewEmpty()
	setTestReadViewDefinition(t, b, "current", "Current Model")
	if err := b.SetProvider(Provider{ID: "provider", Name: "Provider", Models: map[string]*Model{
		"author/old": {ID: "author/old", Name: "Current Offering", ModelRef: "author/current"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetCanonicalAliasRecords([]CanonicalAlias{canonicalAlias("author/old", "author/current", CanonicalAliasRemoved)}); err != nil {
		t.Fatal(err)
	}
	catalog, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.FindModel("author/old"); err == nil {
		t.Fatal("provider convenience name restored a removed canonical alias")
	}
	if _, err := catalog.Definition("author/current"); err != nil {
		t.Fatalf("alias removal removed its target: %v", err)
	}
	if _, err := catalog.Offering("provider", "author/old"); err != nil {
		t.Fatalf("alias removal removed the exact provider offering: %v", err)
	}
}

func TestCanonicalAliasGenericMergeCannotRestoreOrReassign(t *testing.T) {
	left := NewEmpty()
	setTestReadViewDefinition(t, left, "current", "Current Model")
	removed := canonicalAlias("author/old", "author/current", CanonicalAliasRemoved)
	if err := left.SetCanonicalAliasRecords([]CanonicalAlias{removed}); err != nil {
		t.Fatal(err)
	}
	right := NewEmpty()
	active := removed
	active.State = CanonicalAliasActive
	if err := right.SetCanonicalAliasRecords([]CanonicalAlias{active}); err != nil {
		t.Fatal(err)
	}
	if err := left.MergeWith(right); err != nil {
		t.Fatal(err)
	}
	if records := left.CanonicalAliasRecords(); len(records) != 1 || records[0].State != CanonicalAliasRemoved {
		t.Fatal("generic merge restored a removed alias")
	}
	active.TargetID = "author/unrelated"
	if err := right.SetCanonicalAliasRecords([]CanonicalAlias{active}); err != nil {
		t.Fatal(err)
	}
	if err := left.MergeWith(right); err == nil {
		t.Fatal("generic merge reassigned retired identity")
	}
}

func TestCanonicalAliasRejectsConflictingProviderRouteName(t *testing.T) {
	b := NewEmpty()
	setTestReadViewDefinition(t, b, "current", "Current Model")
	setTestReadViewDefinition(t, b, "other", "Other Model")
	if err := b.SetProvider(Provider{ID: "author", Name: "Provider", Models: map[string]*Model{
		"old": {ID: "old", Name: "Unrelated Offering", ModelRef: "author/other"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetCanonicalAliasRecords([]CanonicalAlias{canonicalAlias("author/old", "author/current", CanonicalAliasActive)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Build(); err == nil {
		t.Fatal("one request name identifies two canonical models")
	}
}
