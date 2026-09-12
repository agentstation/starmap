package workspace

import (
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestJournalRecoveryPreservesIdenticalBackupChildren(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			f := interruptedReplacement(t, replacementMarkerSaved)
			var name string
			for _, entry := range f.record.Old.Entries {
				if entry.Path != "." && entry.Directory == directory {
					name = entry.Path
					break
				}
			}
			if name == "" {
				t.Fatal("fixture has no matching backup child")
			}
			child := filepath.Join(filepath.Dir(f.path), f.record.Backup, filepath.FromSlash(name))
			replaceBackupChild(t, child, directory)
			saved, err := os.Stat(child)
			if err != nil {
				t.Fatal(err)
			}
			_, recoveryErr := Repair(t.Context(), f.path, f.catalog, f.identity)
			after, err := os.Stat(child)
			if err != nil || !os.SameFile(saved, after) {
				t.Fatalf("recovery removed identical replacement child: recovery=%v, file=%v", recoveryErr, err)
			}
			assertReadConflict(t, recoveryErr)
			assertWorkspaceModel(t, f.path, "new", "New Model")
		})
	}
}

func replaceBackupChild(t *testing.T, child string, directory bool) {
	t.Helper()
	saved := filepath.Join(t.TempDir(), "original")
	if err := os.Rename(child, saved); err != nil {
		t.Fatal(err)
	}
	if directory {
		if err := os.CopyFS(child, os.DirFS(saved)); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, data, info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
}

func TestJournalRecoveryPreservesLegacyIdentityJournals(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(map[int]string{1: "version-1", 2: "version-2"}[version], func(t *testing.T) {
			f := interruptedReplacement(t, replacementJournalSaved)
			encoded, err := json.Marshal(f.record)
			if err != nil {
				t.Fatal(err)
			}
			var legacy map[string]any
			if err := json.Unmarshal(encoded, &legacy); err != nil {
				t.Fatal(err)
			}
			legacy["version"] = version
			delete(legacy, "old_identities")
			delete(legacy, "new_identities")
			encoded, err = json.Marshal(legacy)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(replacementJournalPath(f.path), encoded, fileMode); err != nil {
				t.Fatal(err)
			}
			before := migrationTree(t, filepath.Dir(f.path))
			_, err = Repair(t.Context(), f.path, f.catalog, f.identity)
			var validation *errors.ValidationError
			if !stderrors.As(err, &validation) || validation.Field != "workspace_replacement.version" {
				t.Fatalf("legacy journal recovery: %v", err)
			}
			if !reflect.DeepEqual(before, migrationTree(t, filepath.Dir(f.path))) {
				t.Fatal("legacy recovery changed workspace state")
			}
		})
	}
}

func TestJournalRecoveryRefusesIdenticalLiveAndCandidateChildren(t *testing.T) {
	for _, location := range []string{"live", "candidate"} {
		for _, directory := range []bool{false, true} {
			t.Run(location+"/"+map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
				f := interruptedReplacement(t, replacementJournalSaved)
				base, tree := f.path, f.record.Old
				if location == "candidate" {
					base = filepath.Join(filepath.Dir(f.path), f.record.Candidate)
					tree = f.record.New
				}
				var child string
				for _, entry := range tree.Entries {
					if entry.Path != "." && entry.Directory == directory {
						child = filepath.Join(base, filepath.FromSlash(entry.Path))
						break
					}
				}
				if child == "" {
					t.Fatal("fixture has no matching child")
				}
				replaceBackupChild(t, child, directory)
				info, err := os.Stat(child)
				if err != nil {
					t.Fatal(err)
				}
				before := migrationTree(t, filepath.Dir(f.path))
				_, err = Repair(t.Context(), f.path, f.catalog, f.identity)
				assertReadConflict(t, err)
				after, err := os.Stat(child)
				if err != nil || !os.SameFile(info, after) {
					t.Fatalf("recovery changed replacement child: %v", err)
				}
				if !reflect.DeepEqual(before, migrationTree(t, filepath.Dir(f.path))) {
					t.Fatal("recovery changed files after identity conflict")
				}
			})
		}
	}
}

func TestJournalCleanupPreservesChildReplacedDuringRemoval(t *testing.T) {
	f := interruptedReplacement(t, replacementMarkerSaved)
	var child string
	for _, entry := range f.record.Old.Entries {
		if !entry.Directory {
			child = filepath.Join(filepath.Dir(f.path), f.record.Backup, filepath.FromSlash(entry.Path))
			break
		}
	}
	if child == "" {
		t.Fatal("fixture has no backup file")
	}
	root, err := os.OpenRoot(filepath.Dir(f.path))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	var replaced os.FileInfo
	_, err = advanceReplacement(t.Context(), root, f.record, replacementHooks{after: func(phase replacementPhase) error {
		if phase == replacementEntryRemoved && replaced == nil {
			replaceBackupChild(t, child, false)
			var err error
			replaced, err = os.Stat(child)
			return err
		}
		return nil
	}})
	assertReadConflict(t, err)
	after, err := os.Stat(child)
	if err != nil || replaced == nil || !os.SameFile(replaced, after) {
		t.Fatalf("cleanup removed replacement child: %v", err)
	}
}

func TestJournalRecoveryRefusesInvalidChildIdentityInventory(t *testing.T) {
	for _, change := range []string{"missing-old", "missing-new", "extra", "empty", "oversize", "root"} {
		t.Run(change, func(t *testing.T) {
			f := interruptedReplacement(t, replacementJournalSaved)
			switch change {
			case "missing-old":
				f.record.OldIdentities = nil
			case "missing-new":
				f.record.NewIdentities = nil
			case "extra":
				f.record.OldIdentities["foreign"] = f.record.Old.ID
			case "empty":
				f.record.NewIdentities["."] = ""
			case "oversize":
				f.record.NewIdentities[f.record.New.Entries[1].Path] = strings.Repeat("x", replacementIdentityMax+1)
			case "root":
				f.record.OldIdentities["."] = f.record.New.ID
			}
			encoded, err := json.Marshal(f.record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(replacementJournalPath(f.path), encoded, fileMode); err != nil {
				t.Fatal(err)
			}
			before := migrationTree(t, filepath.Dir(f.path))
			_, err = Repair(t.Context(), f.path, f.catalog, f.identity)
			var validation *errors.ValidationError
			if !stderrors.As(err, &validation) || validation.Field != "workspace_replacement.identities" {
				t.Fatalf("invalid inventory: %v", err)
			}
			if !reflect.DeepEqual(before, migrationTree(t, filepath.Dir(f.path))) {
				t.Fatal("invalid inventory changed workspace state")
			}
		})
	}
}
