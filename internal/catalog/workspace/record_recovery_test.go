package workspace

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceRecordRecoveryAfterProcessExit(t *testing.T) {
	if target := os.Getenv("STARMAP_TEST_RECORD_EXIT"); target != "" {
		flow := os.Getenv("STARMAP_TEST_RECORD_FLOW")
		phase := os.Getenv("STARMAP_TEST_RECORD_PHASE")
		stop := func(name string) error {
			if selectedRecoveryRecord(name, flow) {
				os.Exit(87)
			}
			return nil
		}
		hooks := workspaceRecordWriter{}
		if phase == "created" || phase == "partial" {
			hooks.afterRecord = func(name string) error {
				if !selectedRecoveryRecord(name, flow) {
					return nil
				}
				info, err := os.Stat(name)
				if err != nil {
					return err
				}
				if phase == "created" || info.Size() == 3 {
					return stop(name)
				}
				return nil
			}
			if phase == "partial" {
				hooks.writeBytes = func(file *os.File, data []byte) (int, error) {
					if !selectedRecoveryRecord(file.Name(), flow) {
						return file.Write(data)
					}
					n, err := file.Write(data[:3])
					return n, stderrors.Join(err, io.ErrShortWrite)
				}
			}
		} else if phase == "prepared" {
			hooks.beforePublish = stop
		} else {
			hooks.afterPublish = stop
		}
		catalog, identity := testCatalog(t, "new", "New Model")
		_, err := (projector{journalReplacement: flow != "projection", recordWrites: hooks}).
			project(t.Context(), target, catalog, identity, InputExpectation{})
		t.Fatalf("child did not exit during record publication: %v", err)
	}
	for _, flow := range []string{"projection", "journal", "journal-marker"} {
		for _, phase := range []string{"created", "partial", "prepared", "published"} {
			t.Run(flow+"/"+phase, func(t *testing.T) {
				target := filepath.Join(t.TempDir(), "workspace")
				crashWorkspaceRecord(t, target, flow, phase)
				catalog, identity := testCatalog(t, "new", "New Model")
				if _, err := Repair(t.Context(), target, catalog, identity); err != nil {
					t.Fatal(err)
				}
				assertWorkspaceModel(t, target, "new", "New Model")
				assertNoProjectionStaging(t, target)
				entries, err := os.ReadDir(filepath.Dir(target))
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if selectedRecoveryRecord(entry.Name(), flow) {
						t.Errorf("temporary publication record survived recovery: %s", entry.Name())
					}
				}
			})
		}
	}
}

func abandonWorkspaceRecord(t *testing.T, target string) (string, string) {
	t.Helper()
	writer := preparationTestWriter(t, target)
	root, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	var temporary string
	fault := stderrors.New("abandon publication after recording ownership")
	hooks := workspaceRecordWriter{writer: writer, checkWriter: writer.check, beforePublish: func(name string) error {
		temporary = name
		writer.close()
		return fault
	}}
	_, err = hooks.publish(t.Context(), root, filepath.Base(projectionMarkerPath(target)), []byte("owned temporary record"), recordPublication{replace: true})
	if !stderrors.Is(err, fault) || temporary == "" {
		t.Fatalf("abandoned publication: %v", err)
	}
	stages, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".workspace.preparing-*"))
	if err != nil || len(stages) != 1 {
		t.Fatalf("record stage: %v, %v", stages, err)
	}
	return temporary, stages[0]
}

func TestWorkspaceRecordRecoveryPreservesChangedState(t *testing.T) {
	for _, change := range []string{"bytes", "identity", "access", "unknown-child", "journal-identity", "journal-truncated", "writer-identity"} {
		t.Run(change, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			temporary, stage := abandonWorkspaceRecord(t, target)
			preserved := temporary
			want, err := os.ReadFile(preserved)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "bytes":
				want = []byte("operator content")
				if err := os.WriteFile(preserved, want, fileMode); err != nil {
					t.Fatal(err)
				}
			case "identity":
				if err := os.Rename(preserved, preserved+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(preserved, want, fileMode); err != nil {
					t.Fatal(err)
				}
			case "access":
				if err := os.Chmod(preserved, 0o444); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(preserved, fileMode) })
			case "unknown-child":
				preserved = filepath.Join(stage, "operator-note")
				want = []byte("keep this note")
				if err := os.WriteFile(preserved, want, fileMode); err != nil {
					t.Fatal(err)
				}
			case "journal-identity", "journal-truncated":
				preserved = filepath.Join(stage, preparationJournalName)
				want, err = os.ReadFile(preserved)
				if err != nil {
					t.Fatal(err)
				}
				if change == "journal-identity" {
					if err := os.Rename(preserved, filepath.Join(filepath.Dir(target), "original-journal")); err != nil {
						t.Fatal(err)
					}
				} else {
					want = want[:len(want)-1]
				}
				if err := os.WriteFile(preserved, want, fileMode); err != nil {
					t.Fatal(err)
				}
			case "writer-identity":
				if err := os.Rename(writerLockPath(target), filepath.Join(filepath.Dir(target), "original-lock")); err != nil {
					t.Fatal(err)
				}
			}
			writer := preparationTestWriter(t, target)
			if err := recoverPreparations(t.Context(), target, writer); err == nil {
				t.Fatal("changed record ownership accepted")
			}
			assertWorkspaceRecordBytes(t, preserved, want)
			if _, err := os.Stat(temporary); err != nil {
				t.Fatalf("changed ownership removed the temporary record: %v", err)
			}
		})
	}
}

