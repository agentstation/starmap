package modelsdev

import (
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
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

func sourceDirectory(explicit string, checkout bool) (string, error) {
	role := "source-http"
	if checkout {
		role = "source-checkout"
	}
	if err := policy.Require(role, policy.DeploymentControlled); err != nil {
		return "", err
	}
	if explicit != "" {
		return expandPath(explicit), nil
	}
	directories, err := productpaths.DefaultSourceDirectories(productpaths.Starmap)
	if err != nil {
		return "", err
	}
	if checkout {
		return directories.Checkouts, nil
	}
	return directories.Cache, nil
}
