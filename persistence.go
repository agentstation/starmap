package starmap

import (
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
)

// Compile-time interface check to ensure proper implementation.
var _ Persistence = (*client)(nil)

// Persistence handles catalog persistence operations.
type Persistence interface {
	// Save with options
	Save(opts ...save.Option) error
}

// Save persists the current catalog to disk using the catalog's native save functionality.
func (c *client) Save(opts ...save.Option) error {

	// Get the catalog
	catalog, err := c.Catalog()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	// Check if the catalog supports saving (e.g., embedded catalog)
	if err := catalog.Save(opts...); err != nil {
		return errors.WrapIO("write", "catalog", err)
	}

	return nil
}
