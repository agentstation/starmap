package starmap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/errors"
)

type filesystemCatalogStore interface {
	Root() string
}

func validateCatalogLayout(store any, catalogPath string) error {
	if err := validateCatalogPathSeparation(store, catalogPath); err != nil {
		return err
	}
	migrationTarget := ""
	if filesystemStore, ok := store.(filesystemCatalogStore); ok {
		migrationTarget = filesystemStore.Root()
	}
	return workspace.ValidateHumanLayout(catalogPath, migrationTarget)
}

func validateCatalogPathSeparation(store any, catalogPath string) error {
	filesystemStore, ok := store.(filesystemCatalogStore)
	if !ok || strings.TrimSpace(catalogPath) == "" {
		return nil
	}
	statePath, err := resolvedFilesystemPath(filesystemStore.Root())
	if err != nil {
		return err
	}
	resolvedCatalogPath, err := resolvedFilesystemPath(catalogPath)
	if err != nil {
		return err
	}
	if pathsContainEachOther(statePath, resolvedCatalogPath) {
		return &errors.ConfigError{
			Component: "catalog filesystem layout",
			Message: fmt.Sprintf(
				"human catalog workspace %q overlaps machine-owned catalog state %q; configure separate workspace and catalog-store roots",
				resolvedCatalogPath,
				statePath,
			),
		}
	}
	return nil
}

func resolvedFilesystemPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", errors.WrapIO("resolve", path, err)
	}
	absolute = filepath.Clean(absolute)
	existing := absolute
	var suffix []string
	for {
		if _, statErr := os.Lstat(existing); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return "", errors.WrapIO("inspect", existing, statErr)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		suffix = append(suffix, filepath.Base(existing))
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", errors.WrapIO("resolve", existing, err)
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, suffix[i])
	}
	return filepath.Clean(resolved), nil
}

func pathsContainEachOther(first, second string) bool {
	return pathContains(first, second) || pathContains(second, first)
}

func pathContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
