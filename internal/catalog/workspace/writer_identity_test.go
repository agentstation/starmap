package workspace

import (
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestWorkspaceWriterReplacementStopsPublication(t *testing.T) {
	for _, journal := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "journal"}[journal], func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			old, oldID := testCatalog(t, "old", "Old Model")
			if _, err := Project(t.Context(), target, old, oldID); err != nil {
				t.Fatal(err)
			}
			next, nextID := testCatalog(t, "new", "New Model")
			var contender func()
			var replacement os.FileInfo
			nativeRefusal := false
			p := projector{journalReplacement: journal, beforePromote: func() error {
				path := writerLockPath(target)
				if err := os.Rename(path, path+".original"); err != nil {
					if runtime.GOOS == "windows" && stderrors.Is(err, fs.ErrPermission) {
						nativeRefusal = true
						return nil
					}
					return err
				}
				if err := os.WriteFile(path, nil, fileMode); err != nil {
					return err
				}
				var err error
				replacement, err = os.Stat(path)
				if err != nil {
					return err
				}
				contender, err = acquireWriterLock(target)
				return err
			}}
			_, err := p.project(t.Context(), target, next, nextID, InputExpectation{})
			if contender != nil {
				defer contender()
			}
			if nativeRefusal {
				if err != nil {
					t.Fatal(err)
				}
				assertWorkspaceModel(t, target, "new", "New Model")
				return
			}
			assertReadConflict(t, err)
			if contender == nil || replacement == nil {
				t.Fatal("replacement writer did not acquire its lock")
			}
			after, statErr := os.Stat(writerLockPath(target))
			if statErr != nil || !os.SameFile(replacement, after) {
				t.Fatalf("old writer changed replacement lock: %v", statErr)
			}
			assertWorkspaceModel(t, target, "old", "Old Model")
		})
	}
}

func TestWorkspaceRecoveryPreservesReplacedWriterJournal(t *testing.T) {
	f := interruptedReplacement(t, replacementMarkerSaved)
	lock := writerLockPath(f.path)
	if err := os.Rename(lock, lock+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, nil, fileMode); err != nil {
		t.Fatal(err)
	}
	before := migrationTree(t, filepath.Dir(f.path))
	_, err := Repair(t.Context(), f.path, f.catalog, f.identity)
	assertReadConflict(t, err)
	if !reflect.DeepEqual(before, migrationTree(t, filepath.Dir(f.path))) {
		t.Fatal("replacement writer changed an earlier writer's recovery state")
	}
}

func TestWorkspaceWriterReplacementStopsJournalPhases(t *testing.T) {
	for _, phase := range []replacementPhase{replacementJournalSaved, replacementBackupMoved, replacementInstalled, replacementMarkerSaved, replacementEntryRemoved, replacementBackupRemoved} {
		t.Run(string(phase), func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			old, oldID := testCatalog(t, "old", "Old Model")
			if _, err := Project(t.Context(), target, old, oldID); err != nil {
				t.Fatal(err)
			}
			next, nextID := testCatalog(t, "new", "New Model")
			var before any
			nativeRefusal := false
			reached := false
			_, err := (projector{journalReplacement: true, afterReplacementPhase: func(current replacementPhase) error {
				if current != phase || reached {
					return nil
				}
				reached = true
				lock := writerLockPath(target)
				if err := os.Rename(lock, lock+".original"); err != nil {
					if runtime.GOOS == "windows" && stderrors.Is(err, fs.ErrPermission) {
						nativeRefusal = true
						return nil
					}
					return err
				}
				if err := os.WriteFile(lock, nil, fileMode); err != nil {
					return err
				}
				before = migrationTree(t, filepath.Dir(target))
				return nil
			}}).project(t.Context(), target, next, nextID, InputExpectation{})
			if !reached {
				t.Fatal("journal phase was not reached")
			}
			if nativeRefusal {
				if err != nil {
					t.Fatal(err)
				}
				assertReplacementFinished(t, target)
				return
			}
			assertReadConflict(t, err)
			if !reflect.DeepEqual(before, migrationTree(t, filepath.Dir(target))) {
				t.Fatal("old writer changed recovery state after lock replacement")
			}
		})
	}
}

func TestWorkspaceWriterLeaseIdentityAndRelease(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	writer, err := acquireWorkspaceWriter(target)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.close()
	if writer.identity == "" {
		t.Fatal("writer has no native identity")
	}
	other, err := acquireWorkspaceWriter(target)
	if other != nil {
		other.close()
		t.Fatal("second writer acquired a live lock")
	}
	assertReadConflict(t, err)
	if err := writer.check(); err != nil {
		t.Fatal(err)
	}
	identity := writer.identity
	writer.close()
	writer.close()
	assertReadConflict(t, writer.check())
	next, err := acquireWorkspaceWriter(target)
	if err != nil {
		t.Fatal(err)
	}
	defer next.close()
	if next.identity != identity {
		t.Fatal("release replaced the stable writer lock")
	}
}
