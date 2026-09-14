package workspace

import (
	"bytes"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestAssemblyRejectsReplacementFileOwnership(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workspace")
	catalog, identity := testCatalog(t, "new", "New")
	var render, preserved string
	var want []byte
	_, err := (projector{
		afterStageRender: func(path string) error { render = path; return nil },
		beforeAccessRestore: func(name string) error {
			if name != "providers.yaml" {
				return nil
			}
			preserved = filepath.Join(filepath.Dir(render), "tree", name)
			var err error
			want, err = os.ReadFile(preserved)
			if err != nil {
				return err
			}
			if err := os.Rename(preserved, filepath.Join(root, "original-providers.yaml")); err != nil {
				return err
			}
			return os.WriteFile(preserved, want, fileMode)
		},
	}).project(t.Context(), target, catalog, identity, InputExpectation{})
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("assembly accepted a replacement file as its own: %v", err)
	}
	data, readErr := os.ReadFile(preserved)
	if readErr != nil || !bytes.Equal(data, want) {
		t.Fatalf("assembly did not preserve the replacement file: %q, %v", data, readErr)
	}
	if _, err := os.Lstat(target); !stderrors.Is(err, os.ErrNotExist) {
		t.Fatalf("assembly published an unowned file: %v", err)
	}
}
