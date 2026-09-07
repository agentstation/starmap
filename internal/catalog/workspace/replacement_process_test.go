package workspace

import (
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	replacementHelperPhase = "STARMAP_REPLACEMENT_HELPER_PHASE"
	replacementHelperExit  = 93
)

func TestJournalReplacementRecoversAfterProcessExit(t *testing.T) {
	for _, phase := range []replacementPhase{replacementJournalSaved, replacementBackupMoved, replacementInstalled, replacementMarkerSaved, replacementEntryRemoved, replacementBackupRemoved} {
		t.Run(string(phase), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			old, oldIdentity := testCatalog(t, "old", "Old Model")
			if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), executable, "-test.run=^TestJournalReplacementProcessHelper$")
			command.Env = append(os.Environ(), replacementHelperPhase+"="+string(phase), workspaceHelperPath+"="+path)
			output, err := command.CombinedOutput()
			var exit *exec.ExitError
			if !stderrors.As(err, &exit) || exit.ExitCode() != replacementHelperExit {
				t.Fatalf("process exit: %v\n%s", err, output)
			}
			if _, err := ObserveInput(path); err == nil {
				t.Fatal("read accepted an unfinished replacement after process exit")
			} else {
				assertReadConflict(t, err)
			}
			catalog, identity := testCatalog(t, "new", "New Model")
			result, err := Repair(t.Context(), path, catalog, identity)
			if err != nil || result.Status != RepairStatusRepaired {
				t.Fatalf("repair after process exit: %+v, %v", result, err)
			}
			assertWorkspaceModel(t, path, "new", "New Model")
			assertReplacementFinished(t, path)
		})
	}
}

func TestJournalReplacementProcessHelper(t *testing.T) {
	phase := os.Getenv(replacementHelperPhase)
	if phase == "" {
		return
	}
	catalog, identity := testCatalog(t, "new", "New Model")
	_, err := (projector{journalReplacement: true, afterReplacementPhase: func(current replacementPhase) error {
		if string(current) == phase {
			os.Exit(replacementHelperExit)
		}
		return nil
	}}).project(t.Context(), os.Getenv(workspaceHelperPath), catalog, identity, InputExpectation{})
	t.Fatalf("replacement did not exit at %s: %v", phase, err)
}
