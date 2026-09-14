package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyLayoutRecoveryAfterProcessExit(t *testing.T) {
	if legacy := os.Getenv("STARMAP_TEST_RELOCATION_EXIT"); legacy != "" {
		state := os.Getenv("STARMAP_TEST_RELOCATION_STATE")
		phase := os.Getenv("STARMAP_TEST_RELOCATION_PHASE")
		stop := func() error { os.Exit(86); return nil }
		m := legacyLayoutMigrator{}
		switch phase {
		case "ready":
			m.beforeMove = stop
		case "finished":
			m.afterProjection = stop
		case "moved":
			m.afterMove = stop
		case "prepared":
			m.projector.beforePromote = stop
		case "installed":
			m.projector.beforeMarker = stop
		}
		_, err := m.migrate(t.Context(), legacy, state)
		t.Fatalf("child did not exit during relocation: %v", err)
	}
	for _, phase := range []string{"ready", "moved", "prepared", "installed", "finished"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
			store := migrationStore(t, legacy)
			first := migrationGeneration(t, "relocation-first", "first", "First")
			second := migrationGeneration(t, "relocation-second", "second", "Second")
			if err := store.Commit(t.Context(), first, ""); err != nil {
				t.Fatal(err)
			}
			if err := store.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			runLegacyRelocationExit(t, legacy, state, phase)
			result, err := MigrateLegacyLayout(t.Context(), legacy, state)
			if err != nil {
				t.Fatalf("resume relocation: %v", err)
			}
			if result.GenerationID != second.Manifest.GenerationID || result.RetainedCount != 2 {
				t.Fatalf("recovered result: %+v", result)
			}
			retained, err := migrationStore(t, state).Get(t.Context(), first.Manifest.GenerationID)
			if err != nil || !sameMigrationGeneration(first, retained) {
				t.Fatalf("retained generation changed: %v", err)
			}
			assertWorkspaceModel(t, legacy, "second", "Second")
			assertNoProjectionStaging(t, legacy)
		})
	}
}

func runLegacyRelocationExit(t *testing.T, legacy, state, phase string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestMigrateLegacyLayoutRecoveryAfterProcessExit$")
	command.Env = append(os.Environ(), "STARMAP_TEST_RELOCATION_EXIT="+legacy, "STARMAP_TEST_RELOCATION_STATE="+state, "STARMAP_TEST_RELOCATION_PHASE="+phase)
	output, err := command.CombinedOutput()
	if command.ProcessState == nil || command.ProcessState.ExitCode() != 86 {
		t.Fatalf("child exit: %v, %s", err, output)
	}
}
