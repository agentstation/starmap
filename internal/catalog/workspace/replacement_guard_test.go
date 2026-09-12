package workspace

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

type replacementFixture struct {
	path     string
	catalog  *catalogs.Catalog
	identity Identity
	record   replacementRecord
}

func interruptedReplacement(t *testing.T, phase replacementPhase) replacementFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	catalog, identity := testCatalog(t, "new", "New Model")
	fault := stderrors.New("interrupted replacement fixture")
	_, err := (projector{journalReplacement: true, afterReplacementPhase: func(current replacementPhase) error {
		if current == phase {
			return fault
		}
		return nil
	}}).project(t.Context(), path, catalog, identity, InputExpectation{})
	if !stderrors.Is(err, fault) {
		t.Fatalf("interrupt at %s: %v", phase, err)
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
	return replacementFixture{path: path, catalog: catalog, identity: identity, record: record}
}

func TestJournalRecoveryRefusesCopiedCandidateIdentity(t *testing.T) {
	f := interruptedReplacement(t, replacementJournalSaved)
	candidate := filepath.Join(filepath.Dir(f.path), f.record.Candidate)
	if err := os.Rename(candidate, candidate+".retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(candidate, os.DirFS(candidate+".retained")); err != nil {
		t.Fatal(err)
	}
	_, err := Repair(t.Context(), f.path, f.catalog, f.identity)
	assertReadConflict(t, err)
	assertWorkspaceModel(t, f.path, "old", "Old Model")
	assertWorkspaceModel(t, candidate, "new", "New Model")
	assertWorkspaceModel(t, candidate+".retained", "new", "New Model")
}

func TestJournalRecoveryValidatesMarkerBeforeMovingWorkspace(t *testing.T) {
	f := interruptedReplacement(t, replacementJournalSaved)
	f.record.Marker.GenerationID = "incorrect-generation"
	data, err := json.Marshal(f.record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacementJournalPath(f.path), data, fileMode); err != nil {
		t.Fatal(err)
	}
	_, err = Repair(t.Context(), f.path, f.catalog, f.identity)
	assertReadConflict(t, err)
	assertWorkspaceModel(t, f.path, "old", "Old Model")
	if _, err := os.Lstat(filepath.Join(filepath.Dir(f.path), f.record.Backup)); !os.IsNotExist(err) {
		t.Fatal("invalid receipt identity moved the old workspace", err)
	}
}

func TestJournalCleanupPreservesChangedBackup(t *testing.T) {
	f := interruptedReplacement(t, replacementMarkerSaved)
	note := filepath.Join(filepath.Dir(f.path), f.record.Backup, "operator.txt")
	if err := os.WriteFile(note, []byte("preserve this edit"), fileMode); err != nil {
		t.Fatal(err)
	}
	_, err := Repair(t.Context(), f.path, f.catalog, f.identity)
	assertReadConflict(t, err)
	data, err := os.ReadFile(note)
	if err != nil || string(data) != "preserve this edit" {
		t.Fatal("cleanup discarded the operator edit", err)
	}
	assertWorkspaceModel(t, f.path, "new", "New Model")
	assertWorkspaceModel(t, filepath.Join(filepath.Dir(f.path), f.record.Backup), "old", "Old Model")
}

func TestJournalRecoveryRestoresOldTreeWhenCandidateIsMissing(t *testing.T) {
	f := interruptedReplacement(t, replacementBackupMoved)
	if err := os.RemoveAll(filepath.Join(filepath.Dir(f.path), f.record.Candidate)); err != nil {
		t.Fatal(err)
	}
	writer, err := acquireWorkspaceWriter(f.path)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := recoverReplacement(t.Context(), f.path, writer)
	writer.close()
	if err != nil || !recovered {
		t.Fatalf("restore: %v, %v", recovered, err)
	}
	current, err := snapshotTree(t.Context(), f.path)
	if err != nil || !sameTree(current, f.record.Old) {
		t.Fatal("recovery did not restore the exact old tree", err)
	}
	assertWorkspaceModel(t, f.path, "old", "Old Model")
	assertReplacementFinished(t, f.path)
}

func TestJournalRecoveryRefusesCorruptIntentWithoutChanges(t *testing.T) {
	f := interruptedReplacement(t, replacementJournalSaved)
	if err := os.WriteFile(replacementJournalPath(f.path), []byte("{partial"), fileMode); err != nil {
		t.Fatal(err)
	}
	before := migrationTree(t, filepath.Dir(f.path))
	if _, err := Repair(t.Context(), f.path, f.catalog, f.identity); err == nil {
		t.Fatal("corrupt journal accepted")
	}
	if !reflect.DeepEqual(before, migrationTree(t, filepath.Dir(f.path))) {
		t.Fatal("recovery changed files under a corrupt journal")
	}
}

func TestJournalRecoveryRefusesInvalidBindingsWithoutChanges(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*replacementRecord)
	}{
		{"unsupported_version", func(r *replacementRecord) { r.Version++ }},
		{"different_workspace", func(r *replacementRecord) { r.Target += ".different" }},
		{"candidate_traversal", func(r *replacementRecord) { r.Candidate = "../candidate" }},
		{"backup_traversal", func(r *replacementRecord) { r.Backup = "../backup" }},
		{"same_directory_identity", func(r *replacementRecord) { r.New.ID = r.Old.ID }},
		{"inventory_digest", func(r *replacementRecord) { r.Old.Entries[0].Mode ^= 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := interruptedReplacement(t, replacementJournalSaved)
			test.change(&f.record)
			data, err := json.Marshal(f.record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(replacementJournalPath(f.path), data, fileMode); err != nil {
				t.Fatal(err)
			}
			before := migrationTree(t, filepath.Dir(f.path))
			if _, err := Repair(t.Context(), f.path, f.catalog, f.identity); err == nil {
				t.Fatal("invalid journal binding accepted")
			}
			if !reflect.DeepEqual(before, migrationTree(t, filepath.Dir(f.path))) {
				t.Fatal("recovery changed files under invalid journal bindings")
			}
		})
	}
}

func TestJournalCancellationRetainsRecoveryState(t *testing.T) {
	for _, phase := range []replacementPhase{replacementBackupMoved, replacementInstalled} {
		t.Run(string(phase), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			old, oldIdentity := testCatalog(t, "old", "Old Model")
			if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
				t.Fatal(err)
			}
			catalog, identity := testCatalog(t, "new", "New Model")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			_, err := (projector{journalReplacement: true, afterReplacementPhase: func(current replacementPhase) error {
				if current == phase {
					cancel()
				}
				return nil
			}}).project(ctx, path, catalog, identity, InputExpectation{})
			if !stderrors.Is(err, context.Canceled) {
				t.Fatalf("cancel at %s: %v", phase, err)
			}
			if _, err := Repair(t.Context(), path, catalog, identity); err != nil {
				t.Fatal(err)
			}
			assertWorkspaceModel(t, path, "new", "New Model")
			assertReplacementFinished(t, path)
		})
	}
}
