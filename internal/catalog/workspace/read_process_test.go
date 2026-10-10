package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const workspaceReaderHelper = "STARMAP_WORKSPACE_READER_HELPER"

func TestReadRefusesActiveWriterProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	control := t.TempDir()
	ready, release := filepath.Join(control, "ready"), filepath.Join(control, "release")
	writer := startWorkspaceHelper(t, workspaceWriterCommand(t, path, "first", ready, release, false))
	writer.waitReady(t, ready)
	err := Read(t.Context(), path, func(InputExpectation) error {
		t.Fatal("read while writer process held the lock")
		return nil
	})
	assertReadConflict(t, err)
	if err := os.WriteFile(release, []byte("release"), fileMode); err != nil {
		t.Fatal(err)
	}
	if output, err := writer.wait(); err != nil {
		t.Fatalf("writer helper: %v\n%s", err, output)
	}
	if _, err := ObserveInput(path); err != nil {
		t.Fatalf("read after writer process: %v", err)
	}
}

func TestReaderProcessBlocksWriterAndReleasesLockOnExit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	release, err := acquireWriterLock(path)
	if err != nil {
		t.Fatal(err)
	}
	release()
	ready := filepath.Join(t.TempDir(), "ready")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), executable, "-test.run=^TestWorkspaceReaderProcessHelper$")
	command.Env = append(os.Environ(), workspaceReaderHelper+"=1", workspaceHelperPath+"="+path, workspaceHelperReady+"="+ready)
	reader := startWorkspaceHelper(t, command)
	reader.waitReady(t, ready)
	catalog, identity := testCatalog(t, "new", "New Model")
	_, err = Project(t.Context(), path, catalog, identity)
	assertReadConflict(t, err)
	if err := reader.command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.wait(); err == nil {
		t.Fatal("killed reader process succeeded")
	}
	if _, err := Project(t.Context(), path, catalog, identity); err != nil {
		t.Fatalf("publication after reader exit: %v", err)
	}
	assertWorkspaceModel(t, path, "new", "New Model")
}

func TestWorkspaceReaderProcessHelper(t *testing.T) {
	if os.Getenv(workspaceReaderHelper) == "" {
		return
	}
	reportHelperStep("started")
	ctx, cancel := context.WithTimeout(t.Context(), workspaceHelperTimeout)
	defer cancel()
	err := Read(ctx, os.Getenv(workspaceHelperPath), func(InputExpectation) error {
		reportHelperStep("holds reader lock")
		if err := os.WriteFile(os.Getenv(workspaceHelperReady), []byte("ready"), fileMode); err != nil {
			return err
		}
		reportHelperStep("wrote ready file")
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
}
