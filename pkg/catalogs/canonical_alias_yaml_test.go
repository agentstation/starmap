package catalogs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestCanonicalAliasWorkspaceRoundTrip(t *testing.T) {
	b := NewEmpty()
	setTestReadViewDefinition(t, b, "current", "Current Model")
	records := []CanonicalAlias{canonicalAlias("author/old", "author/current", CanonicalAliasActive), canonicalAlias("author/removed", "author/current", CanonicalAliasRemoved)}
	if err := b.SetCanonicalAliasRecords(records); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := b.SaveTo(directory); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewFromPath(directory)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := loaded.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(catalog.CanonicalAliasRecords(), records) {
		t.Fatal("workspace lost canonical rename records")
	}
	if definition, err := catalog.FindModel("author/old"); err != nil || definition.ID != "author/current" {
		t.Fatalf("reloaded alias = %s, %v", definition.ID, err)
	}
	if _, err := catalog.FindModel("author/removed"); err == nil {
		t.Fatal("workspace restored removed alias")
	}
	if err := b.SetCanonicalAliasRecords(nil); err != nil {
		t.Fatal(err)
	}
	if err := b.SaveTo(directory); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, "canonical-aliases.yaml")); !os.IsNotExist(err) {
		t.Fatalf("low-level replacement kept stale alias records: %v", err)
	}
}

func TestCanonicalAliasWorkspaceRejectsInvalidRecords(t *testing.T) {
	for _, data := range []string{
		"null\n",
		"- id: author/old\n  target_id: author/current\n  publisher_id: upstream\n  state: active\n  expires_at: never\n",
		"- id: author/old\n  target_id: author/current\n  publisher_id: upstream\n",
		"- id: author/old\n  target_id: author/old\n  publisher_id: upstream\n  state: active\n",
	} {
		t.Run(data, func(t *testing.T) {
			_, err := New(WithFS(fstest.MapFS{"canonical-aliases.yaml": {Data: []byte(data)}}))
			if err == nil {
				t.Fatal("invalid alias authoring was ignored")
			}
		})
	}
}
