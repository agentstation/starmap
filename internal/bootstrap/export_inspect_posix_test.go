//go:build linux || darwin

package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectBaselineExportsRefusesLinksAndPublicAccess(t *testing.T) {
	for _, mode := range []string{"root-link", "export-link", "payload-link", "public-directory", "public-file"} {
		t.Run(mode, func(t *testing.T) {
			root, path, _ := retainedBaselineFixture(t)
			selected := root
			switch mode {
			case "root-link", "export-link", "payload-link":
				original := root
				if mode == "export-link" {
					original = path
				}
				if mode == "payload-link" {
					original = filepath.Join(path, baselinePayloadName)
				}
				moved := original + "-original"
				if err := os.Rename(original, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, original); err != nil {
					t.Fatal(err)
				}
			case "public-directory":
				if err := os.Chmod(path, 0755); err != nil {
					t.Fatal(err)
				}
			case "public-file":
				if err := os.Chmod(filepath.Join(path, baselinePayloadName), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := InspectExports(t.Context(), selected); err == nil {
				t.Fatal("unsafe export accepted")
			}
		})
	}
}
