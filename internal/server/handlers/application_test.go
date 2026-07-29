package handlers

import (
	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

type testApplication struct {
	CatalogFunc      func() (*catalogs.Catalog, error)
	CatalogStateFunc func() (starmap.CatalogState, error)
	ReadinessFunc    func() (starmap.CatalogReadiness, error)
	StarmapFunc      func(...starmap.Option) (*starmap.Client, error)
}

func (a *testApplication) Catalog() (*catalogs.Catalog, error) {
	if a.CatalogFunc != nil {
		return a.CatalogFunc()
	}
	return nil, nil
}

func (a *testApplication) CatalogState() (starmap.CatalogState, error) {
	if a.CatalogStateFunc != nil {
		return a.CatalogStateFunc()
	}
	catalog, err := a.Catalog()
	return starmap.CatalogState{Catalog: catalog}, err
}

func (a *testApplication) Readiness() (starmap.CatalogReadiness, error) {
	if a.ReadinessFunc != nil {
		return a.ReadinessFunc()
	}
	return starmap.CatalogReadiness{Ready: true}, nil
}

func (a *testApplication) Starmap(
	opts ...starmap.Option,
) (*starmap.Client, error) {
	if a.StarmapFunc != nil {
		return a.StarmapFunc(opts...)
	}
	return nil, nil
}
