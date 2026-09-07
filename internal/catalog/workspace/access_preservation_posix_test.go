//go:build darwin || linux

package workspace

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceAccessSurvivesInterruptedReplacement(t *testing.T) {
	for _, phase := range []replacementPhase{replacementJournalSaved, replacementBackupMoved, replacementInstalled, replacementMarkerSaved, replacementEntryRemoved, replacementBackupRemoved} {
		t.Run(string(phase), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			old, oldIdentity := testCatalog(t, "old", "Old")
			if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0o770); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(filepath.Join(path, "providers.yaml"), 0o440); err != nil {
				t.Fatal(err)
			}
			next, identity := testCatalog(t, "new", "New")
			fault := stderrors.New("interrupted replacement")
			_, err := (projector{journalReplacement: true, afterReplacementPhase: func(current replacementPhase) error {
				if current == phase {
					return fault
				}
				return nil
			}}).project(t.Context(), path, next, identity, InputExpectation{})
			if !stderrors.Is(err, fault) {
				t.Fatal("interruption", err)
			}
			if _, err := Repair(t.Context(), path, next, identity); err != nil {
				t.Fatal(err)
			}
			for name, want := range map[string]os.FileMode{".": 0o770, "providers.yaml": 0o440} {
				info, err := os.Stat(filepath.Join(path, name))
				if err != nil || info.Mode().Perm() != want {
					t.Fatal("recovery changed access", name, err)
				}
			}
			assertReplacementFinished(t, path)
		})
	}
}

func TestWorkspaceReplacementPreservesAccessPolicy(t *testing.T) {
	for _, journal := range []bool{false, true} {
		t.Run(map[bool]string{false: "atomic", true: "journal"}[journal], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			old, oldIdentity := testCatalog(t, "old", "Old")
			if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "private-notes.txt"), []byte("operator note"), 0o600); err != nil {
				t.Fatal(err)
			}
			nested := filepath.Join(path, "providers", "test-provider", "models", "private-note.txt")
			if err := os.WriteFile(nested, []byte("nested note"), 0o600); err != nil {
				t.Fatal(err)
			}
			modes := map[string]os.FileMode{"providers/test-provider/models/private-note.txt": 0o600, ".": 0o770, "providers.yaml": 0o640, "authors.yaml": 0o440, "private-notes.txt": 0o400, "providers": 0o750 | os.ModeSetgid}
			for name, mode := range modes {
				if err := os.Chmod(filepath.Join(path, name), mode); err != nil {
					t.Fatal(err)
				}
			}
			next, identity := testCatalog(t, "new", "New")
			if _, err := (projector{journalReplacement: journal}).project(t.Context(), path, next, identity, InputExpectation{}); err != nil {
				t.Fatal(err)
			}
			for name, want := range modes {
				info, err := os.Stat(filepath.Join(path, name))
				if err != nil {
					t.Fatal(err)
				}
				if got := info.Mode() & workspaceAccessMode; got != want {
					t.Errorf("%s mode changed: got %v want %v", name, got, want)
				}
			}
			data, err := os.ReadFile(filepath.Join(path, "private-notes.txt"))
			if err != nil || string(data) != "operator note" {
				t.Fatal("operator note changed", err)
			}
			assertWorkspaceModel(t, path, "new", "New")
			assertReplacementFinished(t, path)
		})
	}
}
