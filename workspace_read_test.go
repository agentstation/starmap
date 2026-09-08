package starmap

import (
	stderrors "errors"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestConstructionAndRollbackRefuseWorkspaceInputDuringWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	lock := flock.New(filepath.Join(filepath.Dir(path), ".workspace.starmap-write.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatal("writer lock", err)
	}
	defer func() { _ = lock.Close() }()
	client, err := NewContext(t.Context(), WithCatalogPath(path))
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) || client != nil {
		t.Fatalf("construction = %v, %v", client, err)
	}
	input, err := observeBoundWorkspaceInput(t.Context(), path)
	if !stderrors.As(err, &conflict) || input != (workspace.InputExpectation{}) {
		t.Fatalf("rollback input = %+v, %v", input, err)
	}
}
