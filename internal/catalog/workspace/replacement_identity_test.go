package workspace

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReplacementCompletionPreservesChangedJournal(t *testing.T) {
	for _, recovery := range []bool{false, true} {
		mode := map[bool]string{false: "publication", true: "recovery"}[recovery]
		for _, change := range []string{"identical-replacement", "equivalent-json", "access"} {
			t.Run(mode+"/"+change, func(t *testing.T) {
				target := filepath.Join(t.TempDir(), "workspace")
				old, oldIdentity := testCatalog(t, "old", "Old Model")
				if _, err := Project(t.Context(), target, old, oldIdentity); err != nil {
					t.Fatal(err)
				}
				catalog, identity := testCatalog(t, "new", "New Model")
				journal := replacementJournalPath(target)
				var saved os.FileInfo
				var want []byte
				changeJournal := func(phase replacementPhase) error {
					if phase != replacementBackupRemoved {
						return nil
					}
					data, err := os.ReadFile(journal)
					if err != nil {
						return err
					}
					info, err := os.Stat(journal)
					if err != nil {
						return err
					}
					switch change {
					case "identical-replacement":
						if err := os.Rename(journal, journal+".original"); err != nil {
							return err
						}
						if err := os.WriteFile(journal, data, info.Mode().Perm()); err != nil {
							return err
						}
					case "equivalent-json":
						data = append([]byte(" "), data...)
						if err := os.WriteFile(journal, data, info.Mode().Perm()); err != nil {
							return err
						}
					case "access":
						if err := os.Chmod(journal, 0444); err != nil {
							return err
						}
						t.Cleanup(func() { _ = os.Chmod(journal, fileMode) })
					}
					want = data
					saved, err = os.Stat(journal)
					return err
				}
				p := projector{journalReplacement: true, afterReplacementPhase: changeJournal}
				if recovery {
					fault := stderrors.New("stop before replacement")
					p.afterReplacementPhase = func(phase replacementPhase) error {
						if phase == replacementJournalSaved {
							return fault
						}
						return nil
					}
					if _, err := p.project(t.Context(), target, catalog, identity, InputExpectation{}); !stderrors.Is(err, fault) {
						t.Fatalf("prepare recovery: %v", err)
					}
					root, err := os.OpenRoot(filepath.Dir(target))
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = root.Close() }()
					record, err := readReplacementRecord(root, target)
					if err != nil {
						t.Fatal(err)
					}
					_, err = advanceReplacement(t.Context(), root, record, replacementHooks{after: changeJournal})
					assertReadConflict(t, err)
				} else {
					_, err := p.project(t.Context(), target, catalog, identity, InputExpectation{})
					assertReadConflict(t, err)
				}
				if saved == nil {
					t.Fatal("journal change did not run")
				}
				actual, err := os.Stat(journal)
				if err != nil || !os.SameFile(saved, actual) || actual.Mode() != saved.Mode() {
					t.Fatalf("completion removed or changed the operator journal: %v", err)
				}
				data, err := os.ReadFile(journal)
				if err != nil || !bytes.Equal(data, want) {
					t.Fatalf("completion changed journal bytes: %v", err)
				}
				assertWorkspaceModel(t, target, "new", "New Model")
			})
		}
	}
}

func TestReplacementCompletionRequiresAcceptedJournal(t *testing.T) {
	for _, guard := range []string{"cancellation", "missing-receipt"} {
		t.Run(guard, func(t *testing.T) {
			f := interruptedReplacement(t, replacementBackupRemoved)
			root, err := os.OpenRoot(filepath.Dir(f.path))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			journal := replacementJournalPath(f.path)
			before, err := os.Stat(journal)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if guard == "cancellation" {
				cancel()
			} else {
				f.record.journal = workspaceRecordState{}
			}
			err = finishReplacementRecord(ctx, root, f.record)
			if guard == "cancellation" {
				if !stderrors.Is(err, context.Canceled) {
					t.Fatalf("completion ignored cancellation: %v", err)
				}
			} else {
				assertReadConflict(t, err)
			}
			after, err := os.Stat(journal)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("guarded completion removed the journal: %v", err)
			}
			if _, err := Repair(t.Context(), f.path, f.catalog, f.identity); err != nil {
				t.Fatalf("retry recovery: %v", err)
			}
			assertReplacementFinished(t, f.path)
		})
	}
}

func TestWorkspacePublicationRetainsOriginalRecordIdentity(t *testing.T) {
	path := t.TempDir()
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	name := "journal.json"
	data := []byte("original journal")
	fault := stderrors.New("failure after publication")
	hooks := workspaceRecordWriter{afterPublish: func(temporary string) error {
		if err := root.Rename(name, "retained.json"); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(path, name), data, fileMode); err != nil {
			return err
		}
		return fault
	}}
	published, err := hooks.publish(t.Context(), root, name, data, recordPublication{})
	if !stderrors.Is(err, fault) || published.identity == "" {
		t.Fatalf("publication lost its receipt or error: %+v, %v", published, err)
	}
	assertReadConflict(t, published.check(t.Context(), root))
	retained, err := optionalWorkspaceRecord(t.Context(), root, "retained.json")
	if err != nil || retained.identity != published.identity {
		t.Fatalf("receipt adopted the replacement file: %v", err)
	}
	assertWorkspaceRecordBytes(t, filepath.Join(path, name), data)
}
