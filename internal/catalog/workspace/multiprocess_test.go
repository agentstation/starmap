package workspace

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	workspaceHelperMode     = "STARMAP_WORKSPACE_WRITER_HELPER"
	workspaceHelperPath     = "STARMAP_WORKSPACE_WRITER_PATH"
	workspaceHelperModel    = "STARMAP_WORKSPACE_WRITER_MODEL"
	workspaceHelperReady    = "STARMAP_WORKSPACE_WRITER_READY"
	workspaceHelperRelease  = "STARMAP_WORKSPACE_WRITER_RELEASE"
	workspaceHelperConflict = "STARMAP_WORKSPACE_WRITER_EXPECT_CONFLICT"
)

func TestWorkspaceWritersAreExcludedAcrossProcessesWhileReadersRemainAvailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog")
	oldCatalog, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(context.Background(), path, oldCatalog, oldIdentity); err != nil {
		t.Fatalf("Project old catalog: %v", err)
	}

	control := t.TempDir()
	ready := filepath.Join(control, "ready")
	release := filepath.Join(control, "release")
	first := workspaceWriterCommand(t, path, "first", ready, release, false)
	var firstOutput bytes.Buffer
	first.Stdout = &firstOutput
	first.Stderr = &firstOutput
	if err := first.Start(); err != nil {
		t.Fatalf("start first writer: %v", err)
	}
	defer func() {
		_ = os.WriteFile(release, []byte("release\n"), constants.FilePermissions)
		if first.Process != nil {
			_ = first.Process.Kill()
		}
	}()
	waitForWorkspaceHelperFile(t, ready)

	assertWorkspaceModel(t, path, "old", "Old Model")
	if _, err := Repair(context.Background(), path, oldCatalog, oldIdentity); err == nil {
		t.Fatal("Repair succeeded while another process held the workspace writer lock")
	} else {
		var conflict *errors.ConflictError
		if !stderrors.As(err, &conflict) {
			t.Fatalf("Repair error = %T %v, want *errors.ConflictError", err, err)
		}
	}
	second := workspaceWriterCommand(t, path, "second", "", "", true)
	if output, err := second.CombinedOutput(); err != nil {
		t.Fatalf("second writer helper: %v\n%s", err, output)
	}
	assertWorkspaceModel(t, path, "old", "Old Model")

	if err := os.WriteFile(release, []byte("release\n"), constants.FilePermissions); err != nil {
		t.Fatalf("release first writer: %v", err)
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("first writer helper: %v\n%s", err, firstOutput.Bytes())
	}
	assertWorkspaceModel(t, path, "first", "Process first")
	assertWorkspaceModelMissing(t, path, "second")
	assertNoProjectionStaging(t, path)
}

func TestWorkspaceWriterLockIsReleasedWhenProcessExits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog")
	oldCatalog, oldIdentity := testCatalog(t, "old", "Old Model")
	if _, err := Project(context.Background(), path, oldCatalog, oldIdentity); err != nil {
		t.Fatalf("Project old catalog: %v", err)
	}

	control := t.TempDir()
	ready := filepath.Join(control, "ready")
	release := filepath.Join(control, "never-release")
	holder := workspaceWriterCommand(t, path, "interrupted", ready, release, false)
	if err := holder.Start(); err != nil {
		t.Fatalf("start lock holder: %v", err)
	}
	waitForWorkspaceHelperFile(t, ready)
	if err := holder.Process.Kill(); err != nil {
		t.Fatalf("kill lock holder: %v", err)
	}
	if err := holder.Wait(); err == nil {
		t.Fatal("killed lock holder exited successfully")
	}

	nextCatalog, nextIdentity := testCatalog(t, "next", "Next Model")
	if _, err := Project(context.Background(), path, nextCatalog, nextIdentity); err != nil {
		t.Fatalf("Project after holder exit: %v", err)
	}
	assertWorkspaceModel(t, path, "next", "Next Model")
}

func TestWorkspaceWriterProcessHelper(t *testing.T) {
	if os.Getenv(workspaceHelperMode) == "" {
		return
	}

	path := os.Getenv(workspaceHelperPath)
	modelID := os.Getenv(workspaceHelperModel)
	catalog, identity := testCatalog(t, modelID, "Process "+modelID)
	p := projector{}
	if ready := os.Getenv(workspaceHelperReady); ready != "" {
		release := os.Getenv(workspaceHelperRelease)
		p.beforePromote = func() error {
			if err := os.WriteFile(ready, []byte("ready\n"), constants.FilePermissions); err != nil {
				return err
			}
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Stat(release); err == nil {
					return nil
				} else if !stderrors.Is(err, os.ErrNotExist) {
					return err
				}
				time.Sleep(10 * time.Millisecond)
			}
			return context.DeadlineExceeded
		}
	}

	_, err := p.project(context.Background(), path, catalog, identity, InputExpectation{})
	if os.Getenv(workspaceHelperConflict) != "" {
		var conflict *errors.ConflictError
		if !stderrors.As(err, &conflict) {
			t.Fatalf("Project error = %T %v, want *errors.ConflictError", err, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
}

func workspaceWriterCommand(
	t testing.TB,
	path, modelID, ready, release string,
	expectConflict bool,
) *exec.Cmd {
	t.Helper()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	command := exec.Command(executable, "-test.run=^TestWorkspaceWriterProcessHelper$")
	command.Env = append(
		os.Environ(),
		workspaceHelperMode+"=1",
		workspaceHelperPath+"="+path,
		workspaceHelperModel+"="+modelID,
		workspaceHelperReady+"="+ready,
		workspaceHelperRelease+"="+release,
	)
	if expectConflict {
		command.Env = append(command.Env, workspaceHelperConflict+"=1")
	}
	return command
}

func waitForWorkspaceHelperFile(t testing.TB, path string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !stderrors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat helper file: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for helper file %q", path)
}
