package embedded

import (
	"embed"

	"github.com/agentstation/starmap/internal/catalogs/base"
	embeddedCatalog "github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// catalog represents the embedded catalog of providers, authors, models and endpoints.
type catalog struct {
	base.BaseCatalog
	FS embed.FS
}

// NewCatalog creates a new embedded catalog
func NewCatalog() catalogs.Catalog {
	return &catalog{
		BaseCatalog: base.NewBaseCatalog(),
		FS:          embeddedCatalog.FS,
	}
}

// All CRUD methods are inherited from BaseCatalog

// Copy copies the catalog
func (c *catalog) Copy() (catalogs.Catalog, error) {
	return c.BaseCatalog.CopyWith(NewCatalog)
}
