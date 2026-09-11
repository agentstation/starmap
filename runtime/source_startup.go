package runtime

import (
	"context"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// prepareSourceStartup applies startup policy before scheduled work starts.
// require_source reads once under the Open context when this runtime owns the lease.
// Authority observation binds after other startup checks, before any source worker can read a manifest.
func (r *Runtime) prepareSourceStartup(ctx context.Context) error {
	if r.config.source.StartupPolicy == StartupRequireSource {
		if !r.automaticSourceReads() {
			r.mu.RLock()
			retained := r.layers.source != nil
			r.mu.RUnlock()
			if !retained {
				return &errors.ConfigError{Component: "catalog startup policy", Message: "require_source needs retained source state when automatic source reads are disabled"}
			}
		} else if r.lease.status() == leaseLost {
			logging.Info().
				Str("holder", r.schedule.identity.Instance).
				Msg("The lease owner supplies the source state; require_source reads nothing here")
		} else if _, err := r.RefreshSource(ctx); err != nil {
			return errors.WrapResource("read", "catalog source", r.source.Identity(), err)
		}
	}
	return r.bindSourceAuthority()
}
