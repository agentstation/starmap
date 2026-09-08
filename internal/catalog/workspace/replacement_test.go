package workspace

import (
	"bytes"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestJournalReplacementPreservesOperatorFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	notes := []byte("operator notes\n")
	if err := os.WriteFile(filepath.Join(path, "notes.txt"), notes, fileMode); err != nil {
		t.Fatal(err)
	}
	catalog, identity := testCatalog(t, "new", "New Model")
	receipt, err := (projector{journalReplacement: true}).project(t.Context(), path, catalog, identity, InputExpectation{})
	if err != nil || receipt.GenerationID != identity.GenerationID {
		t.Fatalf("replace: %+v, %v", receipt, err)
	}
	assertWorkspaceModel(t, path, "new", "New Model")
	assertWorkspaceModelMissing(t, path, "old")
	data, err := os.ReadFile(filepath.Join(path, "notes.txt"))
	if err != nil || !bytes.Equal(data, notes) {
		t.Fatal("operator notes changed", err)
	}
	assertReplacementFinished(t, path)
}

func TestJournalReplacementRecoversEachPhase(t *testing.T) {
	for _, phase := range []replacementPhase{replacementJournalSaved, replacementBackupMoved, replacementInstalled, replacementMarkerSaved, replacementEntryRemoved, replacementBackupRemoved} {
		t.Run(string(phase), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			old, oldIdentity := testCatalog(t, "old", "Old Model")
			if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
				t.Fatal(err)
			}
			catalog, identity := testCatalog(t, "new", "New Model")
			fault := stderrors.New("interrupted replacement")
			called := false
			_, err := (projector{journalReplacement: true, afterReplacementPhase: func(current replacementPhase) error {
				if current == phase {
					called = true
					return fault
				}
				return nil
			}}).project(t.Context(), path, catalog, identity, InputExpectation{})
			if !called || !stderrors.Is(err, fault) {
				t.Fatalf("phase %s: %v", phase, err)
			}
			if _, err := ObserveInput(path); err == nil {
				t.Fatal("pending replacement accepted a workspace read")
			} else {
				assertReadConflict(t, err)
			}
			result, err := Repair(t.Context(), path, catalog, identity)
			if err != nil || result.Status != RepairStatusRepaired {
				t.Fatalf("repair: %+v, %v", result, err)
			}
			assertWorkspaceModel(t, path, "new", "New Model")
			assertReplacementFinished(t, path)
			result, err = Repair(t.Context(), path, catalog, identity)
			if err != nil || result.Status != RepairStatusCurrent {
				t.Fatalf("repeat repair: %+v, %v", result, err)
			}
		})
	}
}

func TestJournalReplacementRefusesLateOperatorEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	catalog, identity := testCatalog(t, "new", "New Model")
	notes := []byte("notes added during staging\n")
	receipt, err := (projector{journalReplacement: true, beforePromote: func() error {
		return os.WriteFile(filepath.Join(path, "notes.txt"), notes, fileMode)
	}}).project(t.Context(), path, catalog, identity, InputExpectation{})
	assertReadConflict(t, err)
	if receipt != (Receipt{}) {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	data, err := os.ReadFile(filepath.Join(path, "notes.txt"))
	if err != nil || !bytes.Equal(data, notes) {
		t.Fatal("operator edit was lost", err)
	}
	assertWorkspaceModel(t, path, "old", "Old Model")
	assertReplacementFinished(t, path)
}

func TestJournalRecoveryPreservesUnexpectedWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	catalog, identity := testCatalog(t, "new", "New Model")
	fault := stderrors.New("stop after backup")
	_, err := (projector{journalReplacement: true, afterReplacementPhase: func(phase replacementPhase) error {
		if phase == replacementBackupMoved {
			return fault
		}
		return nil
	}}).project(t.Context(), path, catalog, identity, InputExpectation{})
	if !stderrors.Is(err, fault) {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, directoryMode); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Repair(t.Context(), path, catalog, identity)
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("foreign workspace: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("recovery replaced the operator directory", err)
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	record, err := readReplacementRecord(root, path)
	if err != nil {
		t.Fatal(err)
	}
	assertWorkspaceModel(t, filepath.Join(filepath.Dir(path), record.Backup), "old", "Old Model")
	assertWorkspaceModel(t, filepath.Join(filepath.Dir(path), record.Candidate), "new", "New Model")
}

func assertReplacementFinished(t *testing.T, path string) {
	t.Helper()
	for _, pattern := range []string{replacementJournalPath(path), filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".backup-*")} {
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) != 0 {
			t.Fatalf("replacement artifacts: %v, %v", matches, err)
		}
	}
	assertNoProjectionStaging(t, path)
}
