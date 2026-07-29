package server

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogscheduler"
)

// Application is the catalog and operational role consumed by the HTTP server.
type Application interface {
	Catalog() (*catalogs.Catalog, error)
	CatalogState() (starmap.CatalogState, error)
	Readiness() (starmap.CatalogReadiness, error)
	OperationalState(context.Context) (catalogscheduler.OperationalState, error)
	Starmap(...starmap.Option) (*starmap.Client, error)
	Logger() *zerolog.Logger
}
