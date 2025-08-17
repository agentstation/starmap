// Package files provides a file-based catalog implementation.
package files

import (
	"github.com/agentstation/starmap/internal/catalogs/base"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// catalog represents a file-based catalog that reads from a directory structure.
type catalog struct {
	base.BaseCatalog
	basePath string
}

// NewCatalog creates a new file-based catalog
func NewCatalog(basePath string) catalogs.Catalog {
	return &catalog{
		BaseCatalog: base.NewBaseCatalog(),
		basePath:    basePath,
	}
}

// All CRUD methods are inherited from BaseCatalog

// Copy copies the catalog
func (c *catalog) Copy() (catalogs.Catalog, error) {
	return c.BaseCatalog.CopyWith(func() catalogs.Catalog {
		return NewCatalog(c.basePath)
	})
}

// Load loads the catalog from YAML files in the directory structure
func (c *catalog) Load() error {
	reader := &base.FilesystemFileReader{BasePath: c.basePath}
	loader := base.NewLoader(reader, &c.BaseCatalog)
	return loader.Load()
}

// Save saves the catalog back to the file system
func (c *catalog) Save() error {
	// For now, this is a no-op since we're using this primarily for reading
	// If needed, this could be implemented to write back to the directory structure
	return nil
}
