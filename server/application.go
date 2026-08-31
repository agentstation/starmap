// Package server provides the public Starmap HTTP server composition.
package server

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type clientApplication struct {
	client *starmap.Client
	logger *zerolog.Logger
	syncer Syncer
}

func (a *clientApplication) Catalog() (*catalogs.Catalog, error) {
	return a.client.Catalog(), nil
}

func (a *clientApplication) CatalogState() (starmap.CatalogState, error) {
	return a.client.CurrentCatalogState(), nil
}

func (a *clientApplication) Readiness() (starmap.CatalogReadiness, error) {
	return a.client.Readiness(), nil
}

func (a *clientApplication) Starmap(...starmap.Option) (*starmap.Client, error) {
	return a.client, nil
}

func (a *clientApplication) Sync(
	ctx context.Context,
	options ...pkgsync.Option,
) (*pkgsync.Result, error) {
	if a.syncer == nil {
		return nil, &errors.ConfigError{
			Component: "server acquisition",
			Message:   "a Syncer must be supplied with server.WithSyncer",
		}
	}
	return a.syncer.Sync(ctx, options...)
}

func (a *clientApplication) UpdatesEnabled() bool {
	return a.syncer != nil
}

func (a *clientApplication) Logger() *zerolog.Logger {
	return a.logger
}
