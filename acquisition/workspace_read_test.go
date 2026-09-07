package acquisition

import (
	stderrors "errors"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestReleaseImportRefusesWorkspaceInputDuringWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	lock := flock.New(filepath.Join(filepath.Dir(path), ".workspace.starmap-write.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatal("writer lock", err)
	}
	defer func() { _ = lock.Close() }()
	input, observation, err := observeImportWorkspace(t.Context(), path)
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) || observation != nil || input != (workspace.InputExpectation{}) {
		t.Fatalf("input = %+v, observation = %+v, error = %v", input, observation, err)
	}
}
