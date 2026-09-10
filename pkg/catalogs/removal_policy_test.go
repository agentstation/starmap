package catalogs

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRemovalPolicySurvivesCatalogCopiesAndPayload(t *testing.T) {
	scope := testMembershipScope()
	target, err := NewScopedRemovalTarget(scope, "model")
	if err != nil {
		t.Fatal(err)
	}
	builder := NewEmpty()
	input := []CatalogRemovalPolicy{{PublisherID: "operator", Targets: []CatalogRemovalTarget{target}}}
	if err := builder.SetRemovalPolicies(input); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	input[0].Targets[0].Scope.AccountID = "mutated"
	if err := builder.SetRemovalPolicies(nil); err != nil {
		t.Fatal(err)
	}
	if !catalog.Removals().ContainsScoped(scope, "model") {
		t.Fatal("builder or caller changed immutable removal")
	}
	encoded, err := EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCatalogPayload(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Removals().ContainsScoped(scope, "model") {
		t.Fatal("payload lost removal")
	}
	copy, err := NewBuilderFrom(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(copy.RemovalPolicies()) != 1 {
		t.Fatal("builder copy lost removal")
	}
	exported := decoded.RemovalPolicies()
	exported[0].Targets[0].Scope.AccountID = "mutated"
	again, err := EncodeCatalogPayload(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, again) {
		t.Fatal("export changed immutable payload")
	}
}

func TestRemovalPolicyMergeCannotRestoreAnEntry(t *testing.T) {
	one, two := NewEmpty(), NewEmpty()
	target, err := NewCanonicalRemovalTarget("author/model")
	if err != nil {
		t.Fatal(err)
	}
	if err := one.SetRemovalPolicies([]CatalogRemovalPolicy{{PublisherID: "operator", Targets: []CatalogRemovalTarget{target}}}); err != nil {
		t.Fatal(err)
	}
	if err := two.SetRemovalPolicies([]CatalogRemovalPolicy{{PublisherID: "operator", Targets: nil}}); err != nil {
		t.Fatal(err)
	}
	if err := one.MergeWith(two); err != nil {
		t.Fatal(err)
	}
	catalog, err := one.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.Removals().ContainsCanonical("author/model") {
		t.Fatal("generic merge cleared explicit removal")
	}
	if err := one.SetRemovalPolicies([]CatalogRemovalPolicy{{PublisherID: "operator"}, {PublisherID: "operator"}}); err == nil {
		t.Fatal("accepted ambiguous publisher snapshots")
	}
	catalog, err = one.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.Removals().ContainsCanonical("author/model") {
		t.Fatal("invalid replacement changed accepted policy")
	}
}

func TestRemovalPolicyRequiresEnforcingSchemaAndCannotBecomeObservation(t *testing.T) {
	builder := NewEmpty()
	target, err := NewCanonicalRemovalTarget("author/model")
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.SetRemovalPolicies([]CatalogRemovalPolicy{{PublisherID: "operator", Targets: []CatalogRemovalTarget{target}}}); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSourceObservationPayload(encoded); err == nil {
		t.Fatal("operator policy became source acquisition evidence")
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	for _, version := range []uint64{6, 7} {
		payload["schema_version"] = version
		legacy, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeCatalogPayload(legacy); err == nil {
			t.Fatalf("schema %d accepted unenforceable policy", version)
		}
	}
}
