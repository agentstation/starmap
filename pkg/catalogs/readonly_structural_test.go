package catalogs

import (
	"reflect"
	"testing"

	"github.com/agentstation/starmap/pkg/save"
)

func TestCatalogDoesNotExposeMutationInterfaces(t *testing.T) {
	catalog := mustCatalog(t, NewEmpty())

	for name, assertion := range map[string]bool{
		"builder": implements[*Builder](catalog),
	} {
		if assertion {
			t.Errorf("Read-only catalog exposes %s interface", name)
		}
	}

	if _, ok := any(catalog.Providers()).(interface {
		Set(ProviderID, *Provider) error
	}); ok {
		t.Error("Read-only providers expose Set")
	}
	if _, ok := any(catalog.Authors()).(interface {
		Delete(AuthorID) error
	}); ok {
		t.Error("Read-only authors expose Delete")
	}
	if _, ok := any(catalog.Endpoints()).(interface{ Clear() }); ok {
		t.Error("Read-only endpoints expose Clear")
	}
	if _, ok := any(catalog.Provenance()).(interface{ Clear() }); ok {
		t.Error("Read-only provenance exposes Clear")
	}

	catalogType := reflect.TypeFor[*Catalog]()
	for _, method := range []string{
		"Build", "ClearProvenance", "Copy", "DeleteAuthor", "DeleteEndpoint",
		"DeleteProvider", "DeleteProviderModel", "MergeProvenance", "MergeWith",
		"LegacyV0", "Models", "ProviderModel", "ProviderModels",
		"ReplaceWith", "Save", "SetAuthor", "SetEndpoint", "SetMergeStrategy",
		"SetProvider", "SetProviderModel", "SetProvenance",
	} {
		if _, found := catalogType.MethodByName(method); found {
			t.Errorf("Catalog exposes forbidden method %s", method)
		}
	}
}

func TestCatalogStateIsUnexported(t *testing.T) {
	catalogType := reflect.TypeFor[Catalog]()
	for index := 0; index < catalogType.NumField(); index++ {
		field := catalogType.Field(index)
		if field.IsExported() {
			t.Errorf("Catalog field %s is exported", field.Name)
		}
	}
}

func TestBuilderIsNotPublishedCatalog(t *testing.T) {
	if _, ok := any(NewEmpty()).(*Catalog); ok {
		t.Fatal("Mutable builder is a published catalog")
	}
}

