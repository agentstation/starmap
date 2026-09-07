package runtime

import (
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/errors"
)

func validateMigrationLocations(journal, source, target string) error {
	paths := []string{journal, source, target}
	for i, path := range paths {
		if !absoluteMigrationPath(path) {
			return invalidMigrationIntent("directories")
		}
		resolved, err := migrationPhysicalPath(path)
		if err != nil {
			return errors.WrapIO("resolve migration path", path, err)
		}
		paths[i] = resolved
	}
	for i, path := range paths {
		for _, other := range paths[i+1:] {
			if migrationPathsOverlap(path, other) {
				return invalidMigrationIntent("overlapping_directories")
			}
		}
	}
	return nil
}

// migrationPhysicalPath resolves existing parents before it compares selected roots.
func migrationPhysicalPath(path string) (string, error) {
	current := path
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}
