package remote

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// PermissionEnvelopePath returns the current authority head with a finite permission receipt.
	PermissionEnvelopePath = CatalogPath + "/permission"
	// PermissionEnvelopeMediaType identifies permission JSON independently of catalog formats.
	PermissionEnvelopeMediaType = "application/vnd.agentstation.starmap.catalog-permission+json"
)

// FetchPermissionEnvelope verifies the configured publisher and reads its bounded permission envelope.
// It reads no catalog manifest or payload and sends no conditional request.
// The caller checks configured authority identity, successor order, permission semantics, and receipt validity before admission.
func (c *Client) FetchPermissionEnvelope(ctx context.Context) (catalogs.CatalogPermissionEnvelope, error) {
	data, _, err := c.fetchConditional(ctx, PermissionEnvelopePath, PermissionEnvelopeMediaType, "", catalogs.MaxCatalogPermissionEnvelopeBytes)
	if err != nil {
		return catalogs.CatalogPermissionEnvelope{}, err
	}
	envelope, err := catalogs.ParseCatalogPermissionEnvelope(data)
	if err != nil {
		return catalogs.CatalogPermissionEnvelope{}, errors.WrapResource("parse", "remote permission envelope", "current", err)
	}
	return envelope, nil
}
