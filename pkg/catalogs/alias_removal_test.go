package catalogs

import (
	"encoding/json"
	"testing"
)

func TestAliasRemovalPreservesTargetAndOtherAliases(t *testing.T) {
	target, err := NewAliasRemovalTarget("author/old")
	if err != nil {
		t.Fatal(err)
	}
	set, err := NewCatalogRemovalSet(target, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Targets()) != 1 || !set.ContainsAlias("author/old") || set.ContainsAlias("author/other") || set.ContainsCanonical("author/old") {
		t.Fatal("alias removal changed another selector")
	}
	if allocs := testing.AllocsPerRun(100, func() {
		if !set.ContainsAlias("author/old") || set.ContainsAlias("author/other") {
			t.Fatal("alias query changed during the allocation probe")
		}
	}); allocs != 0 {
		t.Fatalf("alias removal query allocates %v times", allocs)
	}
	for name, mutate := range map[string]func(*CatalogRemovalTarget){
		"canonical": func(target *CatalogRemovalTarget) { target.DefinitionID = "author/current" },
		"provider":  func(target *CatalogRemovalTarget) { target.ProviderModelID = "wire" },
		"scope":     func(target *CatalogRemovalTarget) { target.Scope = &CatalogRemovalScope{} },
		"malformed": func(target *CatalogRemovalTarget) { target.AliasID = "old" },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := target
			mutate(&invalid)
			if err := invalid.Validate(); err == nil {
				t.Fatal("mixed or malformed alias target was accepted")
			}
		})
	}
}

func TestAliasRemovalRejectsLegacyPayloadSchema(t *testing.T) {
	target, err := NewAliasRemovalTarget("author/old")
	if err != nil {
		t.Fatal(err)
	}
	b := NewEmpty()
	if err := b.SetRemovalPolicies([]CatalogRemovalPolicy{{PublisherID: "operator", Targets: []CatalogRemovalTarget{target}}}); err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeCatalogPayload(b)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCatalogPayload(payload)
	if err != nil || !decoded.Removals().ContainsAlias(target.AliasID) {
		t.Fatalf("current alias removal payload = %v", err)
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
		t.Fatal("schema 8 accepted an alias removal selector")
	}
}
