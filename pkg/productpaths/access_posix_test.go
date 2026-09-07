//go:build darwin || linux

package productpaths

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectionReportsPrivateAccessConflictsWithoutMutation(t *testing.T) {
	for _, test := range []struct {
		name   string
		mode   os.FileMode
		status string
	}{
		{"private", 0o600, "unverified"},
		{"group_read", 0o640, "conflict"},
		{"other_write", 0o602, "conflict"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			contents := []byte("private configuration bytes")
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, test.mode); err != nil {
				t.Fatal(err)
			}
			manifest := FileManifest{Files: []FileEntry{{ID: "configuration", Location: Path{Path: path}, Availability: "available", Policy: FilePolicy{Access: "owner-only"}}}}
			report, err := InspectManifest(t.Context(), manifest, 10)
			if err != nil || len(report.Observations) != 1 {
				t.Fatalf("inspect: %+v, %v", report, err)
			}
			item := report.Observations[0]
			if item.AccessPolicy != "owner-only" || item.AccessStatus != test.status {
				t.Fatalf("unexpected access assessment: %+v", item)
			}
			if test.status == "conflict" && item.AccessReason != "group-or-other-mode-bits" {
				t.Fatalf("missing mode conflict reason: %+v", item)
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(contents, actual) {
				t.Fatal("inspection changed file contents", err)
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != test.mode {
				t.Fatal("inspection changed file permissions", err)
			}
		})
	}
}
