package server

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// Application is the catalog and operational role consumed by the HTTP server.
type Application interface {
	Catalog() (*catalogs.Catalog, error)
	CatalogState() (starmap.CatalogState, error)
	Readiness() (starmap.CatalogReadiness, error)
	Starmap(...starmap.Option) (*starmap.Client, error)
	Sync(context.Context, ...pkgsync.Option) (*pkgsync.Result, error)
	UpdatesEnabled() bool
	Logger() *zerolog.Logger
}
