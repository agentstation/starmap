package catalogs

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

func TestMembershipWorkspacePreservesExactPayload(t *testing.T) {
	for _, mode := range []string{"complete", "empty", "unknown", "addition", "empty_additions", "public"} {
		t.Run(mode, func(t *testing.T) {
			scope := storedMembershipScope()
			switch mode {
			case "empty":
				scope.Inventory.ModelIDs = []string{}
			case "unknown":
				scope.Inventory = nil
			case "addition":
				scope.Additions = []MembershipPresence{{ModelID: "later", ObservationID: "partial", ObservedAt: scope.Inventory.ObservedAt.Add(time.Minute)}}
			case "empty_additions":
				scope.Additions = []MembershipPresence{}
			case "public":
				scope.Public, scope.AccountID = true, ""
			}
			b := NewEmpty()
			if err := b.SetMembershipScopes([]ProviderMembershipScope{scope}); err != nil {
				t.Fatal(err)
			}
			before, err := EncodeCatalogPayload(b)
			if err != nil {
				t.Fatal(err)
			}
			path := t.TempDir()
			if err := b.SaveTo(path); err != nil {
				t.Fatal(err)
			}
			loaded, err := NewFromPath(path)
			if err != nil {
				t.Fatal(err)
			}
			after, err := EncodeCatalogPayload(loaded)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("workspace changed membership or payload identity: before=%s after=%s", before, after)
			}
			if err := b.SetMembershipScopes(nil); err != nil {
				t.Fatal(err)
			}
			if err := b.SaveTo(path); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(path, "membership-scopes.yaml")); !os.IsNotExist(err) {
				t.Fatalf("stale membership file: %v", err)
			}
			if err := loaded.Load(); err != nil {
				t.Fatal(err)
			}
			if len(loaded.MembershipScopes()) != 0 {
				t.Fatal("missing membership file retained old scope")
			}
		})
	}
}

func TestMembershipWorkspaceRejectsMalformedRecords(t *testing.T) {
	for _, data := range []string{"null\n", "- publisher_id: upstream\n", "- unexpected: value\n"} {
		t.Run(data, func(t *testing.T) {
			_, err := New(WithFS(fstest.MapFS{"membership-scopes.yaml": {Data: []byte(data)}}))
			if err == nil {
				t.Fatal("malformed membership file was ignored")
			}
		})
	}
}

func TestMembershipWorkspaceRejectsDuplicateScopes(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetMembershipScopes([]ProviderMembershipScope{storedMembershipScope()}); err != nil {
		t.Fatal(err)
	}
	var encoded []byte
	if err := builder.WriteYAML(func(name string, data []byte) error {
		if name == "membership-scopes.yaml" {
			encoded = bytes.Clone(data)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(encoded) == 0 {
		t.Fatal("missing scope serialization")
	}
	duplicate := append(bytes.Clone(encoded), encoded...)
	if _, err := New(WithFS(fstest.MapFS{"membership-scopes.yaml": {Data: duplicate}})); err == nil {
		t.Fatal("duplicate workspace scopes were accepted")
	}
}
