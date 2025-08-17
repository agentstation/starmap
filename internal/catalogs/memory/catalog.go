package memory

import (
	"github.com/agentstation/starmap/internal/catalogs/base"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// catalog represents an in-memory catalog for testing and temporary operations
type catalog struct {
	base.BaseCatalog
	readOnly bool
}

// NewCatalog creates a new in-memory catalog
func NewCatalog() catalogs.Catalog {
	return &catalog{
		BaseCatalog: base.NewBaseCatalog(),
		readOnly:    false,
	}
}

// NewCatalogWithConfig creates a new in-memory catalog with configuration
func NewCatalogWithConfig(readOnly bool, preloadData []byte) catalogs.Catalog {
	cat := &catalog{
		BaseCatalog: base.NewBaseCatalog(),
		readOnly:    readOnly,
	}

	// Apply preloaded data if provided
	if len(preloadData) > 0 {
		if err := cat.loadPreloadedData(preloadData); err != nil {
			// Log error but don't fail creation
			// In a real implementation, you might want to handle this differently
		}
	}

	return cat
}

// Copy copies the catalog
func (c *catalog) Copy() (catalogs.Catalog, error) {
	return c.BaseCatalog.CopyWith(func() catalogs.Catalog {
		return NewCatalog()
	})
}

// Load is a no-op for memory catalogs (they start empty or with preloaded data)
func (c *catalog) Load() error {
	// Memory catalogs don't need to load from external sources
	return nil
}

// Save is a no-op for memory catalogs unless a specific writer is configured
func (c *catalog) Save() error {
	if c.readOnly {
		// Read-only catalogs cannot be saved
		return nil
	}
	// Memory catalogs don't persist by default
	return nil
}

// loadPreloadedData loads data from a byte slice
func (c *catalog) loadPreloadedData(data []byte) error {
	// This would parse the data format (e.g., JSON, YAML) and populate the catalog
	// For now, this is a placeholder - implementation would depend on the data format
	// The data could be in various formats like:
	// - JSON with providers, authors, models, endpoints
	// - YAML with catalog structure
	// - Binary format for efficient loading

	// TODO: Implement actual data parsing based on requirements
	return nil
}

// Additional methods specific to memory catalogs

// IsReadOnly returns whether the catalog is in read-only mode
func (c *catalog) IsReadOnly() bool {
	return c.readOnly
}

// SetReadOnly sets the read-only mode
func (c *catalog) SetReadOnly(readOnly bool) {
	c.readOnly = readOnly
}

// Clear removes all data from the catalog (useful for testing)
func (c *catalog) Clear() {
	c.Providers().Clear()
	c.Authors().Clear()
	c.Models().Clear()
	c.Endpoints().Clear()
}

// Size returns the total number of items in the catalog
func (c *catalog) Size() (providers, authors, models, endpoints int) {
	return c.Providers().Len(),
		c.Authors().Len(),
		c.Models().Len(),
		c.Endpoints().Len()
}
