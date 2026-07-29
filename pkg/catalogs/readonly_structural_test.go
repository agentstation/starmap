package catalogs

import (
	"reflect"
	"testing"
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
	if _, ok := any(catalog.Provenance()).(interface{ Clear() }); ok {
		t.Error("Read-only provenance exposes Clear")
	}

	catalogType := reflect.TypeFor[*Catalog]()
	for _, method := range []string{
		"Build", "ClearProvenance", "Copy", "DeleteAuthor", "DeleteAuthorModel",
		"DeleteProvider", "DeleteProviderModel", "MergeProvenance", "MergeWith",
		"Models", "ProviderModel", "ProviderModels", "ReplaceWith", "Save",
		"SetAuthor", "SetAuthorModel", "SetMergeStrategy", "SetProvider",
		"SetProviderModel", "SetProvenance",
	} {
		if _, found := catalogType.MethodByName(method); found {
			t.Errorf("Catalog exposes forbidden method %s", method)
		}
	}
}

func TestBuilderRequiresProviderScopedModelReads(t *testing.T) {
	builderType := reflect.TypeFor[*Builder]()
	for _, method := range []string{"FindModel", "Models"} {
		if _, found := builderType.MethodByName(method); found {
			t.Errorf("Builder exposes ambiguous cross-provider method %s", method)
		}
	}
}

func TestCanonicalSchemaHasNoRemovedCompatibilityFields(t *testing.T) {
	for _, check := range []struct {
		name      string
		structure reflect.Type
		forbidden []string
	}{
		{
			name:      "author",
			structure: reflect.TypeFor[Author](),
			forbidden: []string{"Models"},
		},
		{
			name:      "model architecture",
			structure: reflect.TypeFor[ModelArchitecture](),
			forbidden: []string{"Precision"},
		},
		{
			name:      "token pricing",
			structure: reflect.TypeFor[ModelTokenPricing](),
			forbidden: []string{"Cache"},
		},
	} {
		t.Run(check.name, func(t *testing.T) {
			for _, field := range check.forbidden {
				if _, found := check.structure.FieldByName(field); found {
					t.Errorf("%s exposes duplicate compatibility field %s", check.name, field)
				}
			}
		})
	}
}

func TestCatalogPayloadCarriesDisjointAuthorAndProviderRecords(t *testing.T) {
	payload := reflect.TypeFor[CatalogPayload]()
	for _, field := range []string{"AuthorModels", "ProviderModels"} {
		if _, found := payload.FieldByName(field); !found {
			t.Errorf("catalog payload is missing required construction record field %s", field)
		}
	}
	if _, found := reflect.TypeFor[Author]().FieldByName("Models"); found {
		t.Error("Author metadata leaks the authored-model construction collection")
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
	if _, ok := any(catalog).(interface{ Save() error }); ok {
		t.Fatal("Read-only catalog exposes Save")
	}
	if _, ok := any(catalog).(interface{ SaveTo(string) error }); ok {
		t.Fatal("Read-only catalog exposes SaveTo")
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
	setTestReadViewDefinition(t, builder, "shared", "Published Offering")
	if err := builder.SetProvider(Provider{
		ID:      "provider-a",
		Aliases: []ProviderID{"provider-alias"},
		Models: map[string]*Model{
			"shared": {
				ID: "shared", ModelRef: "author/shared", Name: "Published Offering",
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog := mustCatalog(t, builder)

	if err := builder.SetProviderModel("provider-a", Model{
		ID: "shared", ModelRef: "author/shared", Name: "Later Draft",
	}); err != nil {
		t.Fatalf("SetProviderModel: %v", err)
	}
	offering, err := catalog.Offering("provider-alias", "shared")
	if err != nil {
		t.Fatalf("Offering through alias: %v", err)
	}
	if offering.DefinitionID != "author/shared" {
		t.Fatalf("Indexed offering definition = %q, want author/shared", offering.DefinitionID)
	}
	definition, err := catalog.FindModel("shared")
	if err != nil {
		t.Fatalf("FindModel: %v", err)
	}
	if definition.Name != "Published Offering" {
		t.Fatalf("Indexed definition name = %q, want published value", definition.Name)
	}
	offerings, err := catalog.ProviderOfferings("provider-a")
	if err != nil || len(offerings) != 1 {
		t.Fatalf("ProviderOfferings = (%#v, %v), want one", offerings, err)
	}
}

func TestCatalogCanonicalOfferingLookupPreservesDuplicateModelIDs(t *testing.T) {
	builder := NewEmpty()
	setTestReadViewDefinition(t, builder, "shared", "Shared Model")
	providers := []Provider{
		{
			ID: "provider-a", Aliases: []ProviderID{"provider-a-alias"}, Name: "Provider A",
			Models: map[string]*Model{"shared": testReadViewModel("shared", 1, "priority")},
		},
		{
			ID: "provider-b", Name: "Provider B",
			Models: map[string]*Model{"shared": testReadViewModel("shared", 2, "standard")},
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

	definition, err := catalog.Definition("author/shared")
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
