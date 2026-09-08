package productpaths

import (
	"path/filepath"

	"github.com/agentstation/starmap/pkg/errors"
)

// SourceDirectories names parent directories for HTTP caches and managed Git checkouts.
// Each source appends its own name below these directories.
type SourceDirectories struct {
	Cache     string `json:"cache"`
	Checkouts string `json:"checkouts"`
}

// SourceDirectoriesAt places source storage beneath one absolute cache root.
// It does not access the filesystem.
func SourceDirectoriesAt(cacheRoot string) (SourceDirectories, error) {
	directories := SourceDirectories{Cache: cacheRoot, Checkouts: filepath.Join(cacheRoot, "sources")}
	if err := directories.Validate(); err != nil {
		return SourceDirectories{}, err
	}
	return directories, nil
}

// DefaultSourceDirectories resolves the selected product's native source directories.
// It reads platform directory inputs and creates no files.
func DefaultSourceDirectories(product Product) (SourceDirectories, error) {
	cache, err := UserDefaults(product)(Cache)
	if err != nil {
		return SourceDirectories{}, err
	}
	return SourceDirectoriesAt(cache)
}

// Validate requires absolute paths for both source directory roles.
func (directories SourceDirectories) Validate() error {
	for name, value := range map[string]string{"source_cache": directories.Cache, "source_checkouts": directories.Checkouts} {
		if !filepath.IsAbs(value) || filepath.Clean(value) != value {
			return &errors.ValidationError{Field: name, Message: "must be a clean absolute directory"}
		}
	}
	return nil
}
