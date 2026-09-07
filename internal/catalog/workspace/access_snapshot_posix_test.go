//go:build darwin || linux

package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceReplacementRefusesLateModeChanges(t *testing.T) {
	for _, journal := range []bool{false, true} {
		t.Run(map[bool]string{false: "atomic", true: "journal"}[journal], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			old, oldIdentity := testCatalog(t, "old", "Old")
			if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
				t.Fatal(err)
			}
			next, identity := testCatalog(t, "new", "New")
			_, err := (projector{journalReplacement: journal, beforePromote: func() error {
				return os.Chmod(path, 0o700)
			}}).project(t.Context(), path, next, identity, InputExpectation{})
			assertReadConflict(t, err)
			assertWorkspaceModel(t, path, "old", "Old")
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0o700 {
				t.Fatal("replacement lost the operator mode change", err)
			}
			assertReplacementFinished(t, path)
		})
	}
}

func TestTreeSnapshotDetectsSpecialModeChange(t *testing.T) {
	path := t.TempDir()
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotTree(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o700|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotTree(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if sameTree(before, after) {
		t.Fatal("snapshot omitted the sticky bit change")
	}
}
