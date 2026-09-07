package storage

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFilesystemCatalogStoreRefusesNativeACLGrants(t *testing.T) {
	for _, role := range []string{"root", "payload"} {
		t.Run(role, func(t *testing.T) {
			root := privateFilesystemRoot(t)
			store, err := NewFilesystem(root)
			if err != nil {
				t.Fatal(err)
			}
			generation := testGeneration("acl-private", "payload")
			if err := store.Commit(t.Context(), generation, ""); err != nil {
				t.Fatal(err)
			}
			target := root
			if role == "payload" {
				target = filepath.Join(store.generationDir(generation.Manifest.GenerationID), payloadFilename)
			}
			change := func(operation string) {
				t.Helper()
				args := []string{operation, target}
				if operation == "+a" {
					args = []string{operation, "everyone allow read", target}
				}
				if output, err := exec.Command("/bin/chmod", args...).CombinedOutput(); err != nil {
					t.Fatalf("ACL fixture: %v: %s", err, output)
				}
			}
			change("+a")
			t.Cleanup(func() { change("-N") })
			if _, err := store.Current(t.Context()); err == nil {
				t.Fatal("Current accepted non-owner ACL grant")
			}
			if _, err := store.Get(t.Context(), generation.Manifest.GenerationID); err == nil {
				t.Fatal("Get accepted non-owner ACL grant")
			}
			if err := store.Commit(t.Context(), generation, generation.Manifest.GenerationID); err == nil {
				t.Fatal("Commit accepted non-owner ACL grant")
			}
			change("-N")
			assertStoredGeneration(t, store, generation)
		})
	}
}
