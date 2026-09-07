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
	writer := workspaceWriterCommand(t, path, "first", ready, release, false)
	if err := writer.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Process.Kill() })
	waitForWorkspaceHelperFile(t, ready)
	err := Read(t.Context(), path, func(InputExpectation) error {
		t.Fatal("read while writer process held the lock")
		return nil
	})
	assertReadConflict(t, err)
	if err := os.WriteFile(release, []byte("release"), fileMode); err != nil {
		t.Fatal(err)
	}
	if err := writer.Wait(); err != nil {
		t.Fatal(err)
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
	reader := exec.CommandContext(t.Context(), executable, "-test.run=^TestWorkspaceReaderProcessHelper$")
	reader.Env = append(os.Environ(), workspaceReaderHelper+"=1", workspaceHelperPath+"="+path, workspaceHelperReady+"="+ready)
	if err := reader.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Process.Kill() })
	waitForWorkspaceHelperFile(t, ready)
	catalog, identity := testCatalog(t, "new", "New Model")
	_, err = Project(t.Context(), path, catalog, identity)
	assertReadConflict(t, err)
	if err := reader.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := reader.Wait(); err == nil {
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
	ctx, cancel := context.WithTimeout(t.Context(), 1000*lockRetryDelay)
	defer cancel()
	err := Read(ctx, os.Getenv(workspaceHelperPath), func(InputExpectation) error {
		if err := os.WriteFile(os.Getenv(workspaceHelperReady), []byte("ready"), fileMode); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
}
