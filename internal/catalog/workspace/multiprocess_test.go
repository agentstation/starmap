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

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	workspaceHelperMode     = "STARMAP_WORKSPACE_WRITER_HELPER"
	workspaceHelperPath     = "STARMAP_WORKSPACE_WRITER_PATH"
	workspaceHelperModel    = "STARMAP_WORKSPACE_WRITER_MODEL"
	workspaceHelperReady    = "STARMAP_WORKSPACE_WRITER_READY"
	workspaceHelperRelease  = "STARMAP_WORKSPACE_WRITER_RELEASE"
	workspaceHelperConflict = "STARMAP_WORKSPACE_WRITER_EXPECT_CONFLICT"

	// workspaceHelperTimeout bounds each handshake step between a test and a
	// helper process. On a native Windows runner, one helper took more than
	// 10 s to become ready. Neighbor tests on that runner ran up to 6.1 times
	// slower than their 2.1 s to 2.5 s baseline.
	workspaceHelperTimeout = 30 * time.Second
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
	first := startWorkspaceHelper(t, workspaceWriterCommand(t, path, "first", ready, release, false))
	defer func() { _ = os.WriteFile(release, []byte("release\n"), constants.FilePermissions) }()
	first.waitReady(t, ready)

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
	if output, err := first.wait(); err != nil {
		t.Fatalf("first writer helper: %v\n%s", err, output)
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
	holder := startWorkspaceHelper(t, workspaceWriterCommand(t, path, "interrupted", ready, release, false))
	holder.waitReady(t, ready)
	if err := holder.command.Process.Kill(); err != nil {
		t.Fatalf("kill lock holder: %v", err)
	}
	if _, err := holder.wait(); err == nil {
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
			deadline := time.Now().Add(workspaceHelperTimeout)
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

// workspaceHelper is a started helper process. It keeps the exit status and
// the combined output, so a failed handshake reports what the helper did.
type workspaceHelper struct {
	command *exec.Cmd
	output  bytes.Buffer
	done    chan struct{}
	err     error
}

func startWorkspaceHelper(t testing.TB, command *exec.Cmd) *workspaceHelper {
	t.Helper()

	helper := &workspaceHelper{command: command, done: make(chan struct{})}
	command.Stdout = &helper.output
	command.Stderr = &helper.output
	if err := command.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	go func() {
		helper.err = command.Wait()
		close(helper.done)
	}()
	t.Cleanup(func() {
		_ = command.Process.Kill()
		<-helper.done
	})
	return helper
}

// waitReady waits until the helper creates path. It stops at once when the
// helper exits first, and it stops the helper after workspaceHelperTimeout.
func (h *workspaceHelper) waitReady(t testing.TB, path string) {
	t.Helper()

	timeout := time.NewTimer(workspaceHelperTimeout)
	defer timeout.Stop()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !stderrors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat helper file: %v", err)
		}
		select {
		case <-h.done:
			if _, err := os.Stat(path); err == nil {
				return
			}
			t.Fatalf("helper exited before it created %q: %v\n%s", path, h.err, h.output.Bytes())
		case <-timeout.C:
			_ = h.command.Process.Kill()
			output, err := h.wait()
			t.Fatalf("helper did not create %q within %s: %v\n%s", path, workspaceHelperTimeout, err, output)
		case <-poll.C:
		}
	}
}

// wait returns the combined output and the exit error after the helper exits.
func (h *workspaceHelper) wait() ([]byte, error) {
	<-h.done
	return h.output.Bytes(), h.err
}
