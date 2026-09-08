package pipeline

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestPipelineKeepsWorkspaceReadLockThroughLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	if err := os.Mkdir(path, constants.DirPermissions); err != nil {
		t.Fatal(err)
	}
	lock := flock.New(filepath.Join(filepath.Dir(path), ".workspace.starmap-write.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatal("initial writer lock", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	p := New(nil)
	p.loadWorkspace = func(path string) (*catalogs.Builder, error) {
		locked, err := lock.TryLock()
		if err != nil || locked {
			t.Fatalf("writer during workspace load: locked=%v, error=%v", locked, err)
		}
		return loadHumanWorkspace(path)
	}
	p.loadEmbedded = func() (*catalogs.Builder, error) { return catalogs.NewEmpty(), nil }
	inputs, err := p.loadCatalogInputs(t.Context(), path)
	if err != nil || inputs.workspace == nil || !inputs.workspaceInput.Exists || inputs.workspaceInput.Checksum == "" {
		t.Fatalf("workspace input: %+v, %v", inputs, err)
	}
	locked, err = lock.TryLock()
	if err != nil || !locked {
		t.Fatalf("writer after workspace load: locked=%v, error=%v", locked, err)
	}
}

func TestPipelineRefusesWorkspaceInputDuringWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	lock := flock.New(filepath.Join(filepath.Dir(path), ".workspace.starmap-write.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatal("writer lock", err)
	}
	defer func() { _ = lock.Close() }()
	p := New(nil)
	p.loadWorkspace = func(string) (*catalogs.Builder, error) {
		t.Fatal("workspace loaded during replacement")
		return nil, nil
	}
	input, err := p.loadCatalogInputs(t.Context(), path)
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) || input.workspace != nil || input.providerConfig != nil {
		t.Fatalf("input = %+v, %v", input, err)
	}
}
