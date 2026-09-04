package handlers

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

type testApplication struct {
	CatalogFunc      func() (*catalogs.Catalog, error)
	CatalogStateFunc func() (starmap.CatalogState, error)
	ReadinessFunc    func() (starmap.CatalogReadiness, error)
	RuntimeFunc      func() runtime.Status
	StarmapFunc      func(...starmap.Option) (*starmap.Client, error)
	SyncFunc         func(context.Context, ...pkgsync.Option) (*pkgsync.Result, error)
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

func (a *testApplication) RuntimeStatus() runtime.Status {
	if a.RuntimeFunc != nil {
		return a.RuntimeFunc()
	}
	return runtime.Status{Usable: true}
}

func (a *testApplication) Starmap(
	opts ...starmap.Option,
) (*starmap.Client, error) {
	if a.StarmapFunc != nil {
		return a.StarmapFunc(opts...)
	}
	return nil, nil
}

func (a *testApplication) Sync(
	ctx context.Context,
	options ...pkgsync.Option,
) (*pkgsync.Result, error) {
	if a.SyncFunc != nil {
		return a.SyncFunc(ctx, options...)
	}
	return &pkgsync.Result{}, nil
}
