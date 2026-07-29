package handlers

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type application interface {
	Catalog() (*catalogs.Catalog, error)
	CatalogState() (starmap.CatalogState, error)
	Readiness() (starmap.CatalogReadiness, error)
	Starmap(...starmap.Option) (*starmap.Client, error)
	Sync(context.Context, ...pkgsync.Option) (*pkgsync.Result, error)
}
