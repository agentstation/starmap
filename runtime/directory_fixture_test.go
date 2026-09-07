package runtime

import (
	"path/filepath"
	"testing"
)

// privateRuntimeDirectory creates private paths for successful runtime fixtures.
// Tests for exposed permissions must set their rejected modes after fixture creation.
func privateRuntimeDirectory(t *testing.T) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "runtime")
	if err := createPrivateRuntimeDirectory(directory); err != nil {
		t.Fatal(err)
	}
	return directory
}
