package catalogs

import (
	"reflect"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestAuthorsEncodeYAMLPreservesHeaderAndEntryComments(t *testing.T) {
	for _, tc := range []struct {
		name    string
		authors []Author
	}{
		{name: "empty"},
		{name: "one author", authors: []Author{{ID: "alpha", Name: "Alpha"}}},
		{name: "several authors", authors: []Author{
			{ID: "zed", Name: "Zed"},
			{ID: "alpha", Name: "Alpha", Aliases: []AuthorID{"a"}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			authors := NewAuthors()
			for _, author := range tc.authors {
				if err := authors.Add(&author); err != nil {
					t.Fatal(err)
				}
			}
			first, err := authors.EncodeYAML()
			if err != nil {
				t.Fatal(err)
			}
			if len(tc.authors) == 0 {
				if first != "" {
					t.Fatalf("empty authors produced YAML: %q", first)
				}
				return
			}
			const header = "# Known model authors and organizations with their metadata and social links\n" +
				"# This file contains the complete author information that can be loaded at runtime\n\n"
			if !strings.HasPrefix(first, header) {
				t.Fatalf("author file header changed:\n%s", first)
			}
			for _, author := range tc.authors {
				comment := "# " + author.Name + "\n"
				if strings.Count(first, comment) != 1 {
					t.Fatalf("author heading %q is missing or repeated:\n%s", comment, first)
				}
			}
			var decoded []Author
			if err := yaml.Unmarshal([]byte(first), &decoded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded, authors.List()) {
				t.Fatalf("author records or their order changed: %#v", decoded)
			}
			for range 32 {
				got, err := authors.EncodeYAML()
				if err != nil || got != first {
					t.Fatalf("repeated author encoding changed: %v\n%s", err, got)
				}
			}
		})
	}
}
