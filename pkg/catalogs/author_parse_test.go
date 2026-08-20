package catalogs

import "testing"

func TestAuthorIDParsingAndAliasResolutionHaveExplicitOwners(t *testing.T) {
	if got := ParseAuthorID("  Example Author  "); got != "example-author" {
		t.Fatalf("ParseAuthorID = %q, want example-author", got)
	}
	authors := NewAuthors(WithAuthorsMap(map[AuthorID]*Author{
		"canonical": {ID: "canonical", Aliases: []AuthorID{"example-author"}},
	}))
	resolved, found := authors.Resolve("example-author")
	if !found || resolved == nil || resolved.ID != "canonical" {
		t.Fatalf("Resolve = %#v, %t; want canonical author", resolved, found)
	}
}

func TestParseAuthorIDPreservesStableAliases(t *testing.T) {
	tests := map[string]AuthorID{
		"canopylabs":    "canopy-labs",
		"huggingfaceh4": AuthorIDHuggingFace,
		"llama":         AuthorIDMeta,
		"moonshot":      AuthorIDMoonshot,
		"moonshotai":    AuthorIDMoonshot,
		"togetherai":    AuthorIDTogether,
		"zhipuai":       AuthorIDZhipuAI,
	}
	for input, want := range tests {
		if got := ParseAuthorID(input); got != want {
			t.Errorf("ParseAuthorID(%q) = %q, want %q", input, got, want)
		}
	}
	if got := ParseAuthorID(" LLAMA "); got != "llama" {
		t.Errorf("ParseAuthorID with non-exact alias = %q, want llama", got)
	}
}
