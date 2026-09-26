//go:build darwin || linux || windows

package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerInspectionSupportedPlatforms(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "absent")
	owner := DirectoryOwner{Product: "starport", Deployment: "local", Instance: "default"}
	status, err := InspectDirectoryOwnerRecord(t.Context(), directory, owner, "")
	if err != nil || status != OwnerRecordAbsent {
		t.Fatalf("supported platform inspection = %q, %v; want absent", status, err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("inspection created the absent directory")
	}
}