func TestSeamConformanceReaderHasBuilderAndCatalogAdapters(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetProvider(Provider{ID: "reader-adapter", Name: "Builder"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog := mustCatalog(t, builder)

	readers := []Reader{builder, catalog}
	for _, reader := range readers {
		provider, err := reader.Provider("reader-adapter")
		if err != nil {
			t.Fatalf("%T Provider: %v", reader, err)
		}
		if provider.ID != "reader-adapter" {
			t.Fatalf("%T provider ID = %q", reader, provider.ID)
		}
	}
}

func TestCatalogCannotBeSaved(t *testing.T) {
	catalog := mustCatalog(t, NewEmpty())
	if _, ok := any(catalog).(interface {
		Save(...save.Option) error
	}); ok {
		t.Fatal("Read-only catalog exposes Save")
	}
}

func TestCatalogIsolatedFromLaterBuilderMutation(t *testing.T) {
	builder := NewEmpty()
	catalog := mustCatalog(t, builder)

	if err := builder.SetProvider(Provider{ID: "later", Name: "Later"}); err != nil {
		t.Fatalf("Mutate builder: %v", err)
	}
	if _, found := catalog.Providers().Get("later"); found {
		t.Fatal("Read-only catalog observed a later builder mutation")
	}
}

func TestBuilderFromCatalogCannotMutatePublishedCatalog(t *testing.T) {
	published := mustCatalog(t, NewEmpty())
	builder, err := NewBuilderFrom(published)
	if err != nil {
		t.Fatalf("NewBuilderFrom: %v", err)
	}
	if err := builder.SetProvider(Provider{ID: "draft", Name: "Draft"}); err != nil {
		t.Fatalf("Mutate builder: %v", err)
	}
	if _, found := published.Providers().Get("draft"); found {
		t.Fatal("Builder mutation changed its source catalog")
	}
}

func TestCatalogPrecomputesProviderOfferingIndex(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetProvider(Provider{
		ID:      "provider-a",
		Aliases: []ProviderID{"provider-alias"},
		Models: map[string]*Model{
			"shared": {ID: "shared", Name: "Published Offering"},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog := mustCatalog(t, builder)

	if err := builder.SetProviderModel("provider-a", Model{ID: "shared", Name: "Later Draft"}); err != nil {
		t.Fatalf("SetProviderModel: %v", err)
	}
	offering, err := catalog.Offering("provider-alias", "shared")
	if err != nil {
		t.Fatalf("ProviderModel through alias: %v", err)
	}
	if offering.ProviderModelID != "shared" {
		t.Fatalf("Indexed offering ID = %q, want shared", offering.ProviderModelID)
	}

	offerings, err := catalog.ProviderOfferings("provider-a")
	if err != nil {
		t.Fatalf("ProviderOfferings: %v", err)
	}
	if len(offerings) != 1 || offerings[0].ProviderModelID != "shared" {
		t.Fatalf("ProviderOfferings = %#v", offerings)
	}
}

func TestCatalogCanonicalOfferingLookupPreservesDuplicateModelIDs(t *testing.T) {
	builder := NewEmpty()
	providers := []Provider{
		{
			ID: "provider-a", Aliases: []ProviderID{"provider-a-alias"}, Name: "Provider A",
			Models: map[string]*Model{"shared": sourceProjectionTestModel("shared", 1, "priority")},
		},
		{
			ID: "provider-b", Name: "Provider B",
			Models: map[string]*Model{"shared": sourceProjectionTestModel("shared", 2, "standard")},
		},
	}
	for _, provider := range providers {
		if err := builder.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider: %v", err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	definition, err := catalog.Definition("shared")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if definition.Name != "Shared Model" {
		t.Fatalf("definition = %#v", definition)
	}
	aOffering, err := catalog.Offering("provider-a-alias", "shared")
	if err != nil {
		t.Fatalf("Offering(provider-a alias): %v", err)
	}
	bOffering, err := catalog.Offering("provider-b", "shared")
	if err != nil {
		t.Fatalf("Offering(provider-b): %v", err)
	}
	if aOffering.Key() == bOffering.Key() {
		t.Fatal("duplicate provider model IDs collapsed to one offering")
	}
	if aOffering.Pricing.Tokens.Input.Per1M != 1 || bOffering.Pricing.Tokens.Input.Per1M != 2 {
		t.Fatalf("offering prices = (%v, %v), want (1, 2)", aOffering.Pricing.Tokens.Input.Per1M, bOffering.Pricing.Tokens.Input.Per1M)
	}

	aOffering.Pricing.Tokens.Input.Per1M = 99
	mode := aOffering.Modes["fast"]
	mode.Request.Headers["mutated"] = "true"
	again, err := catalog.Offering("provider-a", "shared")
	if err != nil {
		t.Fatalf("Offering again: %v", err)
	}
	if again.Pricing.Tokens.Input.Per1M != 1 {
		t.Fatal("offering read leaked nested pricing mutation")
	}
	if _, found := again.Modes["fast"].Request.Headers["mutated"]; found {
		t.Fatal("offering read leaked nested request mutation")
	}
	all, err := catalog.ProviderOfferings("provider-a")
	if err != nil || len(all) != 1 || all[0].ProviderModelID != "shared" {
		t.Fatalf("ProviderOfferings = (%#v, %v)", all, err)
	}
}

func mustCatalog(t *testing.T, source Reader) *Catalog {
	t.Helper()
	catalog, err := NewCatalog(source)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	return catalog
}

func implements[T any](value any) bool {
	_, ok := value.(T)
	return ok
}
