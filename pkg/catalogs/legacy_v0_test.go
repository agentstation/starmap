package catalogs

import (
	"reflect"
	"testing"
)

func TestLegacyV0CompatibilityAdapter(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{"model": legacyMigrationModel("model", 1, "standard")},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	legacy := catalog.LegacyV0()
	model, err := legacy.FindModel("model")
	if err != nil || model.ID != "model" {
		t.Fatalf("FindModel = (%#v, %v)", model, err)
	}
	providerModel, err := legacy.ProviderModel("provider", "model")
	if err != nil || providerModel.Pricing.Tokens.Input.Per1M != 1 {
		t.Fatalf("ProviderModel = (%#v, %v)", providerModel, err)
	}
	if legacy.Models().Len() != catalog.Models().Len() {
		t.Fatal("legacy adapter and deprecated direct view diverged")
	}
	canonical, err := catalog.Offering("provider", "model")
	if err != nil || canonical.DefinitionID != "model" {
		t.Fatalf("canonical Offering = (%#v, %v)", canonical, err)
	}
	definition, err := catalog.FindModel("model")
	if err != nil {
		t.Fatalf("canonical FindModel: %v", err)
	}
	var _ ModelDefinition = definition

	typeOfLegacy := reflect.TypeOf(legacy)
	for _, forbidden := range []string{"Definition", "Offering", "ProviderOfferings", "MaterializeRouteAlias"} {
		if _, found := typeOfLegacy.MethodByName(forbidden); found {
			t.Fatalf("LegacyCatalogV0 exposes canonical method %s", forbidden)
		}
	}
}

func TestLegacyV0AdapterHasExplicitSchemaVersion(t *testing.T) {
	if LegacyCatalogSchemaVersion != 0 || CurrentCatalogSchemaVersion != 1 {
		t.Fatalf("legacy/current schema versions = (%d, %d), want (0, 1)", LegacyCatalogSchemaVersion, CurrentCatalogSchemaVersion)
	}
}
