package server

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

type permissionReader interface {
	ReadPermission(context.Context) (catalogs.CatalogPermissionEnvelope, error)
}

func (a *clientApplication) ReadPermission(ctx context.Context) (catalogs.CatalogPermissionEnvelope, error) {
	reader, ok := a.runtime.(permissionReader)
	if !ok {
		return catalogs.CatalogPermissionEnvelope{}, &errors.ConfigError{Component: "permission relay", Message: "connected runtime has no permission reader"}
	}
	return reader.ReadPermission(ctx)
}
