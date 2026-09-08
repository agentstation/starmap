package local

import (
	stderrors "errors"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestLocalSourceRefusesWorkspaceInputDuringWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	lock := flock.New(filepath.Join(filepath.Dir(path), ".workspace.starmap-write.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatal("writer lock", err)
	}
	defer func() { _ = lock.Close() }()
	observation, err := New(WithCatalogPath(path)).Observe(t.Context())
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) || observation.Catalog != nil {
		t.Fatalf("observation = %+v, %v", observation, err)
	}
}
