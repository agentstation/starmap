package handlers

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogscheduler"
)

type application interface {
	Catalog() (*catalogs.Catalog, error)
	CatalogState() (starmap.CatalogState, error)
	Readiness() (starmap.CatalogReadiness, error)
	OperationalState(context.Context) (catalogscheduler.OperationalState, error)
	Starmap(...starmap.Option) (*starmap.Client, error)
}
