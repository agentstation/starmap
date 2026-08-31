package workspace

import (
	stderrors "errors"
	"fmt"
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
// immutable catalog store. It is read-only and safe to call before
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

// ValidateMachineSeparation verifies that a machine-owned lifecycle root does
// not equal, contain, or sit beneath the human provider-YAML workspace.
func ValidateMachineSeparation(humanPath, machinePath, machineKind string) error {
	if strings.TrimSpace(humanPath) == "" || strings.TrimSpace(machinePath) == "" {
		return nil
	}
	resolvedHuman, err := resolveFilesystemPath(humanPath)
	if err != nil {
		return err
	}
	resolvedMachine, err := resolveFilesystemPath(machinePath)
	if err != nil {
		return err
	}
	if !pathsOverlap(resolvedHuman, resolvedMachine) {
		return nil
	}
	if strings.TrimSpace(machineKind) == "" {
		machineKind = "machine state"
	}
	return &errors.ConfigError{
		Component: "catalog filesystem layout",
		Message: fmt.Sprintf(
			"human catalog workspace %q overlaps machine-owned %s %q; configure separate roots",
			resolvedHuman,
			machineKind,
			resolvedMachine,
		),
	}
}

func resolveFilesystemPath(path string) (string, error) {
	expanded, err := expandHomePath(path)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(expanded)
	if err != nil {
		return "", errors.WrapIO("resolve", path, err)
	}
	absolute = filepath.Clean(absolute)
	existing := absolute
	var suffix []string
	for {
		if _, statErr := os.Lstat(existing); statErr == nil {
			break
		} else if !stderrors.Is(statErr, fs.ErrNotExist) {
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
	for index := len(suffix) - 1; index >= 0; index-- {
		resolved = filepath.Join(resolved, suffix[index])
	}
	return filepath.Clean(resolved), nil
}

func expandHomePath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.WrapResource("resolve", "home directory", path, err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}

func pathsOverlap(first, second string) bool {
	return pathContains(first, second) || pathContains(second, first)
}

func pathContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return relative == "." ||
		(relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
