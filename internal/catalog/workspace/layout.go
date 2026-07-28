package workspace

import (
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

var legacyGenerationEntries = [...]string{
	".commit.lock",
	"current",
	"generations",
}

// ValidateHumanLayout verifies that path is absent or is not a pre-plan
// immutable generation store. It is read-only and safe to call before
// construction, repair, or publication.
func ValidateHumanLayout(path, migrationTarget string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return errors.WrapIO("resolve", path, err)
	}
	info, err := os.Lstat(target)
	if stderrors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.WrapIO("inspect", target, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return &errors.ValidationError{
			Field:   "catalog_path",
			Value:   target,
			Message: "human catalog workspace cannot be a symbolic link",
		}
	}
	if !info.IsDir() {
		return &errors.ValidationError{
			Field:   "catalog_path",
			Value:   target,
			Message: "human catalog workspace must be a directory",
		}
	}

	entries := make([]string, 0, len(legacyGenerationEntries))
	for _, name := range legacyGenerationEntries {
		if _, statErr := os.Lstat(filepath.Join(target, name)); statErr == nil {
			entries = append(entries, name)
		} else if !stderrors.Is(statErr, fs.ErrNotExist) {
			return errors.WrapIO("inspect", filepath.Join(target, name), statErr)
		}
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Strings(entries)
	return &errors.LegacyCatalogLayoutError{
		Path:            target,
		MigrationTarget: migrationTarget,
		Entries:         entries,
	}
}
