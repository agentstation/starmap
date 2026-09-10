package runtime

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ReadPermission returns the confirmed upstream receipt from memory without renewing it.
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
	if !r.requiresAuthority() {
		return catalogs.CatalogPermissionEnvelope{}, &errors.ConfigError{Component: "permission relay", Message: "requires an internal catalog authority"}
	}
	r.mu.RLock()
	p := r.permissions
	r.mu.RUnlock()
	uncertainty, known := r.permissionClock()
	if !p.retained || p.pending || p.activeReceipt.Head != p.highest || !p.activeReceipt.ValidAt(r.config.now(), uncertainty, known) {
		return catalogs.CatalogPermissionEnvelope{}, &errors.ConflictError{Resource: "catalog permission", Message: "no confirmed valid receipt matches the highest required publication"}
	}
	return p.activeReceipt, nil
}
