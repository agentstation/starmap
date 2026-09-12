package workspace

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func preparationTestWriter(t *testing.T, target string) *workspaceWriter {
	t.Helper()
	writer, err := acquireWorkspaceWriter(target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(writer.close)
	return writer
}

func TestPreparationRecoveryAfterProcessExit(t *testing.T) {
	if target := os.Getenv("STARMAP_TEST_PREPARATION_EXIT"); target != "" {
		phase := os.Getenv("STARMAP_TEST_PREPARATION_PHASE")
		if phase != "rendered" {
			writer := preparationTestWriter(t, target)
			stage, err := prepareWorkspaceStage(t.Context(), target, writer)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "partial" {
				err := stage.trees["render"].writeFrom(t.Context(), "nested/partial", strings.NewReader("partial"), 100, "unused", nil)
				if !stderrors.Is(err, io.EOF) {
					t.Fatalf("partial write: %v", err)
				}
			}
			os.Exit(89)
		}
		catalog, identity := testCatalog(t, "new", "New")
		_, err := (projector{afterStageRender: func(string) error {
			os.Exit(89)
			return nil
		}}).project(t.Context(), target, catalog, identity, InputExpectation{})
		t.Fatalf("child did not exit during preparation: %v", err)
	}
	for _, phase := range []string{"created", "partial", "rendered"} {
		t.Run(phase, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			stage := crashPreparation(t, target, phase)
			catalog, identity := testCatalog(t, "new", "New")
			if _, err := Repair(t.Context(), target, catalog, identity); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(stage); !os.IsNotExist(err) {
				t.Fatalf("owned preparation survived recovery: %v", err)
			}
			assertNoProjectionStaging(t, target)
		})
	}
}

func crashPreparation(t *testing.T, target, phase string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestPreparationRecoveryAfterProcessExit$")
	command.Env = append(os.Environ(), "STARMAP_TEST_PREPARATION_EXIT="+target, "STARMAP_TEST_PREPARATION_PHASE="+phase)
	output, err := command.CombinedOutput()
	if command.ProcessState == nil || command.ProcessState.ExitCode() != 89 {
		t.Fatalf("child exit: %v, %s", err, output)
	}
	stages, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".workspace.preparing-*"))
	if err != nil || len(stages) != 1 {
		t.Fatalf("abandoned preparation: %v, %v", stages, err)
	}
	return stages[0]
}

func TestPreparationRecoveryPreservesChangedState(t *testing.T) {
	for _, change := range []string{"file-bytes", "file-identity", "unknown-file", "journal-identity", "journal-truncated", "writer-identity"} {
		t.Run(change, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			stage := crashPreparation(t, target, "partial")
			preserved := filepath.Join(stage, "render", "nested", "partial")
			want, err := os.ReadFile(preserved)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "file-bytes":
				want = []byte("operator change")
				if err := os.WriteFile(preserved, want, fileMode); err != nil {
					t.Fatal(err)
				}
			case "file-identity":
				if err := os.Rename(preserved, filepath.Join(filepath.Dir(target), "original")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(preserved, want, fileMode); err != nil {
					t.Fatal(err)
				}
			case "unknown-file":
				preserved = filepath.Join(stage, "operator-note")
				want = []byte("operator note")
				if err := os.WriteFile(preserved, want, fileMode); err != nil {
					t.Fatal(err)
				}
			case "journal-identity", "journal-truncated":
				journal := filepath.Join(stage, preparationJournalName)
				data, err := os.ReadFile(journal)
				if err != nil {
					t.Fatal(err)
				}
				if change == "journal-identity" {
					if err := os.Rename(journal, filepath.Join(filepath.Dir(target), "original-journal")); err != nil {
						t.Fatal(err)
					}
				} else {
					data = data[:len(data)-1]
				}
				if err := os.WriteFile(journal, data, 0o600); err != nil {
					t.Fatal(err)
				}
			case "writer-identity":
				if err := os.Rename(writerLockPath(target), filepath.Join(filepath.Dir(target), "old-lock")); err != nil {
					t.Fatal(err)
				}
			}
			catalog, identity := testCatalog(t, "new", "New")
			if _, err := Repair(t.Context(), target, catalog, identity); err == nil {
				t.Fatal("recovery accepted changed preparation")
			}
			actual, err := os.ReadFile(preserved)
			if err != nil || !bytes.Equal(actual, want) {
				t.Fatalf("recovery changed preserved state: %q, %v", actual, err)
			}
		})
	}
}

