// Package catalog provides common catalog operations for CLI commands.
package catalog

import (
	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Load creates a starmap instance and returns its catalog.
// This handles the common pattern of starmap.New() -> sm.Catalog().
func Load() (catalogs.Catalog, error) {
	sm, err := starmap.New()
	if err != nil {
		return nil, errors.WrapResource("create", "starmap", "", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return nil, errors.WrapResource("get", "catalog", "", err)
	}

	return catalog, nil
}

// LoadWithOptions creates a starmap instance with custom options and returns its catalog.
// Useful for commands that need specific catalog configurations (like update command).
func LoadWithOptions(opts ...starmap.Option) (starmap.Starmap, catalogs.Catalog, error) {
	sm, err := starmap.New(opts...)
	if err != nil {
		return nil, nil, errors.WrapResource("create", "starmap", "with options", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return nil, nil, errors.WrapResource("get", "catalog", "", err)
	}

	return sm, catalog, nil
}

// LoadFromFiles creates a starmap instance from a file-based catalog.
// Useful for commands that work with external catalog directories.
func LoadFromFiles(inputPath string) (starmap.Starmap, catalogs.Catalog, error) {
	filesCatalog, err := catalogs.New(catalogs.WithFiles(inputPath))
	if err != nil {
		return nil, nil, errors.WrapResource("create", "catalog", inputPath, err)
	}

	sm, err := starmap.New(starmap.WithInitialCatalog(filesCatalog))
	if err != nil {
		return nil, nil, errors.WrapResource("create", "starmap", "files catalog", err)
	}

	catalog, err := sm.Catalog()
	if err != nil {
		return nil, nil, errors.WrapResource("get", "catalog", "", err)
	}

	return sm, catalog, nil
}
