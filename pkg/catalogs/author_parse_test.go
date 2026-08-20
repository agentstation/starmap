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
