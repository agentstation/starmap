package catalogs

import (
	stderrors "errors"
	"reflect"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestAuthorMappingResolveUsesExactCaseInsensitiveAndSpecificGlobOrder(t *testing.T) {
	mapping := AuthorMapping{Normalized: map[string]AuthorID{
		"system":     AuthorIDOpenAI,
		"Qwen/*":     AuthorIDQwen,
		"Qwen/Coder": "qwen-coder",
	}}
	for _, test := range []struct {
		value string
		want  AuthorID
	}{
		{value: "system", want: AuthorIDOpenAI},
		{value: "SYSTEM", want: AuthorIDOpenAI},
		{value: "qwen/coder", want: "qwen-coder"},
		{value: "qwen/other", want: AuthorIDQwen},
	} {
		got, found := mapping.Resolve(test.value)
		if !found || got != test.want {
			t.Fatalf("Resolve(%q) = %q, %t, want %q, true", test.value, got, found, test.want)
		}
	}
	if got, found := mapping.Resolve("unmapped"); found || got != AuthorIDUnknown {
		t.Fatalf("Resolve(unmapped) = %q, %t", got, found)
	}
}

func TestAuthorMappingValidationRejectsAmbiguousOrInvalidConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		mapping AuthorMapping
	}{
		{
			name: "case-fold duplicate",
			mapping: AuthorMapping{Field: "owned_by", Normalized: map[string]AuthorID{
				"Meta": "meta", "META": "other",
			}},
		},
		{
			name: "invalid glob",
			mapping: AuthorMapping{Field: "id", Normalized: map[string]AuthorID{
				"model[": "author",
			}},
		},
		{
			name: "empty target",
			mapping: AuthorMapping{Field: "id", Normalized: map[string]AuthorID{
				"model": "",
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var validationErr *pkgerrors.ValidationError
			if err := test.mapping.Validate(); !stderrors.As(err, &validationErr) {
				t.Fatalf("error = %T: %v, want ValidationError", err, err)
			}
		})
	}
}

func TestCatalogRejectsNoncanonicalAuthorMappingTarget(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "canonical", Aliases: []AuthorID{"alias"}, Name: "Canonical"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Catalog: &ProviderCatalog{Endpoint: ProviderEndpoint{
			Type: EndpointTypeOpenAI,
			AuthorMapping: &AuthorMapping{Field: "owned_by", Normalized: map[string]AuthorID{
				"upstream": "alias",
			}},
		}},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	var validationErr *pkgerrors.ValidationError
	if _, err := builder.Build(); !stderrors.As(err, &validationErr) {
		t.Fatalf("Build error = %T: %v, want ValidationError", err, err)
	}
}

func TestProviderEndpointHasNoModelFactRuleField(t *testing.T) {
	if field, found := reflect.TypeFor[ProviderEndpoint]().FieldByName("FeatureRules"); found {
		t.Fatalf("provider endpoint retains model-fact rule field %#v", field)
	}
}
