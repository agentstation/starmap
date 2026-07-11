package starmap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

type filesystemCatalogStore interface {
	Root() string
}

func validateCatalogPathSeparation(store any, exportPath string) error {
	filesystemStore, ok := store.(filesystemCatalogStore)
	if !ok || strings.TrimSpace(exportPath) == "" {
		return nil
	}
	databasePath, err := resolvedFilesystemPath(filesystemStore.Root())
	if err != nil {
		return err
	}
	resolvedExportPath, err := resolvedFilesystemPath(exportPath)
	if err != nil {
		return err
	}
	if pathsContainEachOther(databasePath, resolvedExportPath) {
		return &errors.ConfigError{
			Component: "catalog filesystem layout",
			Message: fmt.Sprintf(
				"editable catalog export %q overlaps durable catalog database %q; configure separate catalog_export_path and catalog_path roots",
				resolvedExportPath,
				databasePath,
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
