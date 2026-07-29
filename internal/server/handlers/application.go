package handlers

import (
	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

type application interface {
	Catalog() (*catalogs.Catalog, error)
	CatalogState() (starmap.CatalogState, error)
	Readiness() (starmap.CatalogReadiness, error)
	Starmap(...starmap.Option) (*starmap.Client, error)
}
