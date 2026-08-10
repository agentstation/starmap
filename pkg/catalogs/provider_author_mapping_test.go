package catalogs

import (
	"reflect"
	"testing"
)

func TestAuthorMappingResolveUsesExactCaseInsensitiveAndSpecificGlobOrder(t *testing.T) {
	mapping := AuthorMapping{Normalized: map[string]AuthorID{
		"system":    AuthorIDOpenAI,
		"Qwen/*":    AuthorIDQwen,
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

func TestProviderEndpointHasNoModelFactRuleField(t *testing.T) {
	if field, found := reflect.TypeFor[ProviderEndpoint]().FieldByName("FeatureRules"); found {
		t.Fatalf("provider endpoint retains model-fact rule field %#v", field)
	}
}
