package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectionPresencePreservesRecoverySelection(t *testing.T) {
	for _, kind := range []string{"absent", "tree", "receipt", "journal"} {
		t.Run(kind, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			switch kind {
			case "tree":
				if err := os.Mkdir(target, directoryMode); err != nil {
					t.Fatal(err)
				}
			case "receipt", "journal":
				path := projectionMarkerPath(target)
				if kind == "journal" {
					path = replacementJournalPath(target)
				}
				// Even invalid recovery data must reach the normal repair validator.
				if err := os.WriteFile(path, []byte("incomplete"), fileMode); err != nil {
					t.Fatal(err)
				}
			}
			found, err := HasProjectionState(target)
			if err != nil || found != (kind != "absent") {
				t.Fatalf("presence = %v, %v", found, err)
			}
			if kind == "absent" {
				entries, err := os.ReadDir(filepath.Dir(target))
				if err != nil || len(entries) != 0 {
					t.Fatal("presence inspection created files")
				}
			}
		})
	}
}
