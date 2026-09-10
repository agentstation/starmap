package runtime

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ReadPermission issues an origin receipt or returns a confirmed upstream receipt without renewing it.
// Origins read current authoritative storage with a qualified clock. Relays read retained receipts from memory.
// It forwards a required revision even when this runtime cannot activate its catalog or permission schema.
// A newer manifest without a matching receipt, uncertain retention, or unknown clock validity prevents delivery.
// Callers must authenticate the downstream connection and enforce the receipt's original expiry.
func (r *Runtime) ReadPermission(ctx context.Context) (catalogs.CatalogPermissionEnvelope, error) {
	if r == nil || ctx == nil {
		return catalogs.CatalogPermissionEnvelope{}, &errors.ValidationError{Field: "permission_relay", Message: "requires a runtime and context"}
	}
	if err := ctx.Err(); err != nil {
		return catalogs.CatalogPermissionEnvelope{}, err
	}
	if err := r.ctx.Err(); err != nil {
		return catalogs.CatalogPermissionEnvelope{}, err
	}
	if r.config.origin != nil {
		return r.config.origin.issuer.ReadPermission(ctx)
	}
	if !r.requiresAuthority() {
		return catalogs.CatalogPermissionEnvelope{}, &errors.ConfigError{Component: "permission relay", Message: "requires an internal catalog authority"}
	}
	r.mu.RLock()
	p := r.permissions
	r.mu.RUnlock()
	clock := r.readPermissionClock()
	if !p.retained || p.pending || p.activeReceipt.Head != p.highest || !p.activeReceipt.ValidAt(clock.Time, clock.Uncertainty, clock.Known) {
		return catalogs.CatalogPermissionEnvelope{}, &errors.ConflictError{Resource: "catalog permission", Message: "no confirmed valid receipt matches the highest required publication"}
	}
	return p.activeReceipt, nil
}
