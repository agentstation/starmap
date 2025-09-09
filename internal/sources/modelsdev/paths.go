package modelsdev

import (
	"os"
	"path/filepath"
	"strings"
)

// expandPath expands a path that may contain ~ to the user's home directory.
func expandPath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}