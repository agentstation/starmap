package provenance

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestLoadMissingFileIsOptional(t *testing.T) {
	t.Parallel()

	file, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if file != nil {
		t.Fatalf("Load returned %#v, want nil", file)
	}
}

func TestLoadReturnsTypedFileErrors(t *testing.T) {
	t.Parallel()

	t.Run("read", func(t *testing.T) {
		var target *errors.IOError
		_, err := Load(t.TempDir())
		if !stderrors.As(err, &target) {
			t.Fatalf("Load error = %T %v, want *errors.IOError", err, err)
		}
	})

	t.Run("parse", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "provenance.yaml")
		if err := os.WriteFile(path, []byte("provenance: [\n"), constants.FilePermissions); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		var target *errors.ParseError
		_, err := Load(path)
		if !stderrors.As(err, &target) {
			t.Fatalf("Load error = %T %v, want *errors.ParseError", err, err)
		}
	})
}
