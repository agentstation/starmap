package starmap

import (
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
)

// Save persists the current catalog to disk using the catalog's native save functionality.
func (c *Client) Save(opts ...save.Option) error {
	options := save.Defaults().Apply(opts...)
	writePath := options.Path()
	if writePath == "" && c.options != nil {
		writePath = c.options.catalogExportPath
	}
	if c.options != nil {
		if err := validateCatalogPathSeparation(c.options.catalogStore, writePath); err != nil {
			return err
		}
	}

	// Get the catalog
	catalog, err := c.catalogCopy()
	if err != nil {
		return errors.WrapResource("get", "catalog", "", err)
	}

	// Check if the catalog supports saving (e.g., embedded catalog)
	if err := catalog.Save(opts...); err != nil {
		return errors.WrapIO("write", "catalog", err)
	}

	return nil
}
