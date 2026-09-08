package main

import (
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestManifestRefusesWorkspaceInputDuringWriter(t *testing.T) {
	root := t.TempDir()
	path, output := filepath.Join(root, "workspace"), filepath.Join(root, "manifest.json")
	lock := flock.New(filepath.Join(root, ".workspace.starmap-write.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatal("writer lock", err)
	}
	defer func() { _ = lock.Close() }()
	err = run([]string{"-catalog-dir", path, "-output", output}, io.Discard, time.Now())
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatal("output exists after rejected workspace read", err)
	}
}