func TestWorkspaceRecordRecoveryRejectsInvalidReceipt(t *testing.T) {
	for _, change := range []string{"destination", "path", "identity", "directory", "size", "hash", "version", "mixed-tree"} {
		t.Run(change, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			temporary, stage := abandonWorkspaceRecord(t, target)
			path := filepath.Join(stage, preparationJournalName)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lines := bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'})
			var event preparationEvent
			if err := json.Unmarshal(lines[len(lines)-1], &event); err != nil || event.Record == nil {
				t.Fatalf("record receipt: %v", err)
			}
			switch change {
			case "destination":
				event.Record.Destination = "operator-file"
			case "path":
				event.Record.Entry.Path = event.Record.Destination
			case "identity":
				event.Record.Identity = "different"
			case "directory":
				event.Record.Entry.Directory = true
			case "size":
				event.Record.Entry.Size = replacementJournalMax + 1
			case "hash":
				event.Record.Entry.SHA256 = "invalid"
			case "version":
				lines[0] = bytes.Replace(lines[0], []byte(`"version":3`), []byte(`"version":2`), 1)
			case "mixed-tree":
				event.Tree = "render"
			}
			lines[len(lines)-1], err = json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			data = append(bytes.Join(lines, []byte{'\n'}), '\n')
			if err := os.WriteFile(path, data, fileMode); err != nil {
				t.Fatal(err)
			}
			writer := preparationTestWriter(t, target)
			if err := recoverPreparations(t.Context(), target, writer); err == nil {
				t.Fatal("invalid record receipt accepted")
			}
			assertWorkspaceRecordBytes(t, path, data)
			assertWorkspaceRecordBytes(t, temporary, []byte("owned temporary record"))
		})
	}
}

func TestWorkspaceRecordRecoveryResumesAfterTemporaryRemoval(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	temporary, _ := abandonWorkspaceRecord(t, target)
	if err := os.Remove(temporary); err != nil {
		t.Fatal(err)
	}
	writer := preparationTestWriter(t, target)
	if err := recoverPreparations(t.Context(), target, writer); err != nil {
		t.Fatal(err)
	}
	assertNoProjectionStaging(t, target)
}

func TestWorkspaceRecordPublicationRejectsChangedJournal(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	writer := preparationTestWriter(t, target)
	root, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	var temporary string
	hooks := workspaceRecordWriter{writer: writer, checkWriter: writer.check, beforePublish: func(name string) error {
		temporary = name
		stages, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".workspace.preparing-*"))
		if err != nil || len(stages) != 1 {
			t.Fatalf("publication stage: %v, %v", stages, err)
		}
		journal := filepath.Join(stages[0], preparationJournalName)
		data, err := os.ReadFile(journal)
		if err != nil {
			return err
		}
		return os.WriteFile(journal, append([]byte{' '}, data...), 0o600)
	}}
	destination := filepath.Base(projectionMarkerPath(target))
	published, err := hooks.publish(t.Context(), root, destination, []byte("owned temporary record"), recordPublication{replace: true})
	if err == nil || published.identity != "" || temporary == "" {
		t.Fatalf("changed journal did not stop publication: %v", err)
	}
	if _, err := root.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("changed journal published a marker: %v", err)
	}
	assertWorkspaceRecordBytes(t, temporary, []byte("owned temporary record"))
}

func TestCandidateRecoveryReadsVersionTwo(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	crashCandidate(t, target, "prepared")
	stages, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".workspace.preparing-*"))
	if err != nil || len(stages) != 1 {
		t.Fatalf("candidate stage: %v, %v", stages, err)
	}
	path := filepath.Join(stages[0], preparationJournalName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(`"version":3`), []byte(`"version":2`), 1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	writer := preparationTestWriter(t, target)
	if err := recoverPreparations(t.Context(), target, writer); err != nil {
		t.Fatal(err)
	}
	assertNoProjectionStaging(t, target)
	assertWorkspaceModel(t, target, "old", "Old Model")
}

func selectedRecoveryRecord(name, flow string) bool {
	if flow == "journal" {
		return strings.Contains(filepath.Base(name), ".starmap-replacement.json.")
	}
	return strings.Contains(filepath.Base(name), ".starmap-projection.json.")
}

func crashWorkspaceRecord(t *testing.T, target, flow, phase string) {
	t.Helper()
	if flow != "projection" {
		old, identity := testCatalog(t, "old", "Old Model")
		if _, err := Project(t.Context(), target, old, identity); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestWorkspaceRecordRecoveryAfterProcessExit$")
	command.Env = append(os.Environ(), "STARMAP_TEST_RECORD_EXIT="+target, "STARMAP_TEST_RECORD_FLOW="+flow, "STARMAP_TEST_RECORD_PHASE="+phase)
	output, err := command.CombinedOutput()
	if command.ProcessState == nil || command.ProcessState.ExitCode() != 87 {
		t.Fatalf("child exit: %v, %s", err, output)
	}
}