func TestPreparationRecoveryExcludesActiveWriter(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	writer := preparationTestWriter(t, target)
	stage, err := prepareWorkspaceStage(t.Context(), target, writer)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := stage.close(t.Context()); err != nil {
			t.Error(err)
		}
	}()
	if err := stage.trees["render"].writeFile(t.Context(), "owned", []byte("retained")); err != nil {
		t.Fatal(err)
	}
	catalog, identity := testCatalog(t, "new", "New")
	if _, err := Repair(t.Context(), target, catalog, identity); err == nil {
		t.Fatal("repair entered an active preparation")
	}
	if data, err := os.ReadFile(filepath.Join(stage.renderPath(), "owned")); err != nil || string(data) != "retained" {
		t.Fatalf("active preparation changed: %q, %v", data, err)
	}
}

func TestPreparationRecoveryBoundsScanBeforeCleanup(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	stage := crashPreparation(t, target, "created")
	for i := range preparationScanMax {
		if err := os.WriteFile(filepath.Join(filepath.Dir(target), fmt.Sprintf("unrelated-%04d", i)), nil, fileMode); err != nil {
			t.Fatal(err)
		}
	}
	writer := preparationTestWriter(t, target)
	if err := recoverPreparations(t.Context(), target, writer); err == nil {
		t.Fatal("oversized scan accepted")
	}
	if _, err := os.Stat(stage); err != nil {
		t.Fatalf("scan removed preparation before rejecting size: %v", err)
	}
}

func TestPreparationRecoveryCancellationPreservesStage(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	stage := crashPreparation(t, target, "created")
	writer := preparationTestWriter(t, target)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := recoverPreparations(ctx, target, writer); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled recovery: %v", err)
	}
	if _, err := os.Stat(stage); err != nil {
		t.Fatal(err)
	}
	if err := recoverPreparations(t.Context(), target, writer); err != nil {
		t.Fatal(err)
	}
	assertNoProjectionStaging(t, target)
}

func TestPreparationRecoveryRejectsInvalidJournal(t *testing.T) {
	for _, change := range []string{"size", "event-count", "version", "unknown-field", "unrecorded-write"} {
		t.Run(change, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "workspace")
			stage := crashPreparation(t, target, "created")
			journal := filepath.Join(stage, preparationJournalName)
			data, err := os.ReadFile(journal)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "size":
				if err := os.Truncate(journal, preparationJournalMax+1); err != nil {
					t.Fatal(err)
				}
			case "event-count":
				data = append(data, bytes.Repeat([]byte{'\n'}, preparationEventMax)...)
			case "version":
				data = bytes.Replace(data, []byte(`"version":1`), []byte(`"version":0`), 1)
			case "unknown-field":
				data = bytes.Replace(data, []byte(`{"header":`), []byte(`{"unexpected":true,"header":`), 1)
			case "unrecorded-write":
				if err := os.WriteFile(filepath.Join(stage, "render", "unrecorded"), []byte("preserve"), fileMode); err != nil {
					t.Fatal(err)
				}
			}
			if change != "size" {
				if err := os.WriteFile(journal, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			writer := preparationTestWriter(t, target)
			if err := recoverPreparations(t.Context(), target, writer); err == nil {
				t.Fatal("invalid journal accepted")
			}
			if _, err := os.Stat(journal); err != nil {
				t.Fatalf("invalid journal removed: %v", err)
			}
		})
	}
}

func TestPreparationRecoveryResumesPartialCleanup(t *testing.T) {
	target := filepath.Join(t.TempDir(), "workspace")
	stagePath := crashPreparation(t, target, "partial")
	writer := preparationTestWriter(t, target)
	stage, err := readPreparation(t.Context(), target, filepath.Base(stagePath), writer)
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.removeTree(t.Context(), "render"); err != nil {
		t.Fatal(err)
	}
	if err := stage.journal.file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stage.private.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stage.parent.Close(); err != nil {
		t.Fatal(err)
	}
	if err := recoverPreparations(t.Context(), target, writer); err != nil {
		t.Fatal(err)
	}
	assertNoProjectionStaging(t, target)
}
