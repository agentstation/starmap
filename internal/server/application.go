package server

import (
	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Application is the catalog and operational role consumed by the HTTP server.
type Application interface {
	Catalog() (*catalogs.Catalog, error)
	CatalogState() (starmap.CatalogState, error)
	Readiness() (starmap.CatalogReadiness, error)
	Starmap(...starmap.Option) (*starmap.Client, error)
	Logger() *zerolog.Logger
}
